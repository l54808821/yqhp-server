package executor

import (
	"fmt"
	"time"
)

// ExecutorError represents an error during executor operations.
type ExecutorError struct {
	Code    ErrorCode
	Message string
	StepID  string
	Cause   error
}

// ErrorCode represents the type of executor error.
type ErrorCode string

const (
	// ErrCodeNotFound indicates the executor was not found.
	ErrCodeNotFound ErrorCode = "EXECUTOR_NOT_FOUND"
	// ErrCodeExecution indicates an execution error.
	ErrCodeExecution ErrorCode = "EXECUTION_ERROR"
	// ErrCodeTimeout indicates a timeout error.
	ErrCodeTimeout ErrorCode = "TIMEOUT_ERROR"
	// ErrCodeConfig indicates a configuration error.
	ErrCodeConfig ErrorCode = "CONFIG_ERROR"
	// ErrCodeInit indicates an initialization error.
	ErrCodeInit ErrorCode = "INIT_ERROR"
)

// Error implements the error interface.
func (e *ExecutorError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *ExecutorError) Unwrap() error {
	return e.Cause
}

// NewExecutorError creates a new ExecutorError.
func NewExecutorError(code ErrorCode, message string, cause error) *ExecutorError {
	return &ExecutorError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// NewExecutorNotFoundError creates an error for missing executor.
func NewExecutorNotFoundError(execType string) *ExecutorError {
	return &ExecutorError{
		Code:    ErrCodeNotFound,
		Message: fmt.Sprintf("no executor registered for type: %s", execType),
	}
}

// NewExecutionError creates an error for execution failures.
func NewExecutionError(stepID, message string, cause error) *ExecutorError {
	return &ExecutorError{
		Code:    ErrCodeExecution,
		Message: message,
		StepID:  stepID,
		Cause:   cause,
	}
}

// NewTimeoutError creates an error for timeout.
func NewTimeoutError(stepID string, timeout time.Duration) *ExecutorError {
	return &ExecutorError{
		Code:    ErrCodeTimeout,
		Message: fmt.Sprintf("step execution timed out after %v", timeout),
		StepID:  stepID,
	}
}

// NewConfigError creates an error for configuration issues.
func NewConfigError(message string, cause error) *ExecutorError {
	return &ExecutorError{
		Code:    ErrCodeConfig,
		Message: message,
		Cause:   cause,
	}
}

// NewInitError creates an error for initialization failures.
func NewInitError(execType, message string, cause error) *ExecutorError {
	return &ExecutorError{
		Code:    ErrCodeInit,
		Message: fmt.Sprintf("failed to initialize executor %s: %s", execType, message),
		Cause:   cause,
	}
}

// IsNotFoundError checks if the error is an executor not found error.
func IsNotFoundError(err error) bool {
	if execErr, ok := err.(*ExecutorError); ok {
		return execErr.Code == ErrCodeNotFound
	}
	return false
}

// IsTimeoutError checks if the error is a timeout error.
func IsTimeoutError(err error) bool {
	if execErr, ok := err.(*ExecutorError); ok {
		return execErr.Code == ErrCodeTimeout
	}
	return false
}

// IsExecutionError checks if the error is an execution error.
func IsExecutionError(err error) bool {
	if execErr, ok := err.(*ExecutorError); ok {
		return execErr.Code == ErrCodeExecution
	}
	return false
}
