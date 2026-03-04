package ai

import (
	"context"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

const DirectAgentType = "ai_direct"

// DirectAgentExecutor 纯 Direct 模式的 Step Executor
// 用户在工作流中通过 type: "ai_direct" 使用
type DirectAgentExecutor struct {
	*executor.BaseExecutor
}

func NewDirectAgentExecutor() *DirectAgentExecutor {
	return &DirectAgentExecutor{
		BaseExecutor: executor.NewBaseExecutor(DirectAgentType),
	}
}

func (e *DirectAgentExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

func (e *DirectAgentExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
	req, cancel, err := buildAgentRequest(ctx, step, execCtx, AgentModeDirect)
	if err != nil {
		return executor.CreateFailedResult(step.ID, time.Now(), err), nil
	}
	defer cancel()

	agent := NewDirectAgent()
	output, err := agent.Run(req.ctx, req.agentReq)
	if err != nil {
		return handleAgentError(step, req, err)
	}

	return buildAgentResult(step, req, output)
}

func (e *DirectAgentExecutor) Cleanup(ctx context.Context) error { return nil }
