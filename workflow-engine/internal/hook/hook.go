// Package hook provides the hook execution framework for workflow pre/post hooks.
package hook

import (
	"context"
	"fmt"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// HookType represents the type of hook (pre or post).
type HookType string

const (
	// HookTypePre represents a pre-execution hook.
	HookTypePre HookType = "pre"
	// HookTypePost represents a post-execution hook.
	HookTypePost HookType = "post"
)

// HookLevel represents the level at which the hook is defined.
type HookLevel string

const (
	// HookLevelWorkflow represents a workflow-level hook.
	HookLevelWorkflow HookLevel = "workflow"
	// HookLevelStep represents a step-level hook.
	HookLevelStep HookLevel = "step"
)

// HookResult contains the result of a hook execution.
type HookResult struct {
	HookType  HookType
	HookLevel HookLevel
	StepID    string // Empty for workflow-level hooks
	Status    types.ResultStatus
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Output    any
	Error     error
}

// HookError represents an error during hook execution.
type HookError struct {
	HookType  HookType
	HookLevel HookLevel
	StepID    string
	Message   string
	Cause     error
}

// Error implements the error interface.
func (e *HookError) Error() string {
	level := string(e.HookLevel)
	hookType := string(e.HookType)
	if e.StepID != "" {
		return fmt.Sprintf("[%s-%s hook for step %s] %s: %v", level, hookType, e.StepID, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s-%s hook] %s: %v", level, hookType, e.Message, e.Cause)
}

// Unwrap returns the underlying error.
func (e *HookError) Unwrap() error {
	return e.Cause
}

// NewHookError creates a new HookError.
func NewHookError(hookType HookType, level HookLevel, stepID, message string, cause error) *HookError {
	return &HookError{
		HookType:  hookType,
		HookLevel: level,
		StepID:    stepID,
		Message:   message,
		Cause:     cause,
	}
}

// IsHookError checks if the error is a HookError.
func IsHookError(err error) bool {
	_, ok := err.(*HookError)
	return ok
}

// HookExecutor executes hooks using the appropriate executor.
type HookExecutor struct {
	registry *executor.Registry
}

// NewHookExecutor creates a new HookExecutor.
func NewHookExecutor(registry *executor.Registry) *HookExecutor {
	if registry == nil {
		registry = executor.DefaultRegistry
	}
	return &HookExecutor{
		registry: registry,
	}
}

// ExecuteHook executes a single hook and returns the result.
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

	// Get the executor for the hook type
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
			Error:     NewHookError(hookType, level, stepID, "executor not found", err),
		}, NewHookError(hookType, level, stepID, "executor not found", err)
	}

	// Create a step from the hook for execution
	hookStep := &types.Step{
		ID:     h.generateHookStepID(hookType, level, stepID),
		Name:   fmt.Sprintf("%s-%s-hook", level, hookType),
		Type:   hook.Type,
		Config: hook.Config,
	}

	// Execute the hook
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
		hookResult.Error = NewHookError(hookType, level, stepID, "hook execution failed", err)
		return hookResult, hookResult.Error
	}

	if result != nil {
		hookResult.Status = result.Status
		hookResult.Output = result.Output
		if result.Error != nil {
			hookResult.Error = NewHookError(hookType, level, stepID, "hook execution failed", result.Error)
		}
	}

	// Check if the hook failed
	if hookResult.Status == types.ResultStatusFailed || hookResult.Status == types.ResultStatusTimeout {
		if hookResult.Error == nil {
			hookResult.Error = NewHookError(hookType, level, stepID, "hook execution failed", nil)
		}
		return hookResult, hookResult.Error
	}

	return hookResult, nil
}

// generateHookStepID generates a unique step ID for a hook.
func (h *HookExecutor) generateHookStepID(hookType HookType, level HookLevel, stepID string) string {
	if stepID != "" {
		return fmt.Sprintf("__%s_%s_hook_%s", level, hookType, stepID)
	}
	return fmt.Sprintf("__%s_%s_hook", level, hookType)
}
