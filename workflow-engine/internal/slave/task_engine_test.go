package slave

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// mockExecutor is a simple executor for testing.
type mockExecutor struct {
	*executor.BaseExecutor
	executeFunc func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error)
}

func newMockExecutor(execType string) *mockExecutor {
	return &mockExecutor{
		BaseExecutor: executor.NewBaseExecutor(execType),
	}
}

func (m *mockExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, step, execCtx)
	}
	return executor.CreateSuccessResult(step.ID, time.Now(), nil), nil
}

func TestNewTaskEngine(t *testing.T) {
	registry := executor.NewRegistry()
	engine := NewTaskEngine(registry, 100)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.vuPool)
	assert.NotNil(t, engine.collector)
	assert.Equal(t, 100, engine.maxVUs)
}

func TestTaskEngine_Execute_NilTask(t *testing.T) {
	registry := executor.NewRegistry()
	engine := NewTaskEngine(registry, 100)

	result, err := engine.Execute(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestTaskEngine_Execute_NilWorkflow(t *testing.T) {
	registry := executor.NewRegistry()
	engine := NewTaskEngine(registry, 100)

	task := &types.Task{
		ID:       "task-1",
		Workflow: nil,
	}

	result, err := engine.Execute(context.Background(), task)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestTaskEngine_Execute_SimpleWorkflow(t *testing.T) {
	registry := executor.NewRegistry()

	// Register mock executor
	mock := newMockExecutor("mock")
	mock.executeFunc = func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
		return executor.CreateSuccessResult(step.ID, time.Now(), "success"), nil
	}
	err := registry.Register(mock)
	require.NoError(t, err)

	engine := NewTaskEngine(registry, 10)

	workflow := &types.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Steps: []types.Step{
			{
				ID:   "step-1",
				Name: "Step 1",
				Type: "mock",
			},
		},
		Options: types.ExecutionOptions{
			VUs:        1,
			Iterations: 1,
		},
	}

	task := &types.Task{
		ID:          "task-1",
		ExecutionID: "exec-1",
		Workflow:    workflow,
		Segment:     types.ExecutionSegment{Start: 0, End: 1},
	}

	result, err := engine.Execute(context.Background(), task)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, types.ExecutionStatusCompleted, result.Status)
}

func TestTaskEngine_Execute_WithDuration(t *testing.T) {
	registry := executor.NewRegistry()

	mock := newMockExecutor("mock")
	mock.executeFunc = func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
		time.Sleep(10 * time.Millisecond)
		return executor.CreateSuccessResult(step.ID, time.Now(), "success"), nil
	}
	err := registry.Register(mock)
	require.NoError(t, err)

	engine := NewTaskEngine(registry, 10)

	workflow := &types.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Steps: []types.Step{
			{
				ID:   "step-1",
				Name: "Step 1",
				Type: "mock",
			},
		},
		Options: types.ExecutionOptions{
			VUs:      2,
			Duration: 100 * time.Millisecond,
		},
	}

	task := &types.Task{
		ID:          "task-1",
		ExecutionID: "exec-1",
		Workflow:    workflow,
		Segment:     types.ExecutionSegment{Start: 0, End: 1},
	}

	start := time.Now()
	result, err := engine.Execute(context.Background(), task)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, types.ExecutionStatusCompleted, result.Status)
	// Should complete around the duration time
	assert.True(t, elapsed >= 100*time.Millisecond)
	assert.True(t, elapsed < 200*time.Millisecond)
}

func TestTaskEngine_Execute_PerVUIterations(t *testing.T) {
	registry := executor.NewRegistry()

	executionCount := 0
	mock := newMockExecutor("mock")
	mock.executeFunc = func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
		executionCount++
		return executor.CreateSuccessResult(step.ID, time.Now(), "success"), nil
	}
	err := registry.Register(mock)
	require.NoError(t, err)

	engine := NewTaskEngine(registry, 10)

	workflow := &types.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Steps: []types.Step{
			{
				ID:   "step-1",
				Name: "Step 1",
				Type: "mock",
			},
		},
		Options: types.ExecutionOptions{
			VUs:           2,
			Iterations:    3, // 3 iterations per VU
			ExecutionMode: types.ModePerVUIterations,
		},
	}

	task := &types.Task{
		ID:          "task-1",
		ExecutionID: "exec-1",
		Workflow:    workflow,
		Segment:     types.ExecutionSegment{Start: 0, End: 1},
	}

	result, err := engine.Execute(context.Background(), task)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, types.ExecutionStatusCompleted, result.Status)
	// 2 VUs * 3 iterations = 6 executions
	assert.Equal(t, 6, executionCount)
}

func TestTaskEngine_Execute_SharedIterations(t *testing.T) {
	registry := executor.NewRegistry()

	executionCount := 0
	mock := newMockExecutor("mock")
	mock.executeFunc = func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
		executionCount++
		return executor.CreateSuccessResult(step.ID, time.Now(), "success"), nil
	}
	err := registry.Register(mock)
	require.NoError(t, err)

	engine := NewTaskEngine(registry, 10)

	workflow := &types.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Steps: []types.Step{
			{
				ID:   "step-1",
				Name: "Step 1",
				Type: "mock",
			},
		},
		Options: types.ExecutionOptions{
			VUs:           3,
			Iterations:    10, // 10 total iterations shared
			ExecutionMode: types.ModeSharedIterations,
		},
	}

	task := &types.Task{
		ID:          "task-1",
		ExecutionID: "exec-1",
		Workflow:    workflow,
		Segment:     types.ExecutionSegment{Start: 0, End: 1},
	}

	result, err := engine.Execute(context.Background(), task)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, types.ExecutionStatusCompleted, result.Status)
	// Should have exactly 10 executions
	assert.Equal(t, 10, executionCount)
}

func TestTaskEngine_Execute_WithSegment(t *testing.T) {
	registry := executor.NewRegistry()

	mock := newMockExecutor("mock")
	err := registry.Register(mock)
	require.NoError(t, err)

	engine := NewTaskEngine(registry, 100)

	workflow := &types.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Steps: []types.Step{
			{
				ID:   "step-1",
				Name: "Step 1",
				Type: "mock",
			},
		},
		Options: types.ExecutionOptions{
			VUs:        10,
			Iterations: 1,
		},
	}

	// Only execute 50% of the work
	task := &types.Task{
		ID:          "task-1",
		ExecutionID: "exec-1",
		Workflow:    workflow,
		Segment:     types.ExecutionSegment{Start: 0, End: 0.5},
	}

	result, err := engine.Execute(context.Background(), task)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, types.ExecutionStatusCompleted, result.Status)
}

func TestTaskEngine_GetMetrics(t *testing.T) {
	registry := executor.NewRegistry()
	engine := NewTaskEngine(registry, 100)

	metrics := engine.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, 0, metrics.ActiveVUs)
}

func TestTaskEngine_Stop(t *testing.T) {
	registry := executor.NewRegistry()
	engine := NewTaskEngine(registry, 100)

	err := engine.Stop(context.Background())
	assert.NoError(t, err)
	assert.False(t, engine.running.Load())
}

func TestTaskEngine_CalculateVUs(t *testing.T) {
	registry := executor.NewRegistry()
	engine := NewTaskEngine(registry, 50)

	tests := []struct {
		name     string
		opts     types.ExecutionOptions
		segment  types.ExecutionSegment
		expected int
	}{
		{
			name:     "full segment",
			opts:     types.ExecutionOptions{VUs: 10},
			segment:  types.ExecutionSegment{Start: 0, End: 1},
			expected: 10,
		},
		{
			name:     "half segment",
			opts:     types.ExecutionOptions{VUs: 10},
			segment:  types.ExecutionSegment{Start: 0, End: 0.5},
			expected: 5,
		},
		{
			name:     "quarter segment",
			opts:     types.ExecutionOptions{VUs: 10},
			segment:  types.ExecutionSegment{Start: 0, End: 0.25},
			expected: 2,
		},
		{
			name:     "exceeds max",
			opts:     types.ExecutionOptions{VUs: 100},
			segment:  types.ExecutionSegment{Start: 0, End: 1},
			expected: 50, // capped at maxVUs
		},
		{
			name:     "zero VUs defaults to 1",
			opts:     types.ExecutionOptions{VUs: 0},
			segment:  types.ExecutionSegment{Start: 0, End: 1},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.calculateVUs(tt.opts, tt.segment)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTaskEngine_CalculateIterations(t *testing.T) {
	registry := executor.NewRegistry()
	engine := NewTaskEngine(registry, 100)

	tests := []struct {
		name     string
		opts     types.ExecutionOptions
		segment  types.ExecutionSegment
		expected int
	}{
		{
			name:     "full segment",
			opts:     types.ExecutionOptions{Iterations: 100},
			segment:  types.ExecutionSegment{Start: 0, End: 1},
			expected: 100,
		},
		{
			name:     "half segment",
			opts:     types.ExecutionOptions{Iterations: 100},
			segment:  types.ExecutionSegment{Start: 0, End: 0.5},
			expected: 50,
		},
		{
			name:     "zero iterations (duration-based)",
			opts:     types.ExecutionOptions{Iterations: 0},
			segment:  types.ExecutionSegment{Start: 0, End: 1},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.calculateIterations(tt.opts, tt.segment)
			assert.Equal(t, tt.expected, result)
		})
	}
}
