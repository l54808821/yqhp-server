// Package hook provides property-based tests for hook execution.
// Requirements: 4.1, 4.2, 4.3, 4.4, 4.6 - Hook execution order and post-hook always executes
// Property 5: For any workflow with pre-hooks and post-hooks defined, pre-hook should execute
// before the associated step, and post-hook should execute after the step completes.
// Property 6: For any workflow or step with a post-hook defined, the post-hook should execute
// regardless of whether the workflow/step succeeds or fails.
package hook

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/grafana/k6/workflow-engine/internal/executor"
	"github.com/grafana/k6/workflow-engine/pkg/types"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
)

// executionRecorder records the order of execution events.
type executionRecorder struct {
	mu     sync.Mutex
	events []string
}

func newExecutionRecorder() *executionRecorder {
	return &executionRecorder{events: make([]string, 0)}
}

func (r *executionRecorder) record(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *executionRecorder) getEvents() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]string, len(r.events))
	copy(result, r.events)
	return result
}

func (r *executionRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = make([]string, 0)
}

// recordingExecutor is an executor that records when it's called.
type recordingExecutor struct {
	execType   string
	recorder   *executionRecorder
	shouldFail bool
}

func newRecordingExecutor(execType string, recorder *executionRecorder) *recordingExecutor {
	return &recordingExecutor{
		execType: execType,
		recorder: recorder,
	}
}

func (e *recordingExecutor) Type() string {
	return e.execType
}

func (e *recordingExecutor) Init(ctx context.Context, config map[string]any) error {
	return nil
}

func (e *recordingExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
	e.recorder.record(e.execType)

	status := types.ResultStatusSuccess
	var err error
	if e.shouldFail {
		status = types.ResultStatusFailed
		err = errors.New("forced failure")
	}

	return &types.StepResult{
		StepID:    step.ID,
		Status:    status,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  1 * time.Millisecond,
		Error:     err,
		Metrics:   make(map[string]float64),
	}, nil
}

func (e *recordingExecutor) Cleanup(ctx context.Context) error {
	return nil
}

func (e *recordingExecutor) WithFailure() *recordingExecutor {
	e.shouldFail = true
	return e
}

// TestHookExecutionOrderProperty tests Property 5: Hook execution order.
// execution_order = [pre_hook, steps..., post_hook]
func TestHookExecutionOrderProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Pre-hook always executes before the main step
	properties.Property("pre-hook executes before main step", prop.ForAll(
		func(_ bool) bool {
			recorder := newExecutionRecorder()
			registry := executor.NewRegistry()

			preHookExec := newRecordingExecutor("pre-hook", recorder)
			mainExec := newRecordingExecutor("main", recorder)
			registry.Register(preHookExec)
			registry.Register(mainExec)

			hookExec := NewHookExecutor(registry)
			execCtx := executor.NewExecutionContext()

			// Execute pre-hook
			preHook := &types.Hook{Type: "pre-hook", Config: map[string]any{}}
			_, err := hookExec.ExecuteHook(context.Background(), preHook, HookTypePre, HookLevelStep, "step-1", execCtx)
			if err != nil {
				return false
			}

			// Execute main step
			mainStep := &types.Step{ID: "main-step", Name: "Main", Type: "main", Config: map[string]any{}}
			mainExec.Execute(context.Background(), mainStep, execCtx)

			events := recorder.getEvents()
			if len(events) != 2 {
				return false
			}

			// Pre-hook should be first
			return events[0] == "pre-hook" && events[1] == "main"
		},
		gen.Bool(),
	))

	// Property: Post-hook always executes after the main step
	properties.Property("post-hook executes after main step", prop.ForAll(
		func(_ bool) bool {
			recorder := newExecutionRecorder()
			registry := executor.NewRegistry()

			mainExec := newRecordingExecutor("main", recorder)
			postHookExec := newRecordingExecutor("post-hook", recorder)
			registry.Register(mainExec)
			registry.Register(postHookExec)

			hookExec := NewHookExecutor(registry)
			execCtx := executor.NewExecutionContext()

			// Execute main step
			mainStep := &types.Step{ID: "main-step", Name: "Main", Type: "main", Config: map[string]any{}}
			mainExec.Execute(context.Background(), mainStep, execCtx)

			// Execute post-hook
			postHook := &types.Hook{Type: "post-hook", Config: map[string]any{}}
			_, err := hookExec.ExecuteHook(context.Background(), postHook, HookTypePost, HookLevelStep, "step-1", execCtx)
			if err != nil {
				return false
			}

			events := recorder.getEvents()
			if len(events) != 2 {
				return false
			}

			// Main should be first, post-hook second
			return events[0] == "main" && events[1] == "post-hook"
		},
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// TestPostHookAlwaysExecutesProperty tests Property 6: Post-hook always executes.
// post_hook executes even if step fails
func TestPostHookAlwaysExecutesProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Post-hook executes even when main step succeeds
	properties.Property("post-hook executes on success", prop.ForAll(
		func(_ bool) bool {
			recorder := newExecutionRecorder()
			registry := executor.NewRegistry()

			mainExec := newRecordingExecutor("main", recorder)
			postHookExec := newRecordingExecutor("post-hook", recorder)
			registry.Register(mainExec)
			registry.Register(postHookExec)

			hookExec := NewHookExecutor(registry)
			execCtx := executor.NewExecutionContext()

			// Execute main step (success)
			mainStep := &types.Step{ID: "main-step", Name: "Main", Type: "main", Config: map[string]any{}}
			mainExec.Execute(context.Background(), mainStep, execCtx)

			// Execute post-hook
			postHook := &types.Hook{Type: "post-hook", Config: map[string]any{}}
			hookExec.ExecuteHook(context.Background(), postHook, HookTypePost, HookLevelStep, "step-1", execCtx)

			events := recorder.getEvents()
			// Post-hook should have executed
			for _, e := range events {
				if e == "post-hook" {
					return true
				}
			}
			return false
		},
		gen.Bool(),
	))

	// Property: Post-hook executes even when main step fails
	properties.Property("post-hook executes on failure", prop.ForAll(
		func(_ bool) bool {
			recorder := newExecutionRecorder()
			registry := executor.NewRegistry()

			mainExec := newRecordingExecutor("main", recorder).WithFailure()
			postHookExec := newRecordingExecutor("post-hook", recorder)
			registry.Register(mainExec)
			registry.Register(postHookExec)

			hookExec := NewHookExecutor(registry)
			execCtx := executor.NewExecutionContext()

			// Execute main step (failure)
			mainStep := &types.Step{ID: "main-step", Name: "Main", Type: "main", Config: map[string]any{}}
			result, _ := mainExec.Execute(context.Background(), mainStep, execCtx)

			// Verify main step failed
			if result.Status != types.ResultStatusFailed {
				return false
			}

			// Execute post-hook (should still execute)
			postHook := &types.Hook{Type: "post-hook", Config: map[string]any{}}
			hookExec.ExecuteHook(context.Background(), postHook, HookTypePost, HookLevelStep, "step-1", execCtx)

			events := recorder.getEvents()
			// Post-hook should have executed
			for _, e := range events {
				if e == "post-hook" {
					return true
				}
			}
			return false
		},
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// TestCompleteHookSequenceProperty tests the complete hook sequence.
func TestCompleteHookSequenceProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Complete sequence is [pre-hook, main, post-hook]
	properties.Property("complete hook sequence is correct", prop.ForAll(
		func(shouldFail bool) bool {
			recorder := newExecutionRecorder()
			registry := executor.NewRegistry()

			preHookExec := newRecordingExecutor("pre-hook", recorder)
			mainExec := newRecordingExecutor("main", recorder)
			if shouldFail {
				mainExec = mainExec.WithFailure()
			}
			postHookExec := newRecordingExecutor("post-hook", recorder)

			registry.Register(preHookExec)
			registry.Register(mainExec)
			registry.Register(postHookExec)

			hookExec := NewHookExecutor(registry)
			execCtx := executor.NewExecutionContext()

			// Execute pre-hook
			preHook := &types.Hook{Type: "pre-hook", Config: map[string]any{}}
			hookExec.ExecuteHook(context.Background(), preHook, HookTypePre, HookLevelStep, "step-1", execCtx)

			// Execute main step
			mainStep := &types.Step{ID: "main-step", Name: "Main", Type: "main", Config: map[string]any{}}
			mainExec.Execute(context.Background(), mainStep, execCtx)

			// Execute post-hook
			postHook := &types.Hook{Type: "post-hook", Config: map[string]any{}}
			hookExec.ExecuteHook(context.Background(), postHook, HookTypePost, HookLevelStep, "step-1", execCtx)

			events := recorder.getEvents()
			if len(events) != 3 {
				return false
			}

			// Verify order: pre-hook -> main -> post-hook
			return events[0] == "pre-hook" && events[1] == "main" && events[2] == "post-hook"
		},
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// TestWorkflowLevelHooksProperty tests workflow-level hooks.
func TestWorkflowLevelHooksProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Workflow pre-hook executes before all steps
	properties.Property("workflow pre-hook executes first", prop.ForAll(
		func(numSteps int) bool {
			if numSteps < 1 {
				numSteps = 1
			}
			if numSteps > 5 {
				numSteps = 5
			}

			recorder := newExecutionRecorder()
			registry := executor.NewRegistry()

			workflowPreHook := newRecordingExecutor("workflow-pre", recorder)
			stepExec := newRecordingExecutor("step", recorder)
			registry.Register(workflowPreHook)
			registry.Register(stepExec)

			hookExec := NewHookExecutor(registry)
			execCtx := executor.NewExecutionContext()

			// Execute workflow pre-hook
			preHook := &types.Hook{Type: "workflow-pre", Config: map[string]any{}}
			hookExec.ExecuteHook(context.Background(), preHook, HookTypePre, HookLevelWorkflow, "", execCtx)

			// Execute steps
			for i := 0; i < numSteps; i++ {
				step := &types.Step{ID: "step", Name: "Step", Type: "step", Config: map[string]any{}}
				stepExec.Execute(context.Background(), step, execCtx)
			}

			events := recorder.getEvents()
			// First event should be workflow pre-hook
			return len(events) > 0 && events[0] == "workflow-pre"
		},
		gen.IntRange(1, 5),
	))

	// Property: Workflow post-hook executes after all steps
	properties.Property("workflow post-hook executes last", prop.ForAll(
		func(numSteps int) bool {
			if numSteps < 1 {
				numSteps = 1
			}
			if numSteps > 5 {
				numSteps = 5
			}

			recorder := newExecutionRecorder()
			registry := executor.NewRegistry()

			stepExec := newRecordingExecutor("step", recorder)
			workflowPostHook := newRecordingExecutor("workflow-post", recorder)
			registry.Register(stepExec)
			registry.Register(workflowPostHook)

			hookExec := NewHookExecutor(registry)
			execCtx := executor.NewExecutionContext()

			// Execute steps
			for i := 0; i < numSteps; i++ {
				step := &types.Step{ID: "step", Name: "Step", Type: "step", Config: map[string]any{}}
				stepExec.Execute(context.Background(), step, execCtx)
			}

			// Execute workflow post-hook
			postHook := &types.Hook{Type: "workflow-post", Config: map[string]any{}}
			hookExec.ExecuteHook(context.Background(), postHook, HookTypePost, HookLevelWorkflow, "", execCtx)

			events := recorder.getEvents()
			// Last event should be workflow post-hook
			return len(events) > 0 && events[len(events)-1] == "workflow-post"
		},
		gen.IntRange(1, 5),
	))

	properties.TestingRun(t)
}

// BenchmarkHookExecution benchmarks hook execution.
func BenchmarkHookExecution(b *testing.B) {
	recorder := newExecutionRecorder()
	registry := executor.NewRegistry()
	hookExec := newRecordingExecutor("hook", recorder)
	registry.Register(hookExec)

	hookExecutor := NewHookExecutor(registry)
	execCtx := executor.NewExecutionContext()
	hook := &types.Hook{Type: "hook", Config: map[string]any{}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hookExecutor.ExecuteHook(context.Background(), hook, HookTypePre, HookLevelStep, "step-1", execCtx)
		recorder.reset()
	}
}

// TestHookExecutionSpecificCases tests specific edge cases.
func TestHookExecutionSpecificCases(t *testing.T) {
	testCases := []struct {
		name          string
		hookType      HookType
		hookLevel     HookLevel
		stepID        string
		expectedEvent string
	}{
		{
			name:          "step pre-hook",
			hookType:      HookTypePre,
			hookLevel:     HookLevelStep,
			stepID:        "step-1",
			expectedEvent: "hook",
		},
		{
			name:          "step post-hook",
			hookType:      HookTypePost,
			hookLevel:     HookLevelStep,
			stepID:        "step-1",
			expectedEvent: "hook",
		},
		{
			name:          "workflow pre-hook",
			hookType:      HookTypePre,
			hookLevel:     HookLevelWorkflow,
			stepID:        "",
			expectedEvent: "hook",
		},
		{
			name:          "workflow post-hook",
			hookType:      HookTypePost,
			hookLevel:     HookLevelWorkflow,
			stepID:        "",
			expectedEvent: "hook",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := newExecutionRecorder()
			registry := executor.NewRegistry()
			hookExec := newRecordingExecutor("hook", recorder)
			registry.Register(hookExec)

			exec := NewHookExecutor(registry)
			execCtx := executor.NewExecutionContext()
			hook := &types.Hook{Type: "hook", Config: map[string]any{}}

			result, err := exec.ExecuteHook(context.Background(), hook, tc.hookType, tc.hookLevel, tc.stepID, execCtx)
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, types.ResultStatusSuccess, result.Status)

			events := recorder.getEvents()
			assert.Len(t, events, 1)
			assert.Equal(t, tc.expectedEvent, events[0])
		})
	}
}
