package hook

import (
	"context"
	"fmt"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// Runner 管理工作流和步骤的钩子执行。
type Runner struct {
	hookExecutor *HookExecutor
}

// NewRunner 创建一个新的钩子 Runner。
func NewRunner(registry *executor.Registry) *Runner {
	return &Runner{
		hookExecutor: NewHookExecutor(registry),
	}
}

// ExecuteWorkflowPreHook 执行工作流级前置钩子。
// 如果前置钩子失败则返回错误，表示应跳过工作流。
// Requirements: 4.1, 4.5
func (r *Runner) ExecuteWorkflowPreHook(
	ctx context.Context,
	workflow *types.Workflow,
	execCtx *executor.ExecutionContext,
) (*HookResult, error) {
	if workflow.PreHook == nil {
		return nil, nil
	}

	result, err := r.hookExecutor.ExecuteHook(
		ctx,
		workflow.PreHook,
		HookTypePre,
		HookLevelWorkflow,
		"",
		execCtx,
	)

	if err != nil {
		return result, fmt.Errorf("工作流前置钩子失败: %w", err)
	}

	return result, nil
}

// ExecuteWorkflowPostHook 执行工作流级后置钩子。
// 无论工作流成功或失败，此钩子都会执行。
// Requirements: 4.2, 4.6
func (r *Runner) ExecuteWorkflowPostHook(
	ctx context.Context,
	workflow *types.Workflow,
	execCtx *executor.ExecutionContext,
	workflowErr error,
) (*HookResult, error) {
	if workflow.PostHook == nil {
		return nil, nil
	}

	// 如果存在工作流错误，将其添加到上下文
	if workflowErr != nil {
		execCtx.SetVariable("__workflow_error", workflowErr.Error())
	}

	result, err := r.hookExecutor.ExecuteHook(
		ctx,
		workflow.PostHook,
		HookTypePost,
		HookLevelWorkflow,
		"",
		execCtx,
	)

	// 后置钩子错误会被记录，但不影响工作流结果
	// 无论工作流成功/失败，后置钩子都会执行
	return result, err
}

// ExecuteStepPreHook 执行步骤级前置钩子。
// 如果前置钩子失败则返回错误，表示应跳过该步骤。
// Requirements: 4.3, 4.5
func (r *Runner) ExecuteStepPreHook(
	ctx context.Context,
	step *types.Step,
	execCtx *executor.ExecutionContext,
) (*HookResult, error) {
	if step.PreHook == nil {
		return nil, nil
	}

	result, err := r.hookExecutor.ExecuteHook(
		ctx,
		step.PreHook,
		HookTypePre,
		HookLevelStep,
		step.ID,
		execCtx,
	)

	if err != nil {
		return result, fmt.Errorf("步骤 %s 的前置钩子失败: %w", step.ID, err)
	}

	return result, nil
}

// ExecuteStepPostHook 执行步骤级后置钩子。
// 无论步骤成功或失败，此钩子都会执行。
// Requirements: 4.4, 4.6
func (r *Runner) ExecuteStepPostHook(
	ctx context.Context,
	step *types.Step,
	execCtx *executor.ExecutionContext,
	stepResult *types.StepResult,
) (*HookResult, error) {
	if step.PostHook == nil {
		return nil, nil
	}

	// 将步骤结果添加到上下文
	if stepResult != nil {
		execCtx.SetVariable("__step_result", map[string]any{
			"status":   string(stepResult.Status),
			"duration": stepResult.Duration.Milliseconds(),
			"output":   stepResult.Output,
		})
		if stepResult.Error != nil {
			execCtx.SetVariable("__step_error", stepResult.Error.Error())
		}
	}

	result, err := r.hookExecutor.ExecuteHook(
		ctx,
		step.PostHook,
		HookTypePost,
		HookLevelStep,
		step.ID,
		execCtx,
	)

	// 后置钩子错误会被记录，但不影响步骤结果
	// 无论步骤成功/失败，后置钩子都会执行
	return result, err
}

// StepExecutionResult 包含带钩子执行步骤的完整结果。
type StepExecutionResult struct {
	PreHookResult  *HookResult
	StepResult     *types.StepResult
	PostHookResult *HookResult
	Skipped        bool
	Error          error
}

// ExecuteStepWithHooks 执行带有前置和后置钩子的步骤。
// Requirements: 4.3, 4.4, 4.5, 4.6
func (r *Runner) ExecuteStepWithHooks(
	ctx context.Context,
	step *types.Step,
	execCtx *executor.ExecutionContext,
	stepExecutor func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error),
) *StepExecutionResult {
	result := &StepExecutionResult{}

	// 执行前置钩子
	preHookResult, preHookErr := r.ExecuteStepPreHook(ctx, step, execCtx)
	result.PreHookResult = preHookResult

	// 如果前置钩子失败，跳过步骤但仍执行后置钩子
	if preHookErr != nil {
		result.Skipped = true
		result.Error = preHookErr
		result.StepResult = executor.CreateSkippedResult(step.ID)

		// 即使前置钩子失败也执行后置钩子（需求 4.6）
		postHookResult, _ := r.ExecuteStepPostHook(ctx, step, execCtx, result.StepResult)
		result.PostHookResult = postHookResult

		return result
	}

	// 执行步骤
	stepResult, stepErr := stepExecutor(ctx, step, execCtx)
	result.StepResult = stepResult
	if stepErr != nil {
		result.Error = stepErr
	}

	// 无论步骤成功/失败都执行后置钩子（需求 4.6）
	postHookResult, _ := r.ExecuteStepPostHook(ctx, step, execCtx, stepResult)
	result.PostHookResult = postHookResult

	return result
}

// WorkflowExecutionResult 包含带钩子执行工作流的完整结果。
type WorkflowExecutionResult struct {
	PreHookResult  *HookResult
	StepResults    []*StepExecutionResult
	PostHookResult *HookResult
	Skipped        bool
	Error          error
}

// ExecuteWorkflowWithHooks 执行带有前置和后置钩子的工作流。
// Requirements: 4.1, 4.2, 4.5, 4.6
func (r *Runner) ExecuteWorkflowWithHooks(
	ctx context.Context,
	workflow *types.Workflow,
	execCtx *executor.ExecutionContext,
	workflowExecutor func(ctx context.Context, workflow *types.Workflow, execCtx *executor.ExecutionContext) ([]*StepExecutionResult, error),
) *WorkflowExecutionResult {
	result := &WorkflowExecutionResult{}

	// 执行工作流前置钩子
	preHookResult, preHookErr := r.ExecuteWorkflowPreHook(ctx, workflow, execCtx)
	result.PreHookResult = preHookResult

	// 如果前置钩子失败，跳过工作流但仍执行后置钩子
	if preHookErr != nil {
		result.Skipped = true
		result.Error = preHookErr

		// 即使前置钩子失败也执行后置钩子（需求 4.6）
		postHookResult, _ := r.ExecuteWorkflowPostHook(ctx, workflow, execCtx, preHookErr)
		result.PostHookResult = postHookResult

		return result
	}

	// 执行工作流步骤
	stepResults, workflowErr := workflowExecutor(ctx, workflow, execCtx)
	result.StepResults = stepResults
	if workflowErr != nil {
		result.Error = workflowErr
	}

	// 无论工作流成功/失败都执行后置钩子（需求 4.6）
	postHookResult, _ := r.ExecuteWorkflowPostHook(ctx, workflow, execCtx, workflowErr)
	result.PostHookResult = postHookResult

	return result
}
