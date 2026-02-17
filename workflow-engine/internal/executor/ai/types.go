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

	// defaultMaxReflectionRounds 默认最大反思轮次
	defaultMaxReflectionRounds = 2

	// defaultMaxPlanSteps 默认最大计划步骤数
	defaultMaxPlanSteps = 10

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

// ============ 统一 Agent 轨迹 ============

// AgentTrace 统一的 Agent 模式执行轨迹
type AgentTrace struct {
	Mode           string           `json:"mode"`
	ReAct          []ReActRound     `json:"react,omitempty"`
	PlanAndExecute *PlanExecTrace   `json:"plan_and_execute,omitempty"`
	Reflection     *ReflectionTrace `json:"reflection,omitempty"`
}

// ============ ReAct 模式 ============

// ReActRound 单轮 ReAct 推理记录（Thinking → Action → Observation）
type ReActRound struct {
	Round     int              `json:"round"`
	Thinking  string           `json:"thinking,omitempty"`
	ToolCalls []ToolCallRecord `json:"tool_calls,omitempty"`
}

// ============ Plan-and-Execute 模式 ============

// PlanExecTrace Plan-and-Execute 模式的执行轨迹
type PlanExecTrace struct {
	Plan      string     `json:"plan"`
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

// ============ Reflection 模式 ============

// ReflectionTrace Reflection 模式的执行轨迹
type ReflectionTrace struct {
	Rounds      []ReflectionRound `json:"rounds"`
	FinalAnswer string            `json:"final_answer,omitempty"`
}

// ReflectionRound 单轮反思记录
type ReflectionRound struct {
	Round    int    `json:"round"`
	Draft    string `json:"draft"`
	Critique string `json:"critique"`
}
