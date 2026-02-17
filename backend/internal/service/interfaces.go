// Package service 定义Service层的核心接口
package service

import (
	"context"

	"github.com/strmsync/strmsync/internal/service/types"
)

// JobService Job业务逻辑服务
type JobService interface {
	// Run 运行任务（创建TaskRun并启动执行）
	Run(ctx context.Context, jobID types.JobID) (types.TaskRunID, error)

	// Stop 停止任务（取消正在运行的TaskRun）
	Stop(ctx context.Context, jobID types.JobID) error

	// Validate 验证Job配置是否有效
	Validate(ctx context.Context, jobID types.JobID) error

	// GetRunningTaskRun 获取Job当前正在运行的TaskRun ID（如果有）
	GetRunningTaskRun(ctx context.Context, jobID types.JobID) (types.TaskRunID, bool, error)
}

// TaskExecutor 任务执行器
type TaskExecutor interface {
	// Execute 执行任务（核心执行逻辑）
	// 返回TaskRunSummary和错误
	Execute(ctx context.Context, execCtx *types.ExecutionContext) (*types.TaskRunSummary, error)
}

// FileMonitor 文件监控器
type FileMonitor interface {
	// Watch 监控文件变化
	// 返回FileEvent通道和error通道
	// 当ctx取消时停止监控
	// 实现必须在结束时关闭两个通道（eventCh和errCh），否则会导致Executor死锁
	Watch(ctx context.Context, config *types.JobConfig) (<-chan types.FileEvent, <-chan error)

	// Scan 执行一次性扫描
	// 返回所有符合条件的文件
	Scan(ctx context.Context, config *types.JobConfig) ([]types.FileEvent, error)
}

// SyncPlanner 同步计划器
type SyncPlanner interface {
	// Plan 根据文件事件生成同步计划
	// 处理路径映射、扩展名过滤、URL生成等
	// 实现必须在结束时关闭两个通道（itemCh和errCh），否则会导致Executor死锁
	Plan(ctx context.Context, config *types.JobConfig, events <-chan types.FileEvent) (<-chan types.SyncPlanItem, <-chan error)
}

// StrmGenerator strm文件生成器
type StrmGenerator interface {
	// Apply 执行同步计划（创建/更新/删除strm文件）
	// 返回成功和失败的数量
	Apply(ctx context.Context, items <-chan types.SyncPlanItem) (succeeded int, failed int, err error)
}

// TaskRunService TaskRun记录管理服务
type TaskRunService interface {
	// Start 创建并开始TaskRun
	Start(ctx context.Context, jobID types.JobID) (types.TaskRunID, error)

	// UpdateProgress 更新进度（可选，用于长时间运行的任务）
	UpdateProgress(ctx context.Context, taskRunID types.TaskRunID, processed int, total int) error

	// Complete 标记TaskRun完成（成功）
	Complete(ctx context.Context, taskRunID types.TaskRunID, summary *types.TaskRunSummary) error

	// Fail 标记TaskRun失败
	Fail(ctx context.Context, taskRunID types.TaskRunID, err error) error

	// Cancel 标记TaskRun被取消
	Cancel(ctx context.Context, taskRunID types.TaskRunID) error
}

// DataServerClient 数据服务器客户端接口
type DataServerClient interface {
	// List 列出目录内容
	List(ctx context.Context, path string, recursive bool) ([]types.RemoteFile, error)

	// Watch 监控目录变化（如果支持）
	Watch(ctx context.Context, path string) (<-chan types.FileEvent, error)

	// BuildStreamURL 构建流媒体URL
	BuildStreamURL(ctx context.Context, serverID types.DataServerID, filePath string) (string, error)

	// TestConnection 测试连接
	TestConnection(ctx context.Context) error
}

// MediaServerClient 媒体服务器客户端接口
type MediaServerClient interface {
	// Scan 触发媒体库扫描
	Scan(ctx context.Context, libraryPath string) error

	// TestConnection 测试连接
	TestConnection(ctx context.Context) error
}

// LogService 日志服务接口
type LogService interface {
	// Info 记录信息日志
	Info(ctx context.Context, jobID types.JobID, taskRunID types.TaskRunID, component, message string)

	// Warn 记录警告日志
	Warn(ctx context.Context, jobID types.JobID, taskRunID types.TaskRunID, component, message string)

	// Error 记录错误日志
	Error(ctx context.Context, jobID types.JobID, taskRunID types.TaskRunID, component, message string, err error)
}
