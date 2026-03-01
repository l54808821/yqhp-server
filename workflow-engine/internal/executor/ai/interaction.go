package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"yqhp/workflow-engine/pkg/types"
)

const humanInteractionToolName = "human_interaction"

func humanInteractionToolDef() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        humanInteractionToolName,
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
					"description": "展示给用户的提示信息，应清晰说明需要用户做什么"
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

type humanInteractionArgs struct {
	Type         string   `json:"type"`
	Prompt       string   `json:"prompt"`
	Options      []string `json:"options,omitempty"`
	DefaultValue string   `json:"default_value,omitempty"`
}

func executeHumanInteraction(ctx context.Context, arguments string, stepID string, config *AIConfig, callback types.AICallback) *types.ToolResult {
	var args humanInteractionArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return &types.ToolResult{IsError: true, Content: fmt.Sprintf("参数解析失败: %v", err)}
	}

	if args.Type == "" {
		args.Type = "confirm"
	}
	if args.Prompt == "" {
		return &types.ToolResult{IsError: true, Content: "缺少必填参数: prompt"}
	}

	request := &types.InteractionRequest{
		Type:         types.InteractionType(args.Type),
		Prompt:       args.Prompt,
		DefaultValue: args.DefaultValue,
		Timeout:      config.InteractionTimeout,
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

	resp, err := callback.OnAIInteractionRequired(ctx, stepID, request)
	if err != nil {
		return &types.ToolResult{IsError: true, Content: fmt.Sprintf("交互处理失败: %v", err)}
	}

	if resp == nil || resp.Skipped {
		defaultVal := args.DefaultValue
		if defaultVal == "" {
			defaultVal = "(用户未响应)"
		}
		return &types.ToolResult{IsError: false, Content: fmt.Sprintf(`{"skipped": true, "value": %q}`, defaultVal)}
	}

	return &types.ToolResult{IsError: false, Content: fmt.Sprintf(`{"skipped": false, "value": %q}`, resp.Value)}
}
