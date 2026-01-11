package sse

// ErrorCode 错误码常量
type ErrorCode string

const (
	// 工作流相关错误
	ErrConversionError ErrorCode = "CONVERSION_ERROR" // 工作流转换失败
	ErrExecutorError   ErrorCode = "EXECUTOR_ERROR"   // 执行器初始化失败

	// 远程 Slave 相关错误
	ErrSlaveConnectionError ErrorCode = "SLAVE_CONNECTION_ERROR" // 远程 Slave 连接失败
	ErrSlaveUnavailable     ErrorCode = "SLAVE_UNAVAILABLE"      // 远程 Slave 不可用

	// AI 相关错误
	ErrAIError   ErrorCode = "AI_ERROR"   // AI 节点调用失败
	ErrAITimeout ErrorCode = "AI_TIMEOUT" // AI 调用超时

	// 交互相关错误
	ErrInteractionTimeout ErrorCode = "INTERACTION_TIMEOUT" // 交互超时

	// 会话相关错误
	ErrSessionConflict ErrorCode = "SESSION_CONFLICT"  // 会话冲突
	ErrSessionNotFound ErrorCode = "SESSION_NOT_FOUND" // 会话不存在
	ErrSessionClosed   ErrorCode = "SESSION_CLOSED"    // 会话已关闭

	// 执行相关错误
	ErrTimeout   ErrorCode = "TIMEOUT"   // 执行超时
	ErrCancelled ErrorCode = "CANCELLED" // 用户取消

	// 通用错误
	ErrInternalError ErrorCode = "INTERNAL_ERROR" // 内部错误
	ErrInvalidInput  ErrorCode = "INVALID_INPUT"  // 无效输入
)

// IsRecoverable 判断错误是否可恢复
func (c ErrorCode) IsRecoverable() bool {
	switch c {
	case ErrSlaveConnectionError, ErrSlaveUnavailable, ErrAIError, ErrAITimeout,
		ErrInteractionTimeout, ErrSessionConflict:
		return true
	default:
		return false
	}
}

// String 返回错误码字符串
func (c ErrorCode) String() string {
	return string(c)
}

// NewError 创建错误数据
func NewError(code ErrorCode, message string, details string) *ErrorData {
	return &ErrorData{
		Code:        code.String(),
		Message:     message,
		Details:     details,
		Recoverable: code.IsRecoverable(),
	}
}

// NewErrorWithRecoverable 创建错误数据（自定义可恢复性）
func NewErrorWithRecoverable(code ErrorCode, message string, details string, recoverable bool) *ErrorData {
	return &ErrorData{
		Code:        code.String(),
		Message:     message,
		Details:     details,
		Recoverable: recoverable,
	}
}
