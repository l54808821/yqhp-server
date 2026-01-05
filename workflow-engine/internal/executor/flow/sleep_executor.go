package flow

import (
	"context"
	"time"

	"github.com/grafana/k6/workflow-engine/internal/expression"
)

const (
	// SleepExecutorType is the type identifier for sleep executor.
	SleepExecutorType = "sleep"

	// WaitUntilExecutorType is the type identifier for wait_until executor.
	WaitUntilExecutorType = "wait_until"

	// DefaultWaitInterval 默认等待间隔
	DefaultWaitInterval = time.Second

	// DefaultWaitTimeout 默认等待超时
	DefaultWaitTimeout = 30 * time.Second
)

// SleepConfig Sleep 步骤配置
type SleepConfig struct {
	Duration time.Duration `yaml:"duration" json:"duration"`
}

// SleepOutput Sleep 步骤输出
type SleepOutput struct {
	Duration time.Duration `json:"duration"`
	Actual   time.Duration `json:"actual"`
}

// SleepExecutor executes sleep/delay logic.
type SleepExecutor struct{}

// NewSleepExecutor creates a new sleep executor.
func NewSleepExecutor() *SleepExecutor {
	return &SleepExecutor{}
}

// Execute executes a sleep step.
func (e *SleepExecutor) Execute(ctx context.Context, config *SleepConfig) (*SleepOutput, error) {
	startTime := time.Now()

	select {
	case <-ctx.Done():
		return &SleepOutput{
			Duration: config.Duration,
			Actual:   time.Since(startTime),
		}, ctx.Err()
	case <-time.After(config.Duration):
		return &SleepOutput{
			Duration: config.Duration,
			Actual:   time.Since(startTime),
		}, nil
	}
}

// WaitUntilConfig WaitUntil 步骤配置
type WaitUntilConfig struct {
	Condition string        `yaml:"condition" json:"condition"`
	Timeout   time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Interval  time.Duration `yaml:"interval,omitempty" json:"interval,omitempty"`
}

// WaitUntilOutput WaitUntil 步骤输出
type WaitUntilOutput struct {
	ConditionMet bool          `json:"condition_met"`
	Attempts     int           `json:"attempts"`
	Duration     time.Duration `json:"duration"`
	TerminatedBy string        `json:"terminated_by"` // condition_met, timeout, error
}

// WaitUntilExecutor executes wait_until logic.
type WaitUntilExecutor struct {
	evaluator expression.ExpressionEvaluator
}

// NewWaitUntilExecutor creates a new wait_until executor.
func NewWaitUntilExecutor() *WaitUntilExecutor {
	return &WaitUntilExecutor{
		evaluator: expression.NewEvaluator(),
	}
}

// Execute executes a wait_until step.
func (e *WaitUntilExecutor) Execute(ctx context.Context, config *WaitUntilConfig, execCtx *FlowExecutionContext) (*WaitUntilOutput, error) {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = DefaultWaitTimeout
	}

	interval := config.Interval
	if interval <= 0 {
		interval = DefaultWaitInterval
	}

	startTime := time.Now()
	deadline := startTime.Add(timeout)

	output := &WaitUntilOutput{
		Attempts: 0,
	}

	for {
		output.Attempts++

		// 检查上下文取消
		select {
		case <-ctx.Done():
			output.Duration = time.Since(startTime)
			output.TerminatedBy = "context_cancelled"
			return output, ctx.Err()
		default:
		}

		// 评估条件
		evalCtx := e.buildEvaluationContext(execCtx)
		result, err := e.evaluator.EvaluateString(config.Condition, evalCtx)
		if err != nil {
			output.Duration = time.Since(startTime)
			output.TerminatedBy = "error"
			return output, err
		}

		if result {
			output.ConditionMet = true
			output.Duration = time.Since(startTime)
			output.TerminatedBy = "condition_met"
			return output, nil
		}

		// 检查超时
		if time.Now().After(deadline) {
			output.Duration = time.Since(startTime)
			output.TerminatedBy = "timeout"
			return output, nil
		}

		// 等待间隔
		select {
		case <-ctx.Done():
			output.Duration = time.Since(startTime)
			output.TerminatedBy = "context_cancelled"
			return output, ctx.Err()
		case <-time.After(interval):
			// 继续下一次检查
		}
	}
}

// buildEvaluationContext 构建评估上下文
func (e *WaitUntilExecutor) buildEvaluationContext(execCtx *FlowExecutionContext) *expression.EvaluationContext {
	evalCtx := expression.NewEvaluationContext()

	if execCtx == nil {
		return evalCtx
	}

	for k, v := range execCtx.Variables {
		evalCtx.Set(k, v)
	}

	for stepID, result := range execCtx.Results {
		resultMap := map[string]any{
			"status":   string(result.Status),
			"duration": result.Duration.Milliseconds(),
			"step_id":  result.StepID,
		}
		if result.Output != nil {
			resultMap["output"] = result.Output
		}
		if result.Error != nil {
			resultMap["error"] = result.Error.Error()
		}
		evalCtx.SetResult(stepID, resultMap)
	}

	return evalCtx
}
