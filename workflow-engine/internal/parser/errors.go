package parser

import (
	"fmt"
)

// ParseError 表示带有位置信息的解析错误。
type ParseError struct {
	Line    int    // 错误发生的行号（从 1 开始）
	Column  int    // 错误发生的列号（从 1 开始）
	Message string // 错误消息
	Cause   error  // 底层错误
}

// Error 实现 error 接口。
func (e *ParseError) Error() string {
	if e.Line > 0 && e.Column > 0 {
		return fmt.Sprintf("解析错误，位于第 %d 行第 %d 列: %s", e.Line, e.Column, e.Message)
	}
	if e.Line > 0 {
		return fmt.Sprintf("解析错误，位于第 %d 行: %s", e.Line, e.Message)
	}
	return fmt.Sprintf("解析错误: %s", e.Message)
}

// Unwrap 返回底层错误。
func (e *ParseError) Unwrap() error {
	return e.Cause
}

// NewParseError 创建一个新的 ParseError。
func NewParseError(line, column int, message string, cause error) *ParseError {
	return &ParseError{
		Line:    line,
		Column:  column,
		Message: message,
		Cause:   cause,
	}
}

// ValidationError 表示工作流定义的验证错误。
type ValidationError struct {
	Field   string // 验证失败的字段
	Message string // 错误消息
}

// Error 实现 error 接口。
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("字段 '%s' 验证错误: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("验证错误: %s", e.Message)
}

// NewValidationError 创建一个新的 ValidationError。
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// VariableResolutionError 表示解析变量引用时的错误。
type VariableResolutionError struct {
	Reference string // 解析失败的变量引用
	Message   string // 错误消息
	Cause     error  // 底层错误
}

// Error 实现 error 接口。
func (e *VariableResolutionError) Error() string {
	return fmt.Sprintf("解析变量 '%s' 失败: %s", e.Reference, e.Message)
}

// Unwrap 返回底层错误。
func (e *VariableResolutionError) Unwrap() error {
	return e.Cause
}

// NewVariableResolutionError 创建一个新的 VariableResolutionError。
func NewVariableResolutionError(ref, message string, cause error) *VariableResolutionError {
	return &VariableResolutionError{
		Reference: ref,
		Message:   message,
		Cause:     cause,
	}
}
