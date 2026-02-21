// Package worker 提供基于队列的 Worker 实现
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/strmsync/strmsync/internal/domain/model"
	"github.com/strmsync/strmsync/internal/pkg/logger"
	"github.com/strmsync/strmsync/internal/pkg/requestid"
	"go.uber.org/zap"
)

const (
	// defaultConcurrency 默认并发数
	defaultConcurrency = 4

	// defaultPollInterval 默认轮询间隔
	defaultPollInterval = 3 * time.Second

	// defaultClaimTimeout 默认领取超时
	defaultClaimTimeout = 5 * time.Second
)

// WorkerPool 是固定大小的 Worker 执行池
//
// 设计要点：
// - 固定数量 goroutine 轮询 ClaimNext
// - 使用可取消 context 控制退出
// - 使用结构化日志记录任务生命周期
//
// 线程安全性：
// - 使用 sync.WaitGroup 等待所有 goroutine 退出
// - 使用 atomic.Bool 管理运行状态
//
// 错误处理：
// - 领取任务失败时记录错误并继续
// - 执行任务失败时回写队列状态
type WorkerPool struct {
	cfg WorkerConfig
	log *zap.Logger

	executor *Executor

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	running atomic.Bool
}

// NewWorker 创建 WorkerPool
//
// 参数：
//   - cfg: Worker 配置（Queue、Jobs、DataServers、TaskRuns 必填）
//
// 返回：
//   - *WorkerPool: Worker 实例
//   - error: 配置无效时返回错误
func NewWorker(cfg WorkerConfig) (*WorkerPool, error) {
	if cfg.Queue == nil {
		return nil, fmt.Errorf("worker: queue is nil")
	}
	if cfg.Jobs == nil {
		return nil, fmt.Errorf("worker: job repository is nil")
	}
	if cfg.DataServers == nil {
		return nil, fmt.Errorf("worker: data server repository is nil")
	}
	if cfg.TaskRuns == nil {
		return nil, fmt.Errorf("worker: task run repository is nil")
	}

	// 设置默认值
	if cfg.Logger == nil {
		cfg.Logger = logger.With(zap.String("component", "worker"))
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = defaultConcurrency
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.ClaimTimeout <= 0 {
		cfg.ClaimTimeout = defaultClaimTimeout
	}
	if cfg.WorkerID == "" {
		cfg.WorkerID = "worker-" + requestid.NewRequestID()
	}

	// 创建 Executor
	executor, err := NewExecutor(ExecutorConfig{
		JobRepo:       cfg.Jobs,
		DataServers:   cfg.DataServers,
		TaskRuns:      cfg.TaskRuns,
		TaskRunEvents: cfg.TaskRunEvents,
		DriverFactory: cfg.DriverFactory,
		WriterFactory: cfg.WriterFactory,
		Logger:        cfg.Logger,
	})
	if err != nil {
		return nil, err
	}

	return &WorkerPool{
		cfg:      cfg,
		log:      cfg.Logger.With(zap.String("worker_id", cfg.WorkerID)),
		executor: executor,
	}, nil
}

// Start 启动 Worker 池
//
// 启动固定数量的 goroutine 执行任务领取-执行-回写循环。
//
// 并发安全：使用 atomic.Bool 保证只启动一次
func (w *WorkerPool) Start(ctx context.Context) error {
	if w == nil {
		return fmt.Errorf("worker: nil receiver")
	}
	if !w.running.CompareAndSwap(false, true) {
		return fmt.Errorf("worker: already running")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	w.ctx, w.cancel = context.WithCancel(ctx)

	// 启动 Worker goroutines
	for i := 0; i < w.cfg.Concurrency; i++ {
		w.wg.Add(1)
		go w.runLoop(i)
	}

	w.log.Info("任务执行器启动",
		zap.Int("concurrency", w.cfg.Concurrency))
	return nil
}

// Stop 停止 Worker 池并等待退出
//
// Stop 会阻塞直到所有 goroutine 完成或 ctx 取消。
//
// 并发安全：使用 atomic.Bool 保证只停止一次
func (w *WorkerPool) Stop(ctx context.Context) error {
	if w == nil {
		return fmt.Errorf("worker: nil receiver")
	}
	if !w.running.CompareAndSwap(true, false) {
		return nil // 已经停止，返回 nil（幂等）
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// 取消所有 goroutine
	if w.cancel != nil {
		w.cancel()
	}

	// 等待所有 goroutine 退出
	waitCh := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
	case <-ctx.Done():
		return fmt.Errorf("worker: stop cancelled: %w", ctx.Err())
	}

	w.log.Info("任务执行器停止")
	return nil
}

// runLoop 是单个 worker 的主循环
//
// 循环逻辑：
// 1. 检查 context 是否取消
// 2. 领取下一个任务
// 3. 如果没有任务，等待 PollInterval 后重试
// 4. 如果有任务，执行任务
func (w *WorkerPool) runLoop(index int) {
	defer w.wg.Done()

	log := w.log.With(zap.Int("worker_index", index))

	for {
		// 检查 context 是否取消
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		// 领取任务
		task, err := w.claimNext()
		if err != nil {
			log.Warn("claim next failed", zap.Error(err))
			w.sleepWithContext(w.cfg.PollInterval)
			continue
		}
		if task == nil {
			// 没有可用任务，等待后重试
			w.sleepWithContext(w.cfg.PollInterval)
			continue
		}

		// 执行任务
		if err := w.executeTask(log, task); err != nil {
			jobName := extractJobName(task.Payload)
			log.Error("task execution failed",
				zap.Uint("task_id", task.ID),
				zap.Uint("job_id", task.JobID),
				zap.String("job_name", jobName),
				zap.Error(err))
		}
	}
}

// claimNext 领取任务
//
// 使用 ClaimTimeout 控制超时。
func (w *WorkerPool) claimNext() (*model.TaskRun, error) {
	claimCtx := w.ctx
	var cancel context.CancelFunc
	if w.cfg.ClaimTimeout > 0 {
		claimCtx, cancel = context.WithTimeout(w.ctx, w.cfg.ClaimTimeout)
	}
	if cancel != nil {
		defer cancel()
	}

	return w.cfg.Queue.ClaimNext(claimCtx, w.cfg.WorkerID)
}

// executeTask 执行单个任务并回写队列状态
//
// 执行流程：
// 1. 使用 Executor 执行任务
// 2. 如果执行失败，调用 Queue.Fail
// 3. 如果执行成功，调用 Queue.Complete
func (w *WorkerPool) executeTask(log *zap.Logger, task *model.TaskRun) error {
	if task == nil {
		return nil
	}

	jobName := extractJobName(task.Payload)
	taskLog := log.With(
		zap.Uint("task_id", task.ID),
		zap.Uint("job_id", task.JobID),
		zap.String("job_name", jobName),
	)

	execCtx := w.ctx
	var cancel context.CancelFunc
	if w.cfg.RunTimeout > 0 {
		execCtx, cancel = context.WithTimeout(w.ctx, w.cfg.RunTimeout)
	}
	if cancel != nil {
		defer cancel()
	}

	// 执行任务
	stats, err := w.executor.Run(execCtx, task)

	// 回写队列状态使用独立的 context，避免被执行超时影响
	// 这确保即使任务执行超时，我们仍能成功更新队列状态
	updateCtx, updateCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer updateCancel()

	if err != nil {
		// 执行失败，回写队列
		failErr := w.cfg.Queue.Fail(updateCtx, task.ID, err)
		if failErr != nil {
			taskLog.Error("queue fail update failed",
				zap.Error(failErr))
		}
		// 回写 Job 状态为 error
		if statusErr := w.cfg.Jobs.UpdateStatus(updateCtx, task.JobID, "error"); statusErr != nil {
			taskLog.Warn("update job status to error failed",
				zap.Error(statusErr))
		}
		taskLog.Error("task failed",
			zap.Error(err))
		return err
	}

	// 执行成功，回写队列
	if err := w.cfg.Queue.Complete(updateCtx, task.ID); err != nil {
		taskLog.Error("queue complete update failed",
			zap.Error(err))
		return err
	}
	// 回写 Job 状态为 idle
	if statusErr := w.cfg.Jobs.UpdateStatus(updateCtx, task.JobID, "idle"); statusErr != nil {
		taskLog.Warn("update job status to idle failed",
			zap.Error(statusErr))
	}
	// 更新 Job 的最后运行时间
	if lastRunErr := w.cfg.Jobs.UpdateLastRunAt(updateCtx, task.JobID, time.Now()); lastRunErr != nil {
		taskLog.Warn("update job last_run_at failed",
			zap.Error(lastRunErr))
	}

	taskLog.Info("task completed",
		zap.Int64("processed_files", stats.ProcessedFiles),
		zap.Int64("created_files", stats.CreatedFiles),
		zap.Int64("updated_files", stats.UpdatedFiles),
		zap.Int64("skipped_files", stats.SkippedFiles),
		zap.Int64("filtered_files", stats.FilteredFiles),
		zap.Int64("failed_files", stats.FailedFiles),
		zap.Int64("total_files", stats.TotalFiles))
	return nil
}

// sleepWithContext 支持取消的休眠
//
// 在休眠期间如果 context 取消，会立即返回。
func (w *WorkerPool) sleepWithContext(d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-w.ctx.Done():
	case <-timer.C:
	}
}

func extractJobName(payload string) string {
	if strings.TrimSpace(payload) == "" {
		return ""
	}
	var data struct {
		JobName string `json:"job_name"`
	}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return ""
	}
	return strings.TrimSpace(data.JobName)
}
