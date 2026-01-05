package hook

import (
	"context"
	"fmt"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// Runner manages hook execution for workflows and steps.
type Runner struct {
	hookExecutor *HookExecutor
}

// NewRunner creates a new hook Runner.
func NewRunner(registry *executor.Registry) *Runner {
	return &Runner{
		hookExecutor: NewHookExecutor(registry),
	}
}

// ExecuteWorkflowPreHook executes the workflow-level pre-hook.
// Returns an error if the pre-hook fails, indicating the workflow should be skipped.
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
		return result, fmt.Errorf("workflow pre-hook failed: %w", err)
	}

	return result, nil
}

// ExecuteWorkflowPostHook executes the workflow-level post-hook.
// This hook is always executed regardless of workflow success or failure.
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

	// Add workflow error to context if present
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

	// Post-hook errors are recorded but don't affect the workflow result
	// The post-hook is always executed regardless of workflow success/failure
	return result, err
}

// ExecuteStepPreHook executes the step-level pre-hook.
// Returns an error if the pre-hook fails, indicating the step should be skipped.
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
		return result, fmt.Errorf("step pre-hook failed for step %s: %w", step.ID, err)
	}

	return result, nil
}

// ExecuteStepPostHook executes the step-level post-hook.
// This hook is always executed regardless of step success or failure.
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

	// Add step result to context
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

	// Post-hook errors are recorded but don't affect the step result
	// The post-hook is always executed regardless of step success/failure
	return result, err
}

// StepExecutionResult contains the complete result of executing a step with hooks.
type StepExecutionResult struct {
	PreHookResult  *HookResult
	StepResult     *types.StepResult
	PostHookResult *HookResult
	Skipped        bool
	Error          error
}

// ExecuteStepWithHooks executes a step with its pre and post hooks.
// Requirements: 4.3, 4.4, 4.5, 4.6
func (r *Runner) ExecuteStepWithHooks(
	ctx context.Context,
	step *types.Step,
	execCtx *executor.ExecutionContext,
	stepExecutor func(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error),
) *StepExecutionResult {
	result := &StepExecutionResult{}

	// Execute pre-hook
	preHookResult, preHookErr := r.ExecuteStepPreHook(ctx, step, execCtx)
	result.PreHookResult = preHookResult

	// If pre-hook fails, skip the step but still execute post-hook
	if preHookErr != nil {
		result.Skipped = true
		result.Error = preHookErr
		result.StepResult = executor.CreateSkippedResult(step.ID)

		// Execute post-hook even if pre-hook failed (requirement 4.6)
		postHookResult, _ := r.ExecuteStepPostHook(ctx, step, execCtx, result.StepResult)
		result.PostHookResult = postHookResult

		return result
	}

	// Execute the step
	stepResult, stepErr := stepExecutor(ctx, step, execCtx)
	result.StepResult = stepResult
	if stepErr != nil {
		result.Error = stepErr
	}

	// Execute post-hook regardless of step success/failure (requirement 4.6)
	postHookResult, _ := r.ExecuteStepPostHook(ctx, step, execCtx, stepResult)
	result.PostHookResult = postHookResult

	return result
}

// WorkflowExecutionResult contains the complete result of executing a workflow with hooks.
type WorkflowExecutionResult struct {
	PreHookResult  *HookResult
	StepResults    []*StepExecutionResult
	PostHookResult *HookResult
	Skipped        bool
	Error          error
}

// ExecuteWorkflowWithHooks executes a workflow with its pre and post hooks.
// Requirements: 4.1, 4.2, 4.5, 4.6
func (r *Runner) ExecuteWorkflowWithHooks(
	ctx context.Context,
	workflow *types.Workflow,
	execCtx *executor.ExecutionContext,
	workflowExecutor func(ctx context.Context, workflow *types.Workflow, execCtx *executor.ExecutionContext) ([]*StepExecutionResult, error),
) *WorkflowExecutionResult {
	result := &WorkflowExecutionResult{}

	// Execute workflow pre-hook
	preHookResult, preHookErr := r.ExecuteWorkflowPreHook(ctx, workflow, execCtx)
	result.PreHookResult = preHookResult

	// If pre-hook fails, skip the workflow but still execute post-hook
	if preHookErr != nil {
		result.Skipped = true
		result.Error = preHookErr

		// Execute post-hook even if pre-hook failed (requirement 4.6)
		postHookResult, _ := r.ExecuteWorkflowPostHook(ctx, workflow, execCtx, preHookErr)
		result.PostHookResult = postHookResult

		return result
	}

	// Execute the workflow steps
	stepResults, workflowErr := workflowExecutor(ctx, workflow, execCtx)
	result.StepResults = stepResults
	if workflowErr != nil {
		result.Error = workflowErr
	}

	// Execute post-hook regardless of workflow success/failure (requirement 4.6)
	postHookResult, _ := r.ExecuteWorkflowPostHook(ctx, workflow, execCtx, workflowErr)
	result.PostHookResult = postHookResult

	return result
}
