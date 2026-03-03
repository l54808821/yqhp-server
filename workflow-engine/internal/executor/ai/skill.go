package ai

import (
	"encoding/json"
	"fmt"

	"yqhp/workflow-engine/pkg/types"
)

const readSkillToolName = "read_skill"

func readSkillToolDef(skills []*SkillInfo) *types.ToolDefinition {
	names := make([]string, 0, len(skills))
	for _, s := range skills {
		names = append(names, s.Name)
	}
	namesJSON, _ := json.Marshal(names)
	return &types.ToolDefinition{
		Name:        readSkillToolName,
		Description: "加载指定 Skill 的完整操作指令。当任务需要某个专业领域知识时，先调用此工具获取指令，然后按指令使用现有工具执行。",
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

func executeReadSkill(arguments string, skills []*SkillInfo) *types.ToolResult {
	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return &types.ToolResult{IsError: true, Content: fmt.Sprintf("参数解析失败: %v", err)}
	}

	if args.Name == "" {
		return &types.ToolResult{IsError: true, Content: "缺少 name 参数"}
	}

	for _, s := range skills {
		if s.Name == args.Name {
			return &types.ToolResult{Content: s.Body}
		}
	}
	return &types.ToolResult{IsError: true, Content: fmt.Sprintf("Skill 未找到: %s", args.Name)}
}
