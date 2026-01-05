package executor

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/k6/workflow-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookExecutor_ExecutePreScripts(t *testing.T) {
	scriptExec := NewScriptExecutor()
	hookExec := NewHookExecutor(scriptExec, nil)
	execCtx := NewExecutionContext()

	hooks := &StepHooks{
		PreScripts: []*ScriptHook{
			{
				Name:   "set_var",
				Script: "ctx.SetVariable('pre_executed', true)",
			},
		},
	}

	err := hookExec.ExecutePreScripts(context.Background(), hooks, execCtx)
	require.NoError(t, err)

	val, ok := execCtx.GetVariable("pre_executed")
	assert.True(t, ok)
	assert.Equal(t, true, val)
}

func TestHookExecutor_ExecutePostScripts(t *testing.T) {
	scriptExec := NewScriptExecutor()
	hookExec := NewHookExecutor(scriptExec, nil)
	execCtx := NewExecutionContext()

	stepResult := &types.StepResult{
		StepID:   "test_step",
		Status:   types.ResultStatusSuccess,
		Duration: 100 * time.Millisecond,
		Output:   map[string]any{"data": "test"},
	}

	hooks := &StepHooks{
		PostScripts: []*ScriptHook{
			{
				Name:   "check_result",
				Script: "ctx.SetVariable('post_executed', true)",
			},
		},
	}

	err := hookExec.ExecutePostScripts(context.Background(), hooks, execCtx, stepResult)
	require.NoError(t, err)

	val, ok := execCtx.GetVariable("post_executed")
	assert.True(t, ok)
	assert.Equal(t, true, val)

	// Check step_result is available
	resultVar, ok := execCtx.GetVariable("step_result")
	assert.True(t, ok)
	resultMap := resultVar.(map[string]any)
	assert.Equal(t, "success", resultMap["status"])
}

func TestHookExecutor_ExecutePreScripts_Empty(t *testing.T) {
	hookExec := NewHookExecutor(nil, nil)
	execCtx := NewExecutionContext()

	// Nil hooks
	err := hookExec.ExecutePreScripts(context.Background(), nil, execCtx)
	assert.NoError(t, err)

	// Empty hooks
	err = hookExec.ExecutePreScripts(context.Background(), &StepHooks{}, execCtx)
	assert.NoError(t, err)
}

func TestHookExecutor_ExecutePreScripts_OnErrorContinue(t *testing.T) {
	scriptExec := NewScriptExecutor()
	hookExec := NewHookExecutor(scriptExec, nil)
	execCtx := NewExecutionContext()

	hooks := &StepHooks{
		PreScripts: []*ScriptHook{
			{
				Name:    "failing_script",
				Script:  "invalid syntax !!!",
				OnError: OnErrorContinue,
			},
			{
				Name:   "second_script",
				Script: "ctx.SetVariable('second_executed', true)",
			},
		},
	}

	err := hookExec.ExecutePreScripts(context.Background(), hooks, execCtx)
	require.NoError(t, err)

	// Second script should still execute
	val, ok := execCtx.GetVariable("second_executed")
	assert.True(t, ok)
	assert.Equal(t, true, val)
}

func TestHookExecutor_ExecutePreScripts_OnErrorAbort(t *testing.T) {
	scriptExec := NewScriptExecutor()
	hookExec := NewHookExecutor(scriptExec, nil)
	execCtx := NewExecutionContext()

	hooks := &StepHooks{
		PreScripts: []*ScriptHook{
			{
				Name:    "failing_script",
				Script:  "invalid syntax !!!",
				OnError: OnErrorAbort,
			},
			{
				Name:   "second_script",
				Script: "ctx.SetVariable('second_executed', true)",
			},
		},
	}

	err := hookExec.ExecutePreScripts(context.Background(), hooks, execCtx)
	require.Error(t, err)

	// Second script should not execute
	_, ok := execCtx.GetVariable("second_executed")
	assert.False(t, ok)
}

func TestHookExecutor_ExecutePreScriptsWithResult(t *testing.T) {
	scriptExec := NewScriptExecutor()
	hookExec := NewHookExecutor(scriptExec, nil)
	execCtx := NewExecutionContext()

	hooks := &StepHooks{
		PreScripts: []*ScriptHook{
			{
				Name:   "script1",
				Script: "ctx.SetVariable('v1', 1)",
			},
			{
				Name:   "script2",
				Script: "ctx.SetVariable('v2', 2)",
			},
		},
	}

	result := hookExec.ExecutePreScriptsWithResult(context.Background(), hooks, execCtx)

	assert.True(t, result.Success)
	assert.Nil(t, result.Error)
	assert.Equal(t, 2, result.ExecutedCount)
	assert.Equal(t, 0, result.FailedCount)
}

func TestHookExecutor_ExecutePreScriptsWithResult_PartialFailure(t *testing.T) {
	scriptExec := NewScriptExecutor()
	hookExec := NewHookExecutor(scriptExec, nil)
	execCtx := NewExecutionContext()

	hooks := &StepHooks{
		PreScripts: []*ScriptHook{
			{
				Name:   "script1",
				Script: "ctx.SetVariable('v1', 1)",
			},
			{
				Name:    "failing",
				Script:  "invalid!!!",
				OnError: OnErrorContinue,
			},
			{
				Name:   "script3",
				Script: "ctx.SetVariable('v3', 3)",
			},
		},
	}

	result := hookExec.ExecutePreScriptsWithResult(context.Background(), hooks, execCtx)

	assert.True(t, result.Success)
	assert.Equal(t, 3, result.ExecutedCount)
	assert.Equal(t, 1, result.FailedCount)
}

func TestHookExecutor_MultiplePreAndPostScripts(t *testing.T) {
	scriptExec := NewScriptExecutor()
	hookExec := NewHookExecutor(scriptExec, nil)
	execCtx := NewExecutionContext()
	execCtx.SetVariable("execution_order", []string{})

	hooks := &StepHooks{
		PreScripts: []*ScriptHook{
			{
				Name:   "pre1",
				Script: `order, _ := ctx.GetVariable("execution_order"); ctx.SetVariable("execution_order", append(order.([]string), "pre1"))`,
			},
			{
				Name:   "pre2",
				Script: `order, _ := ctx.GetVariable("execution_order"); ctx.SetVariable("execution_order", append(order.([]string), "pre2"))`,
			},
		},
		PostScripts: []*ScriptHook{
			{
				Name:   "post1",
				Script: `order, _ := ctx.GetVariable("execution_order"); ctx.SetVariable("execution_order", append(order.([]string), "post1"))`,
			},
			{
				Name:   "post2",
				Script: `order, _ := ctx.GetVariable("execution_order"); ctx.SetVariable("execution_order", append(order.([]string), "post2"))`,
			},
		},
	}

	// Execute pre scripts
	err := hookExec.ExecutePreScripts(context.Background(), hooks, execCtx)
	require.NoError(t, err)

	// Simulate step execution
	stepResult := &types.StepResult{
		StepID: "main_step",
		Status: types.ResultStatusSuccess,
	}

	// Execute post scripts
	err = hookExec.ExecutePostScripts(context.Background(), hooks, execCtx, stepResult)
	require.NoError(t, err)

	// Verify execution order
	order, ok := execCtx.GetVariable("execution_order")
	assert.True(t, ok)
	assert.Equal(t, []string{"pre1", "pre2", "post1", "post2"}, order)
}

func TestOnErrorStrategy(t *testing.T) {
	assert.Equal(t, OnErrorStrategy("continue"), OnErrorContinue)
	assert.Equal(t, OnErrorStrategy("abort"), OnErrorAbort)
	assert.Equal(t, OnErrorStrategy("retry"), OnErrorRetry)
}
