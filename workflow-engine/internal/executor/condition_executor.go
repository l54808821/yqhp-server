package executor

import (
	"context"
	"time"

	"yqhp/workflow-engine/internal/expression"
	"yqhp/workflow-engine/pkg/types"
)

const (
	// ConditionExecutorType 是条件执行器的类型标识符。
	ConditionExecutorType = "condition"
)

// ConditionExecutor 执行条件逻辑步骤。
type ConditionExecutor struct {
	*BaseExecutor
	evaluator expression.ExpressionEvaluator
	registry  *Registry
}

// NewConditionExecutor 创建一个新的条件执行器。
func NewConditionExecutor() *ConditionExecutor {
	return &ConditionExecutor{
		BaseExecutor: NewBaseExecutor(ConditionExecutorType),
		evaluator:    expression.NewEvaluator(),
	}
}

// NewConditionExecutorWithRegistry 使用自定义注册表创建一个新的条件执行器。
func NewConditionExecutorWithRegistry(registry *Registry) *ConditionExecutor {
	return &ConditionExecutor{
		BaseExecutor: NewBaseExecutor(ConditionExecutorType),
		evaluator:    expression.NewEvaluator(),
		registry:     registry,
	}
}

// Init 初始化条件执行器。
func (e *ConditionExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

// Execute 执行条件步骤。
func (e *ConditionExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// 从步骤获取条件
	condition := step.Condition
	if condition == nil {
		return CreateFailedResult(step.ID, startTime, NewConfigError("condition step requires 'condition' configuration", nil)), nil
	}

	// 构建求值上下文
	evalCtx := e.buildEvaluationContext(execCtx)

	// 求值条件表达式
	result, err := e.evaluator.EvaluateString(condition.Expression, evalCtx)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "failed to evaluate condition", err)), nil
	}

	// 确定执行哪个分支
	var branchSteps []types.Step
	var branchName string
	if result {
		branchSteps = condition.Then
		branchName = "then"
	} else {
		branchSteps = condition.Else
		branchName = "else"
	}

	// 构建输出
	output := &ConditionOutput{
		Expression:    condition.Expression,
		Result:        result,
		BranchTaken:   branchName,
		StepsExecuted: make([]string, 0),
	}

	// 执行分支步骤
	if len(branchSteps) > 0 {
		branchResults, err := e.executeBranch(ctx, branchSteps, execCtx)
		if err != nil {
			failedResult := CreateFailedResult(step.ID, startTime, err)
			failedResult.Output = output
			return failedResult, nil
		}

		// 收集已执行的步骤 ID
		for _, br := range branchResults {
			output.StepsExecuted = append(output.StepsExecuted, br.StepID)
		}

		// 检查是否有分支步骤失败
		for _, br := range branchResults {
			if br.Status == types.ResultStatusFailed || br.Status == types.ResultStatusTimeout {
				failedResult := CreateFailedResult(step.ID, startTime, br.Error)
				failedResult.Output = output
				return failedResult, nil
			}
		}
	}

	// 创建成功结果
	successResult := CreateSuccessResult(step.ID, startTime, output)
	successResult.Metrics["condition_result"] = boolToFloat(result)
	successResult.Metrics["branch_steps_count"] = float64(len(branchSteps))

	return successResult, nil
}

// Cleanup 释放条件执行器持有的资源。
func (e *ConditionExecutor) Cleanup(ctx context.Context) error {
	return nil
}

// ConditionOutput 表示条件步骤的输出。
type ConditionOutput struct {
	Expression    string   `json:"expression"`
	Result        bool     `json:"result"`
	BranchTaken   string   `json:"branch_taken"`
	StepsExecuted []string `json:"steps_executed"`
}

// buildEvaluationContext 将 ExecutionContext 转换为表达式求值上下文。
func (e *ConditionExecutor) buildEvaluationContext(execCtx *ExecutionContext) *expression.EvaluationContext {
	evalCtx := expression.NewEvaluationContext()

	if execCtx == nil {
		return evalCtx
	}

	// 复制变量
	for k, v := range execCtx.Variables {
		evalCtx.Set(k, v)
	}

	// 将步骤结果转换为求值上下文格式
	for stepID, result := range execCtx.Results {
		resultMap := map[string]any{
			"status":   string(result.Status),
			"duration": result.Duration.Milliseconds(),
			"step_id":  result.StepID,
		}

		// 添加输出字段
		if result.Output != nil {
			resultMap["output"] = result.Output

			// 如果输出是 map，展平以便于访问
			if outputMap, ok := result.Output.(map[string]any); ok {
				for k, v := range outputMap {
					resultMap[k] = v
				}
			}

			// 特殊处理 HTTPResponse
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

// executeBranch 执行分支中的步骤序列。
func (e *ConditionExecutor) executeBranch(ctx context.Context, steps []types.Step, execCtx *ExecutionContext) ([]*types.StepResult, error) {
	results := make([]*types.StepResult, 0, len(steps))

	// 获取回调
	callback := execCtx.GetCallback()

	for i := range steps {
		step := &steps[i]

		// 跳过禁用的步骤
		if step.Disabled {
			if callback != nil {
				callback.OnStepSkipped(ctx, step, "步骤已禁用", "", 0)
			}
			continue
		}

		// 获取步骤类型的执行器
		executor, err := e.getExecutor(step.Type)
		if err != nil {
			return results, err
		}

		// 执行步骤
		result, err := executor.Execute(ctx, step, execCtx)
		if err != nil {
			return results, err
		}

		// 将结果存储到上下文中供后续步骤使用
		execCtx.SetResult(step.ID, result)
		results = append(results, result)

		// 处理错误策略
		if result.Status == types.ResultStatusFailed || result.Status == types.ResultStatusTimeout {
			switch step.OnError {
			case types.ErrorStrategyAbort:
				return results, result.Error
			case types.ErrorStrategyContinue:
				// 继续下一步
			case types.ErrorStrategySkip:
				// 跳过分支中的剩余步骤
				return results, nil
			default:
				// 默认是中止
				return results, result.Error
			}
		}
	}

	return results, nil
}

// getExecutor 获取给定类型的执行器。
func (e *ConditionExecutor) getExecutor(execType string) (Executor, error) {
	// 如果提供了自定义注册表则使用
	if e.registry != nil {
		return e.registry.GetOrError(execType)
	}
	// 回退到默认注册表
	return DefaultRegistry.GetOrError(execType)
}

// boolToFloat 将布尔值转换为 float64。
func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// init 在默认注册表中注册条件执行器。
func init() {
	MustRegister(NewConditionExecutor())
}
