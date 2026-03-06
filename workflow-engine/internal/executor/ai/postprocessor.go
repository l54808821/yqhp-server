package ai

import (
	"context"
	"encoding/json"
	"time"

	"yqhp/workflow-engine/internal/executor"
	pkgExecutor "yqhp/workflow-engine/pkg/executor"
	"yqhp/workflow-engine/pkg/types"
)

// executePostProcessors 执行 AI 节点的后置处理器
func executePostProcessors(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext, output *AIOutput, startTime time.Time) {
	if len(step.PostProcessors) == 0 || execCtx == nil {
		return
	}

	variables := make(map[string]interface{})
	if execCtx.Variables != nil {
		for k, v := range execCtx.Variables {
			variables[k] = v
		}
	}

	procExecutor := pkgExecutor.NewProcessorExecutorWithCallbacks(
		variables,
		func(key string) (interface{}, bool) {
			return execCtx.GetVariable(key)
		},
		func(key string, value interface{}) {
			execCtx.SetVariable(key, value)
		},
	)

	toolCallsJSON := "[]"
	if len(output.ToolCalls) > 0 {
		if data, err := json.Marshal(output.ToolCalls); err == nil {
			toolCallsJSON = string(data)
		}
	}

	responseBody := output.Content
	var jsonTest json.RawMessage
	if json.Unmarshal([]byte(output.Content), &jsonTest) != nil {
		wrapped := map[string]interface{}{
			"content":           output.Content,
			"model":             output.Model,
			"finish_reason":     output.FinishReason,
			"prompt_tokens":     output.PromptTokens,
			"completion_tokens": output.CompletionTokens,
			"total_tokens":      output.TotalTokens,
		}
		if data, err := json.Marshal(wrapped); err == nil {
			responseBody = string(data)
		}
	}

	procExecutor.SetResponse(map[string]interface{}{
		"body":              responseBody,
		"content":           output.Content,
		"model":             output.Model,
		"finish_reason":     output.FinishReason,
		"prompt_tokens":     output.PromptTokens,
		"completion_tokens": output.CompletionTokens,
		"total_tokens":      output.TotalTokens,
		"tool_calls":        toolCallsJSON,
		"duration":          time.Since(startTime).Milliseconds(),
	})

	postLogs := procExecutor.ExecuteProcessors(ctx, step.PostProcessors, "post")
	execCtx.AppendLogs(postLogs)

	allLogs := execCtx.FlushLogs()
	if len(allLogs) > 0 {
		output.ConsoleLogs = allLogs
	}
}
