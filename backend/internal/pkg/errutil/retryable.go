package errutil

// retryableError 用于显式标注可重试错误语义
// 实现 IsRetryable 以便上层通过 errors.As 识别
type retryableError struct {
	err error
}

func (e retryableError) Error() string {
	if e.err == nil {
		return "retryable error"
	}
	return e.err.Error()
}

func (e retryableError) Unwrap() error {
	return e.err
}

func (e retryableError) IsRetryable() bool {
	return true
}

// Retryable 包装错误为可重试错误
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return retryableError{err: err}
}
