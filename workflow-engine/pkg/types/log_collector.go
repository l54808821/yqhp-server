package types

import (
	"sync"
	"time"
)

// LogCollector 日志收集器
// 用于统一收集执行过程中的所有日志，包括普通日志、处理器日志、变量变更日志等
// 线程安全，可在多个执行器中共享使用
type LogCollector struct {
	mu      sync.RWMutex
	logs    []ConsoleLogEntry
	envVars map[string]struct{} // 标记哪些变量是环境变量
}

// NewLogCollector 创建日志收集器
func NewLogCollector() *LogCollector {
	return &LogCollector{
		logs:    make([]ConsoleLogEntry, 0),
		envVars: make(map[string]struct{}),
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

// MarkAsEnvVar 标记某个变量为环境变量
func (c *LogCollector) MarkAsEnvVar(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.envVars[name] = struct{}{}
}

// UnmarkEnvVar 取消环境变量标记
func (c *LogCollector) UnmarkEnvVar(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.envVars, name)
}

// IsEnvVar 检查是否是环境变量
func (c *LogCollector) IsEnvVar(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.envVars[name]
	return ok
}

// GetEnvVarNames 获取所有环境变量名称
func (c *LogCollector) GetEnvVarNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, 0, len(c.envVars))
	for name := range c.envVars {
		names = append(names, name)
	}
	return names
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

// CreateSnapshot 创建变量快照
func (c *LogCollector) CreateSnapshot(variables map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	envVars := make(map[string]any)
	tempVars := make(map[string]any)

	for k, v := range variables {
		if _, isEnv := c.envVars[k]; isEnv {
			envVars[k] = v
		} else {
			tempVars[k] = v
		}
	}

	c.logs = append(c.logs, NewSnapshotEntry(VariableSnapshotInfo{
		EnvVars:  envVars,
		TempVars: tempVars,
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

// Clear 清空所有日志和环境变量标记
func (c *LogCollector) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logs = make([]ConsoleLogEntry, 0)
	c.envVars = make(map[string]struct{})
}

// Clone 克隆日志收集器（用于子上下文）
func (c *LogCollector) Clone() *LogCollector {
	c.mu.RLock()
	defer c.mu.RUnlock()

	newCollector := &LogCollector{
		logs:    make([]ConsoleLogEntry, 0), // 新收集器从空开始
		envVars: make(map[string]struct{}, len(c.envVars)),
	}

	// 复制环境变量标记
	for k := range c.envVars {
		newCollector.envVars[k] = struct{}{}
	}

	return newCollector
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
