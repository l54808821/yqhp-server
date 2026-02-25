package workflow

import (
	"encoding/json"
	"errors"

	"yqhp/workflow-engine/pkg/types"

	"gopkg.in/yaml.v3"
)

// WorkflowDefinition 工作流定义
type WorkflowDefinition struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Version     int                    `json:"version,omitempty" yaml:"version,omitempty"`
	Variables   map[string]interface{} `json:"variables,omitempty" yaml:"variables,omitempty"`
	Params      []WorkflowParam        `json:"params,omitempty" yaml:"params,omitempty"`
	Steps       []types.Step           `json:"steps" yaml:"steps"`
}

// UnmarshalJSON ensures postProcessSteps runs automatically regardless of
// whether the caller uses ParseJSON or raw json.Unmarshal.
func (d *WorkflowDefinition) UnmarshalJSON(data []byte) error {
	type Alias WorkflowDefinition
	aux := (*Alias)(d)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	postProcessSteps(d.Steps)
	return nil
}

// WorkflowParam 工作流输入参数定义（用于子流程调用时的参数接口）
type WorkflowParam struct {
	Name         string `json:"name" yaml:"name"`
	Type         string `json:"type" yaml:"type"`
	DefaultValue string `json:"defaultValue,omitempty" yaml:"default_value,omitempty"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	Required     bool   `json:"required,omitempty" yaml:"required,omitempty"`
}

// ParseYAML 将 YAML 解析为工作流定义
func ParseYAML(yamlContent string) (*WorkflowDefinition, error) {
	if yamlContent == "" {
		return nil, errors.New("YAML 内容不能为空")
	}

	var def WorkflowDefinition
	if err := yaml.Unmarshal([]byte(yamlContent), &def); err != nil {
		return nil, errors.New("YAML 解析失败: " + err.Error())
	}

	postProcessSteps(def.Steps)
	return &def, nil
}

// ToYAML 将工作流定义转换为 YAML
func ToYAML(def *WorkflowDefinition) (string, error) {
	if def == nil {
		return "", errors.New("工作流定义不能为空")
	}

	data, err := yaml.Marshal(def)
	if err != nil {
		return "", errors.New("YAML 序列化失败: " + err.Error())
	}

	return string(data), nil
}

// ParseJSON 将 JSON 解析为工作流定义
// postProcessSteps is called automatically by WorkflowDefinition.UnmarshalJSON.
func ParseJSON(jsonContent string) (*WorkflowDefinition, error) {
	if jsonContent == "" {
		return nil, errors.New("JSON 内容不能为空")
	}

	var def WorkflowDefinition
	if err := json.Unmarshal([]byte(jsonContent), &def); err != nil {
		return nil, errors.New("JSON 解析失败: " + err.Error())
	}

	return &def, nil
}

// ToJSON 将工作流定义转换为 JSON
func ToJSON(def *WorkflowDefinition) (string, error) {
	if def == nil {
		return "", errors.New("工作流定义不能为空")
	}

	data, err := json.Marshal(def)
	if err != nil {
		return "", errors.New("JSON 序列化失败: " + err.Error())
	}

	return string(data), nil
}

// YAMLToJSON 将 YAML 转换为 JSON
func YAMLToJSON(yamlContent string) (string, error) {
	def, err := ParseYAML(yamlContent)
	if err != nil {
		return "", err
	}
	return ToJSON(def)
}

// JSONToYAML 将 JSON 转换为 YAML
func JSONToYAML(jsonContent string) (string, error) {
	def, err := ParseJSON(jsonContent)
	if err != nil {
		return "", err
	}
	return ToYAML(def)
}

// postProcessSteps 对解析后的步骤做后处理：
// - 前端类型映射（"database" → "db"）
// - 将前端 children 字段桥接到引擎 loop.steps（前端将循环体步骤放在
//   children 中，引擎从 loop.steps 读取）
func postProcessSteps(steps []types.Step) {
	for i := range steps {
		steps[i].Type = mapStepType(steps[i].Type)

		// 桥接 children → loop.steps：前端把循环体步骤放在 children，
		// 引擎从 loop.steps 取。优先使用 children，其次 loop.steps。
		if steps[i].Loop != nil {
			if len(steps[i].Children) > 0 && len(steps[i].Loop.Steps) == 0 {
				steps[i].Loop.Steps = steps[i].Children
				steps[i].Children = nil
			}
			postProcessSteps(steps[i].Loop.Steps)
		}

		postProcessSteps(steps[i].Children)
		for j := range steps[i].Branches {
			postProcessSteps(steps[i].Branches[j].Steps)
		}
	}
}

// mapStepType 将前端步骤类型映射为执行器类型
func mapStepType(frontendType string) string {
	switch frontendType {
	case "database":
		return "db"
	default:
		return frontendType
	}
}
