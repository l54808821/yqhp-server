// Package script 提供工作流引擎 v2 的脚本片段系统。
// 脚本片段是可复用的工作流逻辑，可以通过参数调用。
package script

import (
	"fmt"
	"strings"
)

// ParamType 参数类型
type ParamType string

const (
	ParamTypeString  ParamType = "string"
	ParamTypeNumber  ParamType = "number"
	ParamTypeBoolean ParamType = "boolean"
	ParamTypeArray   ParamType = "array"
	ParamTypeObject  ParamType = "object"
	ParamTypeAny     ParamType = "any"
)

// Param 脚本参数定义
type Param struct {
	Name        string    `yaml:"name" json:"name"`                                   // 参数名
	Type        ParamType `yaml:"type,omitempty" json:"type,omitempty"`               // 类型
	Default     any       `yaml:"default,omitempty" json:"default,omitempty"`         // 默认值
	Required    bool      `yaml:"required,omitempty" json:"required,omitempty"`       // 是否必填
	Description string    `yaml:"description,omitempty" json:"description,omitempty"` // 参数描述
}

// Return 脚本返回值定义
type Return struct {
	Name  string `yaml:"name" json:"name"`   // 返回值名称
	Value string `yaml:"value" json:"value"` // 值表达式，如 ${_token}
}

// Fragment 可复用脚本片段
type Fragment struct {
	Name        string   `yaml:"name" json:"name"`                                   // 脚本名称
	Description string   `yaml:"description,omitempty" json:"description,omitempty"` // 描述
	Params      []Param  `yaml:"params,omitempty" json:"params,omitempty"`           // 参数列表
	Steps       []any    `yaml:"steps" json:"steps"`                                 // 步骤列表 (使用 any 避免循环依赖)
	Returns     []Return `yaml:"returns,omitempty" json:"returns,omitempty"`         // 返回值列表
}

// Import 外部脚本导入
type Import struct {
	Path  string `yaml:"path" json:"path"`             // 脚本文件路径
	Alias string `yaml:"alias,omitempty" json:"alias"` // 别名（可选）
}

// CallConfig 脚本调用配置
type CallConfig struct {
	Script  string            `yaml:"script" json:"script"`                       // 脚本名称
	Params  map[string]any    `yaml:"params,omitempty" json:"params,omitempty"`   // 参数
	Results map[string]string `yaml:"results,omitempty" json:"results,omitempty"` // 返回值映射
}

// ValidateParams 验证参数
func (f *Fragment) ValidateParams(provided map[string]any) error {
	for _, param := range f.Params {
		val, exists := provided[param.Name]

		// 检查必填参数
		if param.Required && !exists && param.Default == nil {
			return fmt.Errorf("required parameter '%s' is missing", param.Name)
		}

		// 如果提供了值，验证类型
		if exists && val != nil && param.Type != "" && param.Type != ParamTypeAny {
			if err := validateParamType(param.Name, val, param.Type); err != nil {
				return err
			}
		}
	}
	return nil
}

// ResolveParams 解析参数，应用默认值
func (f *Fragment) ResolveParams(provided map[string]any) map[string]any {
	resolved := make(map[string]any)

	// 首先应用默认值
	for _, param := range f.Params {
		if param.Default != nil {
			resolved[param.Name] = param.Default
		}
	}

	// 然后覆盖提供的值
	for k, v := range provided {
		resolved[k] = v
	}

	return resolved
}

// GetReturnMapping 获取返回值映射
func (f *Fragment) GetReturnMapping() map[string]string {
	mapping := make(map[string]string)
	for _, ret := range f.Returns {
		mapping[ret.Name] = ret.Value
	}
	return mapping
}

// validateParamType 验证参数类型
func validateParamType(name string, value any, expectedType ParamType) error {
	var valid bool

	switch expectedType {
	case ParamTypeString:
		_, valid = value.(string)
	case ParamTypeNumber:
		switch value.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64,
			float32, float64:
			valid = true
		}
	case ParamTypeBoolean:
		_, valid = value.(bool)
	case ParamTypeArray:
		switch value.(type) {
		case []any, []string, []int, []float64, []map[string]any:
			valid = true
		}
	case ParamTypeObject:
		_, valid = value.(map[string]any)
	case ParamTypeAny:
		valid = true
	}

	if !valid {
		return fmt.Errorf("parameter '%s' has invalid type: expected %s, got %T", name, expectedType, value)
	}
	return nil
}

// Registry 脚本注册表
type Registry struct {
	scripts map[string]*Fragment
	imports map[string]*Fragment // 导入的脚本
}

// NewRegistry 创建新的脚本注册表
func NewRegistry() *Registry {
	return &Registry{
		scripts: make(map[string]*Fragment),
		imports: make(map[string]*Fragment),
	}
}

// Register 注册脚本
func (r *Registry) Register(fragment *Fragment) error {
	if fragment.Name == "" {
		return fmt.Errorf("script name cannot be empty")
	}
	if _, exists := r.scripts[fragment.Name]; exists {
		return fmt.Errorf("script '%s' already registered", fragment.Name)
	}
	r.scripts[fragment.Name] = fragment
	return nil
}

// RegisterImport 注册导入的脚本
func (r *Registry) RegisterImport(alias string, fragment *Fragment) error {
	if alias == "" {
		alias = fragment.Name
	}
	if _, exists := r.imports[alias]; exists {
		return fmt.Errorf("imported script '%s' already registered", alias)
	}
	r.imports[alias] = fragment
	return nil
}

// Get 获取脚本
func (r *Registry) Get(name string) (*Fragment, error) {
	// 首先查找本地脚本
	if script, ok := r.scripts[name]; ok {
		return script, nil
	}
	// 然后查找导入的脚本
	if script, ok := r.imports[name]; ok {
		return script, nil
	}
	return nil, fmt.Errorf("script '%s' not found", name)
}

// Has 检查脚本是否存在
func (r *Registry) Has(name string) bool {
	_, ok := r.scripts[name]
	if ok {
		return true
	}
	_, ok = r.imports[name]
	return ok
}

// List 列出所有脚本名称
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.scripts)+len(r.imports))
	for name := range r.scripts {
		names = append(names, name)
	}
	for name := range r.imports {
		names = append(names, name)
	}
	return names
}

// CallStack 调用栈，用于检测循环调用
type CallStack struct {
	stack []string
}

// NewCallStack 创建新的调用栈
func NewCallStack() *CallStack {
	return &CallStack{
		stack: make([]string, 0),
	}
}

// Push 压入脚本名称
func (s *CallStack) Push(name string) error {
	// 检查循环调用
	for _, n := range s.stack {
		if n == name {
			return fmt.Errorf("circular script call detected: %s -> %s",
				strings.Join(s.stack, " -> "), name)
		}
	}
	s.stack = append(s.stack, name)
	return nil
}

// Pop 弹出脚本名称
func (s *CallStack) Pop() {
	if len(s.stack) > 0 {
		s.stack = s.stack[:len(s.stack)-1]
	}
}

// Current 获取当前脚本名称
func (s *CallStack) Current() string {
	if len(s.stack) == 0 {
		return ""
	}
	return s.stack[len(s.stack)-1]
}

// Depth 获取调用深度
func (s *CallStack) Depth() int {
	return len(s.stack)
}

// Clone 克隆调用栈
func (s *CallStack) Clone() *CallStack {
	newStack := &CallStack{
		stack: make([]string, len(s.stack)),
	}
	copy(newStack.stack, s.stack)
	return newStack
}
