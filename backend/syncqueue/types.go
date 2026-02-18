// Package syncqueue 提供任务队列管理功能
//
// 这个包实现了一个基于数据库的任务队列系统，支持：
// - 任务优先级调度
// - 任务状态管理（状态机）
// - 任务重试机制
// - 失败分类
// - 并发安全的任务领取
package syncqueue

import "fmt"

// TaskStatus 任务状态
//
// 状态转移规则：
// - Pending → Running（领取任务）
// - Pending → Cancelled（取消待执行任务）
// - Running → Completed（执行成功）
// - Running → Failed（执行失败，不可重试或超过最大重试次数）
// - Running → Pending（执行失败，可重试且未超过最大重试次数）
// - Running → Cancelled（取消正在执行的任务）
// - Failed → Pending（手动重试）
// - Completed/Cancelled → 不可转移（终态）
type TaskStatus string

const (
	// TaskPending 待执行状态
	// 任务已入队，等待被 Worker 领取
	TaskPending TaskStatus = "pending"

	// TaskRunning 执行中状态
	// 任务已被 Worker 领取，正在执行
	TaskRunning TaskStatus = "running"

	// TaskCompleted 已完成状态（终态）
	// 任务执行成功
	TaskCompleted TaskStatus = "completed"

	// TaskFailed 失败状态（终态或可重试）
	// 任务执行失败，根据失败类型和重试次数决定是否重试
	TaskFailed TaskStatus = "failed"

	// TaskCancelled 已取消状态（终态）
	// 任务被用户或系统取消
	TaskCancelled TaskStatus = "cancelled"
)

// String 返回状态的字符串表示
func (s TaskStatus) String() string {
	return string(s)
}

// CanTransitionTo 检查是否可以转移到目标状态
//
// 这是状态机的核心方法，确保状态转移的合法性。
// 所有状态变更操作都应该先调用此方法进行检查。
//
// 参数：
//   - next: 目标状态
//
// 返回：
//   - true 表示可以转移，false 表示不允许转移
func (s TaskStatus) CanTransitionTo(next TaskStatus) bool {
	switch s {
	case TaskPending:
		// 待执行任务可以被领取或取消
		return next == TaskRunning || next == TaskCancelled

	case TaskRunning:
		// 执行中的任务可以完成、失败、取消或重试（转回 Pending）
		return next == TaskCompleted ||
			next == TaskFailed ||
			next == TaskCancelled ||
			next == TaskPending // 允许重试

	case TaskFailed:
		// 失败的任务可以手动重试（转回 Running 或 Pending）
		return next == TaskRunning || next == TaskPending

	case TaskCompleted, TaskCancelled:
		// 终态，不可转移
		return false

	default:
		// 未知状态，不允许转移
		return false
	}
}

// TaskPriority 任务优先级
//
// 优先级数值越小，优先级越高。
// 在队列调度时，优先级高的任务会被优先领取。
type TaskPriority int

const (
	// TaskPriorityHigh 高优先级
	// 用于紧急任务或重要任务
	TaskPriorityHigh TaskPriority = 1

	// TaskPriorityNormal 普通优先级（默认）
	// 用于常规任务
	TaskPriorityNormal TaskPriority = 2

	// TaskPriorityLow 低优先级
	// 用于非紧急任务或后台任务
	TaskPriorityLow TaskPriority = 3
)

// FailureKind 失败类型
//
// 用于分类任务失败的原因，决定重试策略：
// - Retryable: 临时失败，可以重试（如网络超时、临时IO错误）
// - Permanent: 永久失败，不应重试（如配置错误、权限不足）
// - Cancelled: 任务被取消（如用户取消、超时取消）
type FailureKind string

const (
	// FailureRetryable 可重试的失败
	// 表示失败是临时的，重试可能会成功
	// 例如：网络超时、临时IO错误、数据库锁竞争
	FailureRetryable FailureKind = "retryable"

	// FailurePermanent 永久性失败
	// 表示失败是持久的，重试不会改变结果
	// 例如：配置错误、权限不足、文件不存在、数据格式错误
	FailurePermanent FailureKind = "permanent"

	// FailureCancelled 任务被取消
	// 表示任务被用户或系统主动取消
	// 例如：用户取消、context.Canceled、超时
	FailureCancelled FailureKind = "cancelled"
)

// String 返回失败类型的字符串表示
func (f FailureKind) String() string {
	return string(f)
}

// TaskError 任务错误封装
//
// 将错误与失败类型关联，便于错误处理和重试决策。
// 使用 errors.As 可以从错误链中提取 TaskError。
//
// 使用示例：
//
//	err := &TaskError{
//	    Kind: FailureRetryable,
//	    Err:  errors.New("network timeout"),
//	}
//	return err
type TaskError struct {
	// Kind 失败类型
	Kind FailureKind

	// Err 原始错误
	Err error
}

// Error 实现 error 接口
func (e *TaskError) Error() string {
	if e == nil || e.Err == nil {
		return "task error"
	}
	return fmt.Sprintf("[%s] %s", e.Kind, e.Err.Error())
}

// Unwrap 实现 errors.Unwrap 接口
//
// 支持 errors.Is 和 errors.As 的错误链处理
func (e *TaskError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
