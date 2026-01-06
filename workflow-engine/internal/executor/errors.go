package executor

import (
	"fmt"
	"time"
)

// ExecutorError 表示执行器操作期间的错误。
type ExecutorError struct {
	Code    ErrorCode
	Message string
	StepID  string
	Cause   error
}

// ErrorCode 表示执行器错误的类型。
type ErrorCode string

const (
	// ErrCodeNotFound 表示未找到执行器。
	ErrCodeNotFound ErrorCode = "EXECUTOR_NOT_FOUND"
	// ErrCodeExecution 表示执行错误。
	ErrCodeExecution ErrorCode = "EXECUTION_ERROR"
	// ErrCodeTimeout 表示超时错误。
	ErrCodeTimeout ErrorCode = "TIMEOUT_ERROR"
	// ErrCodeConfig 表示配置错误。
	ErrCodeConfig ErrorCode = "CONFIG_ERROR"
	// ErrCodeInit 表示初始化错误。
	ErrCodeInit ErrorCode = "INIT_ERROR"
)

// Error 实现 error 接口。
func (e *ExecutorError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 返回底层错误。
func (e *ExecutorError) Unwrap() error {
	return e.Cause
}

// NewExecutorError 创建一个新的 ExecutorError。
func NewExecutorError(code ErrorCode, message string, cause error) *ExecutorError {
	return &ExecutorError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// NewExecutorNotFoundError 创建执行器未找到的错误。
func NewExecutorNotFoundError(execType string) *ExecutorError {
	return &ExecutorError{
		Code:    ErrCodeNotFound,
		Message: fmt.Sprintf("no executor registered for type: %s", execType),
	}
}

// NewExecutionError 创建执行失败的错误。
func NewExecutionError(stepID, message string, cause error) *ExecutorError {
	return &ExecutorError{
		Code:    ErrCodeExecution,
		Message: message,
		StepID:  stepID,
		Cause:   cause,
	}
}

// NewTimeoutError 创建超时错误。
func NewTimeoutError(stepID string, timeout time.Duration) *ExecutorError {
	return &ExecutorError{
		Code:    ErrCodeTimeout,
		Message: fmt.Sprintf("step execution timed out after %v", timeout),
		StepID:  stepID,
	}
}

// NewConfigError 创建配置问题的错误。
func NewConfigError(message string, cause error) *ExecutorError {
	return &ExecutorError{
		Code:    ErrCodeConfig,
		Message: message,
		Cause:   cause,
	}
}

// NewInitError 创建初始化失败的错误。
func NewInitError(execType, message string, cause error) *ExecutorError {
	return &ExecutorError{
		Code:    ErrCodeInit,
		Message: fmt.Sprintf("failed to initialize executor %s: %s", execType, message),
		Cause:   cause,
	}
}

// IsNotFoundError 检查错误是否为执行器未找到错误。
func IsNotFoundError(err error) bool {
	if execErr, ok := err.(*ExecutorError); ok {
		return execErr.Code == ErrCodeNotFound
	}
	return false
}

// IsTimeoutError 检查错误是否为超时错误。
func IsTimeoutError(err error) bool {
	if execErr, ok := err.(*ExecutorError); ok {
		return execErr.Code == ErrCodeTimeout
	}
	return false
}

// IsExecutionError 检查错误是否为执行错误。
func IsExecutionError(err error) bool {
	if execErr, ok := err.(*ExecutorError); ok {
		return execErr.Code == ErrCodeExecution
	}
	return false
}
