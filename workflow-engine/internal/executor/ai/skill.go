package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/pkg/types"
)

const skillToolPrefix = "skill__"

func skillToToolDef(skill *SkillInfo) *types.ToolDefinition {
	toolName := skillToolPrefix + sanitizeToolName(skill.Name)
	return &types.ToolDefinition{
		Name:        toolName,
		Description: fmt.Sprintf("[Skill] %s", skill.Description),
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"task": {
					"type": "string",
					"description": "需要该专家处理的具体任务内容，请提供完整上下文"
				}
			},
			"required": ["task"]
		}`),
	}
}

func findSkillByToolName(toolName string, skills []*SkillInfo) *SkillInfo {
	for _, skill := range skills {
		if skillToolPrefix+sanitizeToolName(skill.Name) == toolName {
			return skill
		}
	}
	return nil
}

func executeSkillCall(ctx context.Context, skill *SkillInfo, arguments string, config *AIConfig) *types.ToolResult {
	var args struct {
		Task string `json:"task"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return &types.ToolResult{IsError: true, Content: fmt.Sprintf("Skill 参数解析失败: %v", err)}
	}

	if args.Task == "" {
		return &types.ToolResult{IsError: true, Content: "Skill 调用缺少 task 参数"}
	}

	messages := []*schema.Message{
		schema.SystemMessage(skill.SystemPrompt),
		schema.UserMessage(args.Task),
	}

	chatModel, err := createChatModelFromConfig(ctx, config)
	if err != nil {
		return &types.ToolResult{IsError: true, Content: fmt.Sprintf("Skill 创建模型失败: %v", err)}
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return &types.ToolResult{IsError: true, Content: fmt.Sprintf("Skill 执行失败: %v", err)}
	}

	return &types.ToolResult{IsError: false, Content: resp.Content}
}

// sanitizeToolName converts names with special/Chinese characters into valid tool names.
// NOTE: This is the canonical implementation; the simpler version in unified_tools.go is removed.
func sanitizeToolName(name string) string {
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		} else if r >= 0x4e00 && r <= 0x9fff {
			result.WriteRune(r)
		} else {
			result.WriteRune('_')
		}
	}
	s := result.String()
	if s == "" {
		s = "unnamed"
	}
	return s
}
