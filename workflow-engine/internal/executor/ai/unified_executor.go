package ai

import (
	"context"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

const UnifiedAgentType = "ai_agent"

// UnifiedAgentExecutor 向后兼容的 Agent Executor。
// 内部委托给 RouterAgent，自动选择 Direct/ReAct/Plan 模式。
// 新代码建议直接使用 ai_react / ai_plan / ai_direct 类型。
type UnifiedAgentExecutor struct {
	*executor.BaseExecutor
}

func NewUnifiedAgentExecutor() *UnifiedAgentExecutor {
	return &UnifiedAgentExecutor{
		BaseExecutor: executor.NewBaseExecutor(UnifiedAgentType),
	}
}

func (e *UnifiedAgentExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

func (e *UnifiedAgentExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
	req, cancel, err := buildAgentRequest(ctx, step, execCtx, AgentModeRouter)
	if err != nil {
		return executor.CreateFailedResult(step.ID, time.Now(), err), nil
	}
	defer cancel()

	agent := NewRouterAgent()
	output, err := agent.Run(req.ctx, req.agentReq)
	if err != nil {
		return handleAgentError(step, req, err)
	}

	return buildAgentResult(step, req, output)
}

func (e *UnifiedAgentExecutor) Cleanup(ctx context.Context) error { return nil }
