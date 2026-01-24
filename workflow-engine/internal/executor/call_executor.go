package executor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/script"
	"yqhp/workflow-engine/pkg/types"
)

const (
	// CallExecutorType 是调用执行器的类型标识符。
	CallExecutorType = "call"
)

// CallExecutor 执行脚本片段调用。
type CallExecutor struct {
	*BaseExecutor
	registry  *script.Registry
	callStack *script.CallStack
}

// NewCallExecutor 创建一个新的调用执行器。
func NewCallExecutor() *CallExecutor {
	return &CallExecutor{
		BaseExecutor: NewBaseExecutor(CallExecutorType),
		registry:     script.NewRegistry(),
		callStack:    script.NewCallStack(),
	}
}

// SetRegistry 设置脚本注册表
func (e *CallExecutor) SetRegistry(registry *script.Registry) {
	e.registry = registry
}

// GetRegistry 获取脚本注册表
func (e *CallExecutor) GetRegistry() *script.Registry {
	return e.registry
}

// SetCallStack 设置调用栈
func (e *CallExecutor) SetCallStack(stack *script.CallStack) {
	e.callStack = stack
}

// GetCallStack 获取调用栈
func (e *CallExecutor) GetCallStack() *script.CallStack {
	return e.callStack
}

// Init 使用配置初始化调用执行器。
func (e *CallExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

// Execute 执行脚本调用步骤。
func (e *CallExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// 解析调用配置
	callConfig, err := e.parseCallConfig(step.Config)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// 变量解析
	if execCtx != nil {
		evalCtx := execCtx.ToEvaluationContext()
		callConfig.Script = resolveString(callConfig.Script, evalCtx)
		// 解析参数中的变量
		for k, v := range callConfig.Params {
			if s, ok := v.(string); ok {
				callConfig.Params[k] = resolveString(s, evalCtx)
			}
		}
	}

	// 获取脚本
	fragment, err := e.registry.Get(callConfig.Script)
	if err != nil {
		return CreateFailedResult(step.ID, startTime,
			NewConfigError(fmt.Sprintf("未找到脚本: %s", callConfig.Script), err)), nil
	}

	// 检查循环调用
	if err := e.callStack.Push(callConfig.Script); err != nil {
		return CreateFailedResult(step.ID, startTime,
			NewExecutionError(step.ID, "检测到循环调用", err)), nil
	}
	defer e.callStack.Pop()

	// 验证参数
	if err := fragment.ValidateParams(callConfig.Params); err != nil {
		return CreateFailedResult(step.ID, startTime,
			NewConfigError("参数验证失败", err)), nil
	}

	// 解析参数（应用默认值）
	resolvedParams := fragment.ResolveParams(callConfig.Params)

	// 创建脚本执行上下文
	scriptCtx := e.createScriptContext(execCtx, resolvedParams)

	// 执行脚本步骤
	scriptResult, err := e.executeScriptSteps(ctx, fragment, scriptCtx)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// 处理返回值
	returns := e.processReturns(fragment, scriptCtx, callConfig.Results)

	// 将返回值写入执行上下文
	if execCtx != nil {
		for k, v := range returns {
			execCtx.SetVariable(k, v)
		}
	}

	result := &CallResult{
		Success:   true,
		Script:    callConfig.Script,
		Returns:   returns,
		StepCount: len(fragment.Steps),
		Duration:  time.Since(startTime),
	}

	// 如果脚本执行有错误，标记为失败
	if scriptResult != nil && scriptResult.Status == types.ResultStatusFailed {
		result.Success = false
		if scriptResult.Error != nil {
			result.Error = scriptResult.Error.Error()
		}
	}

	return CreateSuccessResult(step.ID, startTime, result), nil
}

// CallResult 脚本调用结果
type CallResult struct {
	Success   bool           `json:"success"`
	Script    string         `json:"script"`
	Returns   map[string]any `json:"returns,omitempty"`
	StepCount int            `json:"step_count"`
	Duration  time.Duration  `json:"duration"`
	Error     string         `json:"error,omitempty"`
}

// parseCallConfig 解析调用配置
func (e *CallExecutor) parseCallConfig(config map[string]any) (*script.CallConfig, error) {
	callConfig := &script.CallConfig{
		Params:  make(map[string]any),
		Results: make(map[string]string),
	}

	// 解析脚本名称
	if scriptName, ok := config["script"].(string); ok {
		callConfig.Script = scriptName
	} else {
		return nil, NewConfigError("调用步骤需要配置 'script'（脚本路径）", nil)
	}

	// 解析参数
	if params, ok := config["params"].(map[string]any); ok {
		callConfig.Params = params
	}

	// 解析返回值映射
	if results, ok := config["results"].(map[string]any); ok {
		for k, v := range results {
			if s, ok := v.(string); ok {
				callConfig.Results[k] = s
			}
		}
	}

	return callConfig, nil
}

// createScriptContext 创建脚本执行上下文
func (e *CallExecutor) createScriptContext(parentCtx *ExecutionContext, params map[string]any) *ExecutionContext {
	scriptCtx := NewExecutionContext()

	// 复制父上下文的变量
	if parentCtx != nil {
		for k, v := range parentCtx.Variables {
			scriptCtx.SetVariable(k, v)
		}
	}

	// 设置参数为变量
	for k, v := range params {
		scriptCtx.SetVariable(k, v)
	}

	return scriptCtx
}

// executeScriptSteps 执行脚本步骤
func (e *CallExecutor) executeScriptSteps(ctx context.Context, fragment *script.Fragment, scriptCtx *ExecutionContext) (*types.StepResult, error) {
	// 这里简化实现，实际应该调用工作流引擎执行步骤
	// 在完整实现中，需要将步骤转换为 types.Step 并执行

	// 目前返回成功，步骤执行逻辑将在工作流引擎中实现
	return &types.StepResult{
		Status: types.ResultStatusSuccess,
	}, nil
}

// processReturns 处理返回值
func (e *CallExecutor) processReturns(fragment *script.Fragment, scriptCtx *ExecutionContext, resultMapping map[string]string) map[string]any {
	returns := make(map[string]any)

	// 获取脚本定义的返回值
	for _, ret := range fragment.Returns {
		// 解析返回值表达式
		value := e.resolveReturnValue(ret.Value, scriptCtx)

		// 检查是否有映射
		if mappedName, ok := resultMapping[ret.Name]; ok {
			returns[mappedName] = value
		}
	}

	return returns
}

// resolveReturnValue 解析返回值表达式
func (e *CallExecutor) resolveReturnValue(expr string, scriptCtx *ExecutionContext) any {
	// 简单的变量引用解析 ${varName}
	if strings.HasPrefix(expr, "${") && strings.HasSuffix(expr, "}") {
		varName := expr[2 : len(expr)-1]
		if val, ok := scriptCtx.GetVariable(varName); ok {
			return val
		}
	}
	return expr
}

// Cleanup 释放调用执行器持有的资源。
func (e *CallExecutor) Cleanup(ctx context.Context) error {
	return nil
}

// init 在默认注册表中注册调用执行器。
func init() {
	MustRegister(NewCallExecutor())
}
