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
	ID            string `json:"id"`
	Name          string `json:"name"`
	Arguments     string `json:"arguments"`
	PlanStepIndex int    `json:"planStepIndex,omitempty"`
	DurationMs    int64  `json:"durationMs,omitempty"`
}

// ToolResult 工具执行结果（增强版）
// ForLLM: 返回给 LLM 的内容（用于下一轮推理）
// ForUser: 返回给用户的内容（用于前端展示，可选）
// Silent: 为 true 时不向用户展示此工具的输出
// Async: 为 true 时表示工具已异步启动，结果将稍后返回
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`     // 兼容旧字段，等同于 ForLLM
	ForLLM     string `json:"for_llm"`     // 给 LLM 看的内容
	ForUser    string `json:"for_user"`    // 给用户看的内容
	IsError    bool   `json:"is_error"`
	Silent     bool   `json:"silent"`
	Async      bool   `json:"async"`
}

// GetLLMContent 获取给 LLM 的内容，优先使用 ForLLM，回退到 Content
func (r *ToolResult) GetLLMContent() string {
	if r.ForLLM != "" {
		return r.ForLLM
	}
	return r.Content
}

// NewToolResult 创建标准工具结果
func NewToolResult(forLLM string) *ToolResult {
	return &ToolResult{ForLLM: forLLM, Content: forLLM}
}

// NewErrorResult 创建错误结果
func NewErrorResult(msg string) *ToolResult {
	return &ToolResult{ForLLM: msg, Content: msg, IsError: true}
}

// NewSilentResult 创建静默结果（不展示给用户）
func NewSilentResult(forLLM string) *ToolResult {
	return &ToolResult{ForLLM: forLLM, Content: forLLM, Silent: true}
}

// NewUserResult 创建同时给 LLM 和用户的结果
func NewUserResult(forLLM, forUser string) *ToolResult {
	return &ToolResult{ForLLM: forLLM, Content: forLLM, ForUser: forUser}
}

// NewAsyncResult 创建异步结果
func NewAsyncResult(msg string) *ToolResult {
	return &ToolResult{ForLLM: msg, Content: msg, Async: true}
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
