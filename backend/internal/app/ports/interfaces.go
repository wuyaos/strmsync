// Package service 定义Service层的核心接口
package ports

import (
	"context"

	"github.com/strmsync/strmsync/internal/infra/filesystem"
)

// FileService 文件服务接口
type FileService interface {
	// List 获取文件列表
	List(ctx context.Context, req FileListRequest) ([]filesystem.RemoteFile, error)
}

// JobService Job业务逻辑服务
type JobService interface {
	// Run 运行任务（创建TaskRun并启动执行）
	Run(ctx context.Context, jobID JobID) (TaskRunID, error)

	// Stop 停止任务（取消正在运行的TaskRun）
	Stop(ctx context.Context, jobID JobID) error

	// Validate 验证Job配置是否有效
	Validate(ctx context.Context, jobID JobID) error

	// GetRunningTaskRun 获取Job当前正在运行的TaskRun ID（如果有）
	GetRunningTaskRun(ctx context.Context, jobID JobID) (TaskRunID, bool, error)
}

// TaskExecutor 任务执行器
type TaskExecutor interface {
	// Execute 执行任务（核心执行逻辑）
	// 返回TaskRunSummary和错误
	Execute(ctx context.Context, execCtx *ExecutionContext) (*TaskRunSummary, error)
}

// FileMonitor 文件监控器
type FileMonitor interface {
	// Watch 监控文件变化
	// 返回FileEvent通道和error通道
	// 当ctx取消时停止监控
	// 实现必须在结束时关闭两个通道（eventCh和errCh），否则会导致Executor死锁
	Watch(ctx context.Context, config *JobConfig) (<-chan FileEvent, <-chan error)

	// Scan 执行一次性扫描
	// 返回所有符合条件的文件
	Scan(ctx context.Context, config *JobConfig) ([]FileEvent, error)
}

// SyncPlanner 同步计划器
type SyncPlanner interface {
	// Plan 根据文件事件生成同步计划
	// 处理路径映射、扩展名过滤、URL生成等
	// 实现必须在结束时关闭两个通道（itemCh和errCh），否则会导致Executor死锁
	Plan(ctx context.Context, config *JobConfig, events <-chan FileEvent) (<-chan SyncPlanItem, <-chan error)
}

// StrmGenerator strm文件生成器
type StrmGenerator interface {
	// Apply 执行同步计划（创建/更新/删除strm文件）
	// 返回成功和失败的数量
	Apply(ctx context.Context, items <-chan SyncPlanItem) (succeeded int, failed int, err error)
}

// TaskRunService TaskRun记录管理服务
type TaskRunService interface {
	// Start 创建并开始TaskRun
	Start(ctx context.Context, jobID JobID) (TaskRunID, error)

	// UpdateProgress 更新进度（可选，用于长时间运行的任务）
	UpdateProgress(ctx context.Context, taskRunID TaskRunID, processed int, total int) error

	// Complete 标记TaskRun完成（成功）
	Complete(ctx context.Context, taskRunID TaskRunID, summary *TaskRunSummary) error

	// Fail 标记TaskRun失败
	Fail(ctx context.Context, taskRunID TaskRunID, err error) error

	// Cancel 标记TaskRun被取消
	Cancel(ctx context.Context, taskRunID TaskRunID) error
}



// LogService 日志服务接口
type LogService interface {
	// Info 记录信息日志
	Info(ctx context.Context, jobID JobID, taskRunID TaskRunID, component, message string)

	// Warn 记录警告日志
	Warn(ctx context.Context, jobID JobID, taskRunID TaskRunID, component, message string)

	// Error 记录错误日志
	Error(ctx context.Context, jobID JobID, taskRunID TaskRunID, component, message string, err error)
}
