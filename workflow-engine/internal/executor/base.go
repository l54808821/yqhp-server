package executor

import (
	"context"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// BaseExecutor provides common functionality for executors.
type BaseExecutor struct {
	execType string
	config   map[string]any
}

// NewBaseExecutor creates a new BaseExecutor.
func NewBaseExecutor(execType string) *BaseExecutor {
	return &BaseExecutor{
		execType: execType,
		config:   make(map[string]any),
	}
}

// Type returns the executor type.
func (b *BaseExecutor) Type() string {
	return b.execType
}

// Init initializes the base executor.
func (b *BaseExecutor) Init(ctx context.Context, config map[string]any) error {
	b.config = config
	return nil
}

// Cleanup cleans up the base executor.
func (b *BaseExecutor) Cleanup(ctx context.Context) error {
	return nil
}

// GetConfig returns the executor configuration.
func (b *BaseExecutor) GetConfig() map[string]any {
	return b.config
}

// GetConfigString gets a string config value.
func (b *BaseExecutor) GetConfigString(key string, defaultVal string) string {
	if val, ok := b.config[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultVal
}

// GetConfigInt gets an int config value.
func (b *BaseExecutor) GetConfigInt(key string, defaultVal int) int {
	if val, ok := b.config[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return defaultVal
}

// GetConfigBool gets a bool config value.
func (b *BaseExecutor) GetConfigBool(key string, defaultVal bool) bool {
	if val, ok := b.config[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// GetConfigDuration gets a duration config value.
func (b *BaseExecutor) GetConfigDuration(key string, defaultVal time.Duration) time.Duration {
	if val, ok := b.config[key]; ok {
		switch v := val.(type) {
		case time.Duration:
			return v
		case string:
			if d, err := time.ParseDuration(v); err == nil {
				return d
			}
		case int:
			return time.Duration(v) * time.Millisecond
		case int64:
			return time.Duration(v) * time.Millisecond
		case float64:
			return time.Duration(v) * time.Millisecond
		}
	}
	return defaultVal
}

// CreateSuccessResult creates a successful step result.
func CreateSuccessResult(stepID string, startTime time.Time, output any) *types.StepResult {
	endTime := time.Now()
	return &types.StepResult{
		StepID:    stepID,
		Status:    types.ResultStatusSuccess,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  endTime.Sub(startTime),
		Output:    output,
		Metrics:   make(map[string]float64),
	}
}

// CreateFailedResult creates a failed step result.
func CreateFailedResult(stepID string, startTime time.Time, err error) *types.StepResult {
	endTime := time.Now()
	return &types.StepResult{
		StepID:    stepID,
		Status:    types.ResultStatusFailed,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  endTime.Sub(startTime),
		Error:     err,
		Metrics:   make(map[string]float64),
	}
}

// CreateTimeoutResult creates a timeout step result.
func CreateTimeoutResult(stepID string, startTime time.Time, timeout time.Duration) *types.StepResult {
	endTime := time.Now()
	return &types.StepResult{
		StepID:    stepID,
		Status:    types.ResultStatusTimeout,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  endTime.Sub(startTime),
		Error:     NewTimeoutError(stepID, timeout),
		Metrics:   make(map[string]float64),
	}
}

// CreateSkippedResult creates a skipped step result.
func CreateSkippedResult(stepID string) *types.StepResult {
	now := time.Now()
	return &types.StepResult{
		StepID:    stepID,
		Status:    types.ResultStatusSkipped,
		StartTime: now,
		EndTime:   now,
		Duration:  0,
		Metrics:   make(map[string]float64),
	}
}

// ExecuteWithTimeout executes a function with timeout.
func ExecuteWithTimeout(ctx context.Context, timeout time.Duration, fn func(ctx context.Context) error) error {
	if timeout <= 0 {
		return fn(ctx)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- fn(ctx)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return context.DeadlineExceeded
		}
		return ctx.Err()
	}
}
