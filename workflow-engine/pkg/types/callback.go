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
	// step: 当前步骤
	// parentID: 父步骤ID（循环内的子步骤会有这个值）
	// iteration: 迭代次数（从1开始，非循环步骤为0）
	OnStepStart(ctx context.Context, step *Step, parentID string, iteration int)

	// OnStepComplete 步骤执行成功时调用
	// step: 当前步骤
	// result: 执行结果
	// parentID: 父步骤ID
	// iteration: 迭代次数
	OnStepComplete(ctx context.Context, step *Step, result *StepResult, parentID string, iteration int)

	// OnStepFailed 步骤执行失败时调用
	// step: 当前步骤
	// err: 错误信息
	// duration: 执行耗时
	// parentID: 父步骤ID
	// iteration: 迭代次数
	OnStepFailed(ctx context.Context, step *Step, err error, duration time.Duration, parentID string, iteration int)

	// OnStepSkipped 步骤被跳过时调用
	// step: 当前步骤
	// reason: 跳过原因
	// parentID: 父步骤ID
	// iteration: 迭代次数
	OnStepSkipped(ctx context.Context, step *Step, reason string, parentID string, iteration int)

	// OnProgress 进度更新时调用
	// current: 当前步骤序号
	// total: 总步骤数（动态执行时可能不准确）
	// stepName: 当前步骤名称
	OnProgress(ctx context.Context, current, total int, stepName string)

	// OnExecutionComplete 整个执行完成时调用
	// summary: 执行汇总
	OnExecutionComplete(ctx context.Context, summary *ExecutionSummary)
}

// AICallback AI 节点回调接口（可选实现）
type AICallback interface {
	// OnAIChunk AI 流式输出块
	OnAIChunk(ctx context.Context, stepID string, chunk string, index int)

	// OnAIComplete AI 完成
	OnAIComplete(ctx context.Context, stepID string, result *AIResult)

	// OnAIError AI 错误
	OnAIError(ctx context.Context, stepID string, err error)

	// OnAIInteractionRequired AI 需要交互
	// 返回用户响应，如果超时返回 nil
	OnAIInteractionRequired(ctx context.Context, stepID string, request *InteractionRequest) (*InteractionResponse, error)
}

// AIResult AI 执行结果
type AIResult struct {
	Content          string `json:"content"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
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
	DefaultValue string              `json:"default_value,omitempty"`
	Timeout      int                 `json:"timeout"`
}

// InteractionResponse 交互响应
type InteractionResponse struct {
	Value   string `json:"value"`
	Skipped bool   `json:"skipped"`
}

// ExecutionSummary 执行汇总
type ExecutionSummary struct {
	ExecutionID   string        `json:"execution_id"`
	TotalSteps    int           `json:"total_steps"`
	SuccessSteps  int           `json:"success_steps"`
	FailedSteps   int           `json:"failed_steps"`
	TotalDuration time.Duration `json:"total_duration"`
	Status        string        `json:"status"` // success, failed, timeout, stopped
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
}

// NoopCallback 空实现，用于不需要回调的场景
type NoopCallback struct{}

func (n *NoopCallback) OnStepStart(ctx context.Context, step *Step, parentID string, iteration int) {
}

func (n *NoopCallback) OnStepComplete(ctx context.Context, step *Step, result *StepResult, parentID string, iteration int) {
}

func (n *NoopCallback) OnStepFailed(ctx context.Context, step *Step, err error, duration time.Duration, parentID string, iteration int) {
}

func (n *NoopCallback) OnStepSkipped(ctx context.Context, step *Step, reason string, parentID string, iteration int) {
}

func (n *NoopCallback) OnProgress(ctx context.Context, current, total int, stepName string) {}

func (n *NoopCallback) OnExecutionComplete(ctx context.Context, summary *ExecutionSummary) {}

// 确保 NoopCallback 实现了 ExecutionCallback 接口
var _ ExecutionCallback = (*NoopCallback)(nil)

// AIThinkingCallback AI 推理过程回调接口（可选实现）
// 用于 ReAct 等 Agent 模式下，实时推送每轮的推理思考内容
type AIThinkingCallback interface {
	// OnAIThinking AI 推理思考（每轮工具调用前的推理内容）
	OnAIThinking(ctx context.Context, stepID string, round int, thinking string)
}

// AIToolCallback AI 工具调用回调接口（可选实现）
type AIToolCallback interface {
	AICallback // 继承现有接口

	// OnAIToolCallStart 工具调用开始
	OnAIToolCallStart(ctx context.Context, stepID string, toolCall *ToolCall)

	// OnAIToolCallComplete 工具调用完成
	OnAIToolCallComplete(ctx context.Context, stepID string, toolCall *ToolCall, result *ToolResult)
}
