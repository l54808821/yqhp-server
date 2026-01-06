// Package executor 提供工作流步骤的脚本钩子执行功能。
package executor

import (
	"context"
	"fmt"

	"yqhp/workflow-engine/pkg/types"
)

// ScriptHook 脚本钩子定义
type ScriptHook struct {
	// 脚本名称
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// 脚本内容（内联）
	Script string `yaml:"script,omitempty" json:"script,omitempty"`

	// 脚本引用（调用已定义的脚本）
	Call string `yaml:"call,omitempty" json:"call,omitempty"`

	// 脚本参数
	Args map[string]any `yaml:"args,omitempty" json:"args,omitempty"`

	// 错误处理策略
	OnError OnErrorStrategy `yaml:"on_error,omitempty" json:"on_error,omitempty"`
}

// OnErrorStrategy 错误处理策略
type OnErrorStrategy string

const (
	// OnErrorContinue 继续执行
	OnErrorContinue OnErrorStrategy = "continue"
	// OnErrorAbort 中止执行
	OnErrorAbort OnErrorStrategy = "abort"
	// OnErrorRetry 重试
	OnErrorRetry OnErrorStrategy = "retry"
)

// StepHooks 步骤钩子配置
type StepHooks struct {
	// 前置脚本
	PreScripts []*ScriptHook `yaml:"pre_scripts,omitempty" json:"pre_scripts,omitempty"`

	// 后置脚本
	PostScripts []*ScriptHook `yaml:"post_scripts,omitempty" json:"post_scripts,omitempty"`
}

// HookExecutor 钩子执行器
type HookExecutor struct {
	scriptExecutor *ScriptExecutor
	callExecutor   *CallExecutor
}

// NewHookExecutor 创建钩子执行器
func NewHookExecutor(scriptExec *ScriptExecutor, callExec *CallExecutor) *HookExecutor {
	return &HookExecutor{
		scriptExecutor: scriptExec,
		callExecutor:   callExec,
	}
}

// ExecutePreScripts 执行前置脚本
func (h *HookExecutor) ExecutePreScripts(ctx context.Context, hooks *StepHooks, execCtx *ExecutionContext) error {
	if hooks == nil || len(hooks.PreScripts) == 0 {
		return nil
	}

	for i, hook := range hooks.PreScripts {
		if err := h.executeHook(ctx, hook, execCtx, fmt.Sprintf("pre_script_%d", i)); err != nil {
			if hook.OnError == OnErrorContinue {
				continue
			}
			return fmt.Errorf("pre_script[%d] failed: %w", i, err)
		}
	}

	return nil
}

// ExecutePostScripts 执行后置脚本
func (h *HookExecutor) ExecutePostScripts(ctx context.Context, hooks *StepHooks, execCtx *ExecutionContext, stepResult *types.StepResult) error {
	if hooks == nil || len(hooks.PostScripts) == 0 {
		return nil
	}

	// 将步骤结果添加到上下文
	if stepResult != nil {
		execCtx.SetVariable("step_result", map[string]any{
			"status":   string(stepResult.Status),
			"duration": stepResult.Duration.Milliseconds(),
			"output":   stepResult.Output,
			"error":    stepResult.Error,
		})
	}

	for i, hook := range hooks.PostScripts {
		if err := h.executeHook(ctx, hook, execCtx, fmt.Sprintf("post_script_%d", i)); err != nil {
			if hook.OnError == OnErrorContinue {
				continue
			}
			return fmt.Errorf("post_script[%d] failed: %w", i, err)
		}
	}

	return nil
}

// executeHook 执行单个钩子
func (h *HookExecutor) executeHook(ctx context.Context, hook *ScriptHook, execCtx *ExecutionContext, stepID string) error {
	if hook == nil {
		return nil
	}

	// 如果是脚本调用
	if hook.Call != "" {
		return h.executeCallHook(ctx, hook, execCtx, stepID)
	}

	// 如果是内联脚本
	if hook.Script != "" {
		return h.executeInlineScript(ctx, hook, execCtx, stepID)
	}

	return nil
}

// executeCallHook 执行脚本调用钩子
func (h *HookExecutor) executeCallHook(ctx context.Context, hook *ScriptHook, execCtx *ExecutionContext, stepID string) error {
	if h.callExecutor == nil {
		return fmt.Errorf("call executor not available")
	}

	step := &types.Step{
		ID:   stepID,
		Type: "call",
		Config: map[string]any{
			"script": hook.Call,
			"args":   hook.Args,
		},
	}

	_, err := h.callExecutor.Execute(ctx, step, execCtx)
	return err
}

// executeInlineScript 执行内联脚本
func (h *HookExecutor) executeInlineScript(ctx context.Context, hook *ScriptHook, execCtx *ExecutionContext, stepID string) error {
	if h.scriptExecutor == nil {
		return fmt.Errorf("script executor not available")
	}

	step := &types.Step{
		ID:   stepID,
		Type: "script",
		Config: map[string]any{
			"script": hook.Script,
		},
	}

	_, err := h.scriptExecutor.Execute(ctx, step, execCtx)
	return err
}

// HookResult 钩子执行结果
type HookResult struct {
	// 是否成功
	Success bool

	// 错误信息
	Error error

	// 执行的钩子数量
	ExecutedCount int

	// 失败的钩子数量
	FailedCount int
}

// ExecutePreScriptsWithResult 执行前置脚本并返回详细结果
func (h *HookExecutor) ExecutePreScriptsWithResult(ctx context.Context, hooks *StepHooks, execCtx *ExecutionContext) *HookResult {
	result := &HookResult{Success: true}

	if hooks == nil || len(hooks.PreScripts) == 0 {
		return result
	}

	for i, hook := range hooks.PreScripts {
		result.ExecutedCount++
		if err := h.executeHook(ctx, hook, execCtx, fmt.Sprintf("pre_script_%d", i)); err != nil {
			result.FailedCount++
			if hook.OnError != OnErrorContinue {
				result.Success = false
				result.Error = err
				return result
			}
		}
	}

	return result
}

// ExecutePostScriptsWithResult 执行后置脚本并返回详细结果
func (h *HookExecutor) ExecutePostScriptsWithResult(ctx context.Context, hooks *StepHooks, execCtx *ExecutionContext, stepResult *types.StepResult) *HookResult {
	result := &HookResult{Success: true}

	if hooks == nil || len(hooks.PostScripts) == 0 {
		return result
	}

	// 将步骤结果添加到上下文
	if stepResult != nil {
		execCtx.SetVariable("step_result", map[string]any{
			"status":   string(stepResult.Status),
			"duration": stepResult.Duration.Milliseconds(),
			"output":   stepResult.Output,
			"error":    stepResult.Error,
		})
	}

	for i, hook := range hooks.PostScripts {
		result.ExecutedCount++
		if err := h.executeHook(ctx, hook, execCtx, fmt.Sprintf("post_script_%d", i)); err != nil {
			result.FailedCount++
			if hook.OnError != OnErrorContinue {
				result.Success = false
				result.Error = err
				return result
			}
		}
	}

	return result
}
