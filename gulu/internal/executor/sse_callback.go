package executor

import (
	"context"
	"encoding/json"
	"time"

	"yqhp/gulu/internal/sse"
	"yqhp/workflow-engine/pkg/types"
)

// SSECallback SSE 回调实现
type SSECallback struct {
	writer  *sse.Writer
	session *Session
}

// NewSSECallback 创建 SSE 回调
func NewSSECallback(writer *sse.Writer, session *Session) *SSECallback {
	return &SSECallback{
		writer:  writer,
		session: session,
	}
}

// OnStepStart 步骤开始
func (c *SSECallback) OnStepStart(ctx context.Context, step *types.Step, parentID string, iteration int) {
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventStepStarted,
		Data: &sse.StepStartedData{
			StepID:    step.ID,
			StepName:  step.Name,
			StepType:  step.Type,
			ParentID:  parentID,
			Iteration: iteration,
		},
	})
}

// OnStepComplete 步骤完成（成功和失败都会调用此方法）
// task_engine 中所有完成的步骤都统一通过 OnStepComplete 传递完整的 StepResult，
// 失败只是执行结果的一种状态，通过 result.Status 区分。
func (c *SSECallback) OnStepComplete(ctx context.Context, step *types.Step, result *types.StepResult, parentID string, iteration int) {
	isSuccess := result.Status == types.ResultStatusSuccess

	// 更新统计计数
	if isSuccess {
		c.session.IncrementSuccess()
	} else {
		c.session.IncrementFailed()
	}

	// 转换 output 为 map（用于 SSE 事件）
	var outputMap map[string]interface{}
	if result.Output != nil {
		if m, ok := result.Output.(map[string]interface{}); ok {
			outputMap = m
		} else {
			// 通过 JSON 序列化转换
			if jsonBytes, err := json.Marshal(result.Output); err == nil {
				json.Unmarshal(jsonBytes, &outputMap)
			}
		}
	}

	// 收集步骤执行结果（用于阻塞模式返回）
	stepResult := StepExecutionResult{
		StepID:     step.ID,
		StepName:   step.Name,
		StepType:   step.Type,
		Success:    isSuccess,
		DurationMs: result.Duration.Milliseconds(),
		Result:     result.Output,
	}
	if result.Error != nil {
		stepResult.Error = result.Error.Error()
	}
	c.session.AddStepResult(stepResult)

	// 发送 SSE 事件
	if isSuccess {
		c.writer.WriteEvent(&sse.Event{
			Type: sse.EventStepCompleted,
			Data: &sse.StepCompletedData{
				StepID:    step.ID,
				StepName:  step.Name,
				StepType:  step.Type,
				ParentID:  parentID,
				Iteration: iteration,
				Status:    "success",
				Success:   true,
				Duration:  result.Duration.Milliseconds(),
				Output:    outputMap,
				Result:    result.Output,
			},
		})
	} else {
		errMsg := ""
		if result.Error != nil {
			errMsg = result.Error.Error()
		}
		c.writer.WriteEvent(&sse.Event{
			Type: sse.EventStepFailed,
			Data: &sse.StepFailedData{
				StepID:    step.ID,
				StepName:  step.Name,
				StepType:  step.Type,
				ParentID:  parentID,
				Iteration: iteration,
				Status:    "failed",
				Error:     errMsg,
				Duration:  result.Duration.Milliseconds(),
				Result:    result.Output,
			},
		})
	}
}

// OnStepFailed 步骤失败通知
// 结果已在 OnStepComplete 中完整处理（含 Output、Error、统计计数），
// 此方法仅作为额外的失败通知钩子，当前无需额外操作。
func (c *SSECallback) OnStepFailed(ctx context.Context, step *types.Step, err error, duration time.Duration, parentID string, iteration int) {
	// 结果已在 OnStepComplete 中处理，不需要重复
}

// OnStepSkipped 步骤被跳过
func (c *SSECallback) OnStepSkipped(ctx context.Context, step *types.Step, reason string, parentID string, iteration int) {
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventStepSkipped,
		Data: &sse.StepSkippedData{
			StepID:    step.ID,
			StepName:  step.Name,
			StepType:  step.Type,
			ParentID:  parentID,
			Iteration: iteration,
			Reason:    reason,
		},
	})
}

// OnProgress 进度更新
func (c *SSECallback) OnProgress(ctx context.Context, current, total int, stepName string) {
	percentage := 0
	if total > 0 {
		percentage = current * 100 / total
	}

	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventProgress,
		Data: &sse.ProgressData{
			CurrentStep: current,
			TotalSteps:  total,
			Percentage:  percentage,
			StepName:    stepName,
		},
	})
}

// OnExecutionComplete 执行完成
func (c *SSECallback) OnExecutionComplete(ctx context.Context, summary *types.ExecutionSummary) {
	total, success, failed := c.session.GetStats()

	status := "success"
	if failed > 0 {
		status = "failed"
	}
	if c.session.GetStatus() == SessionStatusStopped {
		status = "stopped"
	}

	// 获取最终变量
	finalVars := c.session.GetVariables()
	envVars := c.session.GetEnvVariables()

	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventWorkflowCompleted,
		Data: &sse.WorkflowCompletedData{
			SessionID:     c.session.ID,
			TotalSteps:    total,
			SuccessSteps:  success,
			FailedSteps:   failed,
			TotalDuration: time.Since(c.session.StartTime).Milliseconds(),
			Status:        status,
			Variables:     finalVars,
			EnvVariables:  envVars,
		},
	})
}

// ============ AI 相关回调 (实现 AICallback 接口) ============

// OnAIChunk AI 流式输出块
func (c *SSECallback) OnAIChunk(ctx context.Context, stepID string, chunk string, index int) {
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventAIChunk,
		Data: &sse.AIChunkData{
			StepID: stepID,
			Chunk:  chunk,
			Index:  index,
		},
	})
}

// OnAIComplete AI 完成
func (c *SSECallback) OnAIComplete(ctx context.Context, stepID string, result *types.AIResult) {
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventAIComplete,
		Data: &sse.AICompleteData{
			StepID:           stepID,
			Content:          result.Content,
			PromptTokens:     result.PromptTokens,
			CompletionTokens: result.CompletionTokens,
			TotalTokens:      result.TotalTokens,
		},
	})
}

// OnAIError AI 错误
func (c *SSECallback) OnAIError(ctx context.Context, stepID string, err error) {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventAIError,
		Data: &sse.AIErrorData{
			StepID: stepID,
			Error:  errMsg,
		},
	})
}

// OnAIInteractionRequired AI 需要交互
func (c *SSECallback) OnAIInteractionRequired(ctx context.Context, stepID string, request *types.InteractionRequest) (*types.InteractionResponse, error) {
	// 转换选项
	var options []sse.InteractionOption
	for _, opt := range request.Options {
		options = append(options, sse.InteractionOption{
			Value: opt.Value,
			Label: opt.Label,
		})
	}

	// 发送交互请求事件
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventAIInteraction,
		Data: &sse.AIInteractionData{
			StepID:       stepID,
			Type:         sse.InteractionType(request.Type),
			Prompt:       request.Prompt,
			Options:      options,
			DefaultValue: request.DefaultValue,
			Timeout:      request.Timeout,
		},
	})

	// 等待用户响应
	resp, err := c.session.WaitForInteraction(ctx, time.Duration(request.Timeout)*time.Second)
	if err != nil {
		return nil, err
	}

	return &types.InteractionResponse{
		Value:   resp.Value,
		Skipped: resp.Skipped,
	}, nil
}

// WriteHeartbeat 写入心跳
func (c *SSECallback) WriteHeartbeat() error {
	return c.writer.WriteHeartbeat()
}

// WriteError 写入错误
func (c *SSECallback) WriteError(code, message, details string, recoverable bool) error {
	return c.writer.WriteError(code, message, details, recoverable)
}

// WriteErrorCode 使用错误码写入错误
func (c *SSECallback) WriteErrorCode(code sse.ErrorCode, message string, details string) error {
	return c.writer.WriteErrorCode(code, message, details)
}

// 确保 SSECallback 实现了 ExecutionCallback 接口
var _ types.ExecutionCallback = (*SSECallback)(nil)

// 确保 SSECallback 实现了 AICallback 接口
var _ types.AICallback = (*SSECallback)(nil)

// 确保 SSECallback 实现了 AIToolCallback 接口
var _ types.AIToolCallback = (*SSECallback)(nil)

// ============ AI 推理过程回调 (实现 AIThinkingCallback 接口) ============

// OnAIThinking AI 推理思考（ReAct 模式每轮的推理内容）
func (c *SSECallback) OnAIThinking(ctx context.Context, stepID string, round int, thinking string) {
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventAIThinking,
		Data: &sse.AIThinkingData{
			StepID:   stepID,
			Round:    round,
			Thinking: thinking,
		},
	})
}

// 确保 SSECallback 实现了 AIThinkingCallback 接口
var _ types.AIThinkingCallback = (*SSECallback)(nil)

// ============ AI 工具调用回调 (实现 AIToolCallback 接口) ============

// OnAIToolCallStart 工具调用开始
func (c *SSECallback) OnAIToolCallStart(ctx context.Context, stepID string, toolCall *types.ToolCall) {
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventAIToolCallStart,
		Data: &sse.AIToolCallStartData{
			StepID:    stepID,
			ToolName:  toolCall.Name,
			Arguments: toolCall.Arguments,
		},
	})
}

// OnAIToolCallComplete 工具调用完成
func (c *SSECallback) OnAIToolCallComplete(ctx context.Context, stepID string, toolCall *types.ToolCall, result *types.ToolResult) {
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventAIToolCallComplete,
		Data: &sse.AIToolCallCompleteData{
			StepID:    stepID,
			ToolName:  toolCall.Name,
			Arguments: toolCall.Arguments,
			Result:    result.Content,
			IsError:   result.IsError,
		},
	})
}
