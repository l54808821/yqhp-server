package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/pkg/logger"
)

// ReActAgent 实现 Think → Act → Observe → Reflect 循环。
// 每轮：LLM 输出思考 + 工具调用 → 执行工具 → 将结果反馈给 LLM → 在失败/卡住时触发反思 → 重复直到 LLM 不再调用工具。
type ReActAgent struct{}

const (
	reflectionTriggerRound    = 3  // 连续执行到第 N 轮时触发反思
	reflectionConsecFailLimit = 2  // 连续失败 N 次触发反思
)

func NewReActAgent() *ReActAgent {
	return &ReActAgent{}
}

func (a *ReActAgent) Mode() AgentMode {
	return AgentModeReAct
}

func (a *ReActAgent) Run(ctx context.Context, req *AgentRequest) (*AIOutput, error) {
	output := &AIOutput{
		Model:      req.Config.Model,
		AgentTrace: &AgentTrace{Mode: string(AgentModeReAct)},
	}

	if len(req.SchemaTools) == 0 {
		return NewDirectAgent().Run(ctx, req)
	}

	maxRounds := req.MaxRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxToolRounds
	}

	messages := make([]*schema.Message, len(req.Messages))
	copy(messages, req.Messages)
	toolTimeout := getToolTimeout(req.Config)
	consecutiveFailures := 0

	for round := 1; round <= maxRounds; round++ {
		logger.Debug("[ReAct] ===== 第 %d 轮开始 (stepID=%s, model=%s) =====", round, req.StepID, req.Config.Model)
		resp, err := callLLM(ctx, req.ChatModel, messages, req.SchemaTools, req.Config, req.StepID, req.Callbacks.AI)
		if err != nil {
			logger.Debug("[ReAct] 第 %d 轮 LLM 调用失败: %v", round, err)
			return nil, err
		}

		updateTokenUsage(output, resp)
		roundThinking := resp.Content

		if len(resp.ToolCalls) == 0 {
			logger.Debug("[ReAct] 第 %d 轮 LLM 未返回工具调用，直接输出文本 (长度=%d)", round, len(resp.Content))
			output.Content = selfVerify(ctx, req.ChatModel, req.Config, req.StepID, resp.Content, output)
			if round == 1 {
				output.AgentTrace.Mode = string(AgentModeDirect)
			}
			return output, nil
		}

		toolNames := make([]string, 0, len(resp.ToolCalls))
		for _, tc := range resp.ToolCalls {
			toolNames = append(toolNames, tc.Function.Name)
		}
		logger.Debug("[ReAct] 第 %d 轮 LLM 返回 %d 个工具调用: %s", round, len(resp.ToolCalls), strings.Join(toolNames, ", "))

		a.notifyThinking(ctx, req, round, roundThinking, resp.ToolCalls)

		messages = append(messages, resp)
		toolResults := executeToolsConcurrently(
			ctx, resp.ToolCalls, round, req.ExecCtx, req.ToolRegistry,
			req.StepID, req.Callbacks, 0, toolTimeout,
		)

		var roundToolCalls []ToolCallRecord
		roundHasFailure := false
		for _, r := range toolResults {
			logger.Debug("[ReAct] 第 %d 轮工具 [%s] 执行完成, isError=%v, 耗时=%dms, 结果长度=%d",
				round, r.record.ToolName, r.record.IsError, r.record.Duration, len(r.record.Result))
			messages = append(messages, schema.ToolMessage(r.result.GetLLMContent(), r.tc.ID))
			output.ToolCalls = append(output.ToolCalls, r.record)
			roundToolCalls = append(roundToolCalls, r.record)
			if r.record.IsError {
				roundHasFailure = true
			}
		}

		if roundHasFailure {
			consecutiveFailures++
		} else {
			consecutiveFailures = 0
		}

		// 反思机制：在连续失败或长时间未完成时，插入反思提示引导 LLM 调整策略
		var reflectionContent string
		if needsReflection(round, consecutiveFailures, roundToolCalls) {
			reflectionPrompt := buildReflectionPrompt(round, consecutiveFailures, output.AgentTrace.ReAct, roundToolCalls)
			messages = append(messages, schema.UserMessage(reflectionPrompt))
			reflectionContent = reflectionPrompt
			logger.Debug("[ReAct] 第 %d 轮触发反思机制 (连续失败=%d)", round, consecutiveFailures)
			if req.Callbacks.Thinking != nil {
				req.Callbacks.Thinking.OnAIThinking(ctx, req.StepID, round, "正在反思执行策略，调整方案...")
			}
		}

		output.AgentTrace.ReAct = append(output.AgentTrace.ReAct, ReActRound{
			Round:      round,
			Thinking:   roundThinking,
			ToolCalls:  roundToolCalls,
			Reflection: reflectionContent,
		})
	}

	logger.Warn("[ReAct] 工具调用轮次达到最大值 %d，生成最终回复", maxRounds)
	resp, err := callLLM(ctx, req.ChatModel, messages, nil, req.Config, req.StepID, nil)
	if err != nil {
		return output, fmt.Errorf("最终回复生成失败: %w", err)
	}
	output.Content = selfVerify(ctx, req.ChatModel, req.Config, req.StepID, resp.Content, output)
	updateTokenUsage(output, resp)
	return output, nil
}

func (a *ReActAgent) notifyThinking(ctx context.Context, req *AgentRequest, round int, thinking string, toolCalls []schema.ToolCall) {
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
		req.Callbacks.Thinking.OnAIThinking(ctx, req.StepID, round, fmt.Sprintf("调用工具: %s", strings.Join(toolNames, ", ")))
	}
}

// --- 反思机制 ---

// needsReflection 判断是否需要触发反思
func needsReflection(round int, consecutiveFailures int, roundToolCalls []ToolCallRecord) bool {
	if consecutiveFailures >= reflectionConsecFailLimit {
		return true
	}
	if round > 0 && round%reflectionTriggerRound == 0 {
		return true
	}
	return false
}

// buildReflectionPrompt 构建反思提示词，引导 LLM 审视已有执行并调整策略
func buildReflectionPrompt(round int, consecutiveFailures int, previousRounds []ReActRound, latestToolCalls []ToolCallRecord) string {
	var sb strings.Builder
	sb.WriteString("[反思与策略调整]\n")
	sb.WriteString(fmt.Sprintf("你已经执行了 %d 轮工具调用。请暂停并反思：\n\n", round))

	if consecutiveFailures > 0 {
		sb.WriteString(fmt.Sprintf("注意：最近 %d 轮出现了工具执行失败。\n\n", consecutiveFailures))
	}

	sb.WriteString("最近的工具调用结果：\n")
	for _, tc := range latestToolCalls {
		status := "成功"
		if tc.IsError {
			status = "失败"
		}
		sb.WriteString(fmt.Sprintf("- %s [%s]: %s\n", tc.ToolName, status, truncateRunes(tc.Result, 200)))
	}

	sb.WriteString(`
请按以下框架反思：
1. 进展评估：到目前为止，任务完成了多少？哪些信息已经获取到了？
2. 问题诊断：哪些工具调用是无效的或失败的？失败的原因是什么？
3. 策略调整：接下来的最优行动是什么？是否需要更换工具或调整参数？
4. 终止判断：是否已经有足够的信息可以直接给出最终回答？

如果你已经有足够信息回答用户的问题，请直接给出最终回答，不要再调用工具。`)
	return sb.String()
}
