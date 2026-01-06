// Package hook 提供工作流前置/后置钩子的执行框架。
package hook

import (
	"context"
	"fmt"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// HookType 表示钩子类型（前置或后置）。
type HookType string

const (
	// HookTypePre 表示前置执行钩子。
	HookTypePre HookType = "pre"
	// HookTypePost 表示后置执行钩子。
	HookTypePost HookType = "post"
)

// HookLevel 表示钩子定义的级别。
type HookLevel string

const (
	// HookLevelWorkflow 表示工作流级钩子。
	HookLevelWorkflow HookLevel = "workflow"
	// HookLevelStep 表示步骤级钩子。
	HookLevelStep HookLevel = "step"
)

// HookResult 包含钩子执行的结果。
type HookResult struct {
	HookType  HookType
	HookLevel HookLevel
	StepID    string // 工作流级钩子为空
	Status    types.ResultStatus
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Output    any
	Error     error
}

// HookError 表示钩子执行期间的错误。
type HookError struct {
	HookType  HookType
	HookLevel HookLevel
	StepID    string
	Message   string
	Cause     error
}

// Error 实现 error 接口。
func (e *HookError) Error() string {
	level := string(e.HookLevel)
	hookType := string(e.HookType)
	if e.StepID != "" {
		return fmt.Sprintf("[%s-%s 钩子，步骤 %s] %s: %v", level, hookType, e.StepID, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s-%s 钩子] %s: %v", level, hookType, e.Message, e.Cause)
}

// Unwrap 返回底层错误。
func (e *HookError) Unwrap() error {
	return e.Cause
}

// NewHookError 创建一个新的 HookError。
func NewHookError(hookType HookType, level HookLevel, stepID, message string, cause error) *HookError {
	return &HookError{
		HookType:  hookType,
		HookLevel: level,
		StepID:    stepID,
		Message:   message,
		Cause:     cause,
	}
}

// IsHookError 检查错误是否为 HookError。
func IsHookError(err error) bool {
	_, ok := err.(*HookError)
	return ok
}

// HookExecutor 使用适当的执行器执行钩子。
type HookExecutor struct {
	registry *executor.Registry
}

// NewHookExecutor 创建一个新的 HookExecutor。
func NewHookExecutor(registry *executor.Registry) *HookExecutor {
	if registry == nil {
		registry = executor.DefaultRegistry
	}
	return &HookExecutor{
		registry: registry,
	}
}

// ExecuteHook 执行单个钩子并返回结果。
func (h *HookExecutor) ExecuteHook(
	ctx context.Context,
	hook *types.Hook,
	hookType HookType,
	level HookLevel,
	stepID string,
	execCtx *executor.ExecutionContext,
) (*HookResult, error) {
	if hook == nil {
		return nil, nil
	}

	startTime := time.Now()

	// 获取钩子类型的执行器
	exec, err := h.registry.GetOrError(hook.Type)
	if err != nil {
		return &HookResult{
			HookType:  hookType,
			HookLevel: level,
			StepID:    stepID,
			Status:    types.ResultStatusFailed,
			StartTime: startTime,
			EndTime:   time.Now(),
			Duration:  time.Since(startTime),
			Error:     NewHookError(hookType, level, stepID, "执行器未找到", err),
		}, NewHookError(hookType, level, stepID, "执行器未找到", err)
	}

	// 从钩子创建步骤用于执行
	hookStep := &types.Step{
		ID:     h.generateHookStepID(hookType, level, stepID),
		Name:   fmt.Sprintf("%s-%s-hook", level, hookType),
		Type:   hook.Type,
		Config: hook.Config,
	}

	// 执行钩子
	result, err := exec.Execute(ctx, hookStep, execCtx)
	endTime := time.Now()

	hookResult := &HookResult{
		HookType:  hookType,
		HookLevel: level,
		StepID:    stepID,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  endTime.Sub(startTime),
	}

	if err != nil {
		hookResult.Status = types.ResultStatusFailed
		hookResult.Error = NewHookError(hookType, level, stepID, "钩子执行失败", err)
		return hookResult, hookResult.Error
	}

	if result != nil {
		hookResult.Status = result.Status
		hookResult.Output = result.Output
		if result.Error != nil {
			hookResult.Error = NewHookError(hookType, level, stepID, "钩子执行失败", result.Error)
		}
	}

	// 检查钩子是否失败
	if hookResult.Status == types.ResultStatusFailed || hookResult.Status == types.ResultStatusTimeout {
		if hookResult.Error == nil {
			hookResult.Error = NewHookError(hookType, level, stepID, "钩子执行失败", nil)
		}
		return hookResult, hookResult.Error
	}

	return hookResult, nil
}

// generateHookStepID 为钩子生成唯一的步骤 ID。
func (h *HookExecutor) generateHookStepID(hookType HookType, level HookLevel, stepID string) string {
	if stepID != "" {
		return fmt.Sprintf("__%s_%s_hook_%s", level, hookType, stepID)
	}
	return fmt.Sprintf("__%s_%s_hook", level, hookType)
}
