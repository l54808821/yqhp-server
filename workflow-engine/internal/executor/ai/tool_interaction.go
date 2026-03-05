package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// HumanInteractionTool 人机交互工具，实现统一 Tool 接口
type HumanInteractionTool struct {
	config   *AIConfig
	stepID   string
	callback types.AIStreamCallback
}

func NewHumanInteractionTool(config *AIConfig) *HumanInteractionTool {
	return &HumanInteractionTool{config: config}
}

func (t *HumanInteractionTool) SetContext(stepID string, callback types.AIStreamCallback) {
	t.stepID = stepID
	t.callback = callback
}

func (t *HumanInteractionTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "human_interaction",
		Description: "当你需要用户确认、输入信息或从选项中选择时，调用此工具与用户进行交互。用户将看到你提供的提示并作出响应。仅在确实需要人类介入时使用。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"type": {
					"type": "string",
					"enum": ["confirm", "input", "select"],
					"description": "交互类型：confirm（确认/拒绝）、input（自由文本输入）、select（从选项中选择）"
				},
				"prompt": {
					"type": "string",
					"description": "展示给用户的提示信息"
				},
				"options": {
					"type": "array",
					"items": { "type": "string" },
					"description": "当 type 为 select 时，提供给用户的选项列表"
				},
				"default_value": {
					"type": "string",
					"description": "超时时使用的默认值"
				}
			},
			"required": ["type", "prompt"]
		}`),
	}
}

func (t *HumanInteractionTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	if t.callback == nil {
		return types.NewErrorResult("人机交互不可用：缺少回调接口"), nil
	}

	var args struct {
		Type         string   `json:"type"`
		Prompt       string   `json:"prompt"`
		Options      []string `json:"options,omitempty"`
		DefaultValue string   `json:"default_value,omitempty"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Type == "" {
		args.Type = "confirm"
	}
	if args.Prompt == "" {
		return types.NewErrorResult("缺少必填参数: prompt"), nil
	}

	request := &types.InteractionRequest{
		Type:         types.InteractionType(args.Type),
		Prompt:       args.Prompt,
		DefaultValue: args.DefaultValue,
		Timeout:      t.config.InteractionTimeout,
	}
	if args.Type == "select" && len(args.Options) > 0 {
		request.Options = make([]types.InteractionOption, len(args.Options))
		for i, opt := range args.Options {
			request.Options[i] = types.InteractionOption{Value: opt, Label: opt}
		}
	}
	if request.Timeout <= 0 {
		request.Timeout = 300
	}

	resp, err := t.callback.OnAIInteractionRequired(ctx, t.stepID, request)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("交互处理失败: %v", err)), nil
	}

	if resp == nil || resp.Skipped {
		defaultVal := args.DefaultValue
		if defaultVal == "" {
			defaultVal = "(用户未响应)"
		}
		return types.NewToolResult(fmt.Sprintf(`{"skipped": true, "value": %q}`, defaultVal)), nil
	}

	return types.NewToolResult(fmt.Sprintf(`{"skipped": false, "value": %q}`, resp.Value)), nil
}

var _ executor.ContextualTool = (*HumanInteractionTool)(nil)
