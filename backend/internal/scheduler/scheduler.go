// Package scheduler 提供基于 Cron 的任务调度器实现
package scheduler

import (
	"github.com/strmsync/strmsync/internal/domain/model"
	"github.com/strmsync/strmsync/internal/pkg/logger"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/strmsync/strmsync/internal/queue"
	"go.uber.org/zap"
)

const (
	// defaultEnqueueTimeout 默认入队超时时间
	defaultEnqueueTimeout = 10 * time.Second
)

// CronScheduler 是 Scheduler 接口的生产级实现
//
// 设计要点：
// - 使用 robfig/cron/v3 解析并调度 Cron 表达式
// - 维护 JobID -> CronEntry 的映射，支持动态更新
// - 通过 RWMutex 保证并发安全
// - 在 Cron 回调中自动入队任务（SyncQueue.Enqueue）
//
// 线程安全性：
// - 所有映射操作都通过 mu 保护
// - 使用 atomic.Bool 管理运行状态
//
// 错误处理：
// - Cron 表达式解析失败时记录错误但不阻塞其他任务
// - 入队失败时记录错误但不影响调度器运行
type CronScheduler struct {
	cfg SchedulerConfig

	mu      sync.RWMutex        // 保护 cron 和 entries
	cron    *cron.Cron          // Cron 引擎
	entries map[uint]CronEntry  // JobID -> CronEntry 映射

	running atomic.Bool // 运行状态标志
}

// NewScheduler 创建一个新的 CronScheduler
//
// 参数：
//   - cfg: 调度器配置（Queue 和 Jobs 必填）
//
// 返回：
//   - *CronScheduler: 调度器实例
//   - error: 配置无效时返回错误
func NewScheduler(cfg SchedulerConfig) (*CronScheduler, error) {
	if cfg.Queue == nil {
		return nil, fmt.Errorf("scheduler: queue is nil")
	}
	if cfg.Jobs == nil {
		return nil, fmt.Errorf("scheduler: job repository is nil")
	}

	// 设置默认值
	if cfg.Logger == nil {
		cfg.Logger = logger.With(zap.String("component", "scheduler"))
	}
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.EnqueueTimeout <= 0 {
		cfg.EnqueueTimeout = defaultEnqueueTimeout
	}

	return &CronScheduler{
		cfg:     cfg,
		entries: make(map[uint]CronEntry),
	}, nil
}

// Start 启动调度器并加载所有启用的任务
//
// 启动流程：
// 1. 初始化 Cron 引擎（附带 Location/Options）
// 2. 从 JobRepository 加载启用任务
// 3. 为每个任务注册 Cron 回调
// 4. 启动 Cron 引擎
//
// 并发安全：使用 atomic.Bool 保证只启动一次
func (s *CronScheduler) Start(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("scheduler: nil receiver")
	}
	if !s.running.CompareAndSwap(false, true) {
		return fmt.Errorf("scheduler: already running")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	// 初始化 Cron 引擎
	opts := append([]cron.Option{cron.WithLocation(s.cfg.Location)}, s.cfg.CronOptions...)
	s.mu.Lock()
	s.cron = cron.New(opts...)
	s.entries = make(map[uint]CronEntry) // 清空旧的映射，确保重启后可以重新注册
	s.mu.Unlock()

	// 加载启用的任务
	if err := s.loadEnabledJobs(ctx); err != nil {
		s.running.Store(false)
		return err
	}

	// 启动 Cron 引擎
	s.cron.Start()
	s.cfg.Logger.Info("scheduler started")
	return nil
}

// Stop 停止调度器并等待正在执行的 Cron 回调完成
//
// Stop 会阻塞直到 Cron 引擎全部退出或 ctx 取消。
//
// 并发安全：使用 atomic.Bool 保证只停止一次
func (s *CronScheduler) Stop(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("scheduler: nil receiver")
	}
	if !s.running.CompareAndSwap(true, false) {
		return nil // 已经停止，返回 nil（幂等）
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.RLock()
	engine := s.cron
	s.mu.RUnlock()

	if engine == nil {
		return nil
	}

	// Cron.Stop() 返回一个 context，当所有正在运行的 job 完成时 Done
	doneCtx := engine.Stop()
	select {
	case <-doneCtx.Done():
	case <-ctx.Done():
		return fmt.Errorf("scheduler: stop cancelled: %w", ctx.Err())
	}

	s.cfg.Logger.Info("scheduler stopped")
	return nil
}

// Reload 重新加载所有启用任务
//
// 该方法会移除所有当前 Cron 条目，然后重新加载任务并注册。
//
// 并发安全：通过 mu 保护 entries 和 cron 操作
func (s *CronScheduler) Reload(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("scheduler: nil receiver")
	}
	if !s.running.Load() {
		return fmt.Errorf("scheduler: not running")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.cron == nil {
		s.mu.Unlock()
		return fmt.Errorf("scheduler: cron engine not initialized")
	}
	// 移除所有现有条目
	for _, entry := range s.entries {
		s.cron.Remove(entry.ID)
	}
	s.entries = make(map[uint]CronEntry)
	s.mu.Unlock()

	// 重新加载任务
	return s.loadEnabledJobs(ctx)
}

// UpsertJob 更新或新增单个 Job 的 Cron 调度
//
// 当 Cron 表达式变更时，会移除旧条目并注册新条目。
//
// 并发安全：通过 mu 保护所有映射操作
func (s *CronScheduler) UpsertJob(ctx context.Context, job model.Job) error {
	if s == nil {
		return fmt.Errorf("scheduler: nil receiver")
	}
	if !s.running.Load() {
		return fmt.Errorf("scheduler: not running")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron == nil {
		return fmt.Errorf("scheduler: cron engine not initialized")
	}

	return s.registerJobLocked(job)
}

// RemoveJob 从调度器中移除指定 Job
//
// 并发安全：通过 mu 保护映射操作
func (s *CronScheduler) RemoveJob(ctx context.Context, jobID uint) error {
	if s == nil {
		return fmt.Errorf("scheduler: nil receiver")
	}
	if !s.running.Load() {
		return fmt.Errorf("scheduler: not running")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron == nil {
		return fmt.Errorf("scheduler: cron engine not initialized")
	}

	if entry, ok := s.entries[jobID]; ok {
		s.cron.Remove(entry.ID)
		delete(s.entries, jobID)
	}

	return nil
}

// loadEnabledJobs 加载并注册所有启用的 Job
//
// 错误处理：单个任务注册失败不影响其他任务
func (s *CronScheduler) loadEnabledJobs(ctx context.Context) error {
	jobs, err := s.cfg.Jobs.ListEnabledJobs(ctx)
	if err != nil {
		return fmt.Errorf("scheduler: list enabled jobs: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, job := range jobs {
		if err := s.registerJobLocked(job); err != nil {
			// 单个任务失败不影响整体，仅记录错误
			s.cfg.Logger.Error("register job failed",
				zap.Uint("job_id", job.ID),
				zap.String("job_name", job.Name),
				zap.Error(err))
		}
	}
	return nil
}

// registerJobLocked 在持锁情况下注册 Job 的 Cron 任务
//
// 调用前提：已持有 s.mu 锁
//
// 逻辑：
// 1. 如果 Job 未启用或 Cron 为空，移除现有条目
// 2. 如果 Cron 表达式未变更，跳过注册
// 3. 如果 Cron 表达式变更，移除旧条目并注册新条目
func (s *CronScheduler) registerJobLocked(job model.Job) error {
	if s.cron == nil {
		return fmt.Errorf("scheduler: cron engine not initialized")
	}

	spec := strings.TrimSpace(job.Cron)
	// 如果 Job 未启用或 Cron 为空，移除现有条目
	if !job.Enabled || spec == "" {
		if entry, ok := s.entries[job.ID]; ok {
			s.cron.Remove(entry.ID)
			delete(s.entries, job.ID)
		}
		return nil
	}

	// 如果 Cron 表达式未变更，跳过注册
	if entry, ok := s.entries[job.ID]; ok {
		if entry.Spec == spec {
			return nil
		}
		// Cron 表达式变更，移除旧条目
		s.cron.Remove(entry.ID)
		delete(s.entries, job.ID)
	}

	// 注册新条目
	entryID, err := s.cron.AddFunc(spec, s.makeCronHandler(job.ID))
	if err != nil {
		return fmt.Errorf("scheduler: add cron job %d (%s): %w", job.ID, spec, err)
	}

	s.entries[job.ID] = CronEntry{
		ID:   entryID,
		Spec: spec,
	}

	s.cfg.Logger.Info("job scheduled",
		zap.Uint("job_id", job.ID),
		zap.String("job_name", job.Name),
		zap.String("cron_spec", spec))

	return nil
}

// makeCronHandler 构造 Cron 回调函数
//
// 回调函数负责：
// 1. 获取当前调度时间
// 2. 调用 enqueueJob 入队任务
func (s *CronScheduler) makeCronHandler(jobID uint) func() {
	return func() {
		scheduleTime := s.cfg.Now().UTC()
		s.enqueueJob(jobID, scheduleTime)
	}
}

// enqueueJob 将 Job 入队为 TaskRun
//
// 错误处理：
// - 入队失败时记录错误但不影响调度器运行
// - 重复任务被忽略（通过 DedupKey 控制）
func (s *CronScheduler) enqueueJob(jobID uint, scheduleTime time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.EnqueueTimeout)
	defer cancel()

	// 加载 Job 信息
	job, err := s.cfg.Jobs.GetByID(ctx, jobID)
	if err != nil {
		s.cfg.Logger.Error("load job for cron enqueue failed",
			zap.Uint("job_id", jobID),
			zap.Error(err))
		return
	}
	if !job.Enabled {
		s.cfg.Logger.Info("skip enqueue for disabled job", zap.Uint("job_id", jobID))
		return
	}

	// 构造 DedupKey
	dedupKey := fmt.Sprintf("job:%d:cron:%s", job.ID, scheduleTime.Format(time.RFC3339Nano))

	// 构造 Payload
	payload := s.buildPayload(job, scheduleTime)

	// 构造 TaskRun
	task := &model.TaskRun{
		JobID:       job.ID,
		Priority:    int(syncqueue.TaskPriorityNormal),
		AvailableAt: scheduleTime,
		DedupKey:    dedupKey,
		Payload:     payload,
	}

	// 入队
	if err := s.cfg.Queue.Enqueue(ctx, task); err != nil {
		if errors.Is(err, syncqueue.ErrDuplicateTask) {
			// 重复任务，忽略
			s.cfg.Logger.Debug("duplicate cron enqueue ignored",
				zap.Uint("job_id", job.ID),
				zap.String("dedup_key", dedupKey))
			return
		}
		s.cfg.Logger.Error("cron enqueue failed",
			zap.Uint("job_id", job.ID),
			zap.String("dedup_key", dedupKey),
			zap.Error(err))
		return
	}

	s.cfg.Logger.Info("cron task enqueued",
		zap.Uint("job_id", job.ID),
		zap.Uint("task_id", task.ID),
		zap.String("dedup_key", dedupKey))
}

// buildPayload 构造 TaskRun 的 JSON Payload
//
// Payload 包含：
// - job_id: 任务 ID
// - job_name: 任务名称
// - schedule_time: 调度时间
// - cron_spec: Cron 表达式
func (s *CronScheduler) buildPayload(job model.Job, scheduleTime time.Time) string {
	payload := map[string]any{
		"job_id":        job.ID,
		"job_name":      job.Name,
		"schedule_time": scheduleTime.Format(time.RFC3339Nano),
		"cron_spec":     job.Cron,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		s.cfg.Logger.Warn("marshal cron payload failed", zap.Error(err))
		return ""
	}
	return string(data)
}
