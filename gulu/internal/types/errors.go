package types

import "fmt"

// ErrorCode 错误码
type ErrorCode string

const (
	// 通用错误码
	ErrCodeUnknown          ErrorCode = "UNKNOWN_ERROR"
	ErrCodeInvalidParameter ErrorCode = "INVALID_PARAMETER"
	ErrCodeNotFound         ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized     ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden        ErrorCode = "FORBIDDEN"

	// 工作流相关错误码
	ErrCodeWorkflowNotFound      ErrorCode = "WORKFLOW_NOT_FOUND"
	ErrCodeWorkflowInvalid       ErrorCode = "WORKFLOW_INVALID"
	ErrCodeWorkflowTypeInvalid   ErrorCode = "WORKFLOW_TYPE_INVALID"
	ErrCodeWorkflowParseError    ErrorCode = "WORKFLOW_PARSE_ERROR"
	ErrCodeWorkflowValidateError ErrorCode = "WORKFLOW_VALIDATE_ERROR"

	// 执行相关错误码
	ErrCodeExecutionNotFound    ErrorCode = "EXECUTION_NOT_FOUND"
	ErrCodeExecutionFailed      ErrorCode = "EXECUTION_FAILED"
	ErrCodeExecutionTimeout     ErrorCode = "EXECUTION_TIMEOUT"
	ErrCodeExecutionStopped     ErrorCode = "EXECUTION_STOPPED"
	ErrCodeExecutionModeInvalid ErrorCode = "EXECUTION_MODE_INVALID"

	// 调度相关错误码
	ErrCodeSchedulerError      ErrorCode = "SCHEDULER_ERROR"
	ErrCodeExecutorUnavailable ErrorCode = "EXECUTOR_UNAVAILABLE"
	ErrCodeSlaveNotFound       ErrorCode = "SLAVE_NOT_FOUND"
	ErrCodeSlaveUnavailable    ErrorCode = "SLAVE_UNAVAILABLE"

	// WebSocket 相关错误码
	ErrCodeWebSocketError      ErrorCode = "WEBSOCKET_ERROR"
	ErrCodeWebSocketConnFailed ErrorCode = "WEBSOCKET_CONN_FAILED"
	ErrCodeWebSocketSendFailed ErrorCode = "WEBSOCKET_SEND_FAILED"

	// 调试相关错误码
	ErrCodeDebugSessionNotFound ErrorCode = "DEBUG_SESSION_NOT_FOUND"
	ErrCodeDebugSessionExpired  ErrorCode = "DEBUG_SESSION_EXPIRED"
	ErrCodeDebugStartFailed     ErrorCode = "DEBUG_START_FAILED"
	ErrCodeDebugStopFailed      ErrorCode = "DEBUG_STOP_FAILED"

	// 环境相关错误码
	ErrCodeEnvNotFound      ErrorCode = "ENV_NOT_FOUND"
	ErrCodeEnvMismatch      ErrorCode = "ENV_MISMATCH"
	ErrCodeConfigMergeError ErrorCode = "CONFIG_MERGE_ERROR"

	// 步骤执行相关错误码
	ErrCodeStepFailed  ErrorCode = "STEP_FAILED"
	ErrCodeStepTimeout ErrorCode = "STEP_TIMEOUT"
	ErrCodeStepSkipped ErrorCode = "STEP_SKIPPED"
)

// AppError 应用错误
type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
	Cause   error     `json:"-"`
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 返回原始错误
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewAppError 创建应用错误
func NewAppError(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// NewAppErrorWithDetails 创建带详情的应用错误
func NewAppErrorWithDetails(code ErrorCode, message, details string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// NewAppErrorWithCause 创建带原因的应用错误
func NewAppErrorWithCause(code ErrorCode, message string, cause error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Details: cause.Error(),
		Cause:   cause,
	}
}

// 预定义错误
var (
	ErrWorkflowNotFound    = NewAppError(ErrCodeWorkflowNotFound, "工作流不存在")
	ErrWorkflowInvalid     = NewAppError(ErrCodeWorkflowInvalid, "工作流定义无效")
	ErrWorkflowTypeInvalid = NewAppError(ErrCodeWorkflowTypeInvalid, "无效的工作流类型")

	ErrExecutionNotFound    = NewAppError(ErrCodeExecutionNotFound, "执行记录不存在")
	ErrExecutionFailed      = NewAppError(ErrCodeExecutionFailed, "执行失败")
	ErrExecutionTimeout     = NewAppError(ErrCodeExecutionTimeout, "执行超时")
	ErrExecutionModeInvalid = NewAppError(ErrCodeExecutionModeInvalid, "无效的执行模式")

	ErrExecutorUnavailable = NewAppError(ErrCodeExecutorUnavailable, "执行器不可用")
	ErrSlaveNotFound       = NewAppError(ErrCodeSlaveNotFound, "Slave 节点不存在")
	ErrSlaveUnavailable    = NewAppError(ErrCodeSlaveUnavailable, "Slave 节点不可用")

	ErrDebugSessionNotFound = NewAppError(ErrCodeDebugSessionNotFound, "调试会话不存在")
	ErrDebugSessionExpired  = NewAppError(ErrCodeDebugSessionExpired, "调试会话已过期")

	ErrEnvNotFound = NewAppError(ErrCodeEnvNotFound, "环境不存在")
	ErrEnvMismatch = NewAppError(ErrCodeEnvMismatch, "环境与工作流不属于同一项目")
)

// IsAppError 检查是否为应用错误
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetErrorCode 获取错误码
func GetErrorCode(err error) ErrorCode {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	return ErrCodeUnknown
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ToErrorResponse 转换为错误响应
func (e *AppError) ToErrorResponse() *ErrorResponse {
	return &ErrorResponse{
		Code:    string(e.Code),
		Message: e.Message,
		Details: e.Details,
	}
}
