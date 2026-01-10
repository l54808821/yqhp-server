// Package types 定义工作流执行引擎的核心数据结构
package types

import (
	"context"
	"time"
)

// ExecutionCallback 定义执行过程中的回调接口
// 用于实时通知执行进度和结果
type ExecutionCallback interface {
	// OnStepStart 步骤开始执行时调用
	// step: 当前步骤
	// parentID: 父步骤ID（循环内的子步骤会有这个值）
	// iteration: 迭代次数（从1开始，非循环步骤为0）
	OnStepStart(ctx context.Context, step *Step, parentID string, iteration int)

	// OnStepComplete 步骤执行成功时调用
	// step: 当前步骤
	// result: 执行结果
	// parentID: 父步骤ID
	// iteration: 迭代次数
	OnStepComplete(ctx context.Context, step *Step, result *StepResult, parentID string, iteration int)

	// OnStepFailed 步骤执行失败时调用
	// step: 当前步骤
	// err: 错误信息
	// duration: 执行耗时
	// parentID: 父步骤ID
	// iteration: 迭代次数
	OnStepFailed(ctx context.Context, step *Step, err error, duration time.Duration, parentID string, iteration int)

	// OnProgress 进度更新时调用
	// current: 当前步骤序号
	// total: 总步骤数（动态执行时可能不准确）
	// stepName: 当前步骤名称
	OnProgress(ctx context.Context, current, total int, stepName string)

	// OnExecutionComplete 整个执行完成时调用
	// summary: 执行汇总
	OnExecutionComplete(ctx context.Context, summary *ExecutionSummary)
}

// ExecutionSummary 执行汇总
type ExecutionSummary struct {
	ExecutionID   string        `json:"execution_id"`
	TotalSteps    int           `json:"total_steps"`
	SuccessSteps  int           `json:"success_steps"`
	FailedSteps   int           `json:"failed_steps"`
	TotalDuration time.Duration `json:"total_duration"`
	Status        string        `json:"status"` // success, failed, timeout, stopped
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
}

// NoopCallback 空实现，用于不需要回调的场景
type NoopCallback struct{}

func (n *NoopCallback) OnStepStart(ctx context.Context, step *Step, parentID string, iteration int) {
}

func (n *NoopCallback) OnStepComplete(ctx context.Context, step *Step, result *StepResult, parentID string, iteration int) {
}

func (n *NoopCallback) OnStepFailed(ctx context.Context, step *Step, err error, duration time.Duration, parentID string, iteration int) {
}

func (n *NoopCallback) OnProgress(ctx context.Context, current, total int, stepName string) {}

func (n *NoopCallback) OnExecutionComplete(ctx context.Context, summary *ExecutionSummary) {}

// 确保 NoopCallback 实现了 ExecutionCallback 接口
var _ ExecutionCallback = (*NoopCallback)(nil)
