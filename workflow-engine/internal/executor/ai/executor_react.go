package ai

import (
	"context"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

const ReActAgentType = "ai_react"

// ReActAgentExecutor 纯 ReAct 模式的 Step Executor
// 用户在工作流中通过 type: "ai_react" 使用
type ReActAgentExecutor struct {
	*executor.BaseExecutor
}

func NewReActAgentExecutor() *ReActAgentExecutor {
	return &ReActAgentExecutor{
		BaseExecutor: executor.NewBaseExecutor(ReActAgentType),
	}
}

func (e *ReActAgentExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

func (e *ReActAgentExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
	req, cancel, err := buildAgentRequest(ctx, step, execCtx, AgentModeReAct)
	if err != nil {
		return executor.CreateFailedResult(step.ID, time.Now(), err), nil
	}
	defer cancel()

	agent := NewReActAgent()
	output, err := agent.Run(req.ctx, req.agentReq)
	if err != nil {
		return handleAgentError(step, req, err)
	}

	return buildAgentResult(step, req, output)
}

func (e *ReActAgentExecutor) Cleanup(ctx context.Context) error { return nil }
