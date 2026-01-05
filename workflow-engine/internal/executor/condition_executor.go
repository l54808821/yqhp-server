package executor

import (
	"context"
	"time"

	"github.com/grafana/k6/workflow-engine/internal/expression"
	"github.com/grafana/k6/workflow-engine/pkg/types"
)

const (
	// ConditionExecutorType is the type identifier for condition executor.
	ConditionExecutorType = "condition"
)

// ConditionExecutor executes conditional logic steps.
type ConditionExecutor struct {
	*BaseExecutor
	evaluator expression.ExpressionEvaluator
	registry  *Registry
}

// NewConditionExecutor creates a new condition executor.
func NewConditionExecutor() *ConditionExecutor {
	return &ConditionExecutor{
		BaseExecutor: NewBaseExecutor(ConditionExecutorType),
		evaluator:    expression.NewEvaluator(),
	}
}

// NewConditionExecutorWithRegistry creates a new condition executor with a custom registry.
func NewConditionExecutorWithRegistry(registry *Registry) *ConditionExecutor {
	return &ConditionExecutor{
		BaseExecutor: NewBaseExecutor(ConditionExecutorType),
		evaluator:    expression.NewEvaluator(),
		registry:     registry,
	}
}

// Init initializes the condition executor.
func (e *ConditionExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

// Execute executes a condition step.
func (e *ConditionExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// Get condition from step
	condition := step.Condition
	if condition == nil {
		return CreateFailedResult(step.ID, startTime, NewConfigError("condition step requires 'condition' configuration", nil)), nil
	}

	// Build evaluation context
	evalCtx := e.buildEvaluationContext(execCtx)

	// Evaluate the condition expression
	result, err := e.evaluator.EvaluateString(condition.Expression, evalCtx)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "failed to evaluate condition", err)), nil
	}

	// Determine which branch to execute
	var branchSteps []types.Step
	var branchName string
	if result {
		branchSteps = condition.Then
		branchName = "then"
	} else {
		branchSteps = condition.Else
		branchName = "else"
	}

	// Build output
	output := &ConditionOutput{
		Expression:    condition.Expression,
		Result:        result,
		BranchTaken:   branchName,
		StepsExecuted: make([]string, 0),
	}

	// Execute branch steps
	if len(branchSteps) > 0 {
		branchResults, err := e.executeBranch(ctx, branchSteps, execCtx)
		if err != nil {
			failedResult := CreateFailedResult(step.ID, startTime, err)
			failedResult.Output = output
			return failedResult, nil
		}

		// Collect executed step IDs
		for _, br := range branchResults {
			output.StepsExecuted = append(output.StepsExecuted, br.StepID)
		}

		// Check if any branch step failed
		for _, br := range branchResults {
			if br.Status == types.ResultStatusFailed || br.Status == types.ResultStatusTimeout {
				failedResult := CreateFailedResult(step.ID, startTime, br.Error)
				failedResult.Output = output
				return failedResult, nil
			}
		}
	}

	// Create success result
	successResult := CreateSuccessResult(step.ID, startTime, output)
	successResult.Metrics["condition_result"] = boolToFloat(result)
	successResult.Metrics["branch_steps_count"] = float64(len(branchSteps))

	return successResult, nil
}

// Cleanup releases resources held by the condition executor.
func (e *ConditionExecutor) Cleanup(ctx context.Context) error {
	return nil
}

// ConditionOutput represents the output of a condition step.
type ConditionOutput struct {
	Expression    string   `json:"expression"`
	Result        bool     `json:"result"`
	BranchTaken   string   `json:"branch_taken"`
	StepsExecuted []string `json:"steps_executed"`
}

// buildEvaluationContext converts ExecutionContext to expression.EvaluationContext.
func (e *ConditionExecutor) buildEvaluationContext(execCtx *ExecutionContext) *expression.EvaluationContext {
	evalCtx := expression.NewEvaluationContext()

	if execCtx == nil {
		return evalCtx
	}

	// Copy variables
	for k, v := range execCtx.Variables {
		evalCtx.Set(k, v)
	}

	// Convert step results to evaluation context format
	for stepID, result := range execCtx.Results {
		resultMap := map[string]any{
			"status":   string(result.Status),
			"duration": result.Duration.Milliseconds(),
			"step_id":  result.StepID,
		}

		// Add output fields
		if result.Output != nil {
			resultMap["output"] = result.Output

			// If output is a map, flatten it for easier access
			if outputMap, ok := result.Output.(map[string]any); ok {
				for k, v := range outputMap {
					resultMap[k] = v
				}
			}

			// Handle HTTPResponse specifically
			if httpResp, ok := result.Output.(*HTTPResponse); ok {
				resultMap["status_code"] = httpResp.StatusCode
				resultMap["body"] = httpResp.Body
				resultMap["headers"] = httpResp.Headers
			}
		}

		if result.Error != nil {
			resultMap["error"] = result.Error.Error()
		}

		evalCtx.SetResult(stepID, resultMap)
	}

	return evalCtx
}

// executeBranch executes a sequence of steps in a branch.
func (e *ConditionExecutor) executeBranch(ctx context.Context, steps []types.Step, execCtx *ExecutionContext) ([]*types.StepResult, error) {
	results := make([]*types.StepResult, 0, len(steps))

	for i := range steps {
		step := &steps[i]

		// Get executor for step type
		executor, err := e.getExecutor(step.Type)
		if err != nil {
			return results, err
		}

		// Execute step
		result, err := executor.Execute(ctx, step, execCtx)
		if err != nil {
			return results, err
		}

		// Store result in context for subsequent steps
		execCtx.SetResult(step.ID, result)
		results = append(results, result)

		// Handle error strategy
		if result.Status == types.ResultStatusFailed || result.Status == types.ResultStatusTimeout {
			switch step.OnError {
			case types.ErrorStrategyAbort:
				return results, result.Error
			case types.ErrorStrategyContinue:
				// Continue to next step
			case types.ErrorStrategySkip:
				// Skip remaining steps in branch
				return results, nil
			default:
				// Default is abort
				return results, result.Error
			}
		}
	}

	return results, nil
}

// getExecutor gets an executor for the given type.
func (e *ConditionExecutor) getExecutor(execType string) (Executor, error) {
	// Use custom registry if provided
	if e.registry != nil {
		return e.registry.GetOrError(execType)
	}
	// Fall back to default registry
	return DefaultRegistry.GetOrError(execType)
}

// boolToFloat converts a bool to float64.
func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// init registers the condition executor with the default registry.
func init() {
	MustRegister(NewConditionExecutor())
}
