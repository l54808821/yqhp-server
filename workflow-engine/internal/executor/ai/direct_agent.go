package ai

import (
	"context"
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
	output := &AIOutput{
		Model:      req.Config.Model,
		AgentTrace: &AgentTrace{Mode: string(AgentModeDirect)},
	}

	resp, err := callLLM(ctx, req.ChatModel, req.Messages, nil, req.Config, req.StepID, req.Callbacks)
	if err != nil {
		return nil, err
	}

	output.Content = resp.Content
	updateTokenUsage(output, resp)

	if req.Callbacks.Stream != nil {
		req.Callbacks.Stream.OnMessageComplete(ctx, req.StepID, toAIResult(output))
	}

	return output, nil
}
