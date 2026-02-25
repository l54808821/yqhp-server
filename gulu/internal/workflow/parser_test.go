package workflow

import (
	"encoding/json"
	"testing"

	"yqhp/workflow-engine/pkg/types"

	"pgregory.net/rapid"
)

func TestProperty11_WorkflowYAMLRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_]{0,20}`).Draw(t, "name")
		description := rapid.StringMatching(`[a-zA-Z0-9 ]{0,50}`).Draw(t, "description")

		numSteps := rapid.IntRange(1, 5).Draw(t, "numSteps")
		steps := make([]types.Step, numSteps)
		stepTypes := []string{"http", "script", "wait"}

		for i := 0; i < numSteps; i++ {
			stepID := rapid.StringMatching(`step_[a-z0-9]{1,10}`).Draw(t, "stepID")
			stepName := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9 ]{0,20}`).Draw(t, "stepName")
			stepType := stepTypes[rapid.IntRange(0, len(stepTypes)-1).Draw(t, "stepTypeIdx")]

			step := types.Step{
				ID:   stepID,
				Type: stepType,
				Name: stepName,
			}

			switch stepType {
			case "http":
				step.Config = map[string]interface{}{
					"method": "GET",
					"url":    "/api/test",
				}
			case "script":
				step.Config = map[string]interface{}{
					"script": "console.log('test');",
				}
			case "wait":
				step.Config = map[string]interface{}{
					"duration": "1s",
				}
			}

			steps[i] = step
		}

		original := &WorkflowDefinition{
			Name:        name,
			Description: description,
			Steps:       steps,
		}

		yamlContent, err := ToYAML(original)
		if err != nil {
			t.Fatalf("ToYAML failed: %v", err)
		}

		parsed, err := ParseYAML(yamlContent)
		if err != nil {
			t.Fatalf("ParseYAML failed: %v", err)
		}

		if parsed.Name != original.Name {
			t.Fatalf("Property 11 violated: name mismatch, got %q, want %q", parsed.Name, original.Name)
		}

		if parsed.Description != original.Description {
			t.Fatalf("Property 11 violated: description mismatch, got %q, want %q", parsed.Description, original.Description)
		}

		if len(parsed.Steps) != len(original.Steps) {
			t.Fatalf("Property 11 violated: steps count mismatch, got %d, want %d", len(parsed.Steps), len(original.Steps))
		}

		for i, step := range parsed.Steps {
			origStep := original.Steps[i]
			if step.ID != origStep.ID {
				t.Fatalf("Property 11 violated: step[%d].id mismatch, got %q, want %q", i, step.ID, origStep.ID)
			}
			if step.Type != origStep.Type {
				t.Fatalf("Property 11 violated: step[%d].type mismatch, got %q, want %q", i, step.Type, origStep.Type)
			}
			if step.Name != origStep.Name {
				t.Fatalf("Property 11 violated: step[%d].name mismatch, got %q, want %q", i, step.Name, origStep.Name)
			}
		}
	})
}

func TestProperty12_WorkflowDefinitionValidation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		generateValid := rapid.Bool().Draw(t, "generateValid")

		var def *WorkflowDefinition

		if generateValid {
			name := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_]{1,20}`).Draw(t, "name")
			stepID := rapid.StringMatching(`step_[a-z0-9]{1,10}`).Draw(t, "stepID")
			stepName := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9 ]{1,20}`).Draw(t, "stepName")

			def = &WorkflowDefinition{
				Name: name,
				Steps: []types.Step{
					{
						ID:   stepID,
						Type: "http",
						Name: stepName,
						Config: map[string]interface{}{
							"method": "GET",
							"url":    "/api/test",
						},
					},
				},
			}
		} else {
			invalidCase := rapid.IntRange(0, 3).Draw(t, "invalidCase")

			switch invalidCase {
			case 0:
				def = &WorkflowDefinition{
					Name: "",
					Steps: []types.Step{
						{ID: "step1", Type: "http", Name: "test", Config: map[string]interface{}{"method": "GET", "url": "/test"}},
					},
				}
			case 1:
				def = &WorkflowDefinition{
					Name: "test",
					Steps: []types.Step{
						{ID: "step1", Type: "invalid_type", Name: "test"},
					},
				}
			case 2:
				def = &WorkflowDefinition{
					Name: "test",
					Steps: []types.Step{
						{ID: "", Type: "http", Name: "test", Config: map[string]interface{}{"method": "GET", "url": "/test"}},
					},
				}
			case 3:
				def = &WorkflowDefinition{
					Name: "test",
					Steps: []types.Step{
						{ID: "step1", Type: "http", Name: "test", Config: nil},
					},
				}
			}
		}

		result := Validate(def)

		if generateValid {
			if !result.Valid {
				t.Fatalf("Property 12 violated: valid workflow should pass validation, errors: %v", result.Errors)
			}
		} else {
			if result.Valid {
				t.Fatalf("Property 12 violated: invalid workflow should fail validation")
			}
			if len(result.Errors) == 0 {
				t.Fatalf("Property 12 violated: invalid workflow should have validation errors")
			}
		}
	})
}

func TestProperty12b_WorkflowExecutionValidation(t *testing.T) {
	def := &WorkflowDefinition{
		Name:  "test",
		Steps: []types.Step{},
	}

	result := ValidateForExecution(def)
	if result.Valid {
		t.Fatalf("Empty steps workflow should fail execution validation")
	}

	defWithSteps := &WorkflowDefinition{
		Name: "test",
		Steps: []types.Step{
			{ID: "step1", Type: "http", Name: "test", Config: map[string]interface{}{"method": "GET", "url": "/test"}},
		},
	}

	result = ValidateForExecution(defWithSteps)
	if !result.Valid {
		t.Fatalf("Workflow with steps should pass execution validation, errors: %v", result.Errors)
	}
}

func TestProperty13_NodeTypeCompatibility(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		validTypes := []string{"http", "script", "condition", "loop", "database", "wait", "mq"}
		invalidTypes := []string{"invalid", "unknown", "custom", "test", ""}

		useValidType := rapid.Bool().Draw(t, "useValidType")

		var nodeType string
		if useValidType {
			idx := rapid.IntRange(0, len(validTypes)-1).Draw(t, "validTypeIdx")
			nodeType = validTypes[idx]
		} else {
			idx := rapid.IntRange(0, len(invalidTypes)-1).Draw(t, "invalidTypeIdx")
			nodeType = invalidTypes[idx]
		}

		isValid := IsValidNodeType(nodeType)

		if useValidType && !isValid {
			t.Fatalf("Property 13 violated: valid node type %q should be recognized", nodeType)
		}
		if !useValidType && isValid {
			t.Fatalf("Property 13 violated: invalid node type %q should not be recognized", nodeType)
		}
	})
}

func TestYAMLJSONConversion(t *testing.T) {
	yamlContent := `
name: test-workflow
description: A test workflow
steps:
  - id: step1
    type: http
    name: Send Request
    config:
      method: GET
      url: /api/test
  - id: step2
    type: script
    name: Process Response
    config:
      script: console.log('done');
`

	jsonContent, err := YAMLToJSON(yamlContent)
	if err != nil {
		t.Fatalf("YAMLToJSON failed: %v", err)
	}

	var jsonMap map[string]interface{}
	if err := json.Unmarshal([]byte(jsonContent), &jsonMap); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	yamlOutput, err := JSONToYAML(jsonContent)
	if err != nil {
		t.Fatalf("JSONToYAML failed: %v", err)
	}

	def, err := ParseYAML(yamlOutput)
	if err != nil {
		t.Fatalf("ParseYAML failed: %v", err)
	}

	if def.Name != "test-workflow" {
		t.Errorf("Name mismatch: got %q, want %q", def.Name, "test-workflow")
	}
	if len(def.Steps) != 2 {
		t.Errorf("Steps count mismatch: got %d, want %d", len(def.Steps), 2)
	}
}

func TestValidation_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		def     *WorkflowDefinition
		isValid bool
	}{
		{
			name:    "nil definition",
			def:     nil,
			isValid: false,
		},
		{
			name: "valid minimal workflow",
			def: &WorkflowDefinition{
				Name: "test",
				Steps: []types.Step{
					{ID: "step1", Type: "http", Name: "test", Config: map[string]interface{}{"method": "GET", "url": "/test"}},
				},
			},
			isValid: true,
		},
		{
			name: "duplicate step IDs",
			def: &WorkflowDefinition{
				Name: "test",
				Steps: []types.Step{
					{ID: "step1", Type: "http", Name: "test1", Config: map[string]interface{}{"method": "GET", "url": "/test"}},
					{ID: "step1", Type: "http", Name: "test2", Config: map[string]interface{}{"method": "GET", "url": "/test"}},
				},
			},
			isValid: false,
		},
		{
			name: "condition step without branches",
			def: &WorkflowDefinition{
				Name: "test",
				Steps: []types.Step{
					{ID: "step1", Type: "condition", Name: "test"},
				},
			},
			isValid: false,
		},
		{
			name: "valid condition step with branches",
			def: &WorkflowDefinition{
				Name: "test",
				Steps: []types.Step{
					{
						ID: "step1", Type: "condition", Name: "test",
						Branches: []types.ConditionBranch{
							{ID: "br1", Kind: "if", Expression: "${status} == 200"},
						},
					},
				},
			},
			isValid: true,
		},
		{
			name: "loop step without config",
			def: &WorkflowDefinition{
				Name: "test",
				Steps: []types.Step{
					{ID: "step1", Type: "loop", Name: "test", Loop: nil},
				},
			},
			isValid: false,
		},
		{
			name: "valid loop step with count",
			def: &WorkflowDefinition{
				Name: "test",
				Steps: []types.Step{
					{ID: "step1", Type: "loop", Name: "test", Loop: &types.Loop{Count: 10}},
				},
			},
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Validate(tt.def)
			if result.Valid != tt.isValid {
				t.Errorf("Validate() = %v, want %v, errors: %v", result.Valid, tt.isValid, result.Errors)
			}
		})
	}
}
