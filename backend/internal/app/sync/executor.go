// Package sync 实现STRM同步服务
package sync

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
	fileMonitor      ports.FileMonitor
	syncPlanner      ports.SyncPlanner
	strmGenerator    ports.StrmGenerator
	metaReplicator   ports.MetadataReplicator
	logger           *zap.Logger
}

// NewExecutor 创建任务执行器
func NewExecutor(
	fileMonitor ports.FileMonitor,
	syncPlanner ports.SyncPlanner,
	strmGenerator ports.StrmGenerator,
	metaReplicator ports.MetadataReplicator,
	logger *zap.Logger,
) ports.TaskExecutor {
	return &Executor{
		fileMonitor:    fileMonitor,
		syncPlanner:    syncPlanner,
		strmGenerator:  strmGenerator,
		metaReplicator: metaReplicator,
		logger:         logger,
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

	// 3. 路由计划项到不同的处理器（fan-out）
	e.logger.Info("启动计划项路由器")
	strmCh := make(chan ports.SyncPlanItem, 100)
	metaCh := make(chan ports.SyncPlanItem, 100)

	// 路由goroutine：根据Kind分发计划项
	g.Go(func() error {
		defer close(strmCh)
		defer close(metaCh)

		strmCount := 0
		metaCount := 0

		for item := range planItemCh {
			switch item.Kind {
			case ports.PlanItemStrm:
				select {
				case <-gCtx.Done():
					e.logger.Info("路由器停止（STRM）",
						zap.Int("strm_routed", strmCount),
						zap.Int("meta_routed", metaCount))
					return gCtx.Err()
				case strmCh <- item:
					strmCount++
				}
			case ports.PlanItemMetadata:
				select {
				case <-gCtx.Done():
					e.logger.Info("路由器停止（Metadata）",
						zap.Int("strm_routed", strmCount),
						zap.Int("meta_routed", metaCount))
					return gCtx.Err()
				case metaCh <- item:
					metaCount++
				}
			default:
				e.logger.Warn("未知计划项类型",
					zap.String("kind", item.Kind.String()),
					zap.String("source_path", item.SourcePath))
			}
		}

		e.logger.Info("计划项路由完成",
			zap.Int("strm_routed", strmCount),
			zap.Int("meta_routed", metaCount))
		return nil
	})

	// 4. 并发执行STRM生成和元数据复制
	e.logger.Info("开始应用同步计划（并发执行）")

	var strmSucceeded, strmFailed int
	var metaSucceeded, metaFailed int
	var strmErr, metaErr error

	// STRM生成goroutine
	g.Go(func() error {
		e.logger.Info("STRM生成器开始处理")
		strmSucceeded, strmFailed, strmErr = e.strmGenerator.Apply(gCtx, strmCh)
		e.logger.Info("STRM生成器完成",
			zap.Int("succeeded", strmSucceeded),
			zap.Int("failed", strmFailed),
			zap.Error(strmErr))
		return nil // 不中断其他goroutine
	})

	// 元数据复制goroutine
	g.Go(func() error {
		e.logger.Info("元数据复制器开始处理")
		metaSucceeded, metaFailed, metaErr = e.metaReplicator.Apply(gCtx, metaCh)
		e.logger.Info("元数据复制器完成",
			zap.Int("succeeded", metaSucceeded),
			zap.Int("failed", metaFailed),
			zap.Error(metaErr))
		return nil // 不中断其他goroutine
	})

	// 5. 等待所有goroutine完成
	if waitErr := g.Wait(); waitErr != nil {
		e.logger.Error("等待组件完成时出错", zap.Error(waitErr))
	}

	// 6. 合并处理结果
	totalSucceeded := strmSucceeded + metaSucceeded
	totalFailed := strmFailed + metaFailed

	// 处理错误
	var execErr error
	if strmErr != nil && strmErr != context.Canceled {
		execErr = fmt.Errorf("strm generator: %w", strmErr)
	}
	if metaErr != nil && metaErr != context.Canceled {
		if execErr != nil {
			execErr = fmt.Errorf("%v; metadata replicator: %w", execErr, metaErr)
		} else {
			execErr = fmt.Errorf("metadata replicator: %w", metaErr)
		}
	}

	// 7. 更新摘要
	summary.EndedAt = time.Now()
	summary.Duration = int64(summary.EndedAt.Sub(startTime).Seconds())
	summary.CreatedCount = totalSucceeded // TODO: 区分create/update/delete
	summary.UpdatedCount = 0
	summary.DeletedCount = 0
	summary.FailedCount = totalFailed

	if execErr != nil {
		summary.ErrorMessage = execErr.Error()
	}

	e.logger.Info("任务执行完成",
		zap.Uint("job_id", execCtx.JobID),
		zap.Uint("task_run_id", execCtx.TaskRunID),
		zap.Int("strm_succeeded", strmSucceeded),
		zap.Int("strm_failed", strmFailed),
		zap.Int("meta_succeeded", metaSucceeded),
		zap.Int("meta_failed", metaFailed),
		zap.Int("total_succeeded", totalSucceeded),
		zap.Int("total_failed", totalFailed),
		zap.Int64("duration_seconds", summary.Duration),
		zap.Error(execErr))

	return summary, execErr
}
