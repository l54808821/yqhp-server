// Package types defines the core data structures for the workflow execution engine.
package types

import "time"

// Workflow represents a parsed workflow definition.
type Workflow struct {
	ID          string           `yaml:"id"`
	Name        string           `yaml:"name"`
	Description string           `yaml:"description,omitempty"`
	Variables   map[string]any   `yaml:"variables,omitempty"`
	PreHook     *Hook            `yaml:"pre_hook,omitempty"`
	PostHook    *Hook            `yaml:"post_hook,omitempty"`
	Steps       []Step           `yaml:"steps"`
	Options     ExecutionOptions `yaml:"options,omitempty"`
}

// Step represents a single execution unit in a workflow.
type Step struct {
	ID        string         `yaml:"id"`
	Name      string         `yaml:"name"`
	Type      string         `yaml:"type"` // http, script, grpc, condition
	Config    map[string]any `yaml:"config"`
	PreHook   *Hook          `yaml:"pre_hook,omitempty"`
	PostHook  *Hook          `yaml:"post_hook,omitempty"`
	Condition *Condition     `yaml:"condition,omitempty"`
	OnError   ErrorStrategy  `yaml:"on_error,omitempty"`
	Timeout   time.Duration  `yaml:"timeout,omitempty"`
}

// Condition represents conditional logic (if/else).
type Condition struct {
	Expression string `yaml:"expression"`
	Then       []Step `yaml:"then"`
	Else       []Step `yaml:"else,omitempty"`
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
