package ai

import (
	"context"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

const PlanAgentType = "ai_plan"

// PlanAgentExecutor 纯 Plan 模式的 Step Executor
// 用户在工作流中通过 type: "ai_plan" 使用
type PlanAgentExecutor struct {
	*executor.BaseExecutor
}

func NewPlanAgentExecutor() *PlanAgentExecutor {
	return &PlanAgentExecutor{
		BaseExecutor: executor.NewBaseExecutor(PlanAgentType),
	}
}

func (e *PlanAgentExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

func (e *PlanAgentExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
	req, cancel, err := buildAgentRequest(ctx, step, execCtx, AgentModePlan)
	if err != nil {
		return executor.CreateFailedResult(step.ID, time.Now(), err), nil
	}
	defer cancel()

	agent := NewPlanAgent()
	output, err := agent.Run(req.ctx, req.agentReq)
	if err != nil {
		return handleAgentError(step, req, err)
	}

	return buildAgentResult(step, req, output)
}

func (e *PlanAgentExecutor) Cleanup(ctx context.Context) error { return nil }
