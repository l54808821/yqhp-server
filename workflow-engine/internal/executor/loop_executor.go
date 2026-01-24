package executor

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"yqhp/workflow-engine/internal/expression"
	"yqhp/workflow-engine/pkg/types"
)

const (
	// LoopExecutorType 是循环执行器的类型标识符
	LoopExecutorType = "loop"

	// LoopModeFor 固定次数循环
	LoopModeFor = "for"
	// LoopModeForeach 集合遍历循环
	LoopModeForeach = "foreach"
	// LoopModeWhile 条件循环
	LoopModeWhile = "while"

	// DefaultMaxIterations while 循环的默认最大迭代次数
	DefaultMaxIterations = 1000
	// DefaultItemVar foreach 循环的默认元素变量名
	DefaultItemVar = "item"
)

// LoopExecutor 执行循环类型的步骤
type LoopExecutor struct {
	*BaseExecutor
	evaluator expression.ExpressionEvaluator
	registry  *Registry
}

// LoopOutput 循环执行的输出
type LoopOutput struct {
	Mode            string   `json:"mode"`
	TotalIterations int      `json:"total_iterations"`
	BreakTriggered  bool     `json:"break_triggered"`
	StepsExecuted   []string `json:"steps_executed"`
	Duration        int64    `json:"duration_ms"`
}

// NewLoopExecutor 创建一个新的循环执行器
func NewLoopExecutor() *LoopExecutor {
	return &LoopExecutor{
		BaseExecutor: NewBaseExecutor(LoopExecutorType),
		evaluator:    expression.NewEvaluator(),
	}
}

// NewLoopExecutorWithRegistry 使用自定义注册表创建循环执行器
func NewLoopExecutorWithRegistry(registry *Registry) *LoopExecutor {
	return &LoopExecutor{
		BaseExecutor: NewBaseExecutor(LoopExecutorType),
		evaluator:    expression.NewEvaluator(),
		registry:     registry,
	}
}

// Init 初始化循环执行器
func (e *LoopExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

// Cleanup 清理循环执行器资源
func (e *LoopExecutor) Cleanup(ctx context.Context) error {
	return nil
}

// Execute 执行循环步骤
func (e *LoopExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// 获取循环配置
	loop := step.Loop
	if loop == nil {
		return CreateFailedResult(step.ID, startTime, NewConfigError("循环步骤需要配置 'loop'（循环配置）", nil)), nil
	}

	// 验证配置
	if err := e.validateConfig(loop); err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// 构建求值上下文
	evalCtx := e.buildEvaluationContext(execCtx)

	// 根据模式执行循环
	var output *LoopOutput
	var err error

	switch loop.Mode {
	case LoopModeFor:
		output, err = e.executeForLoop(ctx, step, loop, execCtx, evalCtx)
	case LoopModeForeach:
		output, err = e.executeForeachLoop(ctx, step, loop, execCtx, evalCtx)
	case LoopModeWhile:
		output, err = e.executeWhileLoop(ctx, step, loop, execCtx, evalCtx)
	default:
		return CreateFailedResult(step.ID, startTime, NewConfigError(fmt.Sprintf("不支持的循环模式: %s", loop.Mode), nil)), nil
	}

	if err != nil {
		failedResult := CreateFailedResult(step.ID, startTime, err)
		if output != nil {
			failedResult.Output = output
		}
		return failedResult, nil
	}

	// 计算持续时间
	output.Duration = time.Since(startTime).Milliseconds()

	// 创建成功结果
	result := CreateSuccessResult(step.ID, startTime, output)
	result.Metrics["loop_iterations_total"] = float64(output.TotalIterations)
	result.Metrics["loop_duration_ms"] = float64(output.Duration)

	return result, nil
}

// validateConfig 验证循环配置
func (e *LoopExecutor) validateConfig(loop *types.Loop) error {
	if loop.Mode == "" {
		return NewConfigError("循环模式不能为空", nil)
	}

	switch loop.Mode {
	case LoopModeFor:
		// count 可以为 0 或负数（跳过循环）
	case LoopModeForeach:
		if loop.Items == nil {
			return NewConfigError("foreach 循环需要配置 'items'（遍历集合）", nil)
		}
	case LoopModeWhile:
		if loop.Condition == "" {
			return NewConfigError("while 循环需要配置 'condition'（循环条件）", nil)
		}
	default:
		return NewConfigError(fmt.Sprintf("无效的循环模式: %s，必须是 for、foreach 或 while", loop.Mode), nil)
	}

	if len(loop.Steps) == 0 {
		return NewConfigError("循环体至少需要包含一个步骤", nil)
	}

	return nil
}

// executeForLoop 执行固定次数循环
func (e *LoopExecutor) executeForLoop(ctx context.Context, step *types.Step, loop *types.Loop, execCtx *ExecutionContext, evalCtx *expression.EvaluationContext) (*LoopOutput, error) {
	output := &LoopOutput{
		Mode:          LoopModeFor,
		StepsExecuted: make([]string, 0),
	}

	count := loop.Count
	if count <= 0 {
		// 跳过循环
		return output, nil
	}

	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			return output, ctx.Err()
		default:
		}

		// 设置循环变量
		e.setLoopVariables(execCtx, evalCtx, i, i+1, count, nil, "")

		// 检查 break 条件
		if loop.BreakCondition != "" {
			shouldBreak, err := e.evaluateCondition(loop.BreakCondition, evalCtx)
			if err != nil {
				return output, NewExecutionError(step.ID, fmt.Sprintf("第 %d 次迭代时 break 条件求值失败", i), err)
			}
			if shouldBreak {
				output.BreakTriggered = true
				break
			}
		}

		// 检查 continue 条件
		if loop.ContinueCondition != "" {
			shouldContinue, err := e.evaluateCondition(loop.ContinueCondition, evalCtx)
			if err != nil {
				return output, NewExecutionError(step.ID, fmt.Sprintf("第 %d 次迭代时 continue 条件求值失败", i), err)
			}
			if shouldContinue {
				output.TotalIterations++
				continue
			}
		}

		// 执行循环体
		stepsExecuted, err := e.executeLoopBody(ctx, step, loop.Steps, execCtx, i)
		if err != nil {
			return output, err
		}
		output.StepsExecuted = append(output.StepsExecuted, stepsExecuted...)
		output.TotalIterations++
	}

	return output, nil
}

// executeForeachLoop 执行集合遍历循环
func (e *LoopExecutor) executeForeachLoop(ctx context.Context, step *types.Step, loop *types.Loop, execCtx *ExecutionContext, evalCtx *expression.EvaluationContext) (*LoopOutput, error) {
	output := &LoopOutput{
		Mode:          LoopModeForeach,
		StepsExecuted: make([]string, 0),
	}

	// 解析 items
	items, err := e.resolveItems(loop.Items, evalCtx)
	if err != nil {
		return output, NewExecutionError(step.ID, "解析遍历集合失败", err)
	}

	if len(items) == 0 {
		// 空集合，跳过循环
		return output, nil
	}

	itemVar := loop.ItemVar
	if itemVar == "" {
		itemVar = DefaultItemVar
	}

	for i, item := range items {
		select {
		case <-ctx.Done():
			return output, ctx.Err()
		default:
		}

		// 设置循环变量
		e.setLoopVariables(execCtx, evalCtx, i, i+1, len(items), item, itemVar)

		// 检查 break 条件
		if loop.BreakCondition != "" {
			shouldBreak, err := e.evaluateCondition(loop.BreakCondition, evalCtx)
			if err != nil {
				return output, NewExecutionError(step.ID, fmt.Sprintf("第 %d 次迭代时 break 条件求值失败", i), err)
			}
			if shouldBreak {
				output.BreakTriggered = true
				break
			}
		}

		// 检查 continue 条件
		if loop.ContinueCondition != "" {
			shouldContinue, err := e.evaluateCondition(loop.ContinueCondition, evalCtx)
			if err != nil {
				return output, NewExecutionError(step.ID, fmt.Sprintf("第 %d 次迭代时 continue 条件求值失败", i), err)
			}
			if shouldContinue {
				output.TotalIterations++
				continue
			}
		}

		// 执行循环体
		stepsExecuted, err := e.executeLoopBody(ctx, step, loop.Steps, execCtx, i)
		if err != nil {
			return output, err
		}
		output.StepsExecuted = append(output.StepsExecuted, stepsExecuted...)
		output.TotalIterations++
	}

	return output, nil
}

// executeWhileLoop 执行条件循环
func (e *LoopExecutor) executeWhileLoop(ctx context.Context, step *types.Step, loop *types.Loop, execCtx *ExecutionContext, evalCtx *expression.EvaluationContext) (*LoopOutput, error) {
	output := &LoopOutput{
		Mode:          LoopModeWhile,
		StepsExecuted: make([]string, 0),
	}

	maxIterations := loop.MaxIterations
	if maxIterations <= 0 {
		maxIterations = DefaultMaxIterations
	}

	iteration := 0
	for {
		select {
		case <-ctx.Done():
			return output, ctx.Err()
		default:
		}

		// 检查最大迭代次数
		if iteration >= maxIterations {
			return output, NewExecutionError(step.ID, fmt.Sprintf("while 循环超过最大迭代次数 (%d)", maxIterations), nil)
		}

		// 设置循环变量
		e.setLoopVariables(execCtx, evalCtx, iteration, iteration+1, 0, nil, "")

		// 评估 while 条件
		shouldContinue, err := e.evaluateCondition(loop.Condition, evalCtx)
		if err != nil {
			return output, NewExecutionError(step.ID, fmt.Sprintf("第 %d 次迭代时 while 条件求值失败", iteration), err)
		}
		if !shouldContinue {
			break
		}

		// 检查 break 条件
		if loop.BreakCondition != "" {
			shouldBreak, err := e.evaluateCondition(loop.BreakCondition, evalCtx)
			if err != nil {
				return output, NewExecutionError(step.ID, fmt.Sprintf("第 %d 次迭代时 break 条件求值失败", iteration), err)
			}
			if shouldBreak {
				output.BreakTriggered = true
				break
			}
		}

		// 检查 continue 条件
		if loop.ContinueCondition != "" {
			shouldSkip, err := e.evaluateCondition(loop.ContinueCondition, evalCtx)
			if err != nil {
				return output, NewExecutionError(step.ID, fmt.Sprintf("第 %d 次迭代时 continue 条件求值失败", iteration), err)
			}
			if shouldSkip {
				output.TotalIterations++
				iteration++
				continue
			}
		}

		// 执行循环体
		stepsExecuted, err := e.executeLoopBody(ctx, step, loop.Steps, execCtx, iteration)
		if err != nil {
			return output, err
		}
		output.StepsExecuted = append(output.StepsExecuted, stepsExecuted...)
		output.TotalIterations++
		iteration++
	}

	return output, nil
}

// executeLoopBody 执行循环体中的步骤
func (e *LoopExecutor) executeLoopBody(ctx context.Context, parentStep *types.Step, steps []types.Step, execCtx *ExecutionContext, iteration int) ([]string, error) {
	stepsExecuted := make([]string, 0, len(steps))

	// 获取回调
	callback := execCtx.GetCallback()

	for i := range steps {
		step := &steps[i]

		// 跳过禁用的步骤
		if step.Disabled {
			if callback != nil {
				callback.OnStepSkipped(ctx, step, "步骤已禁用", parentStep.ID, iteration+1)
			}
			continue
		}

		// 触发步骤开始回调
		if callback != nil {
			callback.OnStepStart(ctx, step, parentStep.ID, iteration+1) // iteration 从1开始
			callback.OnProgress(ctx, i+1, len(steps), step.Name)
		}

		// 获取执行器
		executor, err := e.getExecutor(step.Type)
		if err != nil {
			if callback != nil {
				callback.OnStepFailed(ctx, step, err, 0, parentStep.ID, iteration+1)
			}
			return stepsExecuted, NewExecutionError(parentStep.ID, fmt.Sprintf("第 %d 次迭代，步骤 '%s': %v", iteration, step.ID, err), nil)
		}

		// 创建子上下文，设置父步骤信息
		childCtx := execCtx.Clone().WithParentStep(parentStep.ID, iteration+1)

		// 执行步骤
		startTime := time.Now()
		result, err := executor.Execute(ctx, step, childCtx)
		if err != nil {
			if callback != nil {
				callback.OnStepFailed(ctx, step, err, time.Since(startTime), parentStep.ID, iteration+1)
			}
			return stepsExecuted, NewExecutionError(parentStep.ID, fmt.Sprintf("第 %d 次迭代，步骤 '%s': %v", iteration, step.ID, err), nil)
		}

		stepsExecuted = append(stepsExecuted, step.ID)

		// 将结果存储到上下文
		execCtx.SetResult(step.ID, result)

		// 触发步骤完成/失败回调
		if callback != nil {
			if result.Status == types.ResultStatusSuccess {
				callback.OnStepComplete(ctx, step, result, parentStep.ID, iteration+1)
			} else {
				callback.OnStepFailed(ctx, step, result.Error, result.Duration, parentStep.ID, iteration+1)
			}
		}

		// 处理错误
		if result.Status == types.ResultStatusFailed || result.Status == types.ResultStatusTimeout {
			switch step.OnError {
			case types.ErrorStrategyAbort, "":
				return stepsExecuted, NewExecutionError(parentStep.ID, fmt.Sprintf("第 %d 次迭代，步骤 '%s' 执行失败", iteration, step.ID), result.Error)
			case types.ErrorStrategyContinue:
				// 继续下一次迭代
				return stepsExecuted, nil
			case types.ErrorStrategySkip:
				// 跳过当前迭代的剩余步骤
				return stepsExecuted, nil
			}
		}
	}

	return stepsExecuted, nil
}

// setLoopVariables 设置循环变量到执行上下文
func (e *LoopExecutor) setLoopVariables(execCtx *ExecutionContext, evalCtx *expression.EvaluationContext, index, iteration, count int, item any, itemVar string) {
	// 设置到执行上下文
	execCtx.SetVariable("loop.index", index)
	execCtx.SetVariable("loop.iteration", iteration)
	if count > 0 {
		execCtx.SetVariable("loop.count", count)
	}
	if item != nil && itemVar != "" {
		execCtx.SetVariable("loop.item", item)
		execCtx.SetVariable(itemVar, item)
	}

	// 设置到求值上下文
	loopCtx := map[string]any{
		"index":     index,
		"iteration": iteration,
	}
	if count > 0 {
		loopCtx["count"] = count
	}
	if item != nil {
		loopCtx["item"] = item
	}
	evalCtx.Set("loop", loopCtx)
	if item != nil && itemVar != "" {
		evalCtx.Set(itemVar, item)
	}
}

// resolveItems 解析 items 配置
func (e *LoopExecutor) resolveItems(items any, evalCtx *expression.EvaluationContext) ([]any, error) {
	if items == nil {
		return nil, nil
	}

	// 如果是字符串，尝试作为表达式解析
	if str, ok := items.(string); ok {
		// 检查是否是表达式
		if len(str) > 3 && str[0] == '$' && str[1] == '{' && str[len(str)-1] == '}' {
			// 提取表达式内容并从上下文中获取值
			expr := str[2 : len(str)-1]
			// 尝试从变量中获取
			if result, ok := evalCtx.Variables[expr]; ok {
				return e.toSlice(result)
			}
			// 尝试解析点分路径
			result := e.resolvePathValue(expr, evalCtx)
			if result != nil {
				return e.toSlice(result)
			}
			return nil, fmt.Errorf("expression '%s' resolved to nil", expr)
		}
		// 不是表达式，返回单元素数组
		return []any{str}, nil
	}

	// 如果已经是切片
	return e.toSlice(items)
}

// resolvePathValue 解析点分路径的值
func (e *LoopExecutor) resolvePathValue(path string, evalCtx *expression.EvaluationContext) any {
	parts := splitPath(path)
	if len(parts) == 0 {
		return nil
	}

	// 首先尝试从变量中获取
	var current any
	if val, ok := evalCtx.Variables[parts[0]]; ok {
		current = val
	} else if val, ok := evalCtx.Results[parts[0]]; ok {
		current = val
	} else {
		return nil
	}

	// 遍历路径的其余部分
	for i := 1; i < len(parts); i++ {
		if current == nil {
			return nil
		}
		current = getFieldValue(current, parts[i])
	}

	return current
}

// splitPath 分割点分路径
func splitPath(path string) []string {
	var parts []string
	var current string
	for _, c := range path {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// getFieldValue 从 map 或结构体中获取字段值
func getFieldValue(obj any, field string) any {
	if obj == nil {
		return nil
	}

	// 尝试 map[string]any
	if m, ok := obj.(map[string]any); ok {
		return m[field]
	}

	// 尝试反射
	rv := reflect.ValueOf(obj)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Map:
		key := reflect.ValueOf(field)
		val := rv.MapIndex(key)
		if val.IsValid() {
			return val.Interface()
		}
	case reflect.Struct:
		fv := rv.FieldByName(field)
		if fv.IsValid() {
			return fv.Interface()
		}
	}

	return nil
}

// toSlice 将值转换为切片
func (e *LoopExecutor) toSlice(v any) ([]any, error) {
	if v == nil {
		return nil, nil
	}

	// 如果已经是 []any
	if slice, ok := v.([]any); ok {
		return slice, nil
	}

	// 使用反射处理其他切片类型
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		result := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			result[i] = rv.Index(i).Interface()
		}
		return result, nil
	}

	// 单个值，返回单元素数组
	return []any{v}, nil
}

// evaluateCondition 评估条件表达式
func (e *LoopExecutor) evaluateCondition(condition string, evalCtx *expression.EvaluationContext) (bool, error) {
	return e.evaluator.EvaluateString(condition, evalCtx)
}

// buildEvaluationContext 构建求值上下文
func (e *LoopExecutor) buildEvaluationContext(execCtx *ExecutionContext) *expression.EvaluationContext {
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

// getExecutor 获取指定类型的执行器
func (e *LoopExecutor) getExecutor(execType string) (Executor, error) {
	if e.registry != nil {
		return e.registry.GetOrError(execType)
	}
	return DefaultRegistry.GetOrError(execType)
}

// init 在默认注册表中注册循环执行器
func init() {
	MustRegister(NewLoopExecutor())
}
