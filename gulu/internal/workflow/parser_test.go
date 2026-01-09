package workflow

import (
	"encoding/json"
	"testing"

	"pgregory.net/rapid"
)

// TestProperty11_WorkflowYAMLRoundTrip 测试工作流 YAML Round-Trip
// Property 11: 工作流 YAML Round-Trip
// 对于任意有效的工作流定义，导出为 YAML 后再导入，应产生与原始工作流定义等价的数据结构。
// Feature: gulu-extension, Property 11: 工作流 YAML Round-Trip
// Validates: Requirements 8.5
func TestProperty11_WorkflowYAMLRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机的工作流定义
		name := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_]{0,20}`).Draw(t, "name")
		description := rapid.StringMatching(`[a-zA-Z0-9 ]{0,50}`).Draw(t, "description")

		// 生成随机步骤
		numSteps := rapid.IntRange(1, 5).Draw(t, "numSteps")
		steps := make([]Step, numSteps)
		stepTypes := []string{"http", "script", "wait"}

		for i := 0; i < numSteps; i++ {
			stepID := rapid.StringMatching(`step_[a-z0-9]{1,10}`).Draw(t, "stepID")
			stepName := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9 ]{0,20}`).Draw(t, "stepName")
			stepType := stepTypes[rapid.IntRange(0, len(stepTypes)-1).Draw(t, "stepTypeIdx")]

			step := Step{
				ID:   stepID,
				Type: stepType,
				Name: stepName,
			}

			// 根据类型添加配置
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

		// 转换为 YAML
		yamlContent, err := ToYAML(original)
		if err != nil {
			t.Fatalf("ToYAML failed: %v", err)
		}

		// 从 YAML 解析回来
		parsed, err := ParseYAML(yamlContent)
		if err != nil {
			t.Fatalf("ParseYAML failed: %v", err)
		}

		// 验证属性：名称应该相等
		if parsed.Name != original.Name {
			t.Fatalf("Property 11 violated: name mismatch, got %q, want %q", parsed.Name, original.Name)
		}

		// 验证属性：描述应该相等
		if parsed.Description != original.Description {
			t.Fatalf("Property 11 violated: description mismatch, got %q, want %q", parsed.Description, original.Description)
		}

		// 验证属性：步骤数量应该相等
		if len(parsed.Steps) != len(original.Steps) {
			t.Fatalf("Property 11 violated: steps count mismatch, got %d, want %d", len(parsed.Steps), len(original.Steps))
		}

		// 验证每个步骤
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

// TestProperty12_WorkflowDefinitionValidation 测试工作流定义验证
// Property 12: 工作流定义验证
// 对于任意工作流定义，如果包含无效的节点类型或缺少必需字段，验证应返回错误；如果定义有效，验证应通过。
// Feature: gulu-extension, Property 12: 工作流定义验证
// Validates: Requirements 8.6
func TestProperty12_WorkflowDefinitionValidation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 决定是否生成有效的工作流
		generateValid := rapid.Bool().Draw(t, "generateValid")

		var def *WorkflowDefinition

		if generateValid {
			// 生成有效的工作流
			name := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_]{1,20}`).Draw(t, "name")
			stepID := rapid.StringMatching(`step_[a-z0-9]{1,10}`).Draw(t, "stepID")
			stepName := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9 ]{1,20}`).Draw(t, "stepName")

			def = &WorkflowDefinition{
				Name: name,
				Steps: []Step{
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
			// 生成无效的工作流（随机选择一种无效情况）
			// 注意：case 1 (空步骤) 在 Validate 中是允许的，只在 ValidateForExecution 中检查
			invalidCase := rapid.IntRange(0, 3).Draw(t, "invalidCase")

			switch invalidCase {
			case 0:
				// 空名称
				def = &WorkflowDefinition{
					Name: "",
					Steps: []Step{
						{ID: "step1", Type: "http", Name: "test", Config: map[string]interface{}{"method": "GET", "url": "/test"}},
					},
				}
			case 1:
				// 无效的节点类型
				def = &WorkflowDefinition{
					Name: "test",
					Steps: []Step{
						{ID: "step1", Type: "invalid_type", Name: "test"},
					},
				}
			case 2:
				// 缺少步骤 ID
				def = &WorkflowDefinition{
					Name: "test",
					Steps: []Step{
						{ID: "", Type: "http", Name: "test", Config: map[string]interface{}{"method": "GET", "url": "/test"}},
					},
				}
			case 3:
				// HTTP 步骤缺少必需配置
				def = &WorkflowDefinition{
					Name: "test",
					Steps: []Step{
						{ID: "step1", Type: "http", Name: "test", Config: nil},
					},
				}
			}
		}

		result := Validate(def)

		// 验证属性
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

// TestProperty12b_WorkflowExecutionValidation 测试工作流执行前验证
// 对于执行前验证，空步骤的工作流应该验证失败
func TestProperty12b_WorkflowExecutionValidation(t *testing.T) {
	// 空步骤的工作流在执行前验证应该失败
	def := &WorkflowDefinition{
		Name:  "test",
		Steps: []Step{},
	}

	result := ValidateForExecution(def)
	if result.Valid {
		t.Fatalf("Empty steps workflow should fail execution validation")
	}

	// 有步骤的工作流应该通过执行前验证
	defWithSteps := &WorkflowDefinition{
		Name: "test",
		Steps: []Step{
			{ID: "step1", Type: "http", Name: "test", Config: map[string]interface{}{"method": "GET", "url": "/test"}},
		},
	}

	result = ValidateForExecution(defWithSteps)
	if !result.Valid {
		t.Fatalf("Workflow with steps should pass execution validation, errors: %v", result.Errors)
	}
}

// TestProperty13_NodeTypeCompatibility 测试节点类型兼容性
// Property 13: 节点类型兼容性
// 对于任意工作流中的步骤节点，其 type 值应属于 workflow-engine 支持的类型集合。
// Feature: gulu-extension, Property 13: 节点类型兼容性
// Validates: Requirements 9.8
func TestProperty13_NodeTypeCompatibility(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机的节点类型
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

		// 验证属性
		if useValidType && !isValid {
			t.Fatalf("Property 13 violated: valid node type %q should be recognized", nodeType)
		}
		if !useValidType && isValid {
			t.Fatalf("Property 13 violated: invalid node type %q should not be recognized", nodeType)
		}
	})
}

// TestYAMLJSONConversion 测试 YAML 和 JSON 互转
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

	// YAML -> JSON
	jsonContent, err := YAMLToJSON(yamlContent)
	if err != nil {
		t.Fatalf("YAMLToJSON failed: %v", err)
	}

	// 验证 JSON 格式
	var jsonMap map[string]interface{}
	if err := json.Unmarshal([]byte(jsonContent), &jsonMap); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	// JSON -> YAML
	yamlOutput, err := JSONToYAML(jsonContent)
	if err != nil {
		t.Fatalf("JSONToYAML failed: %v", err)
	}

	// 再次解析验证
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

// TestValidation_EdgeCases 测试验证的边界情况
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
				Steps: []Step{
					{ID: "step1", Type: "http", Name: "test", Config: map[string]interface{}{"method": "GET", "url": "/test"}},
				},
			},
			isValid: true,
		},
		{
			name: "duplicate step IDs",
			def: &WorkflowDefinition{
				Name: "test",
				Steps: []Step{
					{ID: "step1", Type: "http", Name: "test1", Config: map[string]interface{}{"method": "GET", "url": "/test"}},
					{ID: "step1", Type: "http", Name: "test2", Config: map[string]interface{}{"method": "GET", "url": "/test"}},
				},
			},
			isValid: false,
		},
		{
			name: "condition step without expression",
			def: &WorkflowDefinition{
				Name: "test",
				Steps: []Step{
					{ID: "step1", Type: "condition", Name: "test", Condition: &ConditionConfig{Expression: ""}},
				},
			},
			isValid: false,
		},
		{
			name: "valid condition step",
			def: &WorkflowDefinition{
				Name: "test",
				Steps: []Step{
					{ID: "step1", Type: "condition", Name: "test", Condition: &ConditionConfig{Expression: "${status} == 200"}},
				},
			},
			isValid: true,
		},
		{
			name: "loop step without config",
			def: &WorkflowDefinition{
				Name: "test",
				Steps: []Step{
					{ID: "step1", Type: "loop", Name: "test", Loop: nil},
				},
			},
			isValid: false,
		},
		{
			name: "valid loop step with count",
			def: &WorkflowDefinition{
				Name: "test",
				Steps: []Step{
					{ID: "step1", Type: "loop", Name: "test", Loop: &LoopConfig{Count: 10}},
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
