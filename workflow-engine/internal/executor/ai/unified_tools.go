package ai

import (
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/pkg/types"
)

const switchToPlanToolName = "switch_to_plan"

func switchToPlanToolInfo() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: switchToPlanToolName,
		Desc: "当你判断当前任务足够复杂，需要分步规划和执行时，调用此工具切换到 Plan 模式。Plan 模式会自动分解任务、逐步执行、最终汇总结果。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"reason": {
				Type:     schema.String,
				Desc:     "说明为什么需要切换到 Plan 模式，例如：任务涉及多个独立子任务、需要按顺序执行等",
				Required: true,
			},
		}),
	}
}

func findSwitchToPlan(toolCalls []schema.ToolCall) string {
	for _, tc := range toolCalls {
		if tc.Function.Name == switchToPlanToolName {
			var args struct {
				Reason string `json:"reason"`
			}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
				return args.Reason
			}
			return "Agent 判断需要 Plan 模式"
		}
	}
	return ""
}

func filterOutSwitchToPlan(tools []*schema.ToolInfo) []*schema.ToolInfo {
	var filtered []*schema.ToolInfo
	for _, t := range tools {
		if t.Name != switchToPlanToolName {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// filterToolsForStep 根据步骤描述过滤工具：保留非 Skill 工具 + 该步骤提及的 Skill 工具。
// 如果步骤描述中没有提及任何 Skill，则保留所有工具（降级为不过滤）。
func filterToolsForStep(tools []*schema.ToolInfo, allToolDefs []*types.ToolDefinition, stepTask string, skills []*SkillInfo) ([]*schema.ToolInfo, []*types.ToolDefinition) {
	if len(skills) == 0 {
		return tools, allToolDefs
	}

	// 找出步骤描述中提及的 Skill 工具名
	var mentionedSkills []string
	for _, skill := range skills {
		toolName := skillToolPrefix + sanitizeToolName(skill.Name)
		if strings.Contains(stepTask, skill.Name) || strings.Contains(stepTask, toolName) {
			mentionedSkills = append(mentionedSkills, toolName)
		}
	}

	// 如果步骤描述没有明确提及任何 Skill，不做过滤
	if len(mentionedSkills) == 0 {
		return tools, allToolDefs
	}

	mentionedSet := make(map[string]bool, len(mentionedSkills))
	for _, name := range mentionedSkills {
		mentionedSet[name] = true
	}

	var filteredTools []*schema.ToolInfo
	for _, t := range tools {
		if !strings.HasPrefix(t.Name, skillToolPrefix) || mentionedSet[t.Name] {
			filteredTools = append(filteredTools, t)
		}
	}

	var filteredDefs []*types.ToolDefinition
	for _, d := range allToolDefs {
		if !strings.HasPrefix(d.Name, skillToolPrefix) || mentionedSet[d.Name] {
			filteredDefs = append(filteredDefs, d)
		}
	}

	return filteredTools, filteredDefs
}

