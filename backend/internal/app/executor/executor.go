// Package executor 实现任务执行器
package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/strmsync/strmsync/internal/app/ports"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Executor 任务执行器实现
type Executor struct {
	fileMonitor   ports.FileMonitor
	syncPlanner   ports.SyncPlanner
	strmGenerator ports.StrmGenerator
	logger        *zap.Logger
}

// NewExecutor 创建任务执行器
func NewExecutor(
	fileMonitor ports.FileMonitor,
	syncPlanner ports.SyncPlanner,
	strmGenerator ports.StrmGenerator,
	logger *zap.Logger,
) ports.TaskExecutor {
	return &Executor{
		fileMonitor:   fileMonitor,
		syncPlanner:   syncPlanner,
		strmGenerator: strmGenerator,
		logger:        logger,
	}
}

// Execute 执行任务
// 使用errgroup确保所有goroutine正常完成，避免死锁
func (e *Executor) Execute(ctx context.Context, execCtx *ports.ExecutionContext) (*ports.TaskRunSummary, error) {
	startTime := time.Now()

	e.logger.Info("开始执行任务",
		zap.Uint("job_id", execCtx.JobID),
		zap.Uint("task_run_id", execCtx.TaskRunID),
		zap.String("job_name", execCtx.JobConfig.Name))

	// 创建摘要
	summary := &ports.TaskRunSummary{
		StartedAt: startTime,
	}

	// 使用errgroup管理并发组件，确保所有goroutine完成
	g, gCtx := errgroup.WithContext(ctx)

	// 1. 启动文件监控
	e.logger.Info("启动文件监控",
		zap.String("watch_mode", execCtx.JobConfig.WatchMode.String()),
		zap.String("source_path", execCtx.JobConfig.SourcePath))

	eventCh, monitorErrCh := e.fileMonitor.Watch(gCtx, execCtx.JobConfig)

	// 2. 启动同步计划器
	e.logger.Info("启动同步计划器")
	planItemCh, planErrCh := e.syncPlanner.Plan(gCtx, execCtx.JobConfig, eventCh)

	// 收集monitor错误（在goroutine中）
	g.Go(func() error {
		for err := range monitorErrCh {
			if err != nil && err != context.Canceled {
				// 记录错误但不中断（继续处理其他事件）
				e.logger.Error("文件监控错误", zap.Error(err))
			}
		}
		return nil
	})

	// 收集planner错误（在goroutine中）
	g.Go(func() error {
		for err := range planErrCh {
			if err != nil && err != context.Canceled {
				e.logger.Error("同步计划器错误", zap.Error(err))
			}
		}
		return nil
	})

	// 3. 执行同步计划（同步执行，会阻塞直到planItemCh关闭）
	e.logger.Info("开始应用同步计划")
	succeeded, failed, genErr := e.strmGenerator.Apply(gCtx, planItemCh)

	// 4. 等待所有error收集goroutine完成
	// errgroup.Wait会等待所有Go()启动的goroutine完成
	if waitErr := g.Wait(); waitErr != nil {
		// errgroup中的错误（通常不会有，因为我们在goroutine内部只是记录错误）
		e.logger.Error("等待组件完成时出错", zap.Error(waitErr))
	}

	// 5. 处理generator错误
	var execErr error
	if genErr != nil && genErr != context.Canceled {
		execErr = fmt.Errorf("generator: %w", genErr)
	}

	// 6. 更新摘要
	summary.EndedAt = time.Now()
	summary.Duration = int64(summary.EndedAt.Sub(startTime).Seconds())
	summary.CreatedCount = succeeded // TODO: 区分create/update/delete
	summary.UpdatedCount = 0
	summary.DeletedCount = 0
	summary.FailedCount = failed

	if execErr != nil {
		summary.ErrorMessage = execErr.Error()
	}

	e.logger.Info("任务执行完成",
		zap.Uint("job_id", execCtx.JobID),
		zap.Uint("task_run_id", execCtx.TaskRunID),
		zap.Int("succeeded", succeeded),
		zap.Int("failed", failed),
		zap.Int64("duration_seconds", summary.Duration),
		zap.Error(execErr))

	return summary, execErr
}
