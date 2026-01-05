package expression

import "fmt"

// ExpressionError represents an error during expression parsing or evaluation.
type ExpressionError struct {
	Position int    // Position in the expression where the error occurred
	Message  string // Error message
	Cause    error  // Underlying error
}

// Error implements the error interface.
func (e *ExpressionError) Error() string {
	if e.Position >= 0 {
		return fmt.Sprintf("expression error at position %d: %s", e.Position, e.Message)
	}
	return fmt.Sprintf("expression error: %s", e.Message)
}

// Unwrap returns the underlying error.
func (e *ExpressionError) Unwrap() error {
	return e.Cause
}

// NewExpressionError creates a new ExpressionError.
func NewExpressionError(pos int, message string, cause error) *ExpressionError {
	return &ExpressionError{
		Position: pos,
		Message:  message,
		Cause:    cause,
	}
}

// ParseError represents a parsing error.
type ParseError struct {
	Position int
	Expected string
	Got      string
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at position %d: expected %s, got %s", e.Position, e.Expected, e.Got)
}

// NewParseError creates a new ParseError.
func NewParseError(pos int, expected, got string) *ParseError {
	return &ParseError{
		Position: pos,
		Expected: expected,
		Got:      got,
	}
}

// EvaluationError represents an error during expression evaluation.
type EvaluationError struct {
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *EvaluationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("evaluation error: %s: %v", e.Message, e.Cause)
	}
	return fmt.Sprintf("evaluation error: %s", e.Message)
}

// Unwrap returns the underlying error.
func (e *EvaluationError) Unwrap() error {
	return e.Cause
}

// NewEvaluationError creates a new EvaluationError.
func NewEvaluationError(message string, cause error) *EvaluationError {
	return &EvaluationError{
		Message: message,
		Cause:   cause,
	}
}

// TypeMismatchError represents a type mismatch during evaluation.
type TypeMismatchError struct {
	Expected string
	Got      string
	Value    any
}

// Error implements the error interface.
func (e *TypeMismatchError) Error() string {
	return fmt.Sprintf("type mismatch: expected %s, got %s (value: %v)", e.Expected, e.Got, e.Value)
}

// NewTypeMismatchError creates a new TypeMismatchError.
func NewTypeMismatchError(expected, got string, value any) *TypeMismatchError {
	return &TypeMismatchError{
		Expected: expected,
		Got:      got,
		Value:    value,
	}
}

// VariableNotFoundError represents a variable not found error.
type VariableNotFoundError struct {
	Name string
}

// Error implements the error interface.
func (e *VariableNotFoundError) Error() string {
	return fmt.Sprintf("variable not found: %s", e.Name)
}

// NewVariableNotFoundError creates a new VariableNotFoundError.
func NewVariableNotFoundError(name string) *VariableNotFoundError {
	return &VariableNotFoundError{Name: name}
}
