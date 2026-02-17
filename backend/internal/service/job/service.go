// Package job 实现Job业务逻辑服务
package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/strmsync/strmsync/internal/database"
	"github.com/strmsync/strmsync/internal/service"
	"github.com/strmsync/strmsync/internal/service/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Service Job服务实现
type Service struct {
	db              *gorm.DB
	taskRunService  service.TaskRunService
	taskExecutor    service.TaskExecutor
	runningJobs     map[types.JobID]*types.ExecutionContext
	runningJobsLock sync.RWMutex
}

// NewService 创建Job服务
func NewService(db *gorm.DB, taskRunService service.TaskRunService, taskExecutor service.TaskExecutor) service.JobService {
	return &Service{
		db:             db,
		taskRunService: taskRunService,
		taskExecutor:   taskExecutor,
		runningJobs:    make(map[types.JobID]*types.ExecutionContext),
	}
}

// Run 运行任务
func (s *Service) Run(ctx context.Context, jobID types.JobID) (types.TaskRunID, error) {
	// 1. 创建执行context（在锁外创建，整个Run过程共享同一个cancel）
	execCtx, cancelFunc := context.WithCancel(ctx)

	// 2. 原子性检查并注册运行状态
	s.runningJobsLock.Lock()
	if _, running := s.runningJobs[jobID]; running {
		s.runningJobsLock.Unlock()
		cancelFunc() // 清理context
		return 0, fmt.Errorf("job: run: job %d is already running", jobID)
	}
	// 注册placeholder（TaskRunID=0表示初始化中，使用统一的cancelFunc）
	placeholder := &types.ExecutionContext{
		JobID:      jobID,
		TaskRunID:  0, // 0表示占位状态
		CancelFunc: cancelFunc,
	}
	s.runningJobs[jobID] = placeholder
	s.runningJobsLock.Unlock()

	// defer清理：如果中途失败，确保从map中移除
	defer func() {
		if r := recover(); r != nil {
			s.runningJobsLock.Lock()
			delete(s.runningJobs, jobID)
			s.runningJobsLock.Unlock()
			cancelFunc()
			panic(r) // re-panic
		}
	}()

	// 3. 加载并验证Job配置（可能被Stop取消）
	if execCtx.Err() != nil {
		s.runningJobsLock.Lock()
		delete(s.runningJobs, jobID)
		s.runningJobsLock.Unlock()
		return 0, fmt.Errorf("job: run: cancelled before load config")
	}
	jobConfig, err := s.loadAndValidateJob(execCtx, jobID)
	if err != nil {
		s.runningJobsLock.Lock()
		delete(s.runningJobs, jobID)
		s.runningJobsLock.Unlock()
		cancelFunc()
		return 0, fmt.Errorf("job: run: %w", err)
	}

	// 4. 创建TaskRun（可能被Stop取消）
	if execCtx.Err() != nil {
		s.runningJobsLock.Lock()
		delete(s.runningJobs, jobID)
		s.runningJobsLock.Unlock()
		return 0, fmt.Errorf("job: run: cancelled before create task_run")
	}
	taskRunID, err := s.taskRunService.Start(execCtx, jobID)
	if err != nil {
		s.runningJobsLock.Lock()
		delete(s.runningJobs, jobID)
		s.runningJobsLock.Unlock()
		cancelFunc()
		return 0, fmt.Errorf("job: run: create task_run: %w", err)
	}

	// 5. 更新Job状态为running（可能被Stop取消）
	if execCtx.Err() != nil {
		// 已创建TaskRun但被取消，标记为cancelled
		_ = s.taskRunService.Cancel(execCtx, taskRunID)
		s.runningJobsLock.Lock()
		delete(s.runningJobs, jobID)
		s.runningJobsLock.Unlock()
		return 0, fmt.Errorf("job: run: cancelled before update status")
	}
	if err := s.updateJobStatus(execCtx, jobID, "running"); err != nil {
		// TaskRun已创建但状态更新失败，标记为失败
		_ = s.taskRunService.Fail(execCtx, taskRunID, err)
		s.runningJobsLock.Lock()
		delete(s.runningJobs, jobID)
		s.runningJobsLock.Unlock()
		cancelFunc()
		return 0, fmt.Errorf("job: run: update job status: %w", err)
	}

	// 6. 更新为真实ExecutionContext（保持同一个cancelFunc）
	executionContext := &types.ExecutionContext{
		JobID:      jobID,
		TaskRunID:  taskRunID,
		JobConfig:  jobConfig,
		CancelFunc: cancelFunc,
	}
	s.runningJobsLock.Lock()
	s.runningJobs[jobID] = executionContext
	s.runningJobsLock.Unlock()

	// 7. 异步执行任务
	go func() {
		defer func() {
			// 清理running jobs
			s.runningJobsLock.Lock()
			delete(s.runningJobs, jobID)
			s.runningJobsLock.Unlock()
		}()

		// 执行任务
		summary, err := s.taskExecutor.Execute(execCtx, executionContext)

		// 检查是否被取消
		if err != nil {
			if execCtx.Err() == context.Canceled {
				// 任务被主动取消，不标记为failed（已在Stop中标记为cancelled）
				return
			}
			// 其他错误，标记为failed
			_ = s.taskRunService.Fail(execCtx, taskRunID, err)
			_ = s.updateJobStatus(execCtx, jobID, "error")
		} else {
			_ = s.taskRunService.Complete(execCtx, taskRunID, summary)
		}
	}()

	return taskRunID, nil
}

// Stop 停止任务
func (s *Service) Stop(ctx context.Context, jobID types.JobID) error {
	// 1. 获取ExecutionContext（快速获取后释放锁）
	s.runningJobsLock.RLock()
	execCtx, running := s.runningJobs[jobID]
	s.runningJobsLock.RUnlock()

	if !running {
		return fmt.Errorf("job: stop: job %d is not running", jobID)
	}

	// 2. 调用cancel函数（不持锁）
	// 无论TaskRunID是0（初始化中）还是非0（执行中），都会取消
	execCtx.CancelFunc()

	// 3. 标记TaskRun为cancelled（仅当TaskRunID非0时）
	if execCtx.TaskRunID != 0 {
		if err := s.taskRunService.Cancel(ctx, execCtx.TaskRunID); err != nil {
			return fmt.Errorf("job: stop: cancel task_run: %w", err)
		}
	}
	// TaskRunID=0表示还在初始化，context已取消，Run会在检查点自行清理

	return nil
}

// Validate 验证Job配置
func (s *Service) Validate(ctx context.Context, jobID types.JobID) error {
	_, err := s.loadAndValidateJob(ctx, jobID)
	return err
}

// GetRunningTaskRun 获取正在运行的TaskRun ID
// 注意：TaskRunID=0 表示Job正在初始化（已占位但TaskRun尚未创建），此时返回 (0, false, nil)
func (s *Service) GetRunningTaskRun(ctx context.Context, jobID types.JobID) (types.TaskRunID, bool, error) {
	s.runningJobsLock.RLock()
	defer s.runningJobsLock.RUnlock()

	execCtx, running := s.runningJobs[jobID]
	if !running {
		return 0, false, nil
	}

	// TaskRunID=0 表示还在初始化中，TaskRun尚未创建，对调用方来说等同于未运行
	if execCtx.TaskRunID == 0 {
		return 0, false, nil
	}

	return execCtx.TaskRunID, true, nil
}

// loadAndValidateJob 加载并验证Job配置
func (s *Service) loadAndValidateJob(ctx context.Context, jobID types.JobID) (*types.JobConfig, error) {
	// 1. 加载Job（使用行锁）
	var job database.Job
	if err := s.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&job, jobID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("job not found: id=%d", jobID)
		}
		return nil, fmt.Errorf("load job: %w", err)
	}

	// 2. 检查Job是否启用
	if !job.Enabled {
		return nil, fmt.Errorf("job is disabled: id=%d", jobID)
	}

	// 3. 解析Options
	var options map[string]interface{}
	if job.Options != "" {
		if err := json.Unmarshal([]byte(job.Options), &options); err != nil {
			return nil, fmt.Errorf("parse options: %w", err)
		}
	}

	// 4. 构建JobConfig
	config := &types.JobConfig{
		ID:            job.ID,
		Name:          job.Name,
		WatchMode:     types.WatchMode(job.WatchMode),
		SourcePath:    job.SourcePath,
		TargetPath:    job.TargetPath,
		Enabled:       job.Enabled,
		Recursive:     getBoolOption(options, "recursive", true),
		Extensions:    getStringSliceOption(options, "extensions", []string{".mkv", ".mp4", ".avi"}),
		Interval:      getIntOption(options, "interval", 300),
		AutoScanLibrary: getBoolOption(options, "auto_scan_library", false),
	}

	// 5. 验证WatchMode
	if !config.WatchMode.IsValid() {
		return nil, fmt.Errorf("invalid watch_mode: %s", config.WatchMode)
	}

	// 6. 根据WatchMode加载关联的Server
	switch config.WatchMode {
	case types.WatchModeAPI:
		// API模式必须有data_server_id
		if job.DataServerID == nil {
			return nil, fmt.Errorf("watch_mode=api requires data_server_id")
		}
		config.DataServerID = *job.DataServerID

		// 验证DataServer存在
		var dataServer database.DataServer
		if err := s.db.WithContext(ctx).First(&dataServer, config.DataServerID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("data_server not found: id=%d", config.DataServerID)
			}
			return nil, fmt.Errorf("load data_server: %w", err)
		}

	case types.WatchModeLocal:
		// Local模式不需要data_server_id
		config.DataServerID = 0
	}

	// 7. 加载MediaServer（如果有）
	if job.MediaServerID != nil {
		config.MediaServerID = *job.MediaServerID

		var mediaServer database.MediaServer
		if err := s.db.WithContext(ctx).First(&mediaServer, config.MediaServerID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("media_server not found: id=%d", config.MediaServerID)
			}
			return nil, fmt.Errorf("load media_server: %w", err)
		}
	}

	// 8. 验证路径
	if config.SourcePath == "" {
		return nil, fmt.Errorf("source_path is required")
	}
	if config.TargetPath == "" {
		return nil, fmt.Errorf("target_path is required")
	}

	return config, nil
}

// updateJobStatus 更新Job状态
func (s *Service) updateJobStatus(ctx context.Context, jobID types.JobID, status string) error {
	if err := s.db.WithContext(ctx).
		Model(&database.Job{}).
		Where("id = ?", jobID).
		Update("status", status).Error; err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	return nil
}

// 辅助函数：从options中获取值

func getBoolOption(options map[string]interface{}, key string, defaultValue bool) bool {
	if options == nil {
		return defaultValue
	}
	if v, ok := options[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultValue
}

func getIntOption(options map[string]interface{}, key string, defaultValue int) int {
	if options == nil {
		return defaultValue
	}
	if v, ok := options[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		}
	}
	return defaultValue
}

func getStringSliceOption(options map[string]interface{}, key string, defaultValue []string) []string {
	if options == nil {
		return defaultValue
	}
	if v, ok := options[key]; ok {
		if slice, ok := v.([]interface{}); ok {
			result := make([]string, 0, len(slice))
			for _, item := range slice {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return defaultValue
}
