package ai

import (
	"time"

	"yqhp/workflow-engine/pkg/types"
)

const (
	defaultAITimeout         = 5 * time.Minute
	defaultMaxToolRounds     = 15
	defaultToolTimeout       = 180 * time.Second
	defaultMaxToolConcurrent = 5

	defaultQdrantHost = "http://127.0.0.1:6333"
	defaultGuluHost   = "http://127.0.0.1:5321"
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
	AgentTrace       *AgentTrace             `json:"agent_trace,omitempty"`
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

// AgentTrace Agent 执行轨迹
type AgentTrace struct {
	Mode   string       `json:"mode"`
	Rounds []AgentRound `json:"rounds,omitempty"`
}

// AgentRound 单轮 Agent 推理记录
type AgentRound struct {
	Round     int              `json:"round"`
	Thinking  string           `json:"thinking,omitempty"`
	ToolCalls []ToolCallRecord `json:"tool_calls,omitempty"`
}
