// Package types defines the core data structures for the workflow execution engine.
package types

import (
	"encoding/json"
	"time"
)

// Workflow represents a parsed workflow definition.
type Workflow struct {
	ID          string            `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Variables   map[string]any    `yaml:"variables,omitempty" json:"variables,omitempty"`
	PreHook     *Hook             `yaml:"pre_hook,omitempty" json:"pre_hook,omitempty"`
	PostHook    *Hook             `yaml:"post_hook,omitempty" json:"post_hook,omitempty"`
	Steps       []Step            `yaml:"steps" json:"steps"`
	Options     ExecutionOptions  `yaml:"options,omitempty" json:"options,omitempty"`
	Callback    ExecutionCallback `yaml:"-" json:"-"`

	// FinalVariables 执行完成后的最终变量快照（调试上下文缓存用）
	FinalVariables map[string]any `yaml:"-" json:"-"`
	// EnvVariables 环境变量快照（在执行前从环境配置加载的变量）
	EnvVariables map[string]any `yaml:"-" json:"-"`
}

// Step represents a single execution unit in a workflow.
type Step struct {
	ID             string            `yaml:"id" json:"id"`
	Name           string            `yaml:"name" json:"name"`
	Type           string            `yaml:"type" json:"type"`
	Disabled       bool              `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Config         map[string]any    `yaml:"config" json:"config,omitempty"`
	PreHook        *Hook             `yaml:"pre_hook,omitempty" json:"pre_hook,omitempty"`
	PostHook       *Hook             `yaml:"post_hook,omitempty" json:"post_hook,omitempty"`
	PreProcessors  []Processor       `yaml:"preProcessors,omitempty" json:"preProcessors,omitempty"`
	PostProcessors []Processor       `yaml:"postProcessors,omitempty" json:"postProcessors,omitempty"`
	Loop           *Loop             `yaml:"loop,omitempty" json:"loop,omitempty"`
	Children       []Step            `yaml:"children,omitempty" json:"children,omitempty"`
	Branches       []ConditionBranch `yaml:"branches,omitempty" json:"branches,omitempty"`
	OnError        ErrorStrategy     `yaml:"on_error,omitempty" json:"on_error,omitempty"`
	Timeout        time.Duration     `yaml:"timeout,omitempty" json:"-"`
}

// UnmarshalJSON handles deserialization from JSON, accepting timeout as a
// duration string (e.g. "5s") which the frontend sends.
func (s *Step) UnmarshalJSON(data []byte) error {
	type Alias Step
	aux := &struct {
		Timeout json.RawMessage `json:"timeout,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if len(aux.Timeout) > 0 && string(aux.Timeout) != "null" && string(aux.Timeout) != `""` {
		var str string
		if json.Unmarshal(aux.Timeout, &str) == nil && str != "" {
			if d, err := time.ParseDuration(str); err == nil {
				s.Timeout = d
			}
		} else {
			var ns int64
			if json.Unmarshal(aux.Timeout, &ns) == nil {
				s.Timeout = time.Duration(ns)
			}
		}
	}
	return nil
}

// MarshalJSON serializes the step to JSON, outputting Timeout as a
// human-readable duration string.
func (s Step) MarshalJSON() ([]byte, error) {
	type Alias Step
	aux := struct {
		Timeout string `json:"timeout,omitempty"`
		Alias
	}{
		Alias: Alias(s),
	}
	if s.Timeout > 0 {
		aux.Timeout = s.Timeout.String()
	}
	return json.Marshal(aux)
}

// Processor 表示一个处理器（前置或后置）
type Processor struct {
	ID      string         `yaml:"id" json:"id"`
	Type    string         `yaml:"type" json:"type"`
	Enabled bool           `yaml:"enabled" json:"enabled"`
	Name    string         `yaml:"name,omitempty" json:"name,omitempty"`
	Config  map[string]any `yaml:"config" json:"config"`
}

// Loop represents loop configuration for iterative execution.
type Loop struct {
	Mode              string `yaml:"mode" json:"mode"`
	Count             int    `yaml:"count,omitempty" json:"count,omitempty"`
	Items             any    `yaml:"items,omitempty" json:"items,omitempty"`
	ItemVar           string `yaml:"item_var,omitempty" json:"item_var,omitempty"`
	Condition         string `yaml:"condition,omitempty" json:"condition,omitempty"`
	MaxIterations     int    `yaml:"max_iterations,omitempty" json:"max_iterations,omitempty"`
	BreakCondition    string `yaml:"break_condition,omitempty" json:"break_condition,omitempty"`
	ContinueCondition string `yaml:"continue_condition,omitempty" json:"continue_condition,omitempty"`
	Steps             []Step `yaml:"steps" json:"steps,omitempty"`
}

// ConditionBranchKind 条件分支类型常量
type ConditionBranchKind string

const (
	ConditionTypeIf     ConditionBranchKind = "if"
	ConditionTypeElseIf ConditionBranchKind = "else_if"
	ConditionTypeElse   ConditionBranchKind = "else"
)

// ConditionBranch 表示条件步骤内部的一个分支
type ConditionBranch struct {
	ID         string              `yaml:"id" json:"id"`
	Name       string              `yaml:"name,omitempty" json:"name,omitempty"`
	Kind       ConditionBranchKind `yaml:"kind" json:"kind"`
	Expression string              `yaml:"expression,omitempty" json:"expression,omitempty"`
	Steps      []Step              `yaml:"steps,omitempty" json:"steps,omitempty"`
}

// Hook represents pre/post execution scripts.
type Hook struct {
	Type   string         `yaml:"type" json:"type,omitempty"`
	Config map[string]any `yaml:"config" json:"config,omitempty"`
}

// ErrorStrategy defines how to handle step execution errors.
type ErrorStrategy string

const (
	ErrorStrategyAbort    ErrorStrategy = "abort"
	ErrorStrategyContinue ErrorStrategy = "continue"
	ErrorStrategyRetry    ErrorStrategy = "retry"
	ErrorStrategySkip     ErrorStrategy = "skip"
)
