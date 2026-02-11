// Package types 定义工作流执行引擎的核心数据结构
package types

import "encoding/json"

// ToolDefinition 工具定义
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// ToolCall AI 模型发出的工具调用
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolResult 工具执行结果
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error"`
}

// OpenAIFunctionTool OpenAI function calling 格式的工具定义
type OpenAIFunctionTool struct {
	Type     string            `json:"type"`
	Function OpenAIFunctionDef `json:"function"`
}

// OpenAIFunctionDef OpenAI function 定义
type OpenAIFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToOpenAIFunction 将 ToolDefinition 序列化为 OpenAI function calling 兼容格式
func (td *ToolDefinition) ToOpenAIFunction() *OpenAIFunctionTool {
	return &OpenAIFunctionTool{
		Type: "function",
		Function: OpenAIFunctionDef{
			Name:        td.Name,
			Description: td.Description,
			Parameters:  td.Parameters,
		},
	}
}
