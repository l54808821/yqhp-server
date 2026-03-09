// Package types 定义工作流执行引擎的核心数据结构
package types

import (
	"context"
	"time"
)

// ExecutionCallback 定义执行过程中的回调接口
// 用于实时通知执行进度和结果
type ExecutionCallback interface {
	// OnStepStart 步骤开始执行时调用
	OnStepStart(ctx context.Context, step *Step, parentID string, iteration int)

	// OnStepComplete 步骤执行完成时调用（成功、失败、跳过统一走此方法）
	// result.Status 区分: success / failed / skipped
	OnStepComplete(ctx context.Context, step *Step, result *StepResult, parentID string, iteration int)

	// OnExecutionComplete 整个执行完成时调用
	OnExecutionComplete(ctx context.Context, summary *ExecutionSummary)
}

// AIStreamCallback 统一的 AI 流式回调接口
// 替代原 AICallback / AIToolCallback / AIThinkingCallback / AIPlanCallback
type AIStreamCallback interface {
	// OnAIChunk AI 流式文本块
	OnAIChunk(ctx context.Context, stepID, blockID, chunk string)

	// OnAIThinking AI 思考内容（纯推理文本，不包含控制信号）
	OnAIThinking(ctx context.Context, stepID, blockID, chunk string)

	// OnAIThinkingComplete 标记某个思考块已结束
	OnAIThinkingComplete(ctx context.Context, stepID, blockID string)

	// OnAIToolCallStart 工具调用开始
	OnAIToolCallStart(ctx context.Context, stepID, blockID string, toolCall *ToolCall)

	// OnAIToolCallComplete 工具调用完成（通过 result.IsError 区分成功/失败）
	OnAIToolCallComplete(ctx context.Context, stepID, blockID string, toolCall *ToolCall, result *ToolResult)

	// OnAIPlanUpdate 计划状态更新（合并原 started/step_update/completed/modified）
	OnAIPlanUpdate(ctx context.Context, stepID, blockID string, update *PlanUpdate)

	// OnMessageComplete AI 消息完成
	OnMessageComplete(ctx context.Context, stepID string, result *AIResult)

	// OnAIArtifactStart 产物生成开始
	OnAIArtifactStart(ctx context.Context, stepID, blockID string, fileType, title string)

	// OnAIArtifactChunk 产物内容流式片段
	OnAIArtifactChunk(ctx context.Context, stepID, blockID, chunk string)

	// OnAIArtifactComplete 产物生成完成
	OnAIArtifactComplete(ctx context.Context, stepID, blockID string, url string)

	// OnAIError AI 错误
	OnAIError(ctx context.Context, stepID string, err error)

	// OnAIInteractionRequired AI 需要交互，返回用户响应
	OnAIInteractionRequired(ctx context.Context, stepID string, request *InteractionRequest) (*InteractionResponse, error)
}

// AIResult AI 执行结果
type AIResult struct {
	Content          string `json:"content"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	TotalTokens      int    `json:"totalTokens"`
	Model            string `json:"model,omitempty"`
	FinishReason     string `json:"finishReason,omitempty"`
}

// PlanUpdateAction 计划更新动作
type PlanUpdateAction string

const (
	PlanActionStarted    PlanUpdateAction = "started"
	PlanActionStepUpdate PlanUpdateAction = "step_update"
	PlanActionModified   PlanUpdateAction = "modified"
	PlanActionCompleted  PlanUpdateAction = "completed"
)

// PlanUpdate 计划状态更新数据
type PlanUpdate struct {
	Action        PlanUpdateAction `json:"action"`
	Reason        string           `json:"reason,omitempty"`
	Steps         []PlanStepInfo   `json:"steps,omitempty"`
	StepIndex     int              `json:"stepIndex,omitempty"`
	Status        string           `json:"status,omitempty"`
	Result        string           `json:"result,omitempty"`
	Error         string           `json:"error,omitempty"`
	FromStepIndex int              `json:"fromStepIndex,omitempty"`
	NewSteps      []PlanStepInfo   `json:"newSteps,omitempty"`
	Synthesis     string           `json:"synthesis,omitempty"`
}

// PlanStepInfo 计划步骤信息
type PlanStepInfo struct {
	Index int    `json:"index"`
	Task  string `json:"task"`
}

// InteractionType 交互类型
type InteractionType string

const (
	InteractionTypeConfirm InteractionType = "confirm"
	InteractionTypeInput   InteractionType = "input"
	InteractionTypeSelect  InteractionType = "select"
)

// InteractionOption 交互选项
type InteractionOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// InteractionRequest 交互请求
type InteractionRequest struct {
	Type         InteractionType     `json:"type"`
	Prompt       string              `json:"prompt"`
	Options      []InteractionOption `json:"options,omitempty"`
	DefaultValue string              `json:"defaultValue,omitempty"`
	Timeout      int                 `json:"timeout"`
}

// InteractionResponse 交互响应
type InteractionResponse struct {
	Value   string `json:"value"`
	Skipped bool   `json:"skipped"`
}

// ExecutionSummary 执行汇总
type ExecutionSummary struct {
	ExecutionID   string        `json:"executionId"`
	TotalSteps    int           `json:"totalSteps"`
	SuccessSteps  int           `json:"successSteps"`
	FailedSteps   int           `json:"failedSteps"`
	TotalDuration time.Duration `json:"totalDuration"`
	Status        string        `json:"status"` // success, failed, timeout, stopped
	StartTime     time.Time     `json:"startTime"`
	EndTime       time.Time     `json:"endTime"`
}

// NoopCallback 空实现，用于不需要回调的场景
type NoopCallback struct{}

func (n *NoopCallback) OnStepStart(ctx context.Context, step *Step, parentID string, iteration int) {
}

func (n *NoopCallback) OnStepComplete(ctx context.Context, step *Step, result *StepResult, parentID string, iteration int) {
}

func (n *NoopCallback) OnExecutionComplete(ctx context.Context, summary *ExecutionSummary) {}

// 确保 NoopCallback 实现了 ExecutionCallback 接口
var _ ExecutionCallback = (*NoopCallback)(nil)
