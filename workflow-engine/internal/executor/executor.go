// Package executor 提供工作流步骤执行的执行器框架。
package executor

import (
	"context"
	"sync"

	"yqhp/workflow-engine/internal/expression"
	"yqhp/workflow-engine/pkg/types"
)

// Executor 定义步骤执行器的接口。
type Executor interface {
	// Type 返回执行器类型标识符。
	Type() string

	// Init 使用配置初始化执行器。
	Init(ctx context.Context, config map[string]any) error

	// Execute 执行步骤并返回结果。
	Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error)

	// Cleanup 释放执行器持有的资源。
	Cleanup(ctx context.Context) error
}

// ExecutionContext 保存步骤执行的运行时状态。
type ExecutionContext struct {
	// Variables 保存执行期间可访问的变量值。
	Variables map[string]any

	// Results 保存按步骤 ID 索引的步骤执行结果。
	Results map[string]*types.StepResult

	// VU 是执行工作流的虚拟用户。
	VU *types.VirtualUser

	// Iteration 是当前迭代次数。
	Iteration int

	// WorkflowID 是正在执行的工作流 ID。
	WorkflowID string

	// ExecutionID 是此次执行的唯一 ID。
	ExecutionID string

	// Callback 执行回调（用于实时通知）
	Callback types.ExecutionCallback

	// ParentStepID 父步骤ID（用于循环等嵌套场景）
	ParentStepID string

	// LoopIteration 循环迭代次数（从1开始）
	LoopIteration int

	// logCollector 日志收集器（用于统一收集执行过程中的日志）
	logCollector *types.LogCollector

	mu sync.RWMutex
}

// NewExecutionContext 创建一个新的 ExecutionContext。
func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		Variables:    make(map[string]any),
		Results:      make(map[string]*types.StepResult),
		logCollector: types.NewLogCollector(),
	}
}

// WithVariables 设置变量映射。
func (c *ExecutionContext) WithVariables(vars map[string]any) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Variables = vars
	return c
}

// WithVU 设置虚拟用户。
func (c *ExecutionContext) WithVU(vu *types.VirtualUser) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.VU = vu
	return c
}

// WithIteration 设置迭代次数。
func (c *ExecutionContext) WithIteration(iteration int) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Iteration = iteration
	return c
}

// WithWorkflowID 设置工作流 ID。
func (c *ExecutionContext) WithWorkflowID(id string) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.WorkflowID = id
	return c
}

// WithExecutionID 设置执行 ID。
func (c *ExecutionContext) WithExecutionID(id string) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ExecutionID = id
	return c
}

// SetVariable 设置变量值。
func (c *ExecutionContext) SetVariable(name string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Variables[name] = value
}

// GetVariable 获取变量值。
func (c *ExecutionContext) GetVariable(name string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.Variables[name]
	return val, ok
}

// SetResult 存储步骤结果。
func (c *ExecutionContext) SetResult(stepID string, result *types.StepResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Results[stepID] = result
}

// GetResult 获取步骤结果。
func (c *ExecutionContext) GetResult(stepID string) (*types.StepResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, ok := c.Results[stepID]
	return result, ok
}

// Clone 创建执行上下文的深拷贝。
// 确保子上下文中对变量的修改不会影响父上下文。
func (c *ExecutionContext) Clone() *ExecutionContext {
	c.mu.RLock()
	defer c.mu.RUnlock()

	newCtx := &ExecutionContext{
		Variables:     make(map[string]any, len(c.Variables)),
		Results:       make(map[string]*types.StepResult, len(c.Results)),
		VU:            c.VU,
		Iteration:     c.Iteration,
		WorkflowID:    c.WorkflowID,
		ExecutionID:   c.ExecutionID,
		Callback:      c.Callback,
		ParentStepID:  c.ParentStepID,
		LoopIteration: c.LoopIteration,
		logCollector:  c.logCollector.Clone(),
	}

	// 深拷贝变量，防止子上下文修改影响父上下文
	for k, v := range c.Variables {
		newCtx.Variables[k] = deepCopyValue(v)
	}
	// Results 保持浅拷贝，因为 StepResult 创建后不应被修改，
	// 且子步骤结果需要被父上下文访问
	for k, v := range c.Results {
		newCtx.Results[k] = v
	}

	return newCtx
}

// deepCopyValue 深拷贝变量值
// 支持基本类型、map、slice 的递归深拷贝
func deepCopyValue(v any) any {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]any:
		newMap := make(map[string]any, len(val))
		for k, v := range val {
			newMap[k] = deepCopyValue(v)
		}
		return newMap
	case []any:
		newSlice := make([]any, len(val))
		for i, v := range val {
			newSlice[i] = deepCopyValue(v)
		}
		return newSlice
	case []string:
		newSlice := make([]string, len(val))
		copy(newSlice, val)
		return newSlice
	case []int:
		newSlice := make([]int, len(val))
		copy(newSlice, val)
		return newSlice
	case []float64:
		newSlice := make([]float64, len(val))
		copy(newSlice, val)
		return newSlice
	case []bool:
		newSlice := make([]bool, len(val))
		copy(newSlice, val)
		return newSlice
	default:
		// 基本类型（string, int, float64, bool 等）直接返回
		// 对于其他复杂类型，保持引用（如 *StepResult, *VirtualUser 等）
		return v
	}
}

// WithCallback 设置执行回调。
func (c *ExecutionContext) WithCallback(callback types.ExecutionCallback) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Callback = callback
	return c
}

// WithParentStep 设置父步骤信息（用于循环等嵌套场景）。
func (c *ExecutionContext) WithParentStep(parentID string, iteration int) *ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ParentStepID = parentID
	c.LoopIteration = iteration
	return c
}

// GetCallback 获取执行回调。
func (c *ExecutionContext) GetCallback() types.ExecutionCallback {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Callback
}

// GetParentStepID 获取父步骤ID。
func (c *ExecutionContext) GetParentStepID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ParentStepID
}

// GetLoopIteration 获取循环迭代次数。
func (c *ExecutionContext) GetLoopIteration() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LoopIteration
}

// ToEvaluationContext 将 ExecutionContext 转换为表达式求值上下文。
func (c *ExecutionContext) ToEvaluationContext() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	evalCtx := make(map[string]any)

	// 复制变量
	for k, v := range c.Variables {
		evalCtx[k] = v
	}

	// 将结果转换为适合表达式求值的格式
	for stepID, result := range c.Results {
		resultMap := map[string]any{
			"status":     string(result.Status),
			"duration":   result.Duration.Milliseconds(),
			"output":     result.Output,
			"step_id":    result.StepID,
			"start_time": result.StartTime,
			"end_time":   result.EndTime,
		}
		if result.Error != nil {
			resultMap["error"] = result.Error.Error()
		}
		if result.Metrics != nil {
			resultMap["metrics"] = result.Metrics
		}
		evalCtx[stepID] = resultMap
	}

	return evalCtx
}

// ========== 日志收集相关方法 ==========

// GetLogCollector 获取日志收集器
func (c *ExecutionContext) GetLogCollector() *types.LogCollector {
	return c.logCollector
}

// AppendLog 添加单条日志
func (c *ExecutionContext) AppendLog(entry types.ConsoleLogEntry) {
	if c.logCollector != nil {
		c.logCollector.AppendLog(entry)
	}
}

// AppendLogs 批量添加日志
func (c *ExecutionContext) AppendLogs(entries []types.ConsoleLogEntry) {
	if c.logCollector != nil {
		c.logCollector.AppendLogs(entries)
	}
}

// FlushLogs 获取并清空所有日志
func (c *ExecutionContext) FlushLogs() []types.ConsoleLogEntry {
	if c.logCollector != nil {
		return c.logCollector.FlushLogs()
	}
	return nil
}

// GetLogs 获取所有日志（不清空）
func (c *ExecutionContext) GetLogs() []types.ConsoleLogEntry {
	if c.logCollector != nil {
		return c.logCollector.GetLogs()
	}
	return nil
}

// SetVariableWithTracking 设置变量并记录变更
// scope: "env" 表示环境变量，"temp" 表示临时变量
// source: 变更来源，如 "set_variable", "extract_param", "js_script", "loop" 等
func (c *ExecutionContext) SetVariableWithTracking(name string, value any, scope, source string) {
	c.mu.Lock()
	oldValue, _ := c.Variables[name]
	c.Variables[name] = value
	c.mu.Unlock()

	if c.logCollector != nil {
		// 标记环境变量
		if scope == "env" {
			c.logCollector.MarkAsEnvVar(name)
		}
		// 记录变量变更
		c.logCollector.RecordVariableChange(name, oldValue, value, scope, source)
	}
}

// MarkAsEnvVar 标记某个变量为环境变量
func (c *ExecutionContext) MarkAsEnvVar(name string) {
	if c.logCollector != nil {
		c.logCollector.MarkAsEnvVar(name)
	}
}

// IsEnvVar 检查是否是环境变量
func (c *ExecutionContext) IsEnvVar(name string) bool {
	if c.logCollector != nil {
		return c.logCollector.IsEnvVar(name)
	}
	return false
}

// CreateVariableSnapshot 创建变量快照
func (c *ExecutionContext) CreateVariableSnapshot() {
	c.CreateVariableSnapshotWithEnvVars(nil)
}

// CreateVariableSnapshotWithEnvVars 创建变量快照（支持单独传入环境变量）
func (c *ExecutionContext) CreateVariableSnapshotWithEnvVars(envVars map[string]interface{}) {
	if c.logCollector == nil {
		return
	}

	c.mu.RLock()
	tempVars := make(map[string]any)
	envVarsAny := make(map[string]any)

	// 分离临时变量和环境变量
	for k, v := range c.Variables {
		if c.logCollector.IsEnvVar(k) {
			envVarsAny[k] = v
		} else {
			tempVars[k] = v
		}
	}
	c.mu.RUnlock()

	// 合并传入的环境变量
	if envVars != nil {
		for k, v := range envVars {
			envVarsAny[k] = v
		}
	}

	// 直接创建快照条目
	c.logCollector.AppendLog(types.NewSnapshotEntry(types.VariableSnapshotInfo{
		EnvVars:  envVarsAny,
		TempVars: tempVars,
	}))
}

// MergeLogsFrom 从另一个上下文合并日志
func (c *ExecutionContext) MergeLogsFrom(other *ExecutionContext) {
	if c.logCollector != nil && other != nil && other.logCollector != nil {
		c.logCollector.MergeFrom(other.logCollector)
	}
}

// ========== 表达式求值上下文转换 ==========

// BuildEvaluationContext 将 ExecutionContext 转换为表达式求值上下文。
// 这是一个公共方法，用于消除 ConditionExecutor 和 LoopExecutor 中的重复代码。
func (c *ExecutionContext) BuildEvaluationContext() *expression.EvaluationContext {
	c.mu.RLock()
	defer c.mu.RUnlock()

	evalCtx := expression.NewEvaluationContext()

	// 复制变量
	for k, v := range c.Variables {
		evalCtx.Set(k, v)
	}

	// 将步骤结果转换为求值上下文格式
	for stepID, result := range c.Results {
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

			// 特殊处理 HTTPResponseData
			if httpResp, ok := result.Output.(*types.HTTPResponseData); ok {
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

// ========== 执行器获取辅助函数 ==========

// GetExecutorFromRegistry 从注册表获取执行器。
// 如果提供了自定义注册表则使用，否则使用默认注册表。
// 这是一个公共函数，用于消除各执行器中的重复代码。
func GetExecutorFromRegistry(execType string, customRegistry *Registry) (Executor, error) {
	if customRegistry != nil {
		return customRegistry.GetOrError(execType)
	}
	return DefaultRegistry.GetOrError(execType)
}

// ========== 嵌套执行器基类 ==========

// NestedExecutorBase 为需要执行子步骤的执行器提供公共功能。
// 适用于 ConditionExecutor、LoopExecutor 等需要执行嵌套步骤的执行器。
type NestedExecutorBase struct {
	*BaseExecutor
	evaluator expression.ExpressionEvaluator
	registry  *Registry
}

// NewNestedExecutorBase 创建嵌套执行器基类。
func NewNestedExecutorBase(execType string) *NestedExecutorBase {
	return &NestedExecutorBase{
		BaseExecutor: NewBaseExecutor(execType),
		evaluator:    expression.NewEvaluator(),
	}
}

// NewNestedExecutorBaseWithRegistry 使用自定义注册表创建嵌套执行器基类。
func NewNestedExecutorBaseWithRegistry(execType string, registry *Registry) *NestedExecutorBase {
	return &NestedExecutorBase{
		BaseExecutor: NewBaseExecutor(execType),
		evaluator:    expression.NewEvaluator(),
		registry:     registry,
	}
}

// SetRegistry 设置自定义注册表。
func (n *NestedExecutorBase) SetRegistry(registry *Registry) {
	n.registry = registry
}

// GetEvaluator 获取表达式求值器。
func (n *NestedExecutorBase) GetEvaluator() expression.ExpressionEvaluator {
	return n.evaluator
}

// GetExecutor 获取指定类型的执行器。
func (n *NestedExecutorBase) GetExecutor(execType string) (Executor, error) {
	return GetExecutorFromRegistry(execType, n.registry)
}

// EvaluateCondition 评估条件表达式。
func (n *NestedExecutorBase) EvaluateCondition(condition string, execCtx *ExecutionContext) (bool, error) {
	evalCtx := execCtx.BuildEvaluationContext()
	return n.evaluator.EvaluateString(condition, evalCtx)
}

// ExecuteNestedSteps 执行嵌套的步骤列表。
// 这是一个通用的嵌套步骤执行方法，适用于条件分支、循环体等场景。
//
// 参数:
//   - ctx: 上下文
//   - steps: 要执行的步骤列表
//   - execCtx: 执行上下文
//   - parentID: 父步骤ID
//   - iteration: 迭代次数（用于循环场景，条件分支传 0）
//
// 返回:
//   - 执行结果列表
//   - 错误（如果需要中断执行）
func (n *NestedExecutorBase) ExecuteNestedSteps(ctx context.Context, steps []types.Step, execCtx *ExecutionContext, parentID string, iteration int) ([]*types.StepResult, error) {
	results := make([]*types.StepResult, 0, len(steps))

	// 获取回调
	callback := execCtx.GetCallback()

	for i := range steps {
		step := &steps[i]

		// 检查上下文取消
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		// 跳过禁用的步骤
		if step.Disabled {
			if callback != nil {
				callback.OnStepSkipped(ctx, step, "步骤已禁用", parentID, iteration)
			}
			continue
		}

		// 发送步骤开始事件
		if callback != nil {
			callback.OnStepStart(ctx, step, parentID, iteration)
		}

		// 获取步骤类型的执行器
		executor, err := n.GetExecutor(step.Type)
		if err != nil {
			if callback != nil {
				callback.OnStepFailed(ctx, step, err, 0, parentID, iteration)
			}
			return results, err
		}

		// 执行步骤
		result, err := executor.Execute(ctx, step, execCtx)
		if err != nil {
			if callback != nil {
				callback.OnStepFailed(ctx, step, err, 0, parentID, iteration)
			}
			return results, err
		}

		// 将结果存储到上下文中供后续步骤使用
		execCtx.SetResult(step.ID, result)
		results = append(results, result)

		// 发送步骤完成/失败事件
		if callback != nil {
			if result.Status == types.ResultStatusSuccess {
				callback.OnStepComplete(ctx, step, result, parentID, iteration)
			} else {
				callback.OnStepFailed(ctx, step, result.Error, result.Duration, parentID, iteration)
			}
		}

		// 处理错误策略
		if result.Status == types.ResultStatusFailed || result.Status == types.ResultStatusTimeout {
			switch step.OnError {
			case types.ErrorStrategyAbort, "":
				// 默认是中止
				return results, result.Error
			case types.ErrorStrategyContinue:
				// 继续下一步
			case types.ErrorStrategySkip:
				// 跳过剩余步骤
				return results, nil
			}
		}
	}

	return results, nil
}
