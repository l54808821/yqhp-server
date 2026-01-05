package flow

import (
	"context"
	"errors"
	"fmt"

	"github.com/grafana/k6/workflow-engine/internal/expression"
	"github.com/grafana/k6/workflow-engine/pkg/types"
)

const (
	// WhileExecutorType is the type identifier for while executor.
	WhileExecutorType = "while"

	// DefaultMaxIterations 默认最大迭代次数
	DefaultMaxIterations = 1000
)

// ErrBreak 表示 break 信号
var ErrBreak = errors.New("break")

// ErrContinue 表示 continue 信号
var ErrContinue = errors.New("continue")

// BreakError 带标签的 break 错误
type BreakError struct {
	Label string
}

func (e *BreakError) Error() string {
	if e.Label == "" {
		return "break"
	}
	return fmt.Sprintf("break:%s", e.Label)
}

// ContinueError 带标签的 continue 错误
type ContinueError struct {
	Label string
}

func (e *ContinueError) Error() string {
	if e.Label == "" {
		return "continue"
	}
	return fmt.Sprintf("continue:%s", e.Label)
}

// WhileConfig While 步骤配置
type WhileConfig struct {
	Label         string       `yaml:"label,omitempty" json:"label,omitempty"`
	Condition     string       `yaml:"condition" json:"condition"`
	Steps         []types.Step `yaml:"steps" json:"steps"`
	MaxIterations int          `yaml:"max_iterations,omitempty" json:"max_iterations,omitempty"`
}

// WhileOutput While 步骤输出
type WhileOutput struct {
	Iterations    int      `json:"iterations"`
	TerminatedBy  string   `json:"terminated_by"` // condition, max_iterations, break, error
	StepsExecuted []string `json:"steps_executed"`
}

// WhileExecutor executes while loop logic.
type WhileExecutor struct {
	evaluator    expression.ExpressionEvaluator
	stepExecutor StepExecutorFunc
}

// NewWhileExecutor creates a new while executor.
func NewWhileExecutor(stepExecutor StepExecutorFunc) *WhileExecutor {
	return &WhileExecutor{
		evaluator:    expression.NewEvaluator(),
		stepExecutor: stepExecutor,
	}
}

// Execute executes a while loop.
func (e *WhileExecutor) Execute(ctx context.Context, config *WhileConfig, execCtx *FlowExecutionContext) (*WhileOutput, error) {
	maxIterations := config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = DefaultMaxIterations
	}

	output := &WhileOutput{
		Iterations:    0,
		StepsExecuted: make([]string, 0),
	}

	for i := 0; i < maxIterations; i++ {
		// 检查上下文取消
		select {
		case <-ctx.Done():
			output.TerminatedBy = "context_cancelled"
			return output, ctx.Err()
		default:
		}

		// 评估条件
		evalCtx := e.buildEvaluationContext(execCtx)
		result, err := e.evaluator.EvaluateString(config.Condition, evalCtx)
		if err != nil {
			output.TerminatedBy = "error"
			return output, err
		}

		// 条件为假，退出循环
		if !result {
			output.TerminatedBy = "condition"
			return output, nil
		}

		// 执行循环体
		err = e.executeLoopBody(ctx, config.Steps, execCtx, output, config.Label)
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

	output.TerminatedBy = "max_iterations"
	return output, nil
}

// executeLoopBody 执行循环体
func (e *WhileExecutor) executeLoopBody(ctx context.Context, steps []types.Step, execCtx *FlowExecutionContext, output *WhileOutput, loopLabel string) error {
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

// buildEvaluationContext 构建评估上下文
func (e *WhileExecutor) buildEvaluationContext(execCtx *FlowExecutionContext) *expression.EvaluationContext {
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
