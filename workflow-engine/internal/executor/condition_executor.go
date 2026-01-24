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
// 新格式：
//
//	step.Branches: []ConditionBranch{
//	  { Kind: if/else_if/else, Expression: "...", Steps: [...] },
//	}
//
// 旧格式（向后兼容）：
//
//	step.Config:   type(if/else_if/else)/expression
//	step.Children: 命中时执行的子步骤
func (e *ConditionExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// 优先使用新格式 Branches
	if len(step.Branches) > 0 {
		return e.executeWithBranches(ctx, step, execCtx, startTime)
	}

	// 兼容旧格式：单步 condition，使用 config.type + config.expression + children
	condType := types.ConditionTypeIf // 默认 if
	if step.Config != nil {
		if t, ok := step.Config["type"].(string); ok && t != "" {
			condType = types.ConditionBranchKind(t)
		}
	}

	expression := ""
	if step.Config != nil {
		if expr, ok := step.Config["expression"].(string); ok {
			expression = expr
		}
	}

	// 构建求值上下文
	evalCtx := e.buildEvaluationContext(execCtx)

	var shouldExecute bool
	var err error

	// 获取条件组的执行状态
	conditionGroupMatched := false
	if matched, ok := execCtx.Variables["__condition_group_matched__"].(bool); ok {
		conditionGroupMatched = matched
	}

	switch condType {
	case types.ConditionTypeIf:
		// if: 重置条件组状态，求值表达式
		execCtx.SetVariable("__condition_group_matched__", false)
		shouldExecute, err = e.evaluator.EvaluateString(expression, evalCtx)
		if err != nil {
			return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "条件表达式求值失败", err)), nil
		}
		if shouldExecute {
			execCtx.SetVariable("__condition_group_matched__", true)
		}

	case types.ConditionTypeElseIf:
		// else_if: 如果前面的条件已匹配，跳过；否则求值表达式
		if conditionGroupMatched {
			shouldExecute = false
		} else {
			shouldExecute, err = e.evaluator.EvaluateString(expression, evalCtx)
			if err != nil {
				return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "条件表达式求值失败", err)), nil
			}
			if shouldExecute {
				execCtx.SetVariable("__condition_group_matched__", true)
			}
		}

	case types.ConditionTypeElse:
		// else: 如果前面的条件都没匹配，执行
		shouldExecute = !conditionGroupMatched
		if shouldExecute {
			execCtx.SetVariable("__condition_group_matched__", true)
		}

	default:
		// 默认当作 if 处理
		execCtx.SetVariable("__condition_group_matched__", false)
		shouldExecute, err = e.evaluator.EvaluateString(expression, evalCtx)
		if err != nil {
			return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "条件表达式求值失败", err)), nil
		}
		if shouldExecute {
			execCtx.SetVariable("__condition_group_matched__", true)
		}
	}

	// 构建输出
	output := &ConditionOutput{
		Expression:    expression,
		Result:        shouldExecute,
		BranchTaken:   string(condType),
		StepsExecuted: make([]string, 0),
	}

	// 如果条件满足，执行子步骤
	if shouldExecute && len(step.Children) > 0 {
		branchResults, err := e.executeBranch(ctx, step.Children, execCtx, step.ID)
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
	successResult.Metrics["condition_result"] = boolToFloat(shouldExecute)
	successResult.Metrics["branch_steps_count"] = float64(len(step.Children))

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

// executeWithBranches 按新格式执行条件步骤（Step.Branches）
func (e *ConditionExecutor) executeWithBranches(ctx context.Context, step *types.Step, execCtx *ExecutionContext, startTime time.Time) (*types.StepResult, error) {
	// 构建求值上下文
	evalCtx := e.buildEvaluationContext(execCtx)

	var (
		conditionGroupMatched bool
		takenBranchKind       types.ConditionBranchKind
		takenExpression       string
		stepsExecuted         []string
	)

	for _, br := range step.Branches {
		kind := br.Kind
		// 默认视为空值为 if（防御性编程）
		if kind == "" {
			kind = types.ConditionTypeIf
		}

		shouldExecute := false

		switch kind {
		case types.ConditionTypeIf:
			// if: 作为一个新的条件组入口，重置 group 状态
			conditionGroupMatched = false
			expr := br.Expression
			if expr == "" {
				// 没有表达式视为 false
				shouldExecute = false
			} else {
				result, err := e.evaluator.EvaluateString(expr, evalCtx)
				if err != nil {
					return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "条件表达式求值失败", err)), nil
				}
				shouldExecute = result
			}
			if shouldExecute {
				conditionGroupMatched = true
				takenBranchKind = kind
				takenExpression = br.Expression
			}

		case types.ConditionTypeElseIf:
			// else_if: 仅在前面没有命中分支时才求值
			if conditionGroupMatched {
				shouldExecute = false
			} else {
				expr := br.Expression
				if expr == "" {
					shouldExecute = false
				} else {
					result, err := e.evaluator.EvaluateString(expr, evalCtx)
					if err != nil {
						return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "条件表达式求值失败", err)), nil
					}
					shouldExecute = result
				}
				if shouldExecute {
					conditionGroupMatched = true
					takenBranchKind = kind
					takenExpression = br.Expression
				}
			}

		case types.ConditionTypeElse:
			// else: 当前组之前都未命中，则执行
			if !conditionGroupMatched {
				shouldExecute = true
				conditionGroupMatched = true
				takenBranchKind = kind
				// else 没有表达式
				takenExpression = ""
			}

		default:
			// 未知 kind，按 if 处理
			conditionGroupMatched = false
			expr := br.Expression
			if expr == "" {
				shouldExecute = false
			} else {
				result, err := e.evaluator.EvaluateString(expr, evalCtx)
				if err != nil {
					return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "条件表达式求值失败", err)), nil
				}
				shouldExecute = result
			}
			if shouldExecute {
				conditionGroupMatched = true
				takenBranchKind = kind
				takenExpression = br.Expression
			}
		}

		// 命中当前分支，执行其 steps 并结束整个条件步骤
		if shouldExecute {
			if len(br.Steps) > 0 {
				branchResults, err := e.executeBranch(ctx, br.Steps, execCtx, step.ID)
				if err != nil {
					failedResult := CreateFailedResult(step.ID, startTime, err)
					failedResult.Output = &ConditionOutput{
						Expression:    takenExpression,
						Result:        true,
						BranchTaken:   string(takenBranchKind),
						StepsExecuted: stepsExecuted,
					}
					return failedResult, nil
				}

				for _, r := range branchResults {
					stepsExecuted = append(stepsExecuted, r.StepID)
				}

				for _, r := range branchResults {
					if r.Status == types.ResultStatusFailed || r.Status == types.ResultStatusTimeout {
						failedResult := CreateFailedResult(step.ID, startTime, r.Error)
						failedResult.Output = &ConditionOutput{
							Expression:    takenExpression,
							Result:        true,
							BranchTaken:   string(takenBranchKind),
							StepsExecuted: stepsExecuted,
						}
						return failedResult, nil
					}
				}
			}

			// 成功执行一个分支后结束循环（first_match 语义）
			successResult := CreateSuccessResult(step.ID, startTime, &ConditionOutput{
				Expression:    takenExpression,
				Result:        true,
				BranchTaken:   string(takenBranchKind),
				StepsExecuted: stepsExecuted,
			})
			successResult.Metrics["condition_result"] = 1
			successResult.Metrics["branch_steps_count"] = float64(len(stepsExecuted))
			return successResult, nil
		}
	}

	// 没有任何分支被命中
	successResult := CreateSuccessResult(step.ID, startTime, &ConditionOutput{
		Expression:    takenExpression,
		Result:        false,
		BranchTaken:   string(takenBranchKind),
		StepsExecuted: []string{},
	})
	successResult.Metrics["condition_result"] = 0
	successResult.Metrics["branch_steps_count"] = 0
	return successResult, nil
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
func (e *ConditionExecutor) executeBranch(ctx context.Context, steps []types.Step, execCtx *ExecutionContext, parentID string) ([]*types.StepResult, error) {
	results := make([]*types.StepResult, 0, len(steps))

	// 获取回调
	callback := execCtx.GetCallback()

	for i := range steps {
		step := &steps[i]

		// 跳过禁用的步骤
		if step.Disabled {
			if callback != nil {
				callback.OnStepSkipped(ctx, step, "步骤已禁用", parentID, 0)
			}
			continue
		}

		// 发送步骤开始事件
		if callback != nil {
			callback.OnStepStart(ctx, step, parentID, 0)
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

		// 发送步骤完成事件
		if callback != nil {
			callback.OnStepComplete(ctx, step, result, parentID, 0)
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
