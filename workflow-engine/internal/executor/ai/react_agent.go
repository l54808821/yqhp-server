package ai

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// ReActAgent 实现 Think → Act → Observe 循环。
// 每轮：LLM 输出思考 + 工具调用 → 执行工具 → 将结果反馈给 LLM → 重复直到 LLM 不再调用工具。
type ReActAgent struct{}

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

	for round := 1; round <= maxRounds; round++ {
		resp, err := callLLM(ctx, req.ChatModel, messages, req.SchemaTools, req.Config, req.StepID, req.Callbacks.AI)
		if err != nil {
			return nil, err
		}

		updateTokenUsage(output, resp)
		roundThinking := resp.Content

		if len(resp.ToolCalls) == 0 {
			output.Content = resp.Content
			if round == 1 {
				output.AgentTrace.Mode = string(AgentModeDirect)
			}
			return output, nil
		}

		a.notifyThinking(ctx, req, round, roundThinking, resp.ToolCalls)

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

	log.Printf("[WARN] ReAct 工具调用轮次达到最大值 %d，生成最终回复", maxRounds)
	resp, err := callLLM(ctx, req.ChatModel, messages, nil, req.Config, req.StepID, nil)
	if err != nil {
		return output, fmt.Errorf("最终回复生成失败: %w", err)
	}
	output.Content = resp.Content
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
