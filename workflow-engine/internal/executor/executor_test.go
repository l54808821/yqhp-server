package executor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

func TestNewExecutionContext(t *testing.T) {
	ctx := NewExecutionContext()

	assert.NotNil(t, ctx)
	assert.NotNil(t, ctx.Variables)
	assert.NotNil(t, ctx.Results)
	assert.Empty(t, ctx.Variables)
	assert.Empty(t, ctx.Results)
}

func TestExecutionContext_SetAndGetVariable(t *testing.T) {
	ctx := NewExecutionContext()

	ctx.SetVariable("key1", "value1")
	ctx.SetVariable("key2", 42)

	val1, ok1 := ctx.GetVariable("key1")
	assert.True(t, ok1)
	assert.Equal(t, "value1", val1)

	val2, ok2 := ctx.GetVariable("key2")
	assert.True(t, ok2)
	assert.Equal(t, 42, val2)

	_, ok3 := ctx.GetVariable("nonexistent")
	assert.False(t, ok3)
}

func TestExecutionContext_SetAndGetResult(t *testing.T) {
	ctx := NewExecutionContext()

	result := &types.StepResult{
		StepID:   "step1",
		Status:   types.ResultStatusSuccess,
		Duration: 100 * time.Millisecond,
	}

	ctx.SetResult("step1", result)

	retrieved, ok := ctx.GetResult("step1")
	assert.True(t, ok)
	assert.Equal(t, result, retrieved)

	_, ok = ctx.GetResult("nonexistent")
	assert.False(t, ok)
}

func TestExecutionContext_WithMethods(t *testing.T) {
	vu := &types.VirtualUser{ID: 1}
	vars := map[string]any{"key": "value"}

	ctx := NewExecutionContext().
		WithVariables(vars).
		WithVU(vu).
		WithIteration(5).
		WithWorkflowID("wf-123").
		WithExecutionID("exec-456")

	assert.Equal(t, vars, ctx.Variables)
	assert.Equal(t, vu, ctx.VU)
	assert.Equal(t, 5, ctx.Iteration)
	assert.Equal(t, "wf-123", ctx.WorkflowID)
	assert.Equal(t, "exec-456", ctx.ExecutionID)
}

func TestExecutionContext_Clone(t *testing.T) {
	original := NewExecutionContext().
		WithWorkflowID("wf-123").
		WithIteration(3)

	original.SetVariable("key", "value")
	original.SetResult("step1", &types.StepResult{StepID: "step1"})

	cloned := original.Clone()

	// Verify values are copied
	assert.Equal(t, original.WorkflowID, cloned.WorkflowID)
	assert.Equal(t, original.Iteration, cloned.Iteration)

	val, ok := cloned.GetVariable("key")
	assert.True(t, ok)
	assert.Equal(t, "value", val)

	result, ok := cloned.GetResult("step1")
	assert.True(t, ok)
	assert.Equal(t, "step1", result.StepID)

	// Verify independence
	cloned.SetVariable("key", "modified")
	originalVal, _ := original.GetVariable("key")
	assert.Equal(t, "value", originalVal)
}

func TestExecutionContext_ToEvaluationContext(t *testing.T) {
	ctx := NewExecutionContext()
	ctx.SetVariable("base_url", "http://example.com")
	ctx.SetResult("login", &types.StepResult{
		StepID:    "login",
		Status:    types.ResultStatusSuccess,
		Duration:  150 * time.Millisecond,
		Output:    map[string]any{"token": "abc123"},
		StartTime: time.Now().Add(-150 * time.Millisecond),
		EndTime:   time.Now(),
	})

	evalCtx := ctx.ToEvaluationContext()

	assert.Equal(t, "http://example.com", evalCtx["base_url"])

	loginResult, ok := evalCtx["login"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "success", loginResult["status"])
	assert.NotNil(t, loginResult["output"])
}
