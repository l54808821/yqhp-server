package types

import (
	"sync"
	"time"
)

// LogCollector 日志收集器
// 用于统一收集执行过程中的所有日志，包括普通日志、处理器日志、变量变更日志等
// 线程安全，可在多个执行器中共享使用
type LogCollector struct {
	mu   sync.RWMutex
	logs []ConsoleLogEntry
}

// NewLogCollector 创建日志收集器
func NewLogCollector() *LogCollector {
	return &LogCollector{
		logs: make([]ConsoleLogEntry, 0),
	}
}

// AppendLog 添加单条日志
func (c *LogCollector) AppendLog(entry ConsoleLogEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry.Timestamp == 0 {
		entry.Timestamp = time.Now().UnixMilli()
	}
	c.logs = append(c.logs, entry)
}

// AppendLogs 批量添加日志
func (c *LogCollector) AppendLogs(entries []ConsoleLogEntry) {
	if len(entries) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logs = append(c.logs, entries...)
}

// RecordVariableChange 记录变量变更
func (c *LogCollector) RecordVariableChange(name string, oldValue, newValue any, scope, source string) {
	c.AppendLog(NewVariableChangeEntry(VariableChangeInfo{
		Name:     name,
		OldValue: oldValue,
		NewValue: newValue,
		Scope:    scope,
		Source:   source,
	}))
}

// FlushLogs 获取并清空所有日志
func (c *LogCollector) FlushLogs() []ConsoleLogEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	logs := c.logs
	c.logs = make([]ConsoleLogEntry, 0)
	return logs
}

// GetLogs 获取所有日志（不清空）
func (c *LogCollector) GetLogs() []ConsoleLogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]ConsoleLogEntry, len(c.logs))
	copy(result, c.logs)
	return result
}

// LogCount 获取日志数量
func (c *LogCollector) LogCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.logs)
}

// Clear 清空所有日志
func (c *LogCollector) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logs = make([]ConsoleLogEntry, 0)
}

// Clone 克隆日志收集器（用于子上下文）
func (c *LogCollector) Clone() *LogCollector {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &LogCollector{
		logs: make([]ConsoleLogEntry, 0),
	}
}

// MergeFrom 从另一个收集器合并日志
func (c *LogCollector) MergeFrom(other *LogCollector) {
	if other == nil {
		return
	}

	other.mu.RLock()
	logs := make([]ConsoleLogEntry, len(other.logs))
	copy(logs, other.logs)
	other.mu.RUnlock()

	c.AppendLogs(logs)
}
