package ai

import (
	"encoding/json"

	"github.com/cloudwego/eino/schema"
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

