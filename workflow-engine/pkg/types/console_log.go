package types

import "time"

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
	// LogTypeVariable 变量变更日志
	LogTypeVariable ConsoleLogType = "variable"
	// LogTypeSnapshot 变量快照日志
	LogTypeSnapshot ConsoleLogType = "snapshot"
)

// VariableChangeInfo 变量变更信息
type VariableChangeInfo struct {
	// Name 变量名
	Name string `json:"name"`
	// OldValue 旧值
	OldValue any `json:"oldValue,omitempty"`
	// NewValue 新值
	NewValue any `json:"newValue"`
	// Scope 作用域：env（环境变量）或 temp（临时变量）
	Scope string `json:"scope"`
	// Source 变更来源：set_variable/extract_param/js_script/loop 等
	Source string `json:"source"`
}

// VariableSnapshotInfo 变量快照信息
type VariableSnapshotInfo struct {
	// EnvVars 环境变量
	EnvVars map[string]any `json:"envVars"`
	// TempVars 临时变量
	TempVars map[string]any `json:"tempVars"`
}

// ConsoleLogEntry 统一的控制台日志条目
// 用于收集执行过程中的所有日志输出，包括处理器执行结果和脚本日志
type ConsoleLogEntry struct {
	// Type 日志类型：log/warn/error/processor/variable/snapshot
	Type ConsoleLogType `json:"type"`
	// Message 日志消息（log/warn/error 类型时使用）
	Message string `json:"message,omitempty"`
	// Timestamp 时间戳（毫秒）
	Timestamp int64 `json:"ts,omitempty"`
	// Processor 处理器详情（type=processor 时有值）
	Processor *ProcessorLogInfo `json:"processor,omitempty"`
	// Variable 变量变更详情（type=variable 时有值）
	Variable *VariableChangeInfo `json:"variable,omitempty"`
	// Snapshot 变量快照详情（type=snapshot 时有值）
	Snapshot *VariableSnapshotInfo `json:"snapshot,omitempty"`
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

// NewVariableChangeEntry 创建变量变更日志条目
func NewVariableChangeEntry(info VariableChangeInfo) ConsoleLogEntry {
	return ConsoleLogEntry{
		Type:      LogTypeVariable,
		Timestamp: time.Now().UnixMilli(),
		Variable:  &info,
	}
}

// NewSnapshotEntry 创建变量快照日志条目
func NewSnapshotEntry(snapshot VariableSnapshotInfo) ConsoleLogEntry {
	return ConsoleLogEntry{
		Type:      LogTypeSnapshot,
		Timestamp: time.Now().UnixMilli(),
		Snapshot:  &snapshot,
	}
}
