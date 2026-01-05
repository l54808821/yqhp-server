// Package parser provides workflow parsing for workflow engine v2.
package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"yqhp/workflow-engine/internal/config"
	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/internal/script"
	"gopkg.in/yaml.v3"
)

// WorkflowV2 工作流 V2 定义
type WorkflowV2 struct {
	// 基本信息
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Version     string `yaml:"version,omitempty" json:"version,omitempty"`

	// 导入
	Imports []string `yaml:"imports,omitempty" json:"imports,omitempty"`

	// 全局配置
	Config *config.WorkflowGlobalConfig `yaml:"config,omitempty" json:"config,omitempty"`

	// 脚本定义
	Scripts map[string]*script.Fragment `yaml:"scripts,omitempty" json:"scripts,omitempty"`

	// 变量
	Variables map[string]any `yaml:"variables,omitempty" json:"variables,omitempty"`

	// 步骤
	Steps []*StepV2 `yaml:"steps" json:"steps"`

	// 执行选项
	Options *ExecutionOptionsV2 `yaml:"options,omitempty" json:"options,omitempty"`
}

// StepV2 步骤 V2 定义
type StepV2 struct {
	// 基本信息
	ID   string `yaml:"id" json:"id"`
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	Type string `yaml:"type" json:"type"`

	// 配置
	Config map[string]any `yaml:"config,omitempty" json:"config,omitempty"`

	// 前后置脚本
	PreScripts  []*executor.ScriptHook `yaml:"pre_scripts,omitempty" json:"pre_scripts,omitempty"`
	PostScripts []*executor.ScriptHook `yaml:"post_scripts,omitempty" json:"post_scripts,omitempty"`

	// 条件
	Condition string `yaml:"condition,omitempty" json:"condition,omitempty"`

	// 错误处理
	OnError string `yaml:"on_error,omitempty" json:"on_error,omitempty"`

	// 超时
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// 重试配置
	Retry *RetryConfigV2 `yaml:"retry,omitempty" json:"retry,omitempty"`

	// 子步骤（用于流程控制）
	Steps []*StepV2 `yaml:"steps,omitempty" json:"steps,omitempty"`

	// If 步骤特有
	Then   []*StepV2   `yaml:"then,omitempty" json:"then,omitempty"`
	ElseIf []*ElseIfV2 `yaml:"else_if,omitempty" json:"else_if,omitempty"`
	Else   []*StepV2   `yaml:"else,omitempty" json:"else,omitempty"`

	// 循环步骤特有
	Items    string `yaml:"items,omitempty" json:"items,omitempty"`
	ItemVar  string `yaml:"item_var,omitempty" json:"item_var,omitempty"`
	IndexVar string `yaml:"index_var,omitempty" json:"index_var,omitempty"`
	Start    int    `yaml:"start,omitempty" json:"start,omitempty"`
	End      int    `yaml:"end,omitempty" json:"end,omitempty"`
	Step     int    `yaml:"step,omitempty" json:"step,omitempty"`

	// While 步骤特有
	MaxIterations int `yaml:"max_iterations,omitempty" json:"max_iterations,omitempty"`

	// Parallel 步骤特有
	MaxConcurrent int  `yaml:"max_concurrent,omitempty" json:"max_concurrent,omitempty"`
	FailFast      bool `yaml:"fail_fast,omitempty" json:"fail_fast,omitempty"`

	// Break/Continue 步骤特有
	Label string `yaml:"label,omitempty" json:"label,omitempty"`

	// Sleep 步骤特有
	Duration time.Duration `yaml:"duration,omitempty" json:"duration,omitempty"`

	// WaitUntil 步骤特有
	Interval time.Duration `yaml:"interval,omitempty" json:"interval,omitempty"`

	// Call 步骤特有
	Script string            `yaml:"script,omitempty" json:"script,omitempty"`
	Args   map[string]any    `yaml:"args,omitempty" json:"args,omitempty"`
	Output map[string]string `yaml:"output,omitempty" json:"output,omitempty"`
}

// ElseIfV2 else_if 分支
type ElseIfV2 struct {
	Condition string    `yaml:"condition" json:"condition"`
	Steps     []*StepV2 `yaml:"steps" json:"steps"`
}

// RetryConfigV2 重试配置
type RetryConfigV2 struct {
	MaxAttempts int           `yaml:"max_attempts,omitempty" json:"max_attempts,omitempty"`
	Delay       time.Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Backoff     string        `yaml:"backoff,omitempty" json:"backoff,omitempty"` // fixed, linear, exponential
	MaxDelay    time.Duration `yaml:"max_delay,omitempty" json:"max_delay,omitempty"`
}

// ExecutionOptionsV2 执行选项
type ExecutionOptionsV2 struct {
	// 并发数
	Concurrency int `yaml:"concurrency,omitempty" json:"concurrency,omitempty"`

	// 迭代次数
	Iterations int `yaml:"iterations,omitempty" json:"iterations,omitempty"`

	// 持续时间
	Duration time.Duration `yaml:"duration,omitempty" json:"duration,omitempty"`

	// 全局超时
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// 失败阈值
	FailureThreshold float64 `yaml:"failure_threshold,omitempty" json:"failure_threshold,omitempty"`
}

// ParserV2 工作流 V2 解析器
type ParserV2 struct {
	basePath string
	imports  map[string]bool
}

// NewParserV2 创建 V2 解析器
func NewParserV2(basePath string) *ParserV2 {
	return &ParserV2{
		basePath: basePath,
		imports:  make(map[string]bool),
	}
}

// ParseFile 解析工作流文件
func (p *ParserV2) ParseFile(path string) (*WorkflowV2, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}

	return p.Parse(data)
}

// Parse 解析工作流 YAML
func (p *ParserV2) Parse(data []byte) (*WorkflowV2, error) {
	var workflow WorkflowV2
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	// 处理导入
	if err := p.processImports(&workflow); err != nil {
		return nil, fmt.Errorf("failed to process imports: %w", err)
	}

	// 验证工作流
	if err := p.validate(&workflow); err != nil {
		return nil, fmt.Errorf("workflow validation failed: %w", err)
	}

	return &workflow, nil
}

// processImports 处理导入
func (p *ParserV2) processImports(workflow *WorkflowV2) error {
	for _, importPath := range workflow.Imports {
		// 检查循环导入
		if p.imports[importPath] {
			return fmt.Errorf("circular import detected: %s", importPath)
		}
		p.imports[importPath] = true

		// 解析导入文件
		fullPath := filepath.Join(p.basePath, importPath)
		importedScripts, err := p.parseScriptFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to import %s: %w", importPath, err)
		}

		// 合并脚本
		if workflow.Scripts == nil {
			workflow.Scripts = make(map[string]*script.Fragment)
		}
		for name, s := range importedScripts {
			if _, exists := workflow.Scripts[name]; exists {
				return fmt.Errorf("duplicate script name: %s", name)
			}
			workflow.Scripts[name] = s
		}
	}

	return nil
}

// parseScriptFile 解析脚本文件
func (p *ParserV2) parseScriptFile(path string) (map[string]*script.Fragment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var scripts map[string]*script.Fragment
	if err := yaml.Unmarshal(data, &scripts); err != nil {
		return nil, err
	}

	return scripts, nil
}

// validate 验证工作流
func (p *ParserV2) validate(workflow *WorkflowV2) error {
	if workflow.ID == "" {
		return fmt.Errorf("workflow ID is required")
	}

	if len(workflow.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}

	// 验证步骤
	stepIDs := make(map[string]bool)
	for _, step := range workflow.Steps {
		if err := p.validateStep(step, stepIDs); err != nil {
			return err
		}
	}

	// 验证脚本
	for name, s := range workflow.Scripts {
		if err := p.validateScript(name, s); err != nil {
			return err
		}
	}

	return nil
}

// validateStep 验证步骤
func (p *ParserV2) validateStep(step *StepV2, stepIDs map[string]bool) error {
	if step.ID == "" {
		return fmt.Errorf("step ID is required")
	}

	if stepIDs[step.ID] {
		return fmt.Errorf("duplicate step ID: %s", step.ID)
	}
	stepIDs[step.ID] = true

	if step.Type == "" {
		return fmt.Errorf("step type is required for step: %s", step.ID)
	}

	// 验证步骤类型
	validTypes := map[string]bool{
		"http": true, "script": true, "call": true,
		"socket": true, "mq": true, "db": true,
		"if": true, "while": true, "for": true, "foreach": true,
		"parallel": true, "sleep": true, "wait_until": true, "retry": true,
		"break": true, "continue": true,
	}

	if !validTypes[step.Type] {
		return fmt.Errorf("invalid step type: %s", step.Type)
	}

	// 验证子步骤
	for _, subStep := range step.Steps {
		if err := p.validateStep(subStep, stepIDs); err != nil {
			return err
		}
	}

	for _, subStep := range step.Then {
		if err := p.validateStep(subStep, stepIDs); err != nil {
			return err
		}
	}

	for _, elseIf := range step.ElseIf {
		for _, subStep := range elseIf.Steps {
			if err := p.validateStep(subStep, stepIDs); err != nil {
				return err
			}
		}
	}

	for _, subStep := range step.Else {
		if err := p.validateStep(subStep, stepIDs); err != nil {
			return err
		}
	}

	return nil
}

// validateScript 验证脚本
func (p *ParserV2) validateScript(name string, s *script.Fragment) error {
	if s == nil {
		return fmt.Errorf("script %s is nil", name)
	}

	if len(s.Steps) == 0 {
		return fmt.Errorf("script %s has no steps", name)
	}

	return nil
}

// ParseString 从字符串解析工作流
func ParseString(yamlStr string) (*WorkflowV2, error) {
	parser := NewParserV2("")
	return parser.Parse([]byte(yamlStr))
}

// ParseFileV2 从文件解析工作流 V2
func ParseFileV2(path string) (*WorkflowV2, error) {
	parser := NewParserV2(filepath.Dir(path))
	return parser.ParseFile(path)
}
