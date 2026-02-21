// Package worker 提供基于队列的 Worker 执行器实现
//
// 本包实现了一个企业级的任务执行系统，支持：
// - 从队列领取任务
// - 执行同步引擎
// - 结果回写队列
// - 并发控制
package worker

import (
	"context"
	"time"

	"github.com/strmsync/strmsync/internal/domain/model"
	"github.com/strmsync/strmsync/internal/engine"
	"go.uber.org/zap"
)

// Worker 表示任务执行器的生命周期接口
//
// Worker 负责从队列中领取任务，执行同步引擎，
// 并将结果回写到队列中。
//
// 使用示例：
//
//	worker, err := NewWorker(WorkerConfig{
//	    Queue:       queue,
//	    Jobs:        jobRepo,
//	    DataServers: serverRepo,
//	    TaskRuns:    taskRunRepo,
//	})
//	if err != nil {
//	    return err
//	}
//
//	if err := worker.Start(ctx); err != nil {
//	    return err
//	}
//	defer worker.Stop(ctx)
type Worker interface {
	// Start 启动 Worker
	//
	// 启动固定数量的 goroutine 执行任务领取-执行-回写循环。
	//
	// 参数：
	//   - ctx: 上下文，用于取消和超时控制
	//
	// 返回：
	//   - error: 启动失败时返回错误
	Start(ctx context.Context) error

	// Stop 停止 Worker 并等待所有 goroutine 退出
	//
	// Stop 会阻塞直到所有 goroutine 完成或 ctx 取消。
	//
	// 参数：
	//   - ctx: 上下文，用于超时控制
	//
	// 返回：
	//   - error: 停止失败或超时时返回错误
	Stop(ctx context.Context) error
}

// TaskQueue 定义 Worker 依赖的队列接口
//
// Worker 通过此接口从队列中领取任务并回写结果。
type TaskQueue interface {
	// ClaimNext 原子领取下一个待执行任务
	//
	// 参数：
	//   - ctx: 上下文
	//   - workerID: Worker 标识
	//
	// 返回：
	//   - *model.TaskRun: 领取到的任务，如果没有可用任务则返回 nil
	//   - error: 领取失败时返回错误
	ClaimNext(ctx context.Context, workerID string) (*model.TaskRun, error)

	// Complete 标记任务完成
	//
	// 参数：
	//   - ctx: 上下文
	//   - taskID: 任务 ID
	//
	// 返回：
	//   - error: 操作失败时返回错误
	Complete(ctx context.Context, taskID uint) error

	// Fail 标记任务失败
	//
	// 参数：
	//   - ctx: 上下文
	//   - taskID: 任务 ID
	//   - err: 失败错误
	//
	// 返回：
	//   - error: 操作失败时返回错误
	Fail(ctx context.Context, taskID uint, err error) error
}

// JobRepository 定义 Job 查询接口
type JobRepository interface {
	// GetByID 获取指定 Job
	//
	// 参数：
	//   - ctx: 上下文
	//   - id: Job ID
	//
	// 返回：
	//   - model.Job: Job 对象
	//   - error: 查询失败或不存在时返回错误
	GetByID(ctx context.Context, id uint) (model.Job, error)

	// UpdateStatus 更新 Job 的运行状态
	//
	// 用于 Worker 在任务完成/失败后将 Job.status 回写为 idle 或 error。
	//
	// 参数：
	//   - ctx: 上下文
	//   - id: Job ID
	//   - status: 目标状态（"idle"/"running"/"error"）
	//
	// 返回：
	//   - error: 更新失败时返回错误
	UpdateStatus(ctx context.Context, id uint, status string) error

	// UpdateLastRunAt 更新 Job 的最后运行时间
	//
	// 用于 Worker 在任务成功完成后更新 Job.last_run_at 字段。
	//
	// 参数：
	//   - ctx: 上下文
	//   - id: Job ID
	//   - lastRunAt: 最后运行时间
	//
	// 返回：
	//   - error: 更新失败时返回错误
	UpdateLastRunAt(ctx context.Context, id uint, lastRunAt time.Time) error
}

// DataServerRepository 定义 DataServer 查询接口
type DataServerRepository interface {
	// GetByID 获取指定 DataServer
	//
	// 参数：
	//   - ctx: 上下文
	//   - id: DataServer ID
	//
	// 返回：
	//   - model.DataServer: DataServer 对象
	//   - error: 查询失败或不存在时返回错误
	GetByID(ctx context.Context, id uint) (model.DataServer, error)
}

// TaskRunProgress 描述 TaskRun 的进度字段
//
// 用于更新 TaskRun 的统计信息。
type TaskRunProgress struct {
	// TotalFiles 总文件数
	TotalFiles int

	// ProcessedFiles 已处理文件数
	ProcessedFiles int

	// FailedFiles 失败文件数
	FailedFiles int

	// CreatedFiles 新建STRM数
	CreatedFiles int

	// UpdatedFiles 更新STRM数
	UpdatedFiles int

	// SkippedFiles 跳过STRM数
	SkippedFiles int

	// FilteredFiles 过滤文件数
	FilteredFiles int

	// MetaTotalFiles 元数据总数
	MetaTotalFiles int

	// MetaCreatedFiles 元数据新增数
	MetaCreatedFiles int

	// MetaUpdatedFiles 元数据更新数
	MetaUpdatedFiles int

	// MetaProcessedFiles 元数据已处理数
	MetaProcessedFiles int

	// MetaFailedFiles 元数据失败数
	MetaFailedFiles int

	// Progress 进度百分比（0-100）
	Progress int
}

// TaskRunRepository 定义 TaskRun 更新接口
type TaskRunRepository interface {
	// UpdateProgress 更新 TaskRun 的统计字段
	//
	// 参数：
	//   - ctx: 上下文
	//   - taskID: 任务 ID
	//   - progress: 进度信息
	//
	// 返回：
	//   - error: 更新失败时返回错误
	UpdateProgress(ctx context.Context, taskID uint, progress TaskRunProgress) error
}

// TaskRunEventRepository 定义 TaskRunEvent 写入接口
type TaskRunEventRepository interface {
	// Create 写入单条执行事件
	Create(ctx context.Context, event *model.TaskRunEvent) error
}

// DriverFactory 根据 DataServer 构建 Driver 实例
//
// 用于构建不同类型的数据源驱动（CloudDrive2、OpenList 等）。
type DriverFactory interface {
	// Build 构建同步引擎驱动
	//
	// 参数：
	//   - ctx: 上下文
	//   - server: 数据服务器配置
	//
	// 返回：
	//   - syncengine.Driver: 驱动实例
	//   - error: 构建失败时返回错误
	Build(ctx context.Context, server model.DataServer) (syncengine.Driver, error)
}

// WriterFactory 根据 Job 构建 Writer 实例
//
// 用于构建 STRM 文件写入器。
type WriterFactory interface {
	// Build 构建同步引擎写入器
	//
	// 参数：
	//   - ctx: 上下文
	//   - job: 任务配置
	//
	// 返回：
	//   - syncengine.Writer: 写入器实例
	//   - error: 构建失败时返回错误
	Build(ctx context.Context, job model.Job) (syncengine.Writer, error)
}

// WorkerConfig 描述 Worker 运行参数
//
// 所有可选字段都有合理的默认值。
type WorkerConfig struct {
	// Queue 任务队列（必填）
	//
	// Worker 从此队列领取任务并回写结果。
	Queue TaskQueue

	// Jobs Job 仓储（必填）
	//
	// Worker 通过此仓储查询任务配置。
	Jobs JobRepository

	// DataServers DataServer 仓储（必填）
	//
	// Worker 通过此仓储查询数据服务器配置。
	DataServers DataServerRepository

	// TaskRuns TaskRun 仓储（必填）
	//
	// Worker 通过此仓储更新任务进度。
	TaskRuns TaskRunRepository

	// TaskRunEvents TaskRunEvent 仓储（可选）
	//
	// Worker 通过此仓储写入执行事件明细。
	TaskRunEvents TaskRunEventRepository

	// DriverFactory 驱动工厂（可选，默认使用 DefaultDriverFactory）
	//
	// 用于构建数据源驱动。
	DriverFactory DriverFactory

	// WriterFactory 写入器工厂（可选，默认使用 DefaultWriterFactory）
	//
	// 用于构建 STRM 文件写入器。
	WriterFactory WriterFactory

	// Logger 日志器（可选，默认使用 utils.With）
	//
	// 用于记录 Worker 的运行日志。
	Logger *zap.Logger

	// Concurrency Worker 并发数（可选，默认 4）
	//
	// 控制同时执行的任务数量。
	Concurrency int

	// PollInterval 无任务时轮询间隔（可选，默认 3s）
	//
	// 当没有可用任务时，Worker 会等待此时间后再次尝试领取。
	PollInterval time.Duration

	// ClaimTimeout 领取任务超时（可选，默认 5s）
	//
	// 领取任务的最大等待时间。
	ClaimTimeout time.Duration

	// RunTimeout 单次执行超时（可选）
	//
	// 单个任务的最大执行时间。0 表示无限制。
	RunTimeout time.Duration

	// WorkerID Worker 标识（可选，自动生成）
	//
	// 用于追踪任务执行的 Worker。
	WorkerID string
}
