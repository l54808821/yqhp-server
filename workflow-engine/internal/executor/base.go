package executor

import (
	"context"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// BaseExecutor 为执行器提供通用功能。
type BaseExecutor struct {
	execType string
	config   map[string]any
}

// NewBaseExecutor 创建一个新的 BaseExecutor。
func NewBaseExecutor(execType string) *BaseExecutor {
	return &BaseExecutor{
		execType: execType,
		config:   make(map[string]any),
	}
}

// Type 返回执行器类型。
func (b *BaseExecutor) Type() string {
	return b.execType
}

// Init 初始化基础执行器。
func (b *BaseExecutor) Init(ctx context.Context, config map[string]any) error {
	b.config = config
	return nil
}

// Cleanup 清理基础执行器。
func (b *BaseExecutor) Cleanup(ctx context.Context) error {
	return nil
}

// GetConfig 返回执行器配置。
func (b *BaseExecutor) GetConfig() map[string]any {
	return b.config
}

// GetConfigString 获取字符串类型的配置值。
func (b *BaseExecutor) GetConfigString(key string, defaultVal string) string {
	if val, ok := b.config[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultVal
}

// GetConfigInt 获取整数类型的配置值。
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

// GetConfigBool 获取布尔类型的配置值。
func (b *BaseExecutor) GetConfigBool(key string, defaultVal bool) bool {
	if val, ok := b.config[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// GetConfigDuration 获取时间间隔类型的配置值。
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

// CreateSuccessResult 创建成功的步骤结果。
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

// CreateFailedResult 创建失败的步骤结果。
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

// CreateTimeoutResult 创建超时的步骤结果。
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

// CreateSkippedResult 创建跳过的步骤结果。
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

// ExecuteWithTimeout 带超时执行函数。
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
