package flow

import (
	"context"

	"yqhp/workflow-engine/pkg/types"
)

const (
	// ForExecutorType is the type identifier for for executor.
	ForExecutorType = "for"
)

// ForConfig For 步骤配置
type ForConfig struct {
	Label    string       `yaml:"label,omitempty" json:"label,omitempty"`
	Start    int          `yaml:"start" json:"start"`
	End      int          `yaml:"end" json:"end"`
	Step     int          `yaml:"step,omitempty" json:"step,omitempty"`
	IndexVar string       `yaml:"index_var" json:"index_var"`
	Steps    []types.Step `yaml:"steps" json:"steps"`
}

// ForOutput For 步骤输出
type ForOutput struct {
	Iterations    int      `json:"iterations"`
	StartValue    int      `json:"start_value"`
	EndValue      int      `json:"end_value"`
	StepValue     int      `json:"step_value"`
	TerminatedBy  string   `json:"terminated_by"` // completed, break, error
	StepsExecuted []string `json:"steps_executed"`
}

// ForExecutor executes for loop logic.
type ForExecutor struct {
	stepExecutor StepExecutorFunc
}

// NewForExecutor creates a new for executor.
func NewForExecutor(stepExecutor StepExecutorFunc) *ForExecutor {
	return &ForExecutor{
		stepExecutor: stepExecutor,
	}
}

// Execute executes a for loop.
func (e *ForExecutor) Execute(ctx context.Context, config *ForConfig, execCtx *FlowExecutionContext) (*ForOutput, error) {
	step := config.Step
	if step == 0 {
		step = 1
	}

	output := &ForOutput{
		Iterations:    0,
		StartValue:    config.Start,
		EndValue:      config.End,
		StepValue:     step,
		StepsExecuted: make([]string, 0),
	}

	// 确定循环方向
	ascending := step > 0

	for i := config.Start; ; i += step {
		// 检查终止条件
		if ascending && i > config.End {
			break
		}
		if !ascending && i < config.End {
			break
		}

		// 检查上下文取消
		select {
		case <-ctx.Done():
			output.TerminatedBy = "context_cancelled"
			return output, ctx.Err()
		default:
		}

		// 设置索引变量
		execCtx.SetVariable(config.IndexVar, i)

		// 执行循环体
		err := e.executeLoopBody(ctx, config.Steps, execCtx, output, config.Label)
		if err != nil {
			// 检查是否是 break
			if breakErr, ok := err.(*BreakError); ok {
				if breakErr.Label == "" || breakErr.Label == config.Label {
					output.TerminatedBy = "break"
					return output, nil
				}
				// 传播到外层循环
				return output, err
			}
			// 检查是否是 continue
			if continueErr, ok := err.(*ContinueError); ok {
				if continueErr.Label == "" || continueErr.Label == config.Label {
					output.Iterations++
					continue
				}
				// 传播到外层循环
				return output, err
			}
			// 其他错误
			output.TerminatedBy = "error"
			return output, err
		}

		output.Iterations++
	}

	output.TerminatedBy = "completed"
	return output, nil
}

// executeLoopBody 执行循环体
func (e *ForExecutor) executeLoopBody(ctx context.Context, steps []types.Step, execCtx *FlowExecutionContext, output *ForOutput, loopLabel string) error {
	for i := range steps {
		step := &steps[i]

		// 检查是否是 break/continue 步骤
		if step.Type == "break" {
			label := ""
			if labelVal, ok := step.Config["label"].(string); ok {
				label = labelVal
			}
			return &BreakError{Label: label}
		}
		if step.Type == "continue" {
			label := ""
			if labelVal, ok := step.Config["label"].(string); ok {
				label = labelVal
			}
			return &ContinueError{Label: label}
		}

		result, err := e.stepExecutor(ctx, step, execCtx)
		if err != nil {
			return err
		}

		// 存储结果
		execCtx.SetResult(step.ID, result)
		output.StepsExecuted = append(output.StepsExecuted, step.ID)

		// 检查失败
		if result.Status == types.ResultStatusFailed || result.Status == types.ResultStatusTimeout {
			return result.Error
		}
	}
	return nil
}
