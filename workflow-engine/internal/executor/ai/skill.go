package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/pkg/types"
)

// skillToolPrefix Skill 工具名称前缀
const skillToolPrefix = "skill__"

// skillToToolDef 将 Skill 转换为工具定义，使 AI 可以像调用工具一样调用 Skill
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

// sanitizeToolName 将中文/特殊字符的 Skill 名称转换为合法的工具名称
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

// findSkillByToolName 根据工具名称查找 Skill
func findSkillByToolName(toolName string, skills []*SkillInfo) *SkillInfo {
	for _, skill := range skills {
		if skillToolPrefix+sanitizeToolName(skill.Name) == toolName {
			return skill
		}
	}
	return nil
}

// executeSkillCall 执行 Skill 调用 -- 使用 Skill 的系统提示词发起子 LLM 调用
func (e *AIExecutor) executeSkillCall(ctx context.Context, skill *SkillInfo, arguments string, config *AIConfig) *types.ToolResult {
	var args struct {
		Task string `json:"task"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("Skill 参数解析失败: %v", err),
		}
	}

	if args.Task == "" {
		return &types.ToolResult{
			IsError: true,
			Content: "Skill 调用缺少 task 参数",
		}
	}

	// 使用 Skill 的系统提示词 + 用户传入的 task 构建消息
	messages := []*schema.Message{
		schema.SystemMessage(skill.SystemPrompt),
		schema.UserMessage(args.Task),
	}

	// 复用同一个模型创建子调用
	chatModel, err := e.createChatModel(ctx, config)
	if err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("Skill 创建模型失败: %v", err),
		}
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("Skill 执行失败: %v", err),
		}
	}

	return &types.ToolResult{
		IsError: false,
		Content: resp.Content,
	}
}
