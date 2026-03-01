package ai

import (
	"time"

	"yqhp/workflow-engine/pkg/types"
)

const (
	defaultAITimeout         = 5 * time.Minute
	defaultMaxToolRounds     = 15
	defaultMaxPlanSteps      = 10

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

// AgentTrace 统一 Agent 执行轨迹
// Mode: "direct" (直接回答) | "react" (ReAct 工具调用) | "plan" (Plan 模式)
type AgentTrace struct {
	Mode  string       `json:"mode"`
	ReAct []ReActRound `json:"react,omitempty"`
	Plan  *PlanTrace   `json:"plan,omitempty"`
}

// ReActRound 单轮 ReAct 推理记录
type ReActRound struct {
	Round     int              `json:"round"`
	Thinking  string           `json:"thinking,omitempty"`
	ToolCalls []ToolCallRecord `json:"tool_calls,omitempty"`
}

// PlanTrace Plan 模式的执行轨迹
type PlanTrace struct {
	Reason    string     `json:"reason"`
	PlanText  string     `json:"plan_text"`
	Steps     []PlanStep `json:"steps"`
	Synthesis string     `json:"synthesis,omitempty"`
}

// PlanStep 计划中的单个步骤
type PlanStep struct {
	Index     int              `json:"index"`
	Task      string           `json:"task"`
	Status    string           `json:"status"`
	Thinking  string           `json:"thinking,omitempty"`
	Result    string           `json:"result,omitempty"`
	ToolCalls []ToolCallRecord `json:"tool_calls,omitempty"`
}
