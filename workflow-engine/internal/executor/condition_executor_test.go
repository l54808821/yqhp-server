package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"yqhp/workflow-engine/pkg/types"
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

func TestConditionExecutor_Execute_If(t *testing.T) {
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
		ID:   "test-if",
		Name: "Test If",
		Type: ConditionExecutorType,
		Config: map[string]any{
			"type":       "if",
			"expression": "${value} > 5",
		},
		Children: []types.Step{
			{ID: "child-step", Name: "Child Step", Type: "mock"},
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.True(t, output.Result)
	assert.Equal(t, "if", output.BranchTaken)
	assert.Contains(t, output.StepsExecuted, "child-step")
}

func TestConditionExecutor_Execute_IfFalse(t *testing.T) {
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
		ID:   "test-if",
		Name: "Test If",
		Type: ConditionExecutorType,
		Config: map[string]any{
			"type":       "if",
			"expression": "${value} > 5",
		},
		Children: []types.Step{
			{ID: "child-step", Name: "Child Step", Type: "mock"},
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.False(t, output.Result)
	assert.Empty(t, output.StepsExecuted)
}

func TestConditionExecutor_Execute_IfElseIf(t *testing.T) {
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

	// 第一个 if 条件不满足
	step1 := &types.Step{
		ID:   "test-if",
		Name: "Test If",
		Type: ConditionExecutorType,
		Config: map[string]any{
			"type":       "if",
			"expression": "${value} > 5",
		},
		Children: []types.Step{
			{ID: "if-child", Name: "If Child", Type: "mock"},
		},
	}

	result1, err := executor.Execute(context.Background(), step1, execCtx)
	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result1.Status)

	output1, ok := result1.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.False(t, output1.Result)
	assert.Empty(t, output1.StepsExecuted)

	// else_if 条件满足
	step2 := &types.Step{
		ID:   "test-else-if",
		Name: "Test Else If",
		Type: ConditionExecutorType,
		Config: map[string]any{
			"type":       "else_if",
			"expression": "${value} > 2",
		},
		Children: []types.Step{
			{ID: "else-if-child", Name: "Else If Child", Type: "mock"},
		},
	}

	result2, err := executor.Execute(context.Background(), step2, execCtx)
	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result2.Status)

	output2, ok := result2.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.True(t, output2.Result)
	assert.Contains(t, output2.StepsExecuted, "else-if-child")
}

func TestConditionExecutor_Execute_IfElse(t *testing.T) {
	registry := NewRegistry()
	mockExec := &trackingExecutor{
		BaseExecutor: NewBaseExecutor("mock"),
		executed:     make([]string, 0),
	}
	registry.MustRegister(mockExec)

	executor := NewConditionExecutorWithRegistry(registry)
	require.NoError(t, executor.Init(context.Background(), nil))

	execCtx := NewExecutionContext()
	execCtx.SetVariable("value", 1)

	// if 条件不满足
	step1 := &types.Step{
		ID:   "test-if",
		Name: "Test If",
		Type: ConditionExecutorType,
		Config: map[string]any{
			"type":       "if",
			"expression": "${value} > 5",
		},
		Children: []types.Step{
			{ID: "if-child", Name: "If Child", Type: "mock"},
		},
	}

	result1, err := executor.Execute(context.Background(), step1, execCtx)
	require.NoError(t, err)
	output1, _ := result1.Output.(*ConditionOutput)
	assert.False(t, output1.Result)

	// else 应该执行
	step2 := &types.Step{
		ID:   "test-else",
		Name: "Test Else",
		Type: ConditionExecutorType,
		Config: map[string]any{
			"type": "else",
		},
		Children: []types.Step{
			{ID: "else-child", Name: "Else Child", Type: "mock"},
		},
	}

	result2, err := executor.Execute(context.Background(), step2, execCtx)
	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result2.Status)

	output2, ok := result2.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.True(t, output2.Result)
	assert.Contains(t, output2.StepsExecuted, "else-child")
}

func TestConditionExecutor_Execute_SkipElseIfAfterMatch(t *testing.T) {
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

	// if 条件满足
	step1 := &types.Step{
		ID:   "test-if",
		Name: "Test If",
		Type: ConditionExecutorType,
		Config: map[string]any{
			"type":       "if",
			"expression": "${value} > 5",
		},
		Children: []types.Step{
			{ID: "if-child", Name: "If Child", Type: "mock"},
		},
	}

	result1, err := executor.Execute(context.Background(), step1, execCtx)
	require.NoError(t, err)
	output1, _ := result1.Output.(*ConditionOutput)
	assert.True(t, output1.Result)
	assert.Contains(t, output1.StepsExecuted, "if-child")

	// else_if 应该被跳过（即使条件也满足）
	step2 := &types.Step{
		ID:   "test-else-if",
		Name: "Test Else If",
		Type: ConditionExecutorType,
		Config: map[string]any{
			"type":       "else_if",
			"expression": "${value} > 8",
		},
		Children: []types.Step{
			{ID: "else-if-child", Name: "Else If Child", Type: "mock"},
		},
	}

	result2, err := executor.Execute(context.Background(), step2, execCtx)
	require.NoError(t, err)

	output2, ok := result2.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.False(t, output2.Result) // 应该不执行
	assert.Empty(t, output2.StepsExecuted)

	// else 也应该被跳过
	step3 := &types.Step{
		ID:   "test-else",
		Name: "Test Else",
		Type: ConditionExecutorType,
		Config: map[string]any{
			"type": "else",
		},
		Children: []types.Step{
			{ID: "else-child", Name: "Else Child", Type: "mock"},
		},
	}

	result3, err := executor.Execute(context.Background(), step3, execCtx)
	require.NoError(t, err)

	output3, ok := result3.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.False(t, output3.Result) // 应该不执行
	assert.Empty(t, output3.StepsExecuted)
}

func TestConditionExecutor_Execute_DefaultTypeIsIf(t *testing.T) {
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

	// 不指定 type，应该默认为 if
	step := &types.Step{
		ID:   "test-default",
		Name: "Test Default",
		Type: ConditionExecutorType,
		Config: map[string]any{
			"expression": "${value} > 5",
		},
		Children: []types.Step{
			{ID: "child-step", Name: "Child Step", Type: "mock"},
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.True(t, output.Result)
	assert.Contains(t, output.StepsExecuted, "child-step")
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
		Config: map[string]any{
			"type":       "if",
			"expression": "${login.status_code} == 200",
		},
		Children: []types.Step{
			{ID: "success-step", Name: "Success Step", Type: "mock"},
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.True(t, output.Result)
	assert.Equal(t, "if", output.BranchTaken)
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
		Config: map[string]any{
			"type":       "if",
			"expression": "${a} > 5 AND ${b} < 30",
		},
		Children: []types.Step{
			{ID: "success-step", Name: "Success Step", Type: "mock"},
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*ConditionOutput)
	require.True(t, ok)
	assert.True(t, output.Result)
}

func TestConditionExecutor_Execute_InvalidExpression(t *testing.T) {
	executor := NewConditionExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-invalid",
		Name: "Test Invalid",
		Type: ConditionExecutorType,
		Config: map[string]any{
			"type":       "if",
			"expression": "${undefined_var} > 5",
		},
		Children: []types.Step{},
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
		Config: map[string]any{
			"type":       "if",
			"expression": "${value} > 5",
		},
		Children: []types.Step{
			{ID: "step1", Type: "mock"},
			{ID: "step2", Type: "mock"},
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
