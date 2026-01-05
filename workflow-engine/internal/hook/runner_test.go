package hook

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grafana/k6/workflow-engine/internal/executor"
	"github.com/grafana/k6/workflow-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRegistry(t *testing.T, mockExec *MockExecutor) *executor.Registry {
	registry := executor.NewRegistry()
	require.NoError(t, registry.Register(mockExec))
	return registry
}

func TestRunner_ExecuteWorkflowPreHook(t *testing.T) {
	t.Run("no pre-hook returns nil", func(t *testing.T) {
		registry := executor.NewRegistry()
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		workflow := &types.Workflow{
			ID:      "test-workflow",
			PreHook: nil,
		}

		result, err := runner.ExecuteWorkflowPreHook(context.Background(), workflow, execCtx)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("successful pre-hook execution", func(t *testing.T) {
		mockExec := NewMockExecutor("script")
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		workflow := &types.Workflow{
			ID: "test-workflow",
			PreHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo pre"},
			},
		}

		result, err := runner.ExecuteWorkflowPreHook(context.Background(), workflow, execCtx)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, HookTypePre, result.HookType)
		assert.Equal(t, HookLevelWorkflow, result.HookLevel)
		assert.Equal(t, types.ResultStatusSuccess, result.Status)
	})

	t.Run("failed pre-hook returns error", func(t *testing.T) {
		mockExec := NewMockExecutor("script").WithExecuteFunc(
			func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
				return &types.StepResult{
					StepID:  step.ID,
					Status:  types.ResultStatusFailed,
					Error:   errors.New("pre-hook failed"),
					Metrics: make(map[string]float64),
				}, nil
			},
		)
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		workflow := &types.Workflow{
			ID: "test-workflow",
			PreHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "exit 1"},
			},
		}

		result, err := runner.ExecuteWorkflowPreHook(context.Background(), workflow, execCtx)
		require.Error(t, err)
		require.NotNil(t, result)
		assert.Equal(t, types.ResultStatusFailed, result.Status)
	})
}

func TestRunner_ExecuteWorkflowPostHook(t *testing.T) {
	t.Run("no post-hook returns nil", func(t *testing.T) {
		registry := executor.NewRegistry()
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		workflow := &types.Workflow{
			ID:       "test-workflow",
			PostHook: nil,
		}

		result, err := runner.ExecuteWorkflowPostHook(context.Background(), workflow, execCtx, nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("successful post-hook execution", func(t *testing.T) {
		mockExec := NewMockExecutor("script")
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		workflow := &types.Workflow{
			ID: "test-workflow",
			PostHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo post"},
			},
		}

		result, err := runner.ExecuteWorkflowPostHook(context.Background(), workflow, execCtx, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, HookTypePost, result.HookType)
		assert.Equal(t, HookLevelWorkflow, result.HookLevel)
		assert.Equal(t, types.ResultStatusSuccess, result.Status)
	})

	t.Run("post-hook receives workflow error in context", func(t *testing.T) {
		var capturedCtx *executor.ExecutionContext
		mockExec := NewMockExecutor("script").WithExecuteFunc(
			func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
				capturedCtx = execCtx
				return &types.StepResult{
					StepID:  step.ID,
					Status:  types.ResultStatusSuccess,
					Metrics: make(map[string]float64),
				}, nil
			},
		)
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		workflow := &types.Workflow{
			ID: "test-workflow",
			PostHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo post"},
			},
		}

		workflowErr := errors.New("workflow failed")
		_, err := runner.ExecuteWorkflowPostHook(context.Background(), workflow, execCtx, workflowErr)
		require.NoError(t, err)

		// Verify the workflow error was added to context
		errVal, ok := capturedCtx.GetVariable("__workflow_error")
		assert.True(t, ok)
		assert.Equal(t, "workflow failed", errVal)
	})
}

func TestRunner_ExecuteStepPreHook(t *testing.T) {
	t.Run("no pre-hook returns nil", func(t *testing.T) {
		registry := executor.NewRegistry()
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		step := &types.Step{
			ID:      "step-1",
			PreHook: nil,
		}

		result, err := runner.ExecuteStepPreHook(context.Background(), step, execCtx)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("successful pre-hook execution", func(t *testing.T) {
		mockExec := NewMockExecutor("script")
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		step := &types.Step{
			ID: "step-1",
			PreHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo pre"},
			},
		}

		result, err := runner.ExecuteStepPreHook(context.Background(), step, execCtx)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, HookTypePre, result.HookType)
		assert.Equal(t, HookLevelStep, result.HookLevel)
		assert.Equal(t, "step-1", result.StepID)
		assert.Equal(t, types.ResultStatusSuccess, result.Status)
	})

	t.Run("failed pre-hook returns error", func(t *testing.T) {
		mockExec := NewMockExecutor("script").WithExecuteFunc(
			func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
				return &types.StepResult{
					StepID:  step.ID,
					Status:  types.ResultStatusFailed,
					Error:   errors.New("pre-hook failed"),
					Metrics: make(map[string]float64),
				}, nil
			},
		)
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		step := &types.Step{
			ID: "step-1",
			PreHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "exit 1"},
			},
		}

		result, err := runner.ExecuteStepPreHook(context.Background(), step, execCtx)
		require.Error(t, err)
		require.NotNil(t, result)
		assert.Equal(t, types.ResultStatusFailed, result.Status)
	})
}

func TestRunner_ExecuteStepPostHook(t *testing.T) {
	t.Run("no post-hook returns nil", func(t *testing.T) {
		registry := executor.NewRegistry()
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		step := &types.Step{
			ID:       "step-1",
			PostHook: nil,
		}

		result, err := runner.ExecuteStepPostHook(context.Background(), step, execCtx, nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("successful post-hook execution", func(t *testing.T) {
		mockExec := NewMockExecutor("script")
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		step := &types.Step{
			ID: "step-1",
			PostHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo post"},
			},
		}

		stepResult := &types.StepResult{
			StepID:   "step-1",
			Status:   types.ResultStatusSuccess,
			Duration: 100 * time.Millisecond,
			Output:   "step output",
		}

		result, err := runner.ExecuteStepPostHook(context.Background(), step, execCtx, stepResult)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, HookTypePost, result.HookType)
		assert.Equal(t, HookLevelStep, result.HookLevel)
		assert.Equal(t, "step-1", result.StepID)
		assert.Equal(t, types.ResultStatusSuccess, result.Status)
	})

	t.Run("post-hook receives step result in context", func(t *testing.T) {
		var capturedCtx *executor.ExecutionContext
		mockExec := NewMockExecutor("script").WithExecuteFunc(
			func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
				capturedCtx = execCtx
				return &types.StepResult{
					StepID:  step.ID,
					Status:  types.ResultStatusSuccess,
					Metrics: make(map[string]float64),
				}, nil
			},
		)
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		step := &types.Step{
			ID: "step-1",
			PostHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo post"},
			},
		}

		stepResult := &types.StepResult{
			StepID:   "step-1",
			Status:   types.ResultStatusFailed,
			Duration: 100 * time.Millisecond,
			Output:   "step output",
			Error:    errors.New("step failed"),
		}

		_, err := runner.ExecuteStepPostHook(context.Background(), step, execCtx, stepResult)
		require.NoError(t, err)

		// Verify the step result was added to context
		resultVal, ok := capturedCtx.GetVariable("__step_result")
		assert.True(t, ok)
		resultMap := resultVal.(map[string]any)
		assert.Equal(t, "failed", resultMap["status"])

		// Verify the step error was added to context
		errVal, ok := capturedCtx.GetVariable("__step_error")
		assert.True(t, ok)
		assert.Equal(t, "step failed", errVal)
	})
}

func TestRunner_ExecuteStepWithHooks(t *testing.T) {
	t.Run("executes step with pre and post hooks", func(t *testing.T) {
		executionOrder := []string{}
		mockExec := NewMockExecutor("script").WithExecuteFunc(
			func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
				executionOrder = append(executionOrder, step.ID)
				return &types.StepResult{
					StepID:  step.ID,
					Status:  types.ResultStatusSuccess,
					Metrics: make(map[string]float64),
				}, nil
			},
		)
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		step := &types.Step{
			ID: "main-step",
			PreHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo pre"},
			},
			PostHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo post"},
			},
		}

		stepExecutor := func(ctx context.Context, s *types.Step, ec *executor.ExecutionContext) (*types.StepResult, error) {
			executionOrder = append(executionOrder, "main-step-execution")
			return &types.StepResult{
				StepID:  s.ID,
				Status:  types.ResultStatusSuccess,
				Metrics: make(map[string]float64),
			}, nil
		}

		result := runner.ExecuteStepWithHooks(context.Background(), step, execCtx, stepExecutor)

		require.NotNil(t, result)
		assert.False(t, result.Skipped)
		assert.Nil(t, result.Error)
		assert.NotNil(t, result.PreHookResult)
		assert.NotNil(t, result.StepResult)
		assert.NotNil(t, result.PostHookResult)

		// Verify execution order: pre-hook -> step -> post-hook
		assert.Equal(t, []string{
			"__step_pre_hook_main-step",
			"main-step-execution",
			"__step_post_hook_main-step",
		}, executionOrder)
	})

	t.Run("skips step when pre-hook fails but still executes post-hook", func(t *testing.T) {
		executionOrder := []string{}
		callCount := 0
		mockExec := NewMockExecutor("script").WithExecuteFunc(
			func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
				executionOrder = append(executionOrder, step.ID)
				callCount++
				// First call is pre-hook - make it fail
				if callCount == 1 {
					return &types.StepResult{
						StepID:  step.ID,
						Status:  types.ResultStatusFailed,
						Error:   errors.New("pre-hook failed"),
						Metrics: make(map[string]float64),
					}, nil
				}
				// Second call is post-hook - should succeed
				return &types.StepResult{
					StepID:  step.ID,
					Status:  types.ResultStatusSuccess,
					Metrics: make(map[string]float64),
				}, nil
			},
		)
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		step := &types.Step{
			ID: "main-step",
			PreHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "exit 1"},
			},
			PostHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo post"},
			},
		}

		stepExecutorCalled := false
		stepExecutor := func(ctx context.Context, s *types.Step, ec *executor.ExecutionContext) (*types.StepResult, error) {
			stepExecutorCalled = true
			return &types.StepResult{
				StepID:  s.ID,
				Status:  types.ResultStatusSuccess,
				Metrics: make(map[string]float64),
			}, nil
		}

		result := runner.ExecuteStepWithHooks(context.Background(), step, execCtx, stepExecutor)

		require.NotNil(t, result)
		assert.True(t, result.Skipped)
		assert.NotNil(t, result.Error)
		assert.False(t, stepExecutorCalled, "step executor should not be called when pre-hook fails")
		assert.NotNil(t, result.PostHookResult, "post-hook should still execute when pre-hook fails")
		assert.Equal(t, types.ResultStatusSkipped, result.StepResult.Status)
	})

	t.Run("executes post-hook even when step fails", func(t *testing.T) {
		mockExec := NewMockExecutor("script")
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		step := &types.Step{
			ID: "main-step",
			PostHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo post"},
			},
		}

		stepExecutor := func(ctx context.Context, s *types.Step, ec *executor.ExecutionContext) (*types.StepResult, error) {
			return &types.StepResult{
				StepID:  s.ID,
				Status:  types.ResultStatusFailed,
				Error:   errors.New("step failed"),
				Metrics: make(map[string]float64),
			}, nil
		}

		result := runner.ExecuteStepWithHooks(context.Background(), step, execCtx, stepExecutor)

		require.NotNil(t, result)
		assert.False(t, result.Skipped)
		assert.Equal(t, types.ResultStatusFailed, result.StepResult.Status)
		assert.NotNil(t, result.PostHookResult, "post-hook should execute even when step fails")
		assert.Equal(t, types.ResultStatusSuccess, result.PostHookResult.Status)
	})
}

func TestRunner_ExecuteWorkflowWithHooks(t *testing.T) {
	t.Run("executes workflow with pre and post hooks", func(t *testing.T) {
		executionOrder := []string{}
		mockExec := NewMockExecutor("script").WithExecuteFunc(
			func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
				executionOrder = append(executionOrder, step.ID)
				return &types.StepResult{
					StepID:  step.ID,
					Status:  types.ResultStatusSuccess,
					Metrics: make(map[string]float64),
				}, nil
			},
		)
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		workflow := &types.Workflow{
			ID: "test-workflow",
			PreHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo pre"},
			},
			PostHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo post"},
			},
		}

		workflowExecutor := func(ctx context.Context, wf *types.Workflow, ec *executor.ExecutionContext) ([]*StepExecutionResult, error) {
			executionOrder = append(executionOrder, "workflow-execution")
			return []*StepExecutionResult{}, nil
		}

		result := runner.ExecuteWorkflowWithHooks(context.Background(), workflow, execCtx, workflowExecutor)

		require.NotNil(t, result)
		assert.False(t, result.Skipped)
		assert.Nil(t, result.Error)
		assert.NotNil(t, result.PreHookResult)
		assert.NotNil(t, result.PostHookResult)

		// Verify execution order: pre-hook -> workflow -> post-hook
		assert.Equal(t, []string{
			"__workflow_pre_hook",
			"workflow-execution",
			"__workflow_post_hook",
		}, executionOrder)
	})

	t.Run("skips workflow when pre-hook fails but still executes post-hook", func(t *testing.T) {
		callCount := 0
		mockExec := NewMockExecutor("script").WithExecuteFunc(
			func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
				callCount++
				// First call is pre-hook - make it fail
				if callCount == 1 {
					return &types.StepResult{
						StepID:  step.ID,
						Status:  types.ResultStatusFailed,
						Error:   errors.New("pre-hook failed"),
						Metrics: make(map[string]float64),
					}, nil
				}
				// Second call is post-hook - should succeed
				return &types.StepResult{
					StepID:  step.ID,
					Status:  types.ResultStatusSuccess,
					Metrics: make(map[string]float64),
				}, nil
			},
		)
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		workflow := &types.Workflow{
			ID: "test-workflow",
			PreHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "exit 1"},
			},
			PostHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo post"},
			},
		}

		workflowExecutorCalled := false
		workflowExecutor := func(ctx context.Context, wf *types.Workflow, ec *executor.ExecutionContext) ([]*StepExecutionResult, error) {
			workflowExecutorCalled = true
			return []*StepExecutionResult{}, nil
		}

		result := runner.ExecuteWorkflowWithHooks(context.Background(), workflow, execCtx, workflowExecutor)

		require.NotNil(t, result)
		assert.True(t, result.Skipped)
		assert.NotNil(t, result.Error)
		assert.False(t, workflowExecutorCalled, "workflow executor should not be called when pre-hook fails")
		assert.NotNil(t, result.PostHookResult, "post-hook should still execute when pre-hook fails")
	})

	t.Run("executes post-hook even when workflow fails", func(t *testing.T) {
		mockExec := NewMockExecutor("script")
		registry := setupTestRegistry(t, mockExec)
		runner := NewRunner(registry)
		execCtx := executor.NewExecutionContext()

		workflow := &types.Workflow{
			ID: "test-workflow",
			PostHook: &types.Hook{
				Type:   "script",
				Config: map[string]any{"inline": "echo post"},
			},
		}

		workflowExecutor := func(ctx context.Context, wf *types.Workflow, ec *executor.ExecutionContext) ([]*StepExecutionResult, error) {
			return nil, errors.New("workflow failed")
		}

		result := runner.ExecuteWorkflowWithHooks(context.Background(), workflow, execCtx, workflowExecutor)

		require.NotNil(t, result)
		assert.False(t, result.Skipped)
		assert.NotNil(t, result.Error)
		assert.NotNil(t, result.PostHookResult, "post-hook should execute even when workflow fails")
		assert.Equal(t, types.ResultStatusSuccess, result.PostHookResult.Status)
	})
}
