package ai

import (
	"time"

	"yqhp/workflow-engine/pkg/types"
)

const (
	// AIExecutorType AI 执行器类型标识符
	AIExecutorType = "ai"

	// defaultAITimeout AI 调用默认超时时间
	defaultAITimeout = 5 * time.Minute

	// defaultMaxToolRounds 默认最大工具调用轮次
	defaultMaxToolRounds = 10

	// defaultMCPProxyBaseURL 默认 MCP 代理服务地址
	defaultMCPProxyBaseURL = "http://localhost:8080"
)

// AIOutput AI 节点输出
type AIOutput struct {
	Content          string                  `json:"content"`
	PromptTokens     int                     `json:"prompt_tokens"`
	CompletionTokens int                     `json:"completion_tokens"`
	TotalTokens      int                     `json:"total_tokens"`
	Model            string                  `json:"model"`
	FinishReason     string                  `json:"finish_reason"`
	SystemPrompt     string                  `json:"system_prompt,omitempty"`
	Prompt           string                  `json:"prompt"`
	ToolCalls        []ToolCallRecord        `json:"tool_calls,omitempty"`
	ConsoleLogs      []types.ConsoleLogEntry `json:"console_logs,omitempty"`
}

// ToolCallRecord 工具调用记录
type ToolCallRecord struct {
	Round     int    `json:"round"`
	ToolName  string `json:"tool_name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
	IsError   bool   `json:"is_error"`
	Duration  int64  `json:"duration_ms"`
}
