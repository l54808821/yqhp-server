// Package types defines the core data structures for the workflow execution engine.
package types

import "time"

// Workflow represents a parsed workflow definition.
type Workflow struct {
	ID          string            `yaml:"id"`
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Variables   map[string]any    `yaml:"variables,omitempty"`
	PreHook     *Hook             `yaml:"pre_hook,omitempty"`
	PostHook    *Hook             `yaml:"post_hook,omitempty"`
	Steps       []Step            `yaml:"steps"`
	Options     ExecutionOptions  `yaml:"options,omitempty"`
	Callback    ExecutionCallback `yaml:"-"` // 执行回调，用于实时通知执行进度（不序列化）
}

// Step represents a single execution unit in a workflow.
type Step struct {
	ID       string         `yaml:"id"`
	Name     string         `yaml:"name"`
	Type     string         `yaml:"type"`               // http, script, grpc, condition, loop
	Disabled bool           `yaml:"disabled,omitempty"` // 是否禁用，禁用的步骤将被跳过
	Config   map[string]any `yaml:"config"`
	PreHook  *Hook          `yaml:"pre_hook,omitempty"`
	PostHook *Hook          `yaml:"post_hook,omitempty"`
	Loop     *Loop          `yaml:"loop,omitempty"`
	Children []Step         `yaml:"children,omitempty"` // 子步骤（用于 loop 以及旧版 condition）
	// Branches 是条件步骤（type=condition）的新结构：
	// 一个 condition 步骤内部包含多个分支（if/else_if/else），每个分支下有自己的 steps。
	// 优先使用 Branches；当 Branches 为空时，为兼容旧格式仍然使用 Config.type + Children。
	Branches []ConditionBranch `yaml:"branches,omitempty"`
	OnError  ErrorStrategy     `yaml:"on_error,omitempty"`
	Timeout  time.Duration     `yaml:"timeout,omitempty"`
}

// Loop represents loop configuration for iterative execution.
type Loop struct {
	// Mode specifies the loop type: "for", "foreach", or "while"
	Mode string `yaml:"mode"`

	// Count specifies the number of iterations for "for" mode
	Count int `yaml:"count,omitempty"`

	// Items specifies the collection to iterate over for "foreach" mode
	// Can be an expression like "${response.data}" or a literal array
	Items any `yaml:"items,omitempty"`

	// ItemVar specifies the variable name for the current item in "foreach" mode
	// Default is "item"
	ItemVar string `yaml:"item_var,omitempty"`

	// Condition specifies the condition expression for "while" mode
	Condition string `yaml:"condition,omitempty"`

	// MaxIterations specifies the maximum number of iterations for "while" mode
	// Default is 1000 to prevent infinite loops
	MaxIterations int `yaml:"max_iterations,omitempty"`

	// BreakCondition specifies a condition that, when true, breaks out of the loop
	BreakCondition string `yaml:"break_condition,omitempty"`

	// ContinueCondition specifies a condition that, when true, skips to the next iteration
	ContinueCondition string `yaml:"continue_condition,omitempty"`

	// Steps contains the steps to execute in each iteration
	Steps []Step `yaml:"steps"`
}

// ConditionBranchKind 条件分支类型常量
type ConditionBranchKind string

// ConditionType 条件类型常量
const (
	ConditionTypeIf     ConditionBranchKind = "if"
	ConditionTypeElseIf ConditionBranchKind = "else_if"
	ConditionTypeElse   ConditionBranchKind = "else"
)

// ConditionBranch 表示条件步骤内部的一个分支
// 该结构仅用于 type=condition 的 Step.Branches 字段。
type ConditionBranch struct {
	ID         string            `yaml:"id" json:"id"`
	Name       string            `yaml:"name,omitempty" json:"name,omitempty"`
	Kind       ConditionBranchKind `yaml:"kind" json:"kind"`                         // if / else_if / else
	Expression string            `yaml:"expression,omitempty" json:"expression,omitempty"` // if/else_if 需要，else 不需要
	Steps      []Step            `yaml:"steps,omitempty" json:"steps,omitempty"`     // 命中该分支时要执行的步骤
}

// Hook represents pre/post execution scripts.
type Hook struct {
	Type   string         `yaml:"type"` // script, http
	Config map[string]any `yaml:"config"`
}

// ErrorStrategy defines how to handle step execution errors.
type ErrorStrategy string

const (
	// ErrorStrategyAbort stops the entire workflow.
	ErrorStrategyAbort ErrorStrategy = "abort"
	// ErrorStrategyContinue proceeds to the next step.
	ErrorStrategyContinue ErrorStrategy = "continue"
	// ErrorStrategyRetry retries with backoff.
	ErrorStrategyRetry ErrorStrategy = "retry"
	// ErrorStrategySkip skips the current step.
	ErrorStrategySkip ErrorStrategy = "skip"
)
