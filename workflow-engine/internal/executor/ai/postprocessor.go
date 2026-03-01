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
	envVars := make(map[string]interface{})
	if execCtx.Variables != nil {
		for k, v := range execCtx.Variables {
			variables[k] = v
		}
	}

	procExecutor := pkgExecutor.NewProcessorExecutor(variables, envVars)

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

	for _, entry := range postLogs {
		if entry.Type != types.LogTypeProcessor || entry.Processor == nil {
			continue
		}
		pOutput := entry.Processor.Output
		if pOutput == nil {
			continue
		}
		if entry.Processor.Type == "set_variable" || entry.Processor.Type == "extract_param" {
			varName, _ := pOutput["variableName"].(string)
			if varName == "" {
				continue
			}
			scope, _ := pOutput["scope"].(string)
			if scope == "" {
				scope = "temp"
			}
			source, _ := pOutput["source"].(string)
			if source == "" {
				source = entry.Processor.Type
			}
			execCtx.AppendLog(types.NewVariableChangeEntry(types.VariableChangeInfo{
				Name:     varName,
				OldValue: pOutput["oldValue"],
				NewValue: pOutput["value"],
				Scope:    scope,
				Source:   source,
			}))
			if scope == "env" {
				execCtx.MarkAsEnvVar(varName)
			}
		}
		if entry.Processor.Type == "js_script" {
			if varChanges, ok := pOutput["varChanges"].([]map[string]any); ok {
				for _, change := range varChanges {
					name, _ := change["name"].(string)
					if name == "" {
						continue
					}
					s, _ := change["scope"].(string)
					src, _ := change["source"].(string)
					execCtx.AppendLog(types.NewVariableChangeEntry(types.VariableChangeInfo{
						Name:     name,
						OldValue: change["oldValue"],
						NewValue: change["newValue"],
						Scope:    s,
						Source:   src,
					}))
					if s == "env" {
						execCtx.MarkAsEnvVar(name)
					}
				}
			}
		}
	}

	if execCtx.Variables != nil {
		for k, v := range procExecutor.GetVariables() {
			execCtx.Variables[k] = v
		}
	}

	allLogs := execCtx.FlushLogs()
	if len(allLogs) > 0 {
		output.ConsoleLogs = allLogs
	}
}
