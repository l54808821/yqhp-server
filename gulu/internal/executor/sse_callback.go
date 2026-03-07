package executor

import (
	"context"
	"time"

	"yqhp/gulu/internal/sse"
	"yqhp/workflow-engine/pkg/types"
)

// SSECallback SSE 回调实现（实现 ExecutionCallback + AIStreamCallback）
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

// ============ ExecutionCallback ============

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

func (c *SSECallback) OnStepComplete(ctx context.Context, step *types.Step, result *types.StepResult, parentID string, iteration int) {
	isSuccess := result.Status == types.ResultStatusSuccess

	if isSuccess {
		c.session.IncrementSuccess()
	} else if result.Status != types.ResultStatusSkipped {
		c.session.IncrementFailed()
	}

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

	status := "success"
	switch result.Status {
	case types.ResultStatusFailed, types.ResultStatusTimeout:
		status = "failed"
	case types.ResultStatusSkipped:
		status = "skipped"
	}

	data := &sse.StepCompletedData{
		StepID:     step.ID,
		StepName:   step.Name,
		StepType:   step.Type,
		ParentID:   parentID,
		Iteration:  iteration,
		Status:     status,
		DurationMs: result.Duration.Milliseconds(),
		Result:     result.Output,
	}
	if result.Error != nil {
		data.Error = result.Error.Error()
	}

	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventStepCompleted,
		Data: data,
	})
}

func (c *SSECallback) OnExecutionComplete(ctx context.Context, summary *types.ExecutionSummary) {
	total, success, failed := c.session.GetStats()

	status := "success"
	if failed > 0 {
		status = "failed"
	}
	if c.session.GetStatus() == SessionStatusStopped {
		status = "stopped"
	}

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

// ============ AIStreamCallback ============

func (c *SSECallback) OnAIChunk(ctx context.Context, stepID, blockID, chunk string) {
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventAIChunk,
		Data: map[string]interface{}{
			"blockId": blockID,
			"stepId":  stepID,
			"chunk":   chunk,
		},
	})
}

func (c *SSECallback) OnAIThinking(ctx context.Context, stepID, blockID, chunk string) {
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventAIThinking,
		Data: map[string]interface{}{
			"blockId": blockID,
			"stepId":  stepID,
			"chunk":   chunk,
		},
	})
}

func (c *SSECallback) OnAIThinkingComplete(ctx context.Context, stepID, blockID string) {
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventAIThinking,
		Data: map[string]interface{}{
			"blockId":    blockID,
			"stepId":     stepID,
			"isComplete": true,
		},
	})
}

func (c *SSECallback) OnAIToolCallStart(ctx context.Context, stepID, blockID string, toolCall *types.ToolCall) {
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventAIToolCallStart,
		Data: map[string]interface{}{
			"blockId":   blockID,
			"stepId":    stepID,
			"toolName":  toolCall.Name,
			"arguments": toolCall.Arguments,
		},
	})
}

func (c *SSECallback) OnAIToolCallComplete(ctx context.Context, stepID, blockID string, toolCall *types.ToolCall, result *types.ToolResult) {
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventAIToolCallComplete,
		Data: map[string]interface{}{
			"blockId":    blockID,
			"stepId":     stepID,
			"toolName":   toolCall.Name,
			"result":     result.GetLLMContent(),
			"isError":    result.IsError,
			"durationMs": toolCall.DurationMs,
		},
	})
}

func (c *SSECallback) OnAIPlanUpdate(ctx context.Context, stepID, blockID string, update *types.PlanUpdate) {
	data := map[string]interface{}{
		"blockId": blockID,
		"stepId":  stepID,
		"action":  update.Action,
	}
	if update.Reason != "" {
		data["reason"] = update.Reason
	}
	if update.Steps != nil {
		data["steps"] = update.Steps
	}
	if update.StepIndex != 0 {
		data["stepIndex"] = update.StepIndex
	}
	if update.Status != "" {
		data["status"] = update.Status
	}
	if update.Result != "" {
		data["result"] = update.Result
	}
	if update.Error != "" {
		data["error"] = update.Error
	}
	if update.FromStepIndex != 0 {
		data["fromStepIndex"] = update.FromStepIndex
	}
	if update.NewSteps != nil {
		data["newSteps"] = update.NewSteps
	}
	if update.Synthesis != "" {
		data["synthesis"] = update.Synthesis
	}
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventAIPlanUpdate,
		Data: data,
	})
}

func (c *SSECallback) OnMessageComplete(ctx context.Context, stepID string, result *types.AIResult) {
	c.writer.WriteEvent(&sse.Event{
		Type: sse.EventMessageComplete,
		Data: map[string]interface{}{
			"stepId":  stepID,
			"content": result.Content,
			"usage": map[string]int{
				"prompt_tokens":     result.PromptTokens,
				"completion_tokens": result.CompletionTokens,
				"total_tokens":      result.TotalTokens,
			},
			"model": result.Model,
		},
	})
}

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

func (c *SSECallback) OnAIInteractionRequired(ctx context.Context, stepID string, request *types.InteractionRequest) (*types.InteractionResponse, error) {
	var options []sse.InteractionOption
	for _, opt := range request.Options {
		options = append(options, sse.InteractionOption{
			Value: opt.Value,
			Label: opt.Label,
		})
	}

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

	resp, err := c.session.WaitForInteraction(ctx, time.Duration(request.Timeout)*time.Second)
	if err != nil {
		return nil, err
	}

	return &types.InteractionResponse{
		Value:   resp.Value,
		Skipped: resp.Skipped,
	}, nil
}

// ============ 辅助方法 ============

func (c *SSECallback) WriteHeartbeat() error {
	return c.writer.WriteHeartbeat()
}

func (c *SSECallback) WriteError(code, message, details string, recoverable bool) error {
	return c.writer.WriteError(code, message, details, recoverable)
}

func (c *SSECallback) WriteErrorCode(code sse.ErrorCode, message string, details string) error {
	return c.writer.WriteErrorCode(code, message, details)
}

var _ types.ExecutionCallback = (*SSECallback)(nil)
var _ types.AIStreamCallback = (*SSECallback)(nil)
