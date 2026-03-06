package ai

import (
	"context"
	"time"

	"yqhp/workflow-engine/pkg/logger"
)

// DirectAgent 直接调用 LLM 获取回答，不使用工具。
// 适用于简单问答、文本生成等不需要工具的场景。
type DirectAgent struct{}

func NewDirectAgent() *DirectAgent {
	return &DirectAgent{}
}

func (a *DirectAgent) Mode() AgentMode {
	return AgentModeDirect
}

func (a *DirectAgent) Run(ctx context.Context, req *AgentRequest) (*AIOutput, error) {
	logger.Debug("[Direct] 开始执行, model=%s, stepID=%s, messages数量=%d",
		req.Config.Model, req.StepID, len(req.Messages))
	startTime := time.Now()

	output := &AIOutput{
		Model:      req.Config.Model,
		AgentTrace: &AgentTrace{Mode: string(AgentModeDirect)},
	}

	resp, err := callLLM(ctx, req.ChatModel, req.Messages, nil, req.Config, req.StepID, req.Callbacks)
	if err != nil {
		logger.Debug("[Direct] 执行失败, stepID=%s, 耗时=%v, error=%v", req.StepID, time.Since(startTime), err)
		return nil, err
	}

	output.Content = resp.Content
	updateTokenUsage(output, resp)

	logger.Debug("[Direct] 执行完成, stepID=%s, 耗时=%v, content长度=%d, promptTokens=%d, completionTokens=%d, totalTokens=%d",
		req.StepID, time.Since(startTime), len([]rune(output.Content)),
		output.PromptTokens, output.CompletionTokens, output.TotalTokens)

	if req.Callbacks.Stream != nil {
		req.Callbacks.Stream.OnMessageComplete(ctx, req.StepID, toAIResult(output))
	}

	return output, nil
}
