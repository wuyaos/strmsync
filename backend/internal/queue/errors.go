package syncqueue

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"syscall"
)

// classifyError 分类错误类型
//
// 根据错误的性质判断是否应该重试。
// 这是重试策略的核心逻辑。
//
// 分类规则：
// - 如果是 TaskError，直接使用其 Kind
// - 如果是 context.Canceled 或 context.DeadlineExceeded，视为 Cancelled
// - 如果是网络错误或临时IO错误，视为 Retryable
// - 其他错误视为 Permanent
//
// 参数：
//   - err: 要分类的错误
//
// 返回：
//   - FailureKind: 失败类型
func classifyError(err error) FailureKind {
	if err == nil {
		return FailurePermanent
	}

	// 如果已经是 TaskError，直接使用其 Kind
	var taskErr *TaskError
	if errors.As(err, &taskErr) && taskErr.Kind != "" {
		return taskErr.Kind
	}

	// Context 取消或超时
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return FailureCancelled
	}

	// 如果驱动层显式标注可重试性，优先使用
	var retryable RetryableError
	if errors.As(err, &retryable) {
		if retryable.IsRetryable() {
			return FailureRetryable
		}
		return FailurePermanent
	}

	// 网络错误或临时IO错误
	if isNetworkError(err) || isTransientIO(err) {
		return FailureRetryable
	}

	// 其他错误视为永久性失败
	return FailurePermanent
}

// ClassifyFailureKind 对外暴露错误分类结果
// 用于在上层统一复用队列的错误语义判断
func ClassifyFailureKind(err error) FailureKind {
	return classifyError(err)
}

// RetryableError 表示可显式标注重试语义的错误
//
// 驱动层应通过实现该接口来声明错误是否可重试。
type RetryableError interface {
	IsRetryable() bool
}

// isTransientIO 判断是否是临时IO错误
//
// 临时IO错误通常是由于系统资源暂时不可用导致的，
// 重试后可能会成功。
//
// 检查的错误类型：
// - io.ErrUnexpectedEOF: 意外的EOF
// - syscall.EAGAIN: 资源暂时不可用
// - syscall.EINTR: 系统调用被中断
//
// 参数：
//   - err: 要检查的错误
//
// 返回：
//   - true 表示是临时IO错误，false 表示不是
func isTransientIO(err error) bool {
	if err == nil {
		return false
	}

	// 意外的EOF（可能是网络断开或文件被截断）
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	// EAGAIN (资源暂时不可用) 或 EINTR (系统调用被中断)
	if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EINTR) {
		return true
	}

	// 检查 os.PathError 包装的系统调用错误
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if errors.Is(pathErr.Err, syscall.EAGAIN) || errors.Is(pathErr.Err, syscall.EINTR) {
			return true
		}
	}

	return false
}

// isNetworkError 判断是否是网络错误
//
// 网络错误通常是由于网络不稳定导致的，
// 重试后可能会成功。
//
// 检查的错误类型：
// - net.Error 的 Timeout 或 Temporary
// - syscall.ECONNRESET: 连接被重置
// - syscall.ECONNREFUSED: 连接被拒绝
// - syscall.ETIMEDOUT: 连接超时
// - syscall.EHOSTUNREACH: 主机不可达
// - syscall.ENETUNREACH: 网络不可达
//
// 参数：
//   - err: 要检查的错误
//
// 返回：
//   - true 表示是网络错误，false 表示不是
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否是 net.Error 接口
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Timeout 或 Temporary 错误视为可重试
		if netErr.Timeout() || netErr.Temporary() {
			return true
		}
	}

	// 检查常见的网络相关系统调用错误
	if errors.Is(err, syscall.ECONNRESET) || // 连接被重置
		errors.Is(err, syscall.ECONNREFUSED) || // 连接被拒绝
		errors.Is(err, syscall.ETIMEDOUT) || // 连接超时
		errors.Is(err, syscall.EHOSTUNREACH) || // 主机不可达
		errors.Is(err, syscall.ENETUNREACH) { // 网络不可达
		return true
	}

	return false
}
