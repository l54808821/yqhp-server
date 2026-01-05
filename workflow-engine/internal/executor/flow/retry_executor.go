package flow

import (
	"context"
	"math"
	"time"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

const (
	// RetryExecutorType is the type identifier for retry executor.
	RetryExecutorType = "retry"

	// DefaultMaxAttempts 默认最大重试次数
	DefaultMaxAttempts = 3

	// DefaultRetryDelay 默认重试延迟
	DefaultRetryDelay = time.Second
)

// BackoffType 退避策略类型
type BackoffType string

const (
	BackoffFixed       BackoffType = "fixed"
	BackoffLinear      BackoffType = "linear"
	BackoffExponential BackoffType = "exponential"
)

// RetryConfig Retry 步骤配置
type RetryConfig struct {
	Steps       []types.Step  `yaml:"steps" json:"steps"`
	MaxAttempts int           `yaml:"max_attempts,omitempty" json:"max_attempts,omitempty"`
	Delay       time.Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	Backoff     BackoffType   `yaml:"backoff,omitempty" json:"backoff,omitempty"`
	MaxDelay    time.Duration `yaml:"max_delay,omitempty" json:"max_delay,omitempty"`
}

// RetryOutput Retry 步骤输出
type RetryOutput struct {
	Attempts     int             `json:"attempts"`
	Success      bool            `json:"success"`
	LastError    string          `json:"last_error,omitempty"`
	Delays       []time.Duration `json:"delays"`
	TerminatedBy string          `json:"terminated_by"` // success, max_attempts, error
}

// RetryExecutor executes steps with retry logic.
type RetryExecutor struct {
	stepExecutor StepExecutorFunc
}

// NewRetryExecutor creates a new retry executor.
func NewRetryExecutor(stepExecutor StepExecutorFunc) *RetryExecutor {
	return &RetryExecutor{
		stepExecutor: stepExecutor,
	}
}

// Execute executes steps with retry logic.
func (e *RetryExecutor) Execute(ctx context.Context, config *RetryConfig, execCtx *FlowExecutionContext) (*RetryOutput, error) {
	maxAttempts := config.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxAttempts
	}

	delay := config.Delay
	if delay <= 0 {
		delay = DefaultRetryDelay
	}

	backoff := config.Backoff
	if backoff == "" {
		backoff = BackoffFixed
	}

	output := &RetryOutput{
		Attempts: 0,
		Delays:   make([]time.Duration, 0),
	}

	var lastError error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		output.Attempts = attempt

		// 检查上下文取消
		select {
		case <-ctx.Done():
			output.TerminatedBy = "context_cancelled"
			return output, ctx.Err()
		default:
		}

		// 执行步骤
		success := true
		for i := range config.Steps {
			step := &config.Steps[i]

			result, err := e.stepExecutor(ctx, step, execCtx)
			if err != nil {
				success = false
				lastError = err
				break
			}

			execCtx.SetResult(step.ID, result)

			if result.Status == types.ResultStatusFailed || result.Status == types.ResultStatusTimeout {
				success = false
				lastError = result.Error
				break
			}
		}

		if success {
			output.Success = true
			output.TerminatedBy = "success"
			return output, nil
		}

		// 如果不是最后一次尝试，等待后重试
		if attempt < maxAttempts {
			waitDuration := CalculateBackoffDelay(delay, attempt, backoff, config.MaxDelay)
			output.Delays = append(output.Delays, waitDuration)

			select {
			case <-ctx.Done():
				output.TerminatedBy = "context_cancelled"
				return output, ctx.Err()
			case <-time.After(waitDuration):
				// 继续下一次尝试
			}
		}
	}

	output.Success = false
	output.TerminatedBy = "max_attempts"
	if lastError != nil {
		output.LastError = lastError.Error()
	}

	return output, lastError
}

// CalculateBackoffDelay 计算退避延迟
func CalculateBackoffDelay(baseDelay time.Duration, attempt int, backoff BackoffType, maxDelay time.Duration) time.Duration {
	var delay time.Duration

	switch backoff {
	case BackoffFixed:
		delay = baseDelay
	case BackoffLinear:
		delay = baseDelay * time.Duration(attempt)
	case BackoffExponential:
		delay = baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
	default:
		delay = baseDelay
	}

	// 应用最大延迟限制
	if maxDelay > 0 && delay > maxDelay {
		delay = maxDelay
	}

	return delay
}
