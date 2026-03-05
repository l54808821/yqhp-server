package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/schema"
)

const switchToPlanToolName = "switch_to_plan"

// RouterAgent 自动选择最佳 Agent 模式执行任务。
// 向后兼容原 UnifiedAgentExecutor 的行为：
// - 无工具 → Direct
// - 有工具 → ReAct 循环，如果 LLM 调用 switch_to_plan 则切换到 Plan
// - 配置强制 Plan → 直接使用 Plan
type RouterAgent struct {
	direct *DirectAgent
	react  *ReActAgent
	plan   *PlanAgent
}

func NewRouterAgent() *RouterAgent {
	return &RouterAgent{
		direct: NewDirectAgent(),
		react:  NewReActAgent(),
		plan:   NewPlanAgent(),
	}
}

func (a *RouterAgent) Mode() AgentMode {
	return AgentModeRouter
}

func (a *RouterAgent) Run(ctx context.Context, req *AgentRequest) (*AIOutput, error) {
	hasTools := len(req.SchemaTools) > 0

	if !hasTools {
		return a.direct.Run(ctx, req)
	}

	enablePlan := req.Config.EnablePlanMode != nil && *req.Config.EnablePlanMode

	// 基于规则的快速任务复杂度预判：满足条件时直接进入 Plan 模式
	if enablePlan && a.shouldDirectPlan(req) {
		return a.plan.RunWithReason(ctx, req, "任务复杂度预判：检测到复杂多步任务")
	}

	schemaTools := req.SchemaTools
	if enablePlan {
		schemaTools = append(schemaTools, switchToPlanToolInfo())
	}

	return a.runReActWithPlanSwitch(ctx, req, schemaTools, enablePlan)
}

// shouldDirectPlan 基于规则的轻量级任务复杂度预判
func (a *RouterAgent) shouldDirectPlan(req *AgentRequest) bool {
	prompt := req.Config.Prompt
	if len([]rune(prompt)) < 50 {
		return false
	}

	planIndicators := []string{
		"分析并", "先.*然后.*最后", "第一步", "第二步",
		"步骤", "依次", "分别", "逐一", "逐步",
		"对比.*和.*", "比较.*和.*",
		"调研", "报告", "方案",
	}

	promptLower := strings.ToLower(prompt)
	matchCount := 0
	for _, indicator := range planIndicators {
		if strings.Contains(promptLower, indicator) {
			matchCount++
		}
	}

	conjunctions := []string{"并且", "同时", "另外", "此外", "还需要", "以及"}
	for _, conj := range conjunctions {
		matchCount += strings.Count(promptLower, conj)
	}

	return matchCount >= 3
}

// runReActWithPlanSwitch 运行 ReAct 循环，支持在运行中切换到 Plan 模式
func (a *RouterAgent) runReActWithPlanSwitch(ctx context.Context, req *AgentRequest, schemaTools []*schema.ToolInfo, enablePlan bool) (*AIOutput, error) {
	output := &AIOutput{
		Model:      req.Config.Model,
		AgentTrace: &AgentTrace{Mode: string(AgentModeReAct)},
	}

	maxRounds := req.MaxRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxToolRounds
	}

	messages := make([]*schema.Message, len(req.Messages))
	copy(messages, req.Messages)
	toolTimeout := getToolTimeout(req.Config)

	for round := 1; round <= maxRounds; round++ {
		resp, err := callLLM(ctx, req.ChatModel, messages, schemaTools, req.Config, req.StepID, req.Callbacks.AI)
		if err != nil {
			return nil, err
		}

		updateTokenUsage(output, resp)
		roundThinking := resp.Content

		if len(resp.ToolCalls) == 0 {
			output.Content = selfVerifyWithCallbacks(ctx, req.ChatModel, req.Config, req.StepID, resp.Content, output, req.Callbacks)
			if round == 1 {
				output.AgentTrace.Mode = string(AgentModeDirect)
			}
			return output, nil
		}

		// 检查是否要切换到 Plan 模式
		if enablePlan {
			if planReason := findSwitchToPlan(resp.ToolCalls); planReason != "" {
				if req.Callbacks.Thinking != nil {
					req.Callbacks.Thinking.OnAIThinking(ctx, req.StepID, round,
						fmt.Sprintf("切换到 Plan 模式：%s", planReason))
				}

				// 构建 Plan 请求，移除 switch_to_plan 工具
				planReq := &AgentRequest{
					Config:       req.Config,
					ChatModel:    req.ChatModel,
					Messages:     messages,
					ToolRegistry: req.ToolRegistry,
					SchemaTools:  filterOutSwitchToPlan(schemaTools),
					AllToolDefs:  req.AllToolDefs,
					StepID:       req.StepID,
					ExecCtx:      req.ExecCtx,
					Callbacks:    req.Callbacks,
					MaxRounds:    req.MaxRounds,
				}

				planOutput, planErr := a.plan.RunWithReason(ctx, planReq, planReason)
				if planErr != nil {
					return nil, planErr
				}
				mergeTokenUsage(output, planOutput)
				output.Content = planOutput.Content
				output.AgentTrace = planOutput.AgentTrace
				return output, nil
			}
		}

		// 正常 ReAct 循环
		notifyThinking(ctx, req, round, roundThinking, resp.ToolCalls)

		messages = append(messages, resp)
		toolResults := executeToolsConcurrently(
			ctx, resp.ToolCalls, round, req.ExecCtx, req.ToolRegistry,
			req.StepID, req.Callbacks, 0, toolTimeout,
		)

		var roundToolCalls []ToolCallRecord
		for _, r := range toolResults {
			messages = append(messages, schema.ToolMessage(r.result.GetLLMContent(), r.tc.ID))
			output.ToolCalls = append(output.ToolCalls, r.record)
			roundToolCalls = append(roundToolCalls, r.record)
		}

		output.AgentTrace.ReAct = append(output.AgentTrace.ReAct, ReActRound{
			Round:     round,
			Thinking:  roundThinking,
			ToolCalls: roundToolCalls,
		})
	}

	log.Printf("[WARN] Router 工具调用轮次达到最大值 %d，生成最终回复", maxRounds)
	resp, err := callLLM(ctx, req.ChatModel, messages, nil, req.Config, req.StepID, nil)
	if err != nil {
		return output, fmt.Errorf("最终回复生成失败: %w", err)
	}
	output.Content = selfVerifyWithCallbacks(ctx, req.ChatModel, req.Config, req.StepID, resp.Content, output, req.Callbacks)
	updateTokenUsage(output, resp)
	return output, nil
}

// --- switch_to_plan 工具辅助 ---

func switchToPlanToolInfo() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: switchToPlanToolName,
		Desc: "当你判断当前任务足够复杂，需要分步规划和执行时，调用此工具切换到 Plan 模式。Plan 模式会自动分解任务、逐步执行、最终汇总结果。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"reason": {
				Type:     schema.String,
				Desc:     "说明为什么需要切换到 Plan 模式",
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

func notifyThinking(ctx context.Context, req *AgentRequest, round int, thinking string, toolCalls []schema.ToolCall) {
	if req.Callbacks.Thinking == nil {
		return
	}
	if thinking != "" {
		req.Callbacks.Thinking.OnAIThinking(ctx, req.StepID, round, thinking)
	} else {
		toolNames := make([]string, 0, len(toolCalls))
		for _, tc := range toolCalls {
			toolNames = append(toolNames, tc.Function.Name)
		}
		req.Callbacks.Thinking.OnAIThinking(ctx, req.StepID, round,
			fmt.Sprintf("调用工具: %s", strings.Join(toolNames, ", ")))
	}
}
