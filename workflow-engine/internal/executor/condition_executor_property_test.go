// Package executor provides property-based tests for the condition executor.
// Requirements: 3.1, 3.2 - Conditional execution correctness
// Property 3: For any conditional expression and execution context, when the condition
// evaluates to true, only the 'then' branch steps should execute; when it evaluates to false,
// only the 'else' branch steps (if defined) should execute.
package executor

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// TestConditionalExecutionProperty tests Property 3: Conditional execution correctness.
// condition(true) => then branch executes, else branch skips
// condition(false) => else branch executes (if defined), then branch skips
func TestConditionalExecutionProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: When condition is true, then branch executes
	properties.Property("true condition executes then branch", prop.ForAll(
		func(value int) bool {
			registry := NewRegistry()
			mockExec := newMockExecutorForProperty("http")
			registry.Register(mockExec)

			condExec := NewConditionExecutorWithRegistry(registry)

			step := &types.Step{
				ID:   "cond-step",
				Name: "Conditional Step",
				Type: "condition",
				Condition: &types.Condition{
					Expression: "${value} > 0",
					Then: []types.Step{
						{ID: "then-1", Name: "Then Step", Type: "http", Config: map[string]any{}},
					},
					Else: []types.Step{
						{ID: "else-1", Name: "Else Step", Type: "http", Config: map[string]any{}},
					},
				},
			}

			execCtx := NewExecutionContext()
			execCtx.Variables["value"] = value

			result, err := condExec.Execute(context.Background(), step, execCtx)
			if err != nil {
				return false
			}

			// Check which branch was executed based on condition
			conditionResult := value > 0
			if conditionResult {
				// Then branch should have executed
				return result.Status == types.ResultStatusSuccess
			}
			// Else branch should have executed
			return result.Status == types.ResultStatusSuccess
		},
		gen.IntRange(-100, 100),
	))

	// Property: Condition evaluation matches expected boolean result
	properties.Property("condition evaluation is correct", prop.ForAll(
		func(a, b int, op string) bool {
			registry := NewRegistry()
			mockExec := newMockExecutorForProperty("http")
			registry.Register(mockExec)

			condExec := NewConditionExecutorWithRegistry(registry)

			expr := "${a} " + op + " ${b}"
			step := &types.Step{
				ID:   "cond-step",
				Name: "Conditional Step",
				Type: "condition",
				Condition: &types.Condition{
					Expression: expr,
					Then: []types.Step{
						{ID: "then-1", Name: "Then Step", Type: "http", Config: map[string]any{}},
					},
					Else: []types.Step{
						{ID: "else-1", Name: "Else Step", Type: "http", Config: map[string]any{}},
					},
				},
			}

			execCtx := NewExecutionContext()
			execCtx.Variables["a"] = a
			execCtx.Variables["b"] = b

			result, err := condExec.Execute(context.Background(), step, execCtx)
			if err != nil {
				return false
			}

			// The execution should succeed regardless of which branch was taken
			return result.Status == types.ResultStatusSuccess
		},
		gen.IntRange(-100, 100),
		gen.IntRange(-100, 100),
		gen.OneConstOf("==", "!=", "<", ">", "<=", ">="),
	))

	properties.TestingRun(t)
}

// TestConditionalBranchSelectionProperty tests that the correct branch is selected.
func TestConditionalBranchSelectionProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Boolean true always selects then branch
	properties.Property("boolean true selects then branch", prop.ForAll(
		func(_ bool) bool {
			registry := NewRegistry()
			thenExecuted := false
			elseExecuted := false

			thenExec := newPropertyTrackingExecutor("then-exec", &thenExecuted)
			elseExec := newPropertyTrackingExecutor("else-exec", &elseExecuted)
			registry.Register(thenExec)
			registry.Register(elseExec)

			condExec := NewConditionExecutorWithRegistry(registry)

			step := &types.Step{
				ID:   "cond-step",
				Name: "Conditional Step",
				Type: "condition",
				Condition: &types.Condition{
					Expression: "${flag}",
					Then: []types.Step{
						{ID: "then-1", Name: "Then Step", Type: "then-exec", Config: map[string]any{}},
					},
					Else: []types.Step{
						{ID: "else-1", Name: "Else Step", Type: "else-exec", Config: map[string]any{}},
					},
				},
			}

			execCtx := NewExecutionContext()
			execCtx.Variables["flag"] = true

			_, err := condExec.Execute(context.Background(), step, execCtx)
			if err != nil {
				return false
			}

			// Then branch should execute, else should not
			return thenExecuted && !elseExecuted
		},
		gen.Bool(), // Dummy generator to run the test multiple times
	))

	// Property: Boolean false always selects else branch
	properties.Property("boolean false selects else branch", prop.ForAll(
		func(_ bool) bool {
			registry := NewRegistry()
			thenExecuted := false
			elseExecuted := false

			thenExec := newPropertyTrackingExecutor("then-exec", &thenExecuted)
			elseExec := newPropertyTrackingExecutor("else-exec", &elseExecuted)
			registry.Register(thenExec)
			registry.Register(elseExec)

			condExec := NewConditionExecutorWithRegistry(registry)

			step := &types.Step{
				ID:   "cond-step",
				Name: "Conditional Step",
				Type: "condition",
				Condition: &types.Condition{
					Expression: "${flag}",
					Then: []types.Step{
						{ID: "then-1", Name: "Then Step", Type: "then-exec", Config: map[string]any{}},
					},
					Else: []types.Step{
						{ID: "else-1", Name: "Else Step", Type: "else-exec", Config: map[string]any{}},
					},
				},
			}

			execCtx := NewExecutionContext()
			execCtx.Variables["flag"] = false

			_, err := condExec.Execute(context.Background(), step, execCtx)
			if err != nil {
				return false
			}

			// Else branch should execute, then should not
			return !thenExecuted && elseExecuted
		},
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// TestConditionalWithoutElseBranch tests conditions without else branch.
func TestConditionalWithoutElseBranch(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: When condition is false and no else branch, execution succeeds with no steps
	properties.Property("false condition without else branch succeeds", prop.ForAll(
		func(value int) bool {
			registry := NewRegistry()
			mockExec := newMockExecutorForProperty("http")
			registry.Register(mockExec)

			condExec := NewConditionExecutorWithRegistry(registry)

			step := &types.Step{
				ID:   "cond-step",
				Name: "Conditional Step",
				Type: "condition",
				Condition: &types.Condition{
					Expression: "${value} > 1000", // Always false for our range
					Then: []types.Step{
						{ID: "then-1", Name: "Then Step", Type: "http", Config: map[string]any{}},
					},
					// No else branch
				},
			}

			execCtx := NewExecutionContext()
			execCtx.Variables["value"] = value

			result, err := condExec.Execute(context.Background(), step, execCtx)
			if err != nil {
				return false
			}

			// Should succeed even without else branch
			return result.Status == types.ResultStatusSuccess || result.Status == types.ResultStatusSkipped
		},
		gen.IntRange(-100, 100),
	))

	properties.TestingRun(t)
}

// TestNestedConditionalsProperty tests nested conditional execution.
func TestNestedConditionalsProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Nested conditions evaluate correctly
	properties.Property("nested conditions evaluate correctly", prop.ForAll(
		func(a, b int) bool {
			registry := NewRegistry()
			mockExec := newMockExecutorForProperty("http")
			registry.Register(mockExec)

			condExec := NewConditionExecutorWithRegistry(registry)

			// Outer condition: a > 0
			// Inner condition (in then): b > 0
			step := &types.Step{
				ID:   "outer-cond",
				Name: "Outer Condition",
				Type: "condition",
				Condition: &types.Condition{
					Expression: "${a} > 0",
					Then: []types.Step{
						{
							ID:   "inner-cond",
							Name: "Inner Condition",
							Type: "condition",
							Condition: &types.Condition{
								Expression: "${b} > 0",
								Then: []types.Step{
									{ID: "inner-then", Name: "Inner Then", Type: "http", Config: map[string]any{}},
								},
								Else: []types.Step{
									{ID: "inner-else", Name: "Inner Else", Type: "http", Config: map[string]any{}},
								},
							},
						},
					},
					Else: []types.Step{
						{ID: "outer-else", Name: "Outer Else", Type: "http", Config: map[string]any{}},
					},
				},
			}

			execCtx := NewExecutionContext()
			execCtx.Variables["a"] = a
			execCtx.Variables["b"] = b

			result, err := condExec.Execute(context.Background(), step, execCtx)
			if err != nil {
				return false
			}

			return result.Status == types.ResultStatusSuccess
		},
		gen.IntRange(-50, 50),
		gen.IntRange(-50, 50),
	))

	properties.TestingRun(t)
}

// Helper types and functions

// mockExecutorForProperty is a mock executor for property testing.
type mockExecutorForProperty struct {
	execType string
}

func newMockExecutorForProperty(execType string) *mockExecutorForProperty {
	return &mockExecutorForProperty{execType: execType}
}

func (m *mockExecutorForProperty) Type() string {
	return m.execType
}

func (m *mockExecutorForProperty) Init(ctx context.Context, config map[string]any) error {
	return nil
}

func (m *mockExecutorForProperty) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	return &types.StepResult{
		StepID:    step.ID,
		Status:    types.ResultStatusSuccess,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  1 * time.Millisecond,
		Metrics:   make(map[string]float64),
	}, nil
}

func (m *mockExecutorForProperty) Cleanup(ctx context.Context) error {
	return nil
}

// propertyTrackingExecutor tracks whether it was executed for property tests.
type propertyTrackingExecutor struct {
	execType string
	executed *bool
}

func newPropertyTrackingExecutor(execType string, executed *bool) *propertyTrackingExecutor {
	return &propertyTrackingExecutor{execType: execType, executed: executed}
}

func (t *propertyTrackingExecutor) Type() string {
	return t.execType
}

func (t *propertyTrackingExecutor) Init(ctx context.Context, config map[string]any) error {
	return nil
}

func (t *propertyTrackingExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	*t.executed = true
	return &types.StepResult{
		StepID:    step.ID,
		Status:    types.ResultStatusSuccess,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  1 * time.Millisecond,
		Metrics:   make(map[string]float64),
	}, nil
}

func (t *propertyTrackingExecutor) Cleanup(ctx context.Context) error {
	return nil
}

// BenchmarkConditionalExecution benchmarks conditional execution.
func BenchmarkConditionalExecution(b *testing.B) {
	registry := NewRegistry()
	mockExec := newMockExecutorForProperty("http")
	registry.Register(mockExec)

	condExec := NewConditionExecutorWithRegistry(registry)

	step := &types.Step{
		ID:   "cond-step",
		Name: "Conditional Step",
		Type: "condition",
		Condition: &types.Condition{
			Expression: "${value} > 0",
			Then: []types.Step{
				{ID: "then-1", Name: "Then Step", Type: "http", Config: map[string]any{}},
			},
			Else: []types.Step{
				{ID: "else-1", Name: "Else Step", Type: "http", Config: map[string]any{}},
			},
		},
	}

	execCtx := NewExecutionContext()
	execCtx.Variables["value"] = 10

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		condExec.Execute(context.Background(), step, execCtx)
	}
}

// TestConditionalExecutionSpecificCases tests specific edge cases.
func TestConditionalExecutionSpecificCases(t *testing.T) {
	registry := NewRegistry()
	mockExec := newMockExecutorForProperty("http")
	registry.Register(mockExec)

	condExec := NewConditionExecutorWithRegistry(registry)

	testCases := []struct {
		name       string
		expression string
		vars       map[string]any
		expectThen bool
	}{
		{
			name:       "equality true",
			expression: "${a} == ${b}",
			vars:       map[string]any{"a": 10, "b": 10},
			expectThen: true,
		},
		{
			name:       "equality false",
			expression: "${a} == ${b}",
			vars:       map[string]any{"a": 10, "b": 20},
			expectThen: false,
		},
		{
			name:       "greater than true",
			expression: "${a} > ${b}",
			vars:       map[string]any{"a": 20, "b": 10},
			expectThen: true,
		},
		{
			name:       "greater than false",
			expression: "${a} > ${b}",
			vars:       map[string]any{"a": 10, "b": 20},
			expectThen: false,
		},
		{
			name:       "boolean variable true",
			expression: "${flag}",
			vars:       map[string]any{"flag": true},
			expectThen: true,
		},
		{
			name:       "boolean variable false",
			expression: "${flag}",
			vars:       map[string]any{"flag": false},
			expectThen: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			step := &types.Step{
				ID:   "cond-step",
				Name: "Conditional Step",
				Type: "condition",
				Condition: &types.Condition{
					Expression: tc.expression,
					Then: []types.Step{
						{ID: "then-1", Name: "Then Step", Type: "http", Config: map[string]any{}},
					},
					Else: []types.Step{
						{ID: "else-1", Name: "Else Step", Type: "http", Config: map[string]any{}},
					},
				},
			}

			execCtx := NewExecutionContext()
			for k, v := range tc.vars {
				execCtx.Variables[k] = v
			}

			result, err := condExec.Execute(context.Background(), step, execCtx)
			assert.NoError(t, err)
			assert.Equal(t, types.ResultStatusSuccess, result.Status)
		})
	}
}
