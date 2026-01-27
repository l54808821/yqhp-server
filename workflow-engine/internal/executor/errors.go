// Package executor 提供工作流步骤执行的执行器框架。
//
// # 错误处理规范
//
// 执行器的 Execute 方法应遵循以下错误处理规范：
//
// ## 返回模式
//
// Execute 方法签名: Execute(ctx, step, execCtx) (*StepResult, error)
//
// 1. **正常执行（成功或业务失败）**：返回 (StepResult, nil)
//   - 步骤执行成功：返回 CreateSuccessResult(...)
//   - 步骤执行失败（如断言失败、HTTP 错误）：返回 CreateFailedResult(...)
//   - 步骤超时：返回 CreateTimeoutResult(...)
//   - 步骤跳过：返回 CreateSkippedResult(...)
//
// 2. **系统级错误**：返回 (nil, error) 或 (partialResult, error)
//   - 仅在以下情况返回 error：
//     - 上下文取消（context.Canceled）
//     - 需要中断整个工作流的严重错误
//   - 一般的执行错误应封装在 StepResult.Error 中
//
// ## 内部方法
//
// 执行器的内部方法（如 parseConfig, buildRequest 等）可以返回 error，
// 但在 Execute 方法中应将其转换为 StepResult：
//
//	config, err := e.parseConfig(step.Config)
//	if err != nil {
//	    return CreateFailedResult(step.ID, startTime, err), nil  // 转换为 FailedResult
//	}
//
// ## 错误类型
//
// 使用预定义的错误构造函数创建错误：
//   - NewConfigError: 配置解析错误
//   - NewExecutionError: 执行过程错误
//   - NewTimeoutError: 超时错误
//   - NewInitError: 初始化错误
package executor

import (
	"context"
	"fmt"
	"time"

	"yqhp/workflow-engine/pkg/types"
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
// 返回用户友好的错误消息，不包含技术性的错误码前缀。
func (e *ExecutorError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// DetailedError 返回包含错误码的详细错误信息，用于日志和调试。
func (e *ExecutorError) DetailedError() string {
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
		Message: fmt.Sprintf("未找到类型为 '%s' 的执行器", execType),
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
		Message: fmt.Sprintf("步骤执行超时，超时时间: %v", timeout),
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
		Message: fmt.Sprintf("初始化执行器 '%s' 失败: %s", execType, message),
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

// ========== 错误转换辅助方法 ==========

// WrapErrorAsResult 将错误转换为失败的 StepResult。
// 这是推荐的错误处理方式，用于将内部错误统一转换为 StepResult。
//
// 使用示例:
//
//	config, err := e.parseConfig(step.Config)
//	if err != nil {
//	    return WrapErrorAsResult(step.ID, startTime, err)
//	}
func WrapErrorAsResult(stepID string, startTime time.Time, err error) (*types.StepResult, error) {
	return CreateFailedResult(stepID, startTime, err), nil
}

// WrapTimeoutAsResult 将超时转换为超时的 StepResult。
func WrapTimeoutAsResult(stepID string, startTime time.Time, timeout time.Duration) (*types.StepResult, error) {
	return CreateTimeoutResult(stepID, startTime, timeout), nil
}

// ShouldPropagateError 判断错误是否需要向上传播（而不是封装到 StepResult）。
// 以下情况需要传播错误：
//   - context.Canceled: 用户主动取消
//   - context.DeadlineExceeded: 上下文超时（注意：步骤级超时应转为 TimeoutResult）
//   - 标记了 propagate 的自定义错误
func ShouldPropagateError(err error) bool {
	if err == nil {
		return false
	}
	// 上下文取消需要传播
	if err == context.Canceled {
		return true
	}
	// 上下文超时在某些场景需要传播
	if err == context.DeadlineExceeded {
		return true
	}
	return false
}

// HandleExecuteError 统一处理 Execute 方法中的错误。
// 根据错误类型决定是封装为 StepResult 还是向上传播。
//
// 使用示例:
//
//	result, err := e.doExecute(ctx, step, execCtx)
//	if err != nil {
//	    return HandleExecuteError(step.ID, startTime, err)
//	}
func HandleExecuteError(stepID string, startTime time.Time, err error) (*types.StepResult, error) {
	if ShouldPropagateError(err) {
		// 需要传播的错误，返回部分结果和错误
		return CreateFailedResult(stepID, startTime, err), err
	}
	// 其他错误封装为 FailedResult
	return CreateFailedResult(stepID, startTime, err), nil
}
