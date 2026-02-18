// Package sync 实现STRM同步服务
package sync

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/strmsync/strmsync/internal/app/ports"
	syncengine "github.com/strmsync/strmsync/internal/engine"
	"go.uber.org/zap"
)

// EngineRunStrategy 控制何时触发engine执行
type EngineRunStrategy int

const (
	// EngineRunOnBatch 当批次大小达到阈值时触发
	EngineRunOnBatch EngineRunStrategy = iota + 1
	// EngineRunOnTimer 按固定间隔触发（如果有待处理事件）
	EngineRunOnTimer
	// EngineRunOnIdle 当一段时间没有新事件时触发
	EngineRunOnIdle
)

// EngineStrmGeneratorConfig 配置EngineStrmGenerator
type EngineStrmGeneratorConfig struct {
	Logger     *zap.Logger
	Driver     syncengine.Driver
	Writer     syncengine.Writer
	SourceRoot string
	EngineOpts syncengine.EngineOptions

	Strategy     EngineRunStrategy
	BatchSize    int
	BatchTimeout time.Duration // 用于EngineRunOnTimer
	IdleTimeout  time.Duration // 用于EngineRunOnIdle
}

// EngineStrmGenerator 将syncengine.Engine适配为ports.StrmGenerator
//
// 它聚合事件驱动的计划项，并根据配置的策略触发engine.RunOnce。
// 这保留了engine的扫描和差异对比优势，同时允许基于事件的编排。
//
// 设计注意事项（步骤1实现）：
//
// 1. 事件聚合目前仅用于触发时机判断，不影响engine的处理范围。
//    无论pending中有多少事件，engine.RunOnce都会扫描整个SourceRoot。
//    这是有意设计，确保engine的完整性检查和差异对比能力。
//
// 2. Delete操作通过启用OrphanCleanup处理，这确保了已删除的远程文件
//    对应的本地STRM文件会被清理。
//
// 3. Engine失败时保留pending事件，调用者可选择重试或放弃。
//    这避免了在临时故障（如网络错误）时丢失事件。
//
// 未来改进（步骤3）：
//
// - 实现engine.RunIncremental，仅处理pending事件对应的文件
// - 这将显著提升增量更新的性能
type EngineStrmGenerator struct {
	cfg EngineStrmGeneratorConfig
}

// NewEngineStrmGenerator 创建新的EngineStrmGenerator
func NewEngineStrmGenerator(cfg EngineStrmGeneratorConfig) (ports.StrmGenerator, error) {
	if cfg.Driver == nil {
		return nil, fmt.Errorf("engine strm generator: driver is nil")
	}
	if cfg.Writer == nil {
		return nil, fmt.Errorf("engine strm generator: writer is nil")
	}
	if cfg.Logger == nil {
		return nil, fmt.Errorf("engine strm generator: logger is nil")
	}
	if cfg.EngineOpts.OutputRoot == "" {
		return nil, fmt.Errorf("engine strm generator: engine output root is empty")
	}

	// 强制启用OrphanCleanup以确保delete操作生效
	// 这是EngineStrmGenerator正确处理delete事件的关键
	cfg.EngineOpts.EnableOrphanCleanup = true

	// 对于DryRun模式，确保孤儿清理也使用dry-run
	// 这保证了delete事件在dry-run时的行为一致性
	if cfg.EngineOpts.DryRun {
		cfg.EngineOpts.OrphanCleanupDryRun = true
	} else {
		// 非dry-run时，强制关闭OrphanCleanupDryRun
		// 确保delete操作真实执行
		cfg.EngineOpts.OrphanCleanupDryRun = false
	}

	cfg.Logger.Info("engine strm generator: orphan cleanup configured",
		zap.Bool("enabled", true),
		zap.Bool("dry_run", cfg.EngineOpts.OrphanCleanupDryRun))

	// 设置合理的默认值
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 200
	}
	if cfg.BatchTimeout <= 0 {
		cfg.BatchTimeout = 2 * time.Second
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 5 * time.Second
	}
	if cfg.Strategy == 0 {
		cfg.Strategy = EngineRunOnBatch
	}

	return &EngineStrmGenerator{cfg: cfg}, nil
}

// Apply 聚合STRM计划项并触发engine.RunOnce
//
// 行为：
// - 仅处理PlanItemStrm类型的项
// - 按SourcePath去重（保留最新的ModTime，delete操作在同时间优先）
// - 根据选择的策略触发RunOnce
// - 返回从syncengine.SyncStats派生的成功/失败计数
func (g *EngineStrmGenerator) Apply(ctx context.Context, items <-chan ports.SyncPlanItem) (int, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	logger := g.cfg.Logger.With(
		zap.String("component", "engine-strm-generator"),
		zap.String("strategy", g.strategyName()),
		zap.Int("batch_size", g.cfg.BatchSize),
		zap.Duration("batch_timeout", g.cfg.BatchTimeout),
		zap.Duration("idle_timeout", g.cfg.IdleTimeout),
	)

	logger.Info("engine strm generator started",
		zap.String("source_root", g.cfg.SourceRoot))

	pending := make(map[string]ports.SyncPlanItem)
	totalSucceeded := 0
	totalFailed := 0
	consecutiveFailures := 0        // 连续失败计数
	const maxConsecutiveFailures = 3 // 最大连续失败次数

	// 失败后重试定时器（用于所有策略）
	var retryTimer *time.Timer
	const retryBackoff = 10 * time.Second // 失败后10秒重试

	// 根据策略创建定时器（可选）
	var ticker *time.Ticker
	var idleTimer *time.Timer
	idleTimerStarted := false // 跟踪idle timer是否已启动

	switch g.cfg.Strategy {
	case EngineRunOnTimer:
		ticker = time.NewTicker(g.cfg.BatchTimeout)
		defer ticker.Stop()
	case EngineRunOnIdle:
		// idle timer在收到第一个事件时启动，避免空闲时无意义的触发
		idleTimer = time.NewTimer(g.cfg.IdleTimeout)
		if !idleTimer.Stop() {
			<-idleTimer.C
		}
		defer idleTimer.Stop()
	case EngineRunOnBatch:
		// 不需要定时器
	default:
		return 0, 0, fmt.Errorf("engine strm generator: unknown strategy: %v", g.cfg.Strategy)
	}

	flush := func(reason string) error {
		if len(pending) == 0 {
			return nil
		}

		// 快照计数用于日志
		pendingCount := len(pending)

		// 将 pending map 转换为 EngineEvent 列表
		events := make([]syncengine.EngineEvent, 0, pendingCount)
		for _, item := range pending {
			var evType syncengine.DriverEventType
			switch item.Op {
			case ports.SyncOpCreate:
				evType = syncengine.DriverEventCreate
			case ports.SyncOpUpdate:
				evType = syncengine.DriverEventUpdate
			case ports.SyncOpDelete:
				evType = syncengine.DriverEventDelete
			default:
				logger.Warn("unknown sync operation, skipping",
					zap.String("op", item.Op.String()),
					zap.String("source_path", item.SourcePath))
				continue
			}
			events = append(events, syncengine.EngineEvent{
				Type:    evType,
				AbsPath: item.SourcePath,
				Size:    item.Size,
				ModTime: item.ModTime,
			})
		}

		if len(events) == 0 {
			// 所有 pending 项都被跳过（如未知 op），清空并返回
			clearMap(pending)
			return nil
		}

		logger.Info("triggering incremental engine run",
			zap.String("reason", reason),
			zap.Int("pending_events", pendingCount),
			zap.Int("engine_events", len(events)))

		// 为每次执行创建新的 Engine 实例（避免 running atomic.Bool 冲突）
		engine, err := syncengine.NewEngine(g.cfg.Driver, g.cfg.Writer, logger, g.cfg.EngineOpts)
		if err != nil {
			return fmt.Errorf("create engine: %w", err)
		}

		stats, err := engine.RunIncremental(ctx, events)
		if err != nil {
			logger.Error("engine incremental run failed",
				zap.String("reason", reason),
				zap.Int("pending_events", pendingCount),
				zap.Error(err))
			// 即使出错，也从 stats 中统计失败数
			succeeded, failed := statsToCounts(stats)
			totalSucceeded += succeeded
			totalFailed += failed
			consecutiveFailures++

			// 启动重试定时器（使用 stop+drain 保证安全）
			if retryTimer == nil {
				retryTimer = time.NewTimer(retryBackoff)
			} else {
				// 先安全停止并排空，再重置
				safeStopTimer(retryTimer)
				retryTimer.Reset(retryBackoff)
			}

			// 注意：保留 pending，让重试定时器或下一次触发继续尝试
			logger.Warn("engine run failed, will retry after backoff",
				zap.Int("consecutive_failures", consecutiveFailures),
				zap.Int("pending_events", len(pending)),
				zap.Duration("retry_after", retryBackoff))
			return fmt.Errorf("engine run incremental: %w", err)
		}

		succeeded, failed := statsToCounts(stats)
		totalSucceeded += succeeded
		totalFailed += failed
		consecutiveFailures = 0 // 成功后重置连续失败计数

		// 成功后安全停止重试定时器（含排空）
		safeStopTimer(retryTimer)

		logger.Info("engine incremental run completed",
			zap.Int("pending_events", pendingCount),
			zap.Int("succeeded", succeeded),
			zap.Int("failed", failed),
			zap.Int64("processed_files", stats.ProcessedFiles),
			zap.Int64("created_files", stats.CreatedFiles),
			zap.Int64("updated_files", stats.UpdatedFiles),
			zap.Int64("failed_files", stats.FailedFiles),
			zap.Int64("deleted_orphans", stats.DeletedOrphans),
			zap.Duration("duration", stats.Duration))

		// 成功执行后清空 pending
		clearMap(pending)
		return nil
	}

	for {
		// 检查连续失败次数
		if consecutiveFailures >= maxConsecutiveFailures {
			logger.Error("too many consecutive failures, aborting",
				zap.Int("consecutive_failures", consecutiveFailures),
				zap.Int("pending_events", len(pending)))
			return totalSucceeded, totalFailed, fmt.Errorf("engine strm generator: %d consecutive failures", consecutiveFailures)
		}

		select {
		case <-ctx.Done():
			logger.Warn("engine strm generator cancelled", zap.Error(ctx.Err()))
			// 尽力flush待处理项
			_ = flush("context_cancelled")
			return totalSucceeded, totalFailed, ctx.Err()

		case item, ok := <-items:
			if !ok {
				// 通道关闭：flush剩余事件
				err := flush("channel_closed")
				if err != nil {
					logger.Error("final flush failed",
						zap.Error(err),
						zap.Int("pending_events", len(pending)))
					// 最终flush失败应该返回错误，避免丢失pending事件
					return totalSucceeded, totalFailed, fmt.Errorf("final flush failed: %w", err)
				}
				logger.Info("engine strm generator completed",
					zap.Int("total_succeeded", totalSucceeded),
					zap.Int("total_failed", totalFailed))
				return totalSucceeded, totalFailed, nil
			}

			if item.Kind != ports.PlanItemStrm {
				logger.Debug("skip non-strm plan item",
					zap.String("kind", item.Kind.String()),
					zap.String("source_path", item.SourcePath))
				continue
			}

			// 按SourcePath去重，保留最新
			mergePlanItem(pending, item)

			logger.Debug("plan item buffered",
				zap.String("source_path", item.SourcePath),
				zap.String("op", item.Op.String()),
				zap.Int("pending_events", len(pending)))

			// 策略特定的触发逻辑
			switch g.cfg.Strategy {
			case EngineRunOnBatch:
				if len(pending) >= g.cfg.BatchSize {
					// flush失败时记录错误但继续运行
					if err := flush("batch_size_reached"); err != nil {
						logger.Error("flush failed, will retry on next trigger",
							zap.Error(err))
					}
				}
			case EngineRunOnTimer:
				// 定时器在单独的case中处理
			case EngineRunOnIdle:
				// 首次收到事件时启动idle timer
				if !idleTimerStarted {
					idleTimer.Reset(g.cfg.IdleTimeout)
					idleTimerStarted = true
				} else {
					// 每次收到事件时重置空闲定时器
					if !idleTimer.Stop() {
						select {
						case <-idleTimer.C:
						default:
						}
					}
					idleTimer.Reset(g.cfg.IdleTimeout)
				}
			default:
				return totalSucceeded, totalFailed, fmt.Errorf("engine strm generator: unknown strategy: %v", g.cfg.Strategy)
			}

		case <-tickerChan(ticker):
			if g.cfg.Strategy != EngineRunOnTimer {
				continue
			}
			// flush失败时记录错误但继续运行
			if err := flush("timer_tick"); err != nil {
				logger.Error("flush failed, will retry on next timer tick",
					zap.Error(err))
			}

		case <-timerChan(idleTimer):
			if g.cfg.Strategy != EngineRunOnIdle {
				continue
			}
			// flush失败时记录错误但继续运行
			if err := flush("idle_timeout"); err != nil {
				logger.Error("flush failed, will retry after backoff",
					zap.Error(err))
			}
			// 重置idle timer以便继续监控
			idleTimerStarted = false

		case <-timerChan(retryTimer):
			// 重试定时器触发，尝试flush pending事件
			logger.Info("retry timer triggered, attempting flush",
				zap.Int("pending_events", len(pending)),
				zap.Int("consecutive_failures", consecutiveFailures))
			if err := flush("retry_after_failure"); err != nil {
				logger.Error("retry flush failed",
					zap.Error(err),
					zap.Int("consecutive_failures", consecutiveFailures))
			}
		}
	}
}

func (g *EngineStrmGenerator) strategyName() string {
	switch g.cfg.Strategy {
	case EngineRunOnBatch:
		return "batch"
	case EngineRunOnTimer:
		return "timer"
	case EngineRunOnIdle:
		return "idle"
	default:
		return "unknown"
	}
}

// mergePlanItem 按SourcePath去重，保留最新的项
// 如果ModTime相同，delete操作覆盖create/update
func mergePlanItem(pending map[string]ports.SyncPlanItem, incoming ports.SyncPlanItem) {
	key := incoming.SourcePath
	if key == "" {
		return
	}

	existing, ok := pending[key]
	if !ok {
		pending[key] = incoming
		return
	}

	// 优先选择较新的ModTime
	switch {
	case existing.ModTime.IsZero() && !incoming.ModTime.IsZero():
		pending[key] = incoming
		return
	case incoming.ModTime.After(existing.ModTime):
		pending[key] = incoming
		return
	case incoming.ModTime.Equal(existing.ModTime):
		// ModTime相同时，delete操作优先（避免留下陈旧文件）
		if incoming.Op == ports.SyncOpDelete && existing.Op != ports.SyncOpDelete {
			pending[key] = incoming
		}
		return
	default:
		// 保留existing（更新）
		return
	}
}

// statsToCounts 将syncengine.SyncStats映射为成功/失败计数
//
// succeeded = max(ProcessedFiles - FailedFiles, 0)
// failed = FailedFiles
func statsToCounts(stats syncengine.SyncStats) (int, int) {
	processed := safeInt64ToInt(stats.ProcessedFiles)
	failed := safeInt64ToInt(stats.FailedFiles)
	succeeded := processed - failed
	if succeeded < 0 {
		succeeded = 0
	}
	if failed < 0 {
		failed = 0
	}
	return succeeded, failed
}

func safeInt64ToInt(v int64) int {
	if v > math.MaxInt {
		return math.MaxInt
	}
	if v < math.MinInt {
		return math.MinInt
	}
	return int(v)
}

func clearMap(m map[string]ports.SyncPlanItem) {
	for k := range m {
		delete(m, k)
	}
}

func tickerChan(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

func timerChan(t *time.Timer) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

// safeStopTimer 安全地停止定时器并排空已触发但未读取的信号
//
// 根据 time.Timer.Stop() 文档，如果 Stop() 返回 false，表示定时器已触发，
// 需要显式排空 channel 以防止后续误触发。
func safeStopTimer(t *time.Timer) {
	if t == nil {
		return
	}
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}
