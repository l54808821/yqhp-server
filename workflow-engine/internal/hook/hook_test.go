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

// MockExecutor is a mock executor for testing.
type MockExecutor struct {
	execType      string
	executeFunc   func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error)
	initCalled    bool
	cleanupCalled bool
}

func NewMockExecutor(execType string) *MockExecutor {
	return &MockExecutor{
		execType: execType,
	}
}

func (m *MockExecutor) Type() string {
	return m.execType
}

func (m *MockExecutor) Init(ctx context.Context, config map[string]any) error {
	m.initCalled = true
	return nil
}

func (m *MockExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, step, execCtx)
	}
	return &types.StepResult{
		StepID:    step.ID,
		Status:    types.ResultStatusSuccess,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  10 * time.Millisecond,
		Output:    "mock output",
		Metrics:   make(map[string]float64),
	}, nil
}

func (m *MockExecutor) Cleanup(ctx context.Context) error {
	m.cleanupCalled = true
	return nil
}

func (m *MockExecutor) WithExecuteFunc(fn func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error)) *MockExecutor {
	m.executeFunc = fn
	return m
}

func TestHookError(t *testing.T) {
	t.Run("error message with step ID", func(t *testing.T) {
		err := NewHookError(HookTypePre, HookLevelStep, "step-1", "test error", errors.New("cause"))
		assert.Contains(t, err.Error(), "step-pre hook for step step-1")
		assert.Contains(t, err.Error(), "test error")
		assert.Contains(t, err.Error(), "cause")
	})

	t.Run("error message without step ID", func(t *testing.T) {
		err := NewHookError(HookTypePost, HookLevelWorkflow, "", "test error", errors.New("cause"))
		assert.Contains(t, err.Error(), "workflow-post hook")
		assert.Contains(t, err.Error(), "test error")
	})

	t.Run("unwrap returns cause", func(t *testing.T) {
		cause := errors.New("original error")
		err := NewHookError(HookTypePre, HookLevelStep, "step-1", "wrapped", cause)
		assert.Equal(t, cause, err.Unwrap())
	})
}

func TestIsHookError(t *testing.T) {
	t.Run("returns true for HookError", func(t *testing.T) {
		err := NewHookError(HookTypePre, HookLevelStep, "step-1", "test", nil)
		assert.True(t, IsHookError(err))
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		err := errors.New("not a hook error")
		assert.False(t, IsHookError(err))
	})
}

func TestHookExecutor(t *testing.T) {
	t.Run("execute hook successfully", func(t *testing.T) {
		registry := executor.NewRegistry()
		mockExec := NewMockExecutor("script")
		require.NoError(t, registry.Register(mockExec))

		hookExec := NewHookExecutor(registry)
		execCtx := executor.NewExecutionContext()

		hook := &types.Hook{
			Type: "script",
			Config: map[string]any{
				"inline": "echo hello",
			},
		}

		result, err := hookExec.ExecuteHook(
			context.Background(),
			hook,
			HookTypePre,
			HookLevelWorkflow,
			"",
			execCtx,
		)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, HookTypePre, result.HookType)
		assert.Equal(t, HookLevelWorkflow, result.HookLevel)
		assert.Equal(t, types.ResultStatusSuccess, result.Status)
	})

	t.Run("execute hook with nil hook returns nil", func(t *testing.T) {
		registry := executor.NewRegistry()
		hookExec := NewHookExecutor(registry)
		execCtx := executor.NewExecutionContext()

		result, err := hookExec.ExecuteHook(
			context.Background(),
			nil,
			HookTypePre,
			HookLevelWorkflow,
			"",
			execCtx,
		)

		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("execute hook with unknown executor type", func(t *testing.T) {
		registry := executor.NewRegistry()
		hookExec := NewHookExecutor(registry)
		execCtx := executor.NewExecutionContext()

		hook := &types.Hook{
			Type:   "unknown",
			Config: map[string]any{},
		}

		result, err := hookExec.ExecuteHook(
			context.Background(),
			hook,
			HookTypePre,
			HookLevelStep,
			"step-1",
			execCtx,
		)

		require.Error(t, err)
		require.NotNil(t, result)
		assert.Equal(t, types.ResultStatusFailed, result.Status)
		assert.True(t, IsHookError(err))
	})

	t.Run("execute hook with failed execution", func(t *testing.T) {
		registry := executor.NewRegistry()
		mockExec := NewMockExecutor("script").WithExecuteFunc(
			func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
				return &types.StepResult{
					StepID:    step.ID,
					Status:    types.ResultStatusFailed,
					StartTime: time.Now(),
					EndTime:   time.Now(),
					Error:     errors.New("execution failed"),
					Metrics:   make(map[string]float64),
				}, nil
			},
		)
		require.NoError(t, registry.Register(mockExec))

		hookExec := NewHookExecutor(registry)
		execCtx := executor.NewExecutionContext()

		hook := &types.Hook{
			Type:   "script",
			Config: map[string]any{},
		}

		result, err := hookExec.ExecuteHook(
			context.Background(),
			hook,
			HookTypePre,
			HookLevelStep,
			"step-1",
			execCtx,
		)

		require.Error(t, err)
		require.NotNil(t, result)
		assert.Equal(t, types.ResultStatusFailed, result.Status)
	})

	t.Run("uses default registry when nil", func(t *testing.T) {
		hookExec := NewHookExecutor(nil)
		assert.NotNil(t, hookExec.registry)
	})
}

func TestGenerateHookStepID(t *testing.T) {
	hookExec := NewHookExecutor(nil)

	t.Run("workflow level hook", func(t *testing.T) {
		id := hookExec.generateHookStepID(HookTypePre, HookLevelWorkflow, "")
		assert.Equal(t, "__workflow_pre_hook", id)
	})

	t.Run("step level hook", func(t *testing.T) {
		id := hookExec.generateHookStepID(HookTypePost, HookLevelStep, "my-step")
		assert.Equal(t, "__step_post_hook_my-step", id)
	})
}
