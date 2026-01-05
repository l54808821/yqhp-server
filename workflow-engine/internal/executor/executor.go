// Package executor provides the executor framework for workflow step execution.
package executor

import (
	"context"
	"sync"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// Executor defines the interface for step executors.
type Executor interface {
	// Type returns the executor type identifier.
	Type() string

	// Init initializes the executor with configuration.
	Init(ctx context.Context, config map[string]any) error

	// Execute executes a step and returns the result.
	Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error)

	// Cleanup releases resources held by the executor.
	Cleanup(ctx context.Context) error
}

// ExecutionContext holds runtime state for step execution.
type ExecutionContext struct {
	// Variables holds variable values accessible during execution.
	Variables map[string]any

	// Results holds step execution results keyed by step ID.
	Results map[string]*types.StepResult

	// VU is the virtual user executing the workflow.
	VU *types.VirtualUser

	// Iteration is the current iteration number.
	Iteration int

	// WorkflowID is the ID of the workflow being executed.
	WorkflowID string

	// ExecutionID is the unique ID of this execution.
	ExecutionID string

	mu sync.RWMutex
}

// NewExecutionContext creates a new ExecutionContext.
func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		Variables: make(map[string]any),
		Results:   make(map[string]*types.StepResult),
	}
}

// WithVariables sets the variables map.
func (c *ExecutionContext) WithVariables(vars map[string]any) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Variables = vars
	return c
}

// WithVU sets the virtual user.
func (c *ExecutionContext) WithVU(vu *types.VirtualUser) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.VU = vu
	return c
}

// WithIteration sets the iteration number.
func (c *ExecutionContext) WithIteration(iteration int) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Iteration = iteration
	return c
}

// WithWorkflowID sets the workflow ID.
func (c *ExecutionContext) WithWorkflowID(id string) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.WorkflowID = id
	return c
}

// WithExecutionID sets the execution ID.
func (c *ExecutionContext) WithExecutionID(id string) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ExecutionID = id
	return c
}

// SetVariable sets a variable value.
func (c *ExecutionContext) SetVariable(name string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Variables[name] = value
}

// GetVariable gets a variable value.
func (c *ExecutionContext) GetVariable(name string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.Variables[name]
	return val, ok
}

// SetResult stores a step result.
func (c *ExecutionContext) SetResult(stepID string, result *types.StepResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Results[stepID] = result
}

// GetResult retrieves a step result.
func (c *ExecutionContext) GetResult(stepID string) (*types.StepResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, ok := c.Results[stepID]
	return result, ok
}

// Clone creates a shallow copy of the execution context.
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

// ToEvaluationContext converts ExecutionContext to expression.EvaluationContext.
func (c *ExecutionContext) ToEvaluationContext() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	evalCtx := make(map[string]any)

	// Copy variables
	for k, v := range c.Variables {
		evalCtx[k] = v
	}

	// Convert results to a format suitable for expression evaluation
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
