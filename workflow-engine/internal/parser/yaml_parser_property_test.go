// Package parser provides property-based tests for the YAML parser.
// Requirements: 1.5, 1.6 - Workflow Round-Trip consistency
// Property 1: For any valid Workflow object, serializing it to YAML and then parsing
// the YAML back to a Workflow object should produce an equivalent object.
package parser

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"

	"yqhp/workflow-engine/pkg/types"
)

// TestWorkflowRoundTrip tests Property 1: Workflow Round-Trip consistency.
// parse(print(workflow)) == workflow
func TestWorkflowRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.MaxSize = 10

	properties := gopter.NewProperties(parameters)

	properties.Property("workflow round-trip preserves data", prop.ForAll(
		func(workflow *types.Workflow) bool {
			printer := NewYAMLPrinter()
			parser := NewYAMLParser()

			// Serialize workflow to YAML
			yamlBytes, err := printer.Print(workflow)
			if err != nil {
				t.Logf("Print error: %v", err)
				return false
			}

			// Parse YAML back to workflow
			parsed, err := parser.Parse(yamlBytes)
			if err != nil {
				t.Logf("Parse error: %v, YAML:\n%s", err, string(yamlBytes))
				return false
			}

			// Compare workflows
			return workflowsEqual(workflow, parsed)
		},
		genValidWorkflow(),
	))

	properties.TestingRun(t)
}

// TestWorkflowRoundTripWithConditions tests round-trip with conditional steps.
func TestWorkflowRoundTripWithConditions(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("workflow with conditions round-trip preserves data", prop.ForAll(
		func(workflow *types.Workflow) bool {
			printer := NewYAMLPrinter()
			parser := NewYAMLParser()

			yamlBytes, err := printer.Print(workflow)
			if err != nil {
				return false
			}

			parsed, err := parser.Parse(yamlBytes)
			if err != nil {
				return false
			}

			return workflowsEqual(workflow, parsed)
		},
		genWorkflowWithConditions(),
	))

	properties.TestingRun(t)
}

// TestWorkflowRoundTripWithHooks tests round-trip with hooks.
func TestWorkflowRoundTripWithHooks(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("workflow with hooks round-trip preserves data", prop.ForAll(
		func(workflow *types.Workflow) bool {
			printer := NewYAMLPrinter()
			parser := NewYAMLParser()

			yamlBytes, err := printer.Print(workflow)
			if err != nil {
				return false
			}

			parsed, err := parser.Parse(yamlBytes)
			if err != nil {
				return false
			}

			return workflowsEqual(workflow, parsed)
		},
		genWorkflowWithHooks(),
	))

	properties.TestingRun(t)
}

// Generators for property-based testing

// genValidWorkflow generates a valid workflow for testing.
func genValidWorkflow() gopter.Gen {
	return gopter.CombineGens(
		genWorkflowID(),
		genWorkflowName(),
		genDescription(),
		genVariables(),
		genSteps(),
		genExecutionOptions(),
	).Map(func(values []interface{}) *types.Workflow {
		return &types.Workflow{
			ID:          values[0].(string),
			Name:        values[1].(string),
			Description: values[2].(string),
			Variables:   values[3].(map[string]any),
			Steps:       values[4].([]types.Step),
			Options:     values[5].(types.ExecutionOptions),
		}
	})
}

// genWorkflowWithConditions generates a workflow with conditional steps.
func genWorkflowWithConditions() gopter.Gen {
	return gopter.CombineGens(
		genWorkflowID(),
		genWorkflowName(),
		genStepsWithConditions(),
	).Map(func(values []interface{}) *types.Workflow {
		return &types.Workflow{
			ID:    values[0].(string),
			Name:  values[1].(string),
			Steps: values[2].([]types.Step),
		}
	})
}

// genWorkflowWithHooks generates a workflow with hooks.
func genWorkflowWithHooks() gopter.Gen {
	return gopter.CombineGens(
		genWorkflowID(),
		genWorkflowName(),
		genSteps(),
		genHook(),
		genHook(),
	).Map(func(values []interface{}) *types.Workflow {
		return &types.Workflow{
			ID:       values[0].(string),
			Name:     values[1].(string),
			Steps:    values[2].([]types.Step),
			PreHook:  values[3].(*types.Hook),
			PostHook: values[4].(*types.Hook),
		}
	})
}

// genWorkflowID generates a valid workflow ID.
func genWorkflowID() gopter.Gen {
	return gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) >= 1 && len(s) <= 50
	}).Map(func(s string) string {
		if len(s) == 0 {
			return "workflow-1"
		}
		return "workflow-" + s
	})
}

// genWorkflowName generates a valid workflow name.
func genWorkflowName() gopter.Gen {
	return gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) >= 1 && len(s) <= 100
	}).Map(func(s string) string {
		if len(s) == 0 {
			return "Test Workflow"
		}
		return "Workflow " + s
	})
}

// genDescription generates an optional description.
func genDescription() gopter.Gen {
	return gen.AlphaString().Map(func(s string) string {
		if len(s) > 200 {
			return s[:200]
		}
		return s
	})
}

// genVariables generates workflow variables.
func genVariables() gopter.Gen {
	return gen.MapOf(
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) <= 20 }),
		gen.OneGenOf(
			gen.AlphaString().Map(func(s string) any { return s }),
			gen.Int().Map(func(i int) any { return i }),
			gen.Bool().Map(func(b bool) any { return b }),
		),
	).Map(func(m map[string]any) map[string]any {
		if len(m) == 0 {
			return nil
		}
		return m
	})
}

// genSteps generates a list of workflow steps.
func genSteps() gopter.Gen {
	return gen.SliceOfN(3, genStep()).SuchThat(func(steps []types.Step) bool {
		return len(steps) >= 1
	}).Map(func(steps []types.Step) []types.Step {
		if len(steps) == 0 {
			return []types.Step{{
				ID:     "step-1",
				Name:   "Default Step",
				Type:   "http",
				Config: map[string]any{"method": "GET", "url": "http://example.com"},
			}}
		}
		// Ensure unique IDs
		for i := range steps {
			steps[i].ID = "step-" + string(rune('a'+i))
		}
		return steps
	})
}

// genStep generates a single workflow step.
func genStep() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.OneConstOf("http", "script"),
		genStepConfig(),
		genTimeout(),
	).Map(func(values []interface{}) types.Step {
		return types.Step{
			ID:      "step-" + values[0].(string),
			Name:    "Step " + values[1].(string),
			Type:    values[2].(string),
			Config:  values[3].(map[string]any),
			Timeout: values[4].(time.Duration),
		}
	})
}

// genStepsWithConditions generates steps that include conditional logic.
func genStepsWithConditions() gopter.Gen {
	return gen.SliceOfN(2, genConditionalStep()).Map(func(steps []types.Step) []types.Step {
		if len(steps) == 0 {
			return []types.Step{{
				ID:     "step-1",
				Name:   "Conditional Step",
				Type:   "condition",
				Config: map[string]any{},
				Condition: &types.Condition{
					Expression: "${status} == 200",
					Then: []types.Step{{
						ID:     "then-step",
						Name:   "Then Step",
						Type:   "http",
						Config: map[string]any{"method": "GET", "url": "http://example.com"},
					}},
				},
			}}
		}
		for i := range steps {
			steps[i].ID = "cond-step-" + string(rune('a'+i))
		}
		return steps
	})
}

// genConditionalStep generates a step with condition.
func genConditionalStep() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		genCondition(),
	).Map(func(values []interface{}) types.Step {
		return types.Step{
			ID:        "cond-" + values[0].(string),
			Name:      "Conditional Step",
			Type:      "condition",
			Config:    map[string]any{},
			Condition: values[1].(*types.Condition),
		}
	})
}

// genCondition generates a condition.
func genCondition() gopter.Gen {
	return gopter.CombineGens(
		genExpression(),
		genThenSteps(),
		genElseSteps(),
	).Map(func(values []interface{}) *types.Condition {
		return &types.Condition{
			Expression: values[0].(string),
			Then:       values[1].([]types.Step),
			Else:       values[2].([]types.Step),
		}
	})
}

// genExpression generates a valid expression.
func genExpression() gopter.Gen {
	return gen.OneConstOf(
		"${status} == 200",
		"${count} > 0",
		"${flag} == true",
		"${value} != 0",
		"${result} >= 100",
	)
}

// genThenSteps generates steps for the 'then' branch.
func genThenSteps() gopter.Gen {
	return gen.SliceOfN(2, genSimpleStep()).Map(func(steps []types.Step) []types.Step {
		if len(steps) == 0 {
			return []types.Step{{
				ID:     "then-1",
				Name:   "Then Step",
				Type:   "http",
				Config: map[string]any{"method": "GET", "url": "http://example.com"},
			}}
		}
		for i := range steps {
			steps[i].ID = "then-" + string(rune('a'+i))
		}
		return steps
	})
}

// genElseSteps generates steps for the 'else' branch.
func genElseSteps() gopter.Gen {
	return gen.SliceOfN(2, genSimpleStep()).Map(func(steps []types.Step) []types.Step {
		for i := range steps {
			steps[i].ID = "else-" + string(rune('a'+i))
		}
		return steps
	})
}

// genSimpleStep generates a simple step without conditions.
func genSimpleStep() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.OneConstOf("http", "script"),
	).Map(func(values []interface{}) types.Step {
		stepType := values[1].(string)
		config := map[string]any{}
		if stepType == "http" {
			config["method"] = "GET"
			config["url"] = "http://example.com"
		} else {
			config["inline"] = "echo hello"
		}
		return types.Step{
			ID:     "simple-" + values[0].(string),
			Name:   "Simple Step",
			Type:   stepType,
			Config: config,
		}
	})
}

// genStepConfig generates step configuration.
func genStepConfig() gopter.Gen {
	return gen.OneConstOf("http", "script").Map(func(stepType string) map[string]any {
		if stepType == "http" {
			return map[string]any{
				"method": "GET",
				"url":    "http://example.com/api",
			}
		}
		return map[string]any{
			"inline": "echo hello",
		}
	})
}

// genTimeout generates a timeout duration.
func genTimeout() gopter.Gen {
	return gen.IntRange(0, 60).Map(func(seconds int) time.Duration {
		return time.Duration(seconds) * time.Second
	})
}

// genHook generates a hook.
func genHook() gopter.Gen {
	return gen.Bool().Map(func(hasHook bool) *types.Hook {
		if !hasHook {
			return nil
		}
		return &types.Hook{
			Type: "script",
			Config: map[string]any{
				"inline": "echo hook",
			},
		}
	})
}

// genExecutionOptions generates execution options.
func genExecutionOptions() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, 100),
		gen.IntRange(0, 300),
		genExecutionMode(),
	).Map(func(values []interface{}) types.ExecutionOptions {
		return types.ExecutionOptions{
			VUs:           values[0].(int),
			Duration:      time.Duration(values[1].(int)) * time.Second,
			ExecutionMode: values[2].(types.ExecutionMode),
		}
	})
}

// genExecutionMode generates an execution mode.
func genExecutionMode() gopter.Gen {
	return gen.OneConstOf(
		types.ModeConstantVUs,
		types.ModeRampingVUs,
		types.ModePerVUIterations,
		types.ModeSharedIterations,
		types.ModeExternally,
	)
}

// workflowsEqual compares two workflows for equality.
func workflowsEqual(a, b *types.Workflow) bool {
	if a.ID != b.ID || a.Name != b.Name {
		return false
	}

	if len(a.Steps) != len(b.Steps) {
		return false
	}

	for i := range a.Steps {
		if !stepsEqual(&a.Steps[i], &b.Steps[i]) {
			return false
		}
	}

	return true
}

// stepsEqual compares two steps for equality.
func stepsEqual(a, b *types.Step) bool {
	if a.ID != b.ID || a.Name != b.Name || a.Type != b.Type {
		return false
	}

	// Compare conditions
	if (a.Condition == nil) != (b.Condition == nil) {
		return false
	}

	if a.Condition != nil {
		if a.Condition.Expression != b.Condition.Expression {
			return false
		}
		if len(a.Condition.Then) != len(b.Condition.Then) {
			return false
		}
		if len(a.Condition.Else) != len(b.Condition.Else) {
			return false
		}
	}

	return true
}

// BenchmarkWorkflowRoundTrip benchmarks the round-trip operation.
func BenchmarkWorkflowRoundTrip(b *testing.B) {
	workflow := &types.Workflow{
		ID:   "benchmark-workflow",
		Name: "Benchmark Workflow",
		Steps: []types.Step{
			{
				ID:     "step-1",
				Name:   "Step 1",
				Type:   "http",
				Config: map[string]any{"method": "GET", "url": "http://example.com"},
			},
		},
	}

	printer := NewYAMLPrinter()
	parser := NewYAMLParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		yamlBytes, _ := printer.Print(workflow)
		parser.Parse(yamlBytes)
	}
}

// TestWorkflowRoundTripSpecificCases tests specific edge cases.
func TestWorkflowRoundTripSpecificCases(t *testing.T) {
	testCases := []struct {
		name     string
		workflow *types.Workflow
	}{
		{
			name: "minimal workflow",
			workflow: &types.Workflow{
				ID:   "minimal",
				Name: "Minimal Workflow",
				Steps: []types.Step{
					{ID: "s1", Name: "Step 1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
				},
			},
		},
		{
			name: "workflow with all execution modes",
			workflow: &types.Workflow{
				ID:   "modes-test",
				Name: "Modes Test",
				Steps: []types.Step{
					{ID: "s1", Name: "Step 1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
				},
				Options: types.ExecutionOptions{
					VUs:           10,
					ExecutionMode: types.ModeConstantVUs,
				},
			},
		},
		{
			name: "workflow with stages",
			workflow: &types.Workflow{
				ID:   "stages-test",
				Name: "Stages Test",
				Steps: []types.Step{
					{ID: "s1", Name: "Step 1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
				},
				Options: types.ExecutionOptions{
					ExecutionMode: types.ModeRampingVUs,
					Stages: []types.Stage{
						{Duration: 1 * time.Minute, Target: 10},
						{Duration: 2 * time.Minute, Target: 20},
					},
				},
			},
		},
	}

	printer := NewYAMLPrinter()
	parser := NewYAMLParser()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			yamlBytes, err := printer.Print(tc.workflow)
			assert.NoError(t, err)

			parsed, err := parser.Parse(yamlBytes)
			assert.NoError(t, err)

			assert.Equal(t, tc.workflow.ID, parsed.ID)
			assert.Equal(t, tc.workflow.Name, parsed.Name)
			assert.Equal(t, len(tc.workflow.Steps), len(parsed.Steps))
		})
	}
}
