// Package keyword provides the keyword-driven system for workflow engine v2.
// Keywords are predefined executable operations that can be used in pre/post scripts.
package keyword

import (
	"context"
	"fmt"
)

// Category represents the category of a keyword.
type Category string

const (
	CategoryAssertion Category = "assertion" // Assertion keywords for validation
	CategoryExtractor Category = "extractor" // Extractor keywords for variable extraction
	CategoryAction    Category = "action"    // Action keywords for operations
	CategoryControl   Category = "control"   // Control keywords for flow control
)

// Result represents the result of a keyword execution.
type Result struct {
	Success bool   `json:"success"`         // Whether the execution succeeded
	Message string `json:"message"`         // Result message
	Data    any    `json:"data,omitempty"`  // Return data
	Error   error  `json:"error,omitempty"` // Error information
}

// NewSuccessResult creates a successful result.
func NewSuccessResult(message string, data any) *Result {
	return &Result{
		Success: true,
		Message: message,
		Data:    data,
	}
}

// NewFailureResult creates a failure result.
func NewFailureResult(message string, err error) *Result {
	return &Result{
		Success: false,
		Message: message,
		Error:   err,
	}
}

// Keyword defines the interface for all keywords.
type Keyword interface {
	// Name returns the keyword name.
	Name() string

	// Category returns the keyword category.
	Category() Category

	// Execute executes the keyword with given context and parameters.
	Execute(ctx context.Context, execCtx *ExecutionContext, params map[string]any) (*Result, error)

	// Validate validates the parameters before execution.
	Validate(params map[string]any) error

	// Description returns a human-readable description of the keyword.
	Description() string
}

// BaseKeyword provides common functionality for keywords.
type BaseKeyword struct {
	name        string
	category    Category
	description string
}

// NewBaseKeyword creates a new base keyword.
func NewBaseKeyword(name string, category Category, description string) BaseKeyword {
	return BaseKeyword{
		name:        name,
		category:    category,
		description: description,
	}
}

// Name returns the keyword name.
func (b BaseKeyword) Name() string {
	return b.name
}

// Category returns the keyword category.
func (b BaseKeyword) Category() Category {
	return b.category
}

// Description returns the keyword description.
func (b BaseKeyword) Description() string {
	return b.description
}

// RequiredParam extracts a required parameter from params map.
func RequiredParam[T any](params map[string]any, key string) (T, error) {
	var zero T
	val, ok := params[key]
	if !ok {
		return zero, fmt.Errorf("必需参数 '%s' 缺失", key)
	}
	typed, ok := val.(T)
	if !ok {
		return zero, fmt.Errorf("参数 '%s' 类型无效，期望 %T，实际 %T", key, zero, val)
	}
	return typed, nil
}

// OptionalParam extracts an optional parameter from params map with default value.
func OptionalParam[T any](params map[string]any, key string, defaultVal T) T {
	val, ok := params[key]
	if !ok {
		return defaultVal
	}
	typed, ok := val.(T)
	if !ok {
		return defaultVal
	}
	return typed
}
