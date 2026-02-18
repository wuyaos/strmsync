// Package scheduler 提供基于 Cron 的任务调度器实现
//
// 本包实现了一个企业级的任务调度系统，支持：
// - Cron 表达式调度
// - 动态任务管理（添加/删除/更新）
// - 与 SyncQueue 集成
// - 并发安全
package scheduler

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/strmsync/strmsync/core"
	"go.uber.org/zap"
)

// Scheduler 定义调度器的生命周期与管理接口
//
// 调度器负责根据 Job 的 Cron 表达式定时触发任务，
// 并将任务提交到队列中等待 Worker 执行。
//
// 使用示例：
//
//	scheduler, err := NewScheduler(SchedulerConfig{
//	    Queue: queue,
//	    Jobs:  jobRepo,
//	})
//	if err != nil {
//	    return err
//	}
//
//	if err := scheduler.Start(ctx); err != nil {
//	    return err
//	}
//	defer scheduler.Stop(ctx)
type Scheduler interface {
	// Start 启动调度器并加载所有启用的 Job
	//
	// 启动流程：
	// 1. 初始化 Cron 引擎
	// 2. 从 JobRepository 加载启用的任务
	// 3. 为每个任务注册 Cron 回调
	// 4. 启动 Cron 引擎
	//
	// 参数：
	//   - ctx: 上下文，用于取消和超时控制
	//
	// 返回：
	//   - error: 启动失败时返回错误
	Start(ctx context.Context) error

	// Stop 停止调度器并等待正在执行的 Cron 回调完成
	//
	// Stop 会阻塞直到所有 Cron 回调完成或 ctx 取消。
	//
	// 参数：
	//   - ctx: 上下文，用于超时控制
	//
	// 返回：
	//   - error: 停止失败或超时时返回错误
	Stop(ctx context.Context) error

	// Reload 重新加载所有启用任务
	//
	// 该方法会移除所有当前的 Cron 条目，
	// 然后从 JobRepository 重新加载并注册任务。
	//
	// 参数：
	//   - ctx: 上下文，用于取消和超时控制
	//
	// 返回：
	//   - error: 重新加载失败时返回错误
	Reload(ctx context.Context) error

	// UpsertJob 更新或新增单个 Job 的 Cron 调度
	//
	// 当 Job 的 Cron 表达式变更时，会移除旧条目并注册新条目。
	//
	// 参数：
	//   - ctx: 上下文
	//   - job: 要更新或添加的 Job
	//
	// 返回：
	//   - error: 操作失败时返回错误
	UpsertJob(ctx context.Context, job core.Job) error

	// RemoveJob 从调度器中移除指定 Job
	//
	// 参数：
	//   - ctx: 上下文
	//   - jobID: 要移除的 Job ID
	//
	// 返回：
	//   - error: 操作失败时返回错误
	RemoveJob(ctx context.Context, jobID uint) error
}

// TaskQueue 定义 Scheduler 依赖的任务队列接口
//
// 调度器通过此接口将触发的任务提交到队列中。
type TaskQueue interface {
	// Enqueue 将 TaskRun 入队
	//
	// 参数：
	//   - ctx: 上下文
	//   - task: 要入队的任务
	//
	// 返回：
	//   - error: 入队失败时返回错误（包括重复任务）
	Enqueue(ctx context.Context, task *core.TaskRun) error
}

// JobRepository 定义 Job 查询接口
//
// 调度器通过此接口查询需要调度的任务列表。
type JobRepository interface {
	// ListEnabledJobs 返回所有启用的 Job 列表
	//
	// 仅返回 Enabled=true 的 Job。
	//
	// 参数：
	//   - ctx: 上下文
	//
	// 返回：
	//   - []core.Job: Job 列表
	//   - error: 查询失败时返回错误
	ListEnabledJobs(ctx context.Context) ([]core.Job, error)

	// GetByID 获取指定 Job
	//
	// 参数：
	//   - ctx: 上下文
	//   - id: Job ID
	//
	// 返回：
	//   - core.Job: Job 对象
	//   - error: 查询失败或不存在时返回错误
	GetByID(ctx context.Context, id uint) (core.Job, error)
}

// Clock 提供可注入的时间函数，便于测试
//
// 默认实现可以使用 time.Now。
// 测试时可以注入固定时间或可控制的时间。
type Clock func() time.Time

// SchedulerConfig 是 Scheduler 的构建参数
//
// 所有可选字段都有合理的默认值。
type SchedulerConfig struct {
	// Queue 任务队列（必填）
	//
	// 调度器通过此队列提交触发的任务。
	Queue TaskQueue

	// Jobs Job 仓储（必填）
	//
	// 调度器通过此仓储查询要调度的任务列表。
	Jobs JobRepository

	// Logger 日志器（可选，默认使用 utils.With）
	//
	// 用于记录调度器的运行日志。
	Logger *zap.Logger

	// Location Cron 时区（可选，默认本地时区）
	//
	// 用于解析 Cron 表达式的时区。
	Location *time.Location

	// CronOptions Cron 引擎选项（可选）
	//
	// 可以用于配置 Cron 引擎的行为，如秒级精度、解析器等。
	CronOptions []cron.Option

	// Now 当前时间函数（可选，默认 time.Now）
	//
	// 便于测试时注入固定时间。
	Now Clock

	// EnqueueTimeout 入队超时（可选，默认 10s）
	//
	// Cron 回调中入队任务的最大等待时间。
	EnqueueTimeout time.Duration
}

// CronEntry 表示一个已注册的 Cron 任务
//
// 用于跟踪 Job 与 Cron 引擎 Entry 的映射关系。
type CronEntry struct {
	// ID Cron 引擎中的 Entry ID
	//
	// 用于移除或更新 Cron 条目。
	ID cron.EntryID

	// Spec Cron 表达式
	//
	// 用于判断 Cron 表达式是否变更。
	Spec string
}
