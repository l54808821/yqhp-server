package workflow

import (
	"encoding/json"
	"errors"

	"gopkg.in/yaml.v3"
)

// WorkflowDefinition 工作流定义
type WorkflowDefinition struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Version     int                    `json:"version,omitempty" yaml:"version,omitempty"`
	Variables   map[string]interface{} `json:"variables,omitempty" yaml:"variables,omitempty"`
	Steps       []Step                 `json:"steps" yaml:"steps"`
}

// Step 工作流步骤
type Step struct {
	ID        string                 `json:"id" yaml:"id"`
	Type      string                 `json:"type" yaml:"type"`
	Name      string                 `json:"name" yaml:"name"`
	Disabled  bool                   `json:"disabled,omitempty" yaml:"disabled,omitempty"` // 是否禁用
	Config    map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
	Timeout   string                 `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	OnError   string                 `json:"on_error,omitempty" yaml:"on_error,omitempty"`
	Condition *ConditionConfig       `json:"condition,omitempty" yaml:"condition,omitempty"`
	Loop      *LoopConfig            `json:"loop,omitempty" yaml:"loop,omitempty"`
	Children  []Step                 `json:"children,omitempty" yaml:"children,omitempty"` // 子步骤（用于循环等控制器）
}

// ConditionConfig 条件配置
type ConditionConfig struct {
	Expression string `json:"expression" yaml:"expression"`
	Then       []Step `json:"then,omitempty" yaml:"then,omitempty"`
	Else       []Step `json:"else,omitempty" yaml:"else,omitempty"`
}

// LoopConfig 循环配置
type LoopConfig struct {
	Mode              string `json:"mode" yaml:"mode"`                                                 // 循环模式: for, foreach, while
	Count             int    `json:"count,omitempty" yaml:"count,omitempty"`                           // for 模式的迭代次数
	Items             any    `json:"items,omitempty" yaml:"items,omitempty"`                           // foreach 模式的集合
	ItemVar           string `json:"item_var,omitempty" yaml:"item_var,omitempty"`                     // foreach 模式的元素变量名
	Condition         string `json:"condition,omitempty" yaml:"condition,omitempty"`                   // while 模式的条件表达式
	MaxIterations     int    `json:"max_iterations,omitempty" yaml:"max_iterations,omitempty"`         // while 模式的最大迭代次数
	BreakCondition    string `json:"break_condition,omitempty" yaml:"break_condition,omitempty"`       // 跳出条件
	ContinueCondition string `json:"continue_condition,omitempty" yaml:"continue_condition,omitempty"` // 跳过条件
	Steps             []Step `json:"steps,omitempty" yaml:"steps,omitempty"`                           // 循环体步骤
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
