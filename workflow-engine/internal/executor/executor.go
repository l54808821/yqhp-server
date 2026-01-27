// Package executor 提供工作流步骤执行的执行器框架。
package executor

import (
	"context"
	"sync"

	"yqhp/workflow-engine/pkg/types"
)

// Executor 定义步骤执行器的接口。
type Executor interface {
	// Type 返回执行器类型标识符。
	Type() string

	// Init 使用配置初始化执行器。
	Init(ctx context.Context, config map[string]any) error

	// Execute 执行步骤并返回结果。
	Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error)

	// Cleanup 释放执行器持有的资源。
	Cleanup(ctx context.Context) error
}

// ExecutionContext 保存步骤执行的运行时状态。
type ExecutionContext struct {
	// Variables 保存执行期间可访问的变量值。
	Variables map[string]any

	// Results 保存按步骤 ID 索引的步骤执行结果。
	Results map[string]*types.StepResult

	// VU 是执行工作流的虚拟用户。
	VU *types.VirtualUser

	// Iteration 是当前迭代次数。
	Iteration int

	// WorkflowID 是正在执行的工作流 ID。
	WorkflowID string

	// ExecutionID 是此次执行的唯一 ID。
	ExecutionID string

	// Callback 执行回调（用于实时通知）
	Callback types.ExecutionCallback

	// ParentStepID 父步骤ID（用于循环等嵌套场景）
	ParentStepID string

	// LoopIteration 循环迭代次数（从1开始）
	LoopIteration int

	// logCollector 日志收集器（用于统一收集执行过程中的日志）
	logCollector *types.LogCollector

	mu sync.RWMutex
}

// NewExecutionContext 创建一个新的 ExecutionContext。
func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		Variables:    make(map[string]any),
		Results:      make(map[string]*types.StepResult),
		logCollector: types.NewLogCollector(),
	}
}

// WithVariables 设置变量映射。
func (c *ExecutionContext) WithVariables(vars map[string]any) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Variables = vars
	return c
}

// WithVU 设置虚拟用户。
func (c *ExecutionContext) WithVU(vu *types.VirtualUser) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.VU = vu
	return c
}

// WithIteration 设置迭代次数。
func (c *ExecutionContext) WithIteration(iteration int) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Iteration = iteration
	return c
}

// WithWorkflowID 设置工作流 ID。
func (c *ExecutionContext) WithWorkflowID(id string) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.WorkflowID = id
	return c
}

// WithExecutionID 设置执行 ID。
func (c *ExecutionContext) WithExecutionID(id string) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ExecutionID = id
	return c
}

// SetVariable 设置变量值。
func (c *ExecutionContext) SetVariable(name string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Variables[name] = value
}

// GetVariable 获取变量值。
func (c *ExecutionContext) GetVariable(name string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.Variables[name]
	return val, ok
}

// SetResult 存储步骤结果。
func (c *ExecutionContext) SetResult(stepID string, result *types.StepResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Results[stepID] = result
}

// GetResult 获取步骤结果。
func (c *ExecutionContext) GetResult(stepID string) (*types.StepResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, ok := c.Results[stepID]
	return result, ok
}

// Clone 创建执行上下文的浅拷贝。
func (c *ExecutionContext) Clone() *ExecutionContext {
	c.mu.RLock()
	defer c.mu.RUnlock()

	newCtx := &ExecutionContext{
		Variables:     make(map[string]any, len(c.Variables)),
		Results:       make(map[string]*types.StepResult, len(c.Results)),
		VU:            c.VU,
		Iteration:     c.Iteration,
		WorkflowID:    c.WorkflowID,
		ExecutionID:   c.ExecutionID,
		Callback:      c.Callback,
		ParentStepID:  c.ParentStepID,
		LoopIteration: c.LoopIteration,
		logCollector:  c.logCollector.Clone(),
	}

	for k, v := range c.Variables {
		newCtx.Variables[k] = v
	}
	for k, v := range c.Results {
		newCtx.Results[k] = v
	}

	return newCtx
}

// WithCallback 设置执行回调。
func (c *ExecutionContext) WithCallback(callback types.ExecutionCallback) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Callback = callback
	return c
}

// WithParentStep 设置父步骤信息（用于循环等嵌套场景）。
func (c *ExecutionContext) WithParentStep(parentID string, iteration int) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ParentStepID = parentID
	c.LoopIteration = iteration
	return c
}

// GetCallback 获取执行回调。
func (c *ExecutionContext) GetCallback() types.ExecutionCallback {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Callback
}

// GetParentStepID 获取父步骤ID。
func (c *ExecutionContext) GetParentStepID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ParentStepID
}

// GetLoopIteration 获取循环迭代次数。
func (c *ExecutionContext) GetLoopIteration() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LoopIteration
}

// ToEvaluationContext 将 ExecutionContext 转换为表达式求值上下文。
func (c *ExecutionContext) ToEvaluationContext() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	evalCtx := make(map[string]any)

	// 复制变量
	for k, v := range c.Variables {
		evalCtx[k] = v
	}

	// 将结果转换为适合表达式求值的格式
	for stepID, result := range c.Results {
		resultMap := map[string]any{
			"status":     string(result.Status),
			"duration":   result.Duration.Milliseconds(),
			"output":     result.Output,
			"step_id":    result.StepID,
			"start_time": result.StartTime,
			"end_time":   result.EndTime,
		}
		if result.Error != nil {
			resultMap["error"] = result.Error.Error()
		}
		if result.Metrics != nil {
			resultMap["metrics"] = result.Metrics
		}
		evalCtx[stepID] = resultMap
	}

	return evalCtx
}

// ========== 日志收集相关方法 ==========

// GetLogCollector 获取日志收集器
func (c *ExecutionContext) GetLogCollector() *types.LogCollector {
	return c.logCollector
}

// AppendLog 添加单条日志
func (c *ExecutionContext) AppendLog(entry types.ConsoleLogEntry) {
	if c.logCollector != nil {
		c.logCollector.AppendLog(entry)
	}
}

// AppendLogs 批量添加日志
func (c *ExecutionContext) AppendLogs(entries []types.ConsoleLogEntry) {
	if c.logCollector != nil {
		c.logCollector.AppendLogs(entries)
	}
}

// FlushLogs 获取并清空所有日志
func (c *ExecutionContext) FlushLogs() []types.ConsoleLogEntry {
	if c.logCollector != nil {
		return c.logCollector.FlushLogs()
	}
	return nil
}

// GetLogs 获取所有日志（不清空）
func (c *ExecutionContext) GetLogs() []types.ConsoleLogEntry {
	if c.logCollector != nil {
		return c.logCollector.GetLogs()
	}
	return nil
}

// SetVariableWithTracking 设置变量并记录变更
// scope: "env" 表示环境变量，"temp" 表示临时变量
// source: 变更来源，如 "set_variable", "extract_param", "js_script", "loop" 等
func (c *ExecutionContext) SetVariableWithTracking(name string, value any, scope, source string) {
	c.mu.Lock()
	oldValue, _ := c.Variables[name]
	c.Variables[name] = value
	c.mu.Unlock()

	if c.logCollector != nil {
		// 标记环境变量
		if scope == "env" {
			c.logCollector.MarkAsEnvVar(name)
		}
		// 记录变量变更
		c.logCollector.RecordVariableChange(name, oldValue, value, scope, source)
	}
}

// MarkAsEnvVar 标记某个变量为环境变量
func (c *ExecutionContext) MarkAsEnvVar(name string) {
	if c.logCollector != nil {
		c.logCollector.MarkAsEnvVar(name)
	}
}

// IsEnvVar 检查是否是环境变量
func (c *ExecutionContext) IsEnvVar(name string) bool {
	if c.logCollector != nil {
		return c.logCollector.IsEnvVar(name)
	}
	return false
}

// CreateVariableSnapshot 创建变量快照
func (c *ExecutionContext) CreateVariableSnapshot() {
	c.CreateVariableSnapshotWithEnvVars(nil)
}

// CreateVariableSnapshotWithEnvVars 创建变量快照（支持单独传入环境变量）
func (c *ExecutionContext) CreateVariableSnapshotWithEnvVars(envVars map[string]interface{}) {
	if c.logCollector == nil {
		return
	}

	c.mu.RLock()
	tempVars := make(map[string]any)
	envVarsAny := make(map[string]any)

	// 分离临时变量和环境变量
	for k, v := range c.Variables {
		if c.logCollector.IsEnvVar(k) {
			envVarsAny[k] = v
		} else {
			tempVars[k] = v
		}
	}
	c.mu.RUnlock()

	// 合并传入的环境变量
	if envVars != nil {
		for k, v := range envVars {
			envVarsAny[k] = v
		}
	}

	// 直接创建快照条目
	c.logCollector.AppendLog(types.NewSnapshotEntry(types.VariableSnapshotInfo{
		EnvVars:  envVarsAny,
		TempVars: tempVars,
	}))
}

// MergeLogsFrom 从另一个上下文合并日志
func (c *ExecutionContext) MergeLogsFrom(other *ExecutionContext) {
	if c.logCollector != nil && other != nil && other.logCollector != nil {
		c.logCollector.MergeFrom(other.logCollector)
	}
}
