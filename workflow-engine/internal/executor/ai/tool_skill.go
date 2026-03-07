package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// SkillTool read_skill 工具，实现统一 Tool 接口
type SkillTool struct {
	skills []*SkillInfo
}

func NewSkillTool(skills []*SkillInfo) *SkillTool {
	return &SkillTool{skills: skills}
}

func (t *SkillTool) Definition() *types.ToolDefinition {
	names := make([]string, 0, len(t.skills))
	for _, s := range t.skills {
		names = append(names, s.Name)
	}
	namesJSON, _ := json.Marshal(names)

	var sb strings.Builder
	sb.WriteString("加载专业能力（Skill）的完整操作指令。需要时先调用此工具获取指令，然后按指令使用现有工具执行。\n\n可用 Skills：\n")
	for _, s := range t.skills {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
	}

	return &types.ToolDefinition{
		Name:        "read_skill",
		Description: sb.String(),
		Parameters: json.RawMessage(fmt.Sprintf(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"enum": %s,
					"description": "要加载的 Skill 名称"
				}
			},
			"required": ["name"]
		}`, string(namesJSON))),
	}
}

func (t *SkillTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Name == "" {
		return types.NewErrorResult("缺少 name 参数"), nil
	}

	for _, s := range t.skills {
		if s.Name == args.Name {
			return types.NewSilentResult(s.Body), nil
		}
	}
	return types.NewErrorResult(fmt.Sprintf("Skill 未找到: %s", args.Name)), nil
}
