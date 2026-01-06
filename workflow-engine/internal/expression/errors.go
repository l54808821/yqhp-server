package expression

import "fmt"

// ExpressionError 表示表达式解析或求值期间的错误。
type ExpressionError struct {
	Position int    // 错误发生的位置
	Message  string // 错误消息
	Cause    error  // 底层错误
}

// Error 实现 error 接口。
func (e *ExpressionError) Error() string {
	if e.Position >= 0 {
		return fmt.Sprintf("表达式错误，位于位置 %d: %s", e.Position, e.Message)
	}
	return fmt.Sprintf("表达式错误: %s", e.Message)
}

// Unwrap 返回底层错误。
func (e *ExpressionError) Unwrap() error {
	return e.Cause
}

// NewExpressionError 创建一个新的 ExpressionError。
func NewExpressionError(pos int, message string, cause error) *ExpressionError {
	return &ExpressionError{
		Position: pos,
		Message:  message,
		Cause:    cause,
	}
}

// ParseError 表示解析错误。
type ParseError struct {
	Position int
	Expected string
	Got      string
}

// Error 实现 error 接口。
func (e *ParseError) Error() string {
	return fmt.Sprintf("解析错误，位于位置 %d: 期望 %s，得到 %s", e.Position, e.Expected, e.Got)
}

// NewParseError 创建一个新的 ParseError。
func NewParseError(pos int, expected, got string) *ParseError {
	return &ParseError{
		Position: pos,
		Expected: expected,
		Got:      got,
	}
}

// EvaluationError 表示表达式求值期间的错误。
type EvaluationError struct {
	Message string
	Cause   error
}

// Error 实现 error 接口。
func (e *EvaluationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("求值错误: %s: %v", e.Message, e.Cause)
	}
	return fmt.Sprintf("求值错误: %s", e.Message)
}

// Unwrap 返回底层错误。
func (e *EvaluationError) Unwrap() error {
	return e.Cause
}

// NewEvaluationError 创建一个新的 EvaluationError。
func NewEvaluationError(message string, cause error) *EvaluationError {
	return &EvaluationError{
		Message: message,
		Cause:   cause,
	}
}

// TypeMismatchError 表示求值期间的类型不匹配错误。
type TypeMismatchError struct {
	Expected string
	Got      string
	Value    any
}

// Error 实现 error 接口。
func (e *TypeMismatchError) Error() string {
	return fmt.Sprintf("类型不匹配: 期望 %s，得到 %s (值: %v)", e.Expected, e.Got, e.Value)
}

// NewTypeMismatchError 创建一个新的 TypeMismatchError。
func NewTypeMismatchError(expected, got string, value any) *TypeMismatchError {
	return &TypeMismatchError{
		Expected: expected,
		Got:      got,
		Value:    value,
	}
}

// VariableNotFoundError 表示变量未找到错误。
type VariableNotFoundError struct {
	Name string
}

// Error 实现 error 接口。
func (e *VariableNotFoundError) Error() string {
	return fmt.Sprintf("变量未找到: %s", e.Name)
}

// NewVariableNotFoundError 创建一个新的 VariableNotFoundError。
func NewVariableNotFoundError(name string) *VariableNotFoundError {
	return &VariableNotFoundError{Name: name}
}
