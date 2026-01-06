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

	mu sync.RWMutex
}

// NewExecutionContext 创建一个新的 ExecutionContext。
func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		Variables: make(map[string]any),
		Results:   make(map[string]*types.StepResult),
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
		Variables:   make(map[string]any, len(c.Variables)),
		Results:     make(map[string]*types.StepResult, len(c.Results)),
		VU:          c.VU,
		Iteration:   c.Iteration,
		WorkflowID:  c.WorkflowID,
		ExecutionID: c.ExecutionID,
	}

	for k, v := range c.Variables {
		newCtx.Variables[k] = v
	}
	for k, v := range c.Results {
		newCtx.Results[k] = v
	}

	return newCtx
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
