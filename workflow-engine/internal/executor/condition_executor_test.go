package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

func TestConditionExecutor_Type(t *testing.T) {
	executor := NewConditionExecutor()
	assert.Equal(t, ConditionExecutorType, executor.Type())
}

func TestConditionExecutor_Init(t *testing.T) {
	executor := NewConditionExecutor()

	err := executor.Init(context.Background(), nil)

	assert.NoError(t, err)
}

func TestConditionExecutor_Execute_ThenBranch(t *testing.T) {
	// Create a registry with a mock executor for the then branch
	registry := NewRegistry()
	mockExec := &trackingExecutor{
		BaseExecutor: NewBaseExecutor("mock"),
		executed:     make([]string, 0),
	}
	registry.MustRegister(mockExec)

	executor := NewConditionExecutorWithRegistry(registry)
	require.NoError(t, executor.Init(context.Background(), nil))

	execCtx := NewExecutionContext()
	execCtx.SetVariable("value", 10)

	step := &types.Step{
		ID:   "test-condition",
		Name: "Test Condition",
		Type: ConditionExecutorType,
		Condition: &types.Condition{
			Expression: "${value} > 5",
			Then: []types.Step{
				{ID: "then-step", Name: "Then Step", Type: "mock"},
			},
			Else: []types.Step{
				{ID: "else-step", Name: "Else Step", Type: "mock"},
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.True(t, output.Result)
	assert.Equal(t, "then", output.BranchTaken)
	assert.Contains(t, output.StepsExecuted, "then-step")
	assert.NotContains(t, output.StepsExecuted, "else-step")
}

func TestConditionExecutor_Execute_ElseBranch(t *testing.T) {
	registry := NewRegistry()
	mockExec := &trackingExecutor{
		BaseExecutor: NewBaseExecutor("mock"),
		executed:     make([]string, 0),
	}
	registry.MustRegister(mockExec)

	executor := NewConditionExecutorWithRegistry(registry)
	require.NoError(t, executor.Init(context.Background(), nil))

	execCtx := NewExecutionContext()
	execCtx.SetVariable("value", 3)

	step := &types.Step{
		ID:   "test-condition",
		Name: "Test Condition",
		Type: ConditionExecutorType,
		Condition: &types.Condition{
			Expression: "${value} > 5",
			Then: []types.Step{
				{ID: "then-step", Name: "Then Step", Type: "mock"},
			},
			Else: []types.Step{
				{ID: "else-step", Name: "Else Step", Type: "mock"},
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.False(t, output.Result)
	assert.Equal(t, "else", output.BranchTaken)
	assert.Contains(t, output.StepsExecuted, "else-step")
	assert.NotContains(t, output.StepsExecuted, "then-step")
}

func TestConditionExecutor_Execute_EmptyElseBranch(t *testing.T) {
	registry := NewRegistry()
	mockExec := &trackingExecutor{
		BaseExecutor: NewBaseExecutor("mock"),
		executed:     make([]string, 0),
	}
	registry.MustRegister(mockExec)

	executor := NewConditionExecutorWithRegistry(registry)
	require.NoError(t, executor.Init(context.Background(), nil))

	execCtx := NewExecutionContext()
	execCtx.SetVariable("value", 3)

	step := &types.Step{
		ID:   "test-condition",
		Name: "Test Condition",
		Type: ConditionExecutorType,
		Condition: &types.Condition{
			Expression: "${value} > 5",
			Then: []types.Step{
				{ID: "then-step", Name: "Then Step", Type: "mock"},
			},
			// No else branch
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.False(t, output.Result)
	assert.Equal(t, "else", output.BranchTaken)
	assert.Empty(t, output.StepsExecuted)
}

func TestConditionExecutor_Execute_WithStepResults(t *testing.T) {
	registry := NewRegistry()
	mockExec := &trackingExecutor{
		BaseExecutor: NewBaseExecutor("mock"),
		executed:     make([]string, 0),
	}
	registry.MustRegister(mockExec)

	executor := NewConditionExecutorWithRegistry(registry)
	require.NoError(t, executor.Init(context.Background(), nil))

	execCtx := NewExecutionContext()
	execCtx.SetResult("login", &types.StepResult{
		StepID: "login",
		Status: types.ResultStatusSuccess,
		Output: &HTTPResponse{
			StatusCode: 200,
			Body:       map[string]any{"token": "abc123"},
		},
	})

	step := &types.Step{
		ID:   "test-condition",
		Name: "Test Condition",
		Type: ConditionExecutorType,
		Condition: &types.Condition{
			Expression: "${login.status_code} == 200",
			Then: []types.Step{
				{ID: "success-step", Name: "Success Step", Type: "mock"},
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.True(t, output.Result)
	assert.Equal(t, "then", output.BranchTaken)
}

func TestConditionExecutor_Execute_LogicalOperators(t *testing.T) {
	registry := NewRegistry()
	mockExec := &trackingExecutor{
		BaseExecutor: NewBaseExecutor("mock"),
		executed:     make([]string, 0),
	}
	registry.MustRegister(mockExec)

	executor := NewConditionExecutorWithRegistry(registry)
	require.NoError(t, executor.Init(context.Background(), nil))

	execCtx := NewExecutionContext()
	execCtx.SetVariable("a", 10)
	execCtx.SetVariable("b", 20)

	step := &types.Step{
		ID:   "test-condition",
		Name: "Test Condition",
		Type: ConditionExecutorType,
		Condition: &types.Condition{
			Expression: "${a} > 5 AND ${b} < 30",
			Then: []types.Step{
				{ID: "success-step", Name: "Success Step", Type: "mock"},
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.True(t, output.Result)
}

func TestConditionExecutor_Execute_MissingCondition(t *testing.T) {
	executor := NewConditionExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-no-condition",
		Name: "Test No Condition",
		Type: ConditionExecutorType,
		// No condition
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusFailed, result.Status)
	assert.NotNil(t, result.Error)
}

func TestConditionExecutor_Execute_InvalidExpression(t *testing.T) {
	executor := NewConditionExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-invalid",
		Name: "Test Invalid",
		Type: ConditionExecutorType,
		Condition: &types.Condition{
			Expression: "${undefined_var} > 5",
			Then:       []types.Step{},
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusFailed, result.Status)
}

func TestConditionExecutor_Execute_Metrics(t *testing.T) {
	registry := NewRegistry()
	mockExec := &trackingExecutor{
		BaseExecutor: NewBaseExecutor("mock"),
		executed:     make([]string, 0),
	}
	registry.MustRegister(mockExec)

	executor := NewConditionExecutorWithRegistry(registry)
	require.NoError(t, executor.Init(context.Background(), nil))

	execCtx := NewExecutionContext()
	execCtx.SetVariable("value", 10)

	step := &types.Step{
		ID:   "test-metrics",
		Name: "Test Metrics",
		Type: ConditionExecutorType,
		Condition: &types.Condition{
			Expression: "${value} > 5",
			Then: []types.Step{
				{ID: "step1", Type: "mock"},
				{ID: "step2", Type: "mock"},
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, float64(1), result.Metrics["condition_result"])
	assert.Equal(t, float64(2), result.Metrics["branch_steps_count"])
}

func TestConditionExecutor_Cleanup(t *testing.T) {
	executor := NewConditionExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	err := executor.Cleanup(context.Background())
	assert.NoError(t, err)
}

// trackingExecutor is a test executor that tracks executed steps.
type trackingExecutor struct {
	*BaseExecutor
	executed []string
}

func (e *trackingExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	e.executed = append(e.executed, step.ID)
	return CreateSuccessResult(step.ID, time.Now(), nil), nil
}
