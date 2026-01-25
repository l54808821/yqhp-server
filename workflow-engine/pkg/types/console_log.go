package types

// ConsoleLogType 日志类型
type ConsoleLogType string

const (
	// LogTypeLog 普通日志
	LogTypeLog ConsoleLogType = "log"
	// LogTypeWarn 警告日志
	LogTypeWarn ConsoleLogType = "warn"
	// LogTypeError 错误日志
	LogTypeError ConsoleLogType = "error"
	// LogTypeProcessor 处理器执行日志
	LogTypeProcessor ConsoleLogType = "processor"
)

// ConsoleLogEntry 统一的控制台日志条目
// 用于收集执行过程中的所有日志输出，包括处理器执行结果和脚本日志
type ConsoleLogEntry struct {
	// Type 日志类型：log/warn/error/processor
	Type ConsoleLogType `json:"type"`
	// Message 日志消息（log/warn/error 类型时使用）
	Message string `json:"message,omitempty"`
	// Timestamp 时间戳（毫秒）
	Timestamp int64 `json:"ts,omitempty"`
	// Processor 处理器详情（type=processor 时有值）
	Processor *ProcessorLogInfo `json:"processor,omitempty"`
}

// ProcessorLogInfo 处理器执行日志详情
type ProcessorLogInfo struct {
	// ID 处理器 ID
	ID string `json:"id"`
	// Phase 执行阶段：pre（前置）或 post（后置）
	Phase string `json:"phase"`
	// Type 处理器类型：js_script/set_variable/wait/assertion/extract_param 等
	Type string `json:"procType"`
	// Name 处理器名称
	Name string `json:"name,omitempty"`
	// Success 是否执行成功
	Success bool `json:"success"`
	// Message 执行结果消息
	Message string `json:"message,omitempty"`
	// Output 输出数据（如设置的变量值等）
	Output map[string]any `json:"output,omitempty"`
}

// NewLogEntry 创建普通日志条目
func NewLogEntry(message string) ConsoleLogEntry {
	return ConsoleLogEntry{
		Type:    LogTypeLog,
		Message: message,
	}
}

// NewWarnEntry 创建警告日志条目
func NewWarnEntry(message string) ConsoleLogEntry {
	return ConsoleLogEntry{
		Type:    LogTypeWarn,
		Message: message,
	}
}

// NewErrorEntry 创建错误日志条目
func NewErrorEntry(message string) ConsoleLogEntry {
	return ConsoleLogEntry{
		Type:    LogTypeError,
		Message: message,
	}
}

// NewProcessorEntry 创建处理器日志条目
func NewProcessorEntry(phase string, info ProcessorLogInfo) ConsoleLogEntry {
	info.Phase = phase
	return ConsoleLogEntry{
		Type:      LogTypeProcessor,
		Processor: &info,
	}
}
