package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/pkg/logger"
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
	enablePlan := req.Config.EnablePlanMode != nil && *req.Config.EnablePlanMode

	logger.Debug("[Router] 开始执行, model=%s, stepID=%s, hasTools=%v, enablePlan=%v, tools数量=%d",
		req.Config.Model, req.StepID, hasTools, enablePlan, len(req.SchemaTools))

	if !hasTools {
		logger.Debug("[Router] 无工具, 委托给 Direct 模式, stepID=%s", req.StepID)
		return a.direct.Run(ctx, req)
	}

	if enablePlan && a.shouldDirectPlan(req) {
		logger.Debug("[Router] 任务复杂度预判命中, 直接进入 Plan 模式, stepID=%s", req.StepID)
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
	startTime := time.Now()

	for round := 1; round <= maxRounds; round++ {
		logger.Debug("[Router] ===== 第 %d/%d 轮开始 (stepID=%s, 当前messages数=%d) =====",
			round, maxRounds, req.StepID, len(messages))
		resp, err := callLLM(ctx, req.ChatModel, messages, schemaTools, req.Config, req.StepID, req.Callbacks)
		if err != nil {
			logger.Debug("[Router] 第 %d 轮 LLM 调用失败, 总耗时=%v: %v", round, time.Since(startTime), err)
			return nil, err
		}

		updateTokenUsage(output, resp)
		roundThinking := resp.Content

		if len(resp.ToolCalls) == 0 {
			logger.Debug("[Router] 第 %d 轮 LLM 未返回工具调用, 直接输出文本 (长度=%d), 总耗时=%v",
				round, len([]rune(resp.Content)), time.Since(startTime))
			output.Content = resp.Content
			if round == 1 {
				output.AgentTrace.Mode = string(AgentModeDirect)
			}
			if req.Callbacks.Stream != nil {
				req.Callbacks.Stream.OnMessageComplete(ctx, req.StepID, toAIResult(output))
			}
			return output, nil
		}

		toolNames := make([]string, 0, len(resp.ToolCalls))
		for _, tc := range resp.ToolCalls {
			toolNames = append(toolNames, tc.Function.Name)
		}
		logger.Debug("[Router] 第 %d 轮 LLM 返回 %d 个工具调用: %s, thinking长度=%d",
			round, len(resp.ToolCalls), strings.Join(toolNames, ", "), len([]rune(roundThinking)))

		if enablePlan {
			if planReason := findSwitchToPlan(resp.ToolCalls); planReason != "" {
				logger.Debug("[Router] LLM 请求切换到 Plan 模式, reason=%s, stepID=%s", planReason, req.StepID)
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

		// 只推送 LLM 返回的真实思考文本
		if roundThinking != "" && req.Callbacks.Stream != nil {
			thinkBlockID := req.Callbacks.BlockID.Next()
			req.Callbacks.Stream.OnAIThinking(ctx, req.StepID, thinkBlockID, roundThinking)
		}

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

	logger.Warn("[Router] 工具调用轮次达到最大值 %d, 生成最终回复 (stepID=%s)", maxRounds, req.StepID)
	resp, err := callLLM(ctx, req.ChatModel, messages, nil, req.Config, req.StepID, nil)
	if err != nil {
		logger.Debug("[Router] 最终回复生成失败, stepID=%s, 总耗时=%v: %v", req.StepID, time.Since(startTime), err)
		return output, fmt.Errorf("最终回复生成失败: %w", err)
	}
	output.Content = resp.Content
	updateTokenUsage(output, resp)
	logger.Debug("[Router] 执行完成, stepID=%s, 总轮次=%d, 总耗时=%v, content长度=%d, 累计tokens=%d",
		req.StepID, maxRounds, time.Since(startTime), len([]rune(output.Content)), output.TotalTokens)
	if req.Callbacks.Stream != nil {
		req.Callbacks.Stream.OnMessageComplete(ctx, req.StepID, toAIResult(output))
	}
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
