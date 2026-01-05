package parser

import (
	"fmt"
)

// ParseError represents a parsing error with location information.
type ParseError struct {
	Line    int    // Line number where the error occurred (1-based)
	Column  int    // Column number where the error occurred (1-based)
	Message string // Error message
	Cause   error  // Underlying error
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	if e.Line > 0 && e.Column > 0 {
		return fmt.Sprintf("parse error at line %d, column %d: %s", e.Line, e.Column, e.Message)
	}
	if e.Line > 0 {
		return fmt.Sprintf("parse error at line %d: %s", e.Line, e.Message)
	}
	return fmt.Sprintf("parse error: %s", e.Message)
}

// Unwrap returns the underlying error.
func (e *ParseError) Unwrap() error {
	return e.Cause
}

// NewParseError creates a new ParseError.
func NewParseError(line, column int, message string, cause error) *ParseError {
	return &ParseError{
		Line:    line,
		Column:  column,
		Message: message,
		Cause:   cause,
	}
}

// ValidationError represents a validation error for workflow definitions.
type ValidationError struct {
	Field   string // Field that failed validation
	Message string // Error message
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// NewValidationError creates a new ValidationError.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// VariableResolutionError represents an error resolving a variable reference.
type VariableResolutionError struct {
	Reference string // The variable reference that failed
	Message   string // Error message
	Cause     error  // Underlying error
}

// Error implements the error interface.
func (e *VariableResolutionError) Error() string {
	return fmt.Sprintf("failed to resolve variable '%s': %s", e.Reference, e.Message)
}

// Unwrap returns the underlying error.
func (e *VariableResolutionError) Unwrap() error {
	return e.Cause
}

// NewVariableResolutionError creates a new VariableResolutionError.
func NewVariableResolutionError(ref, message string, cause error) *VariableResolutionError {
	return &VariableResolutionError{
		Reference: ref,
		Message:   message,
		Cause:     cause,
	}
}
