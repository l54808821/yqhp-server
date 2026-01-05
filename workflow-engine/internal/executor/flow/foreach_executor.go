package flow

import (
	"context"
	"fmt"
	"reflect"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

const (
	// ForeachExecutorType is the type identifier for foreach executor.
	ForeachExecutorType = "foreach"
)

// ForeachConfig Foreach 步骤配置
type ForeachConfig struct {
	Label    string       `yaml:"label,omitempty" json:"label,omitempty"`
	Items    string       `yaml:"items" json:"items"`                             // 要遍历的列表表达式
	ItemVar  string       `yaml:"item_var" json:"item_var"`                       // 当前项变量名
	IndexVar string       `yaml:"index_var,omitempty" json:"index_var,omitempty"` // 索引变量名
	Steps    []types.Step `yaml:"steps" json:"steps"`
}

// ForeachOutput Foreach 步骤输出
type ForeachOutput struct {
	Iterations    int      `json:"iterations"`
	TotalItems    int      `json:"total_items"`
	TerminatedBy  string   `json:"terminated_by"` // completed, break, error
	StepsExecuted []string `json:"steps_executed"`
}

// ForeachExecutor executes foreach loop logic.
type ForeachExecutor struct {
	stepExecutor StepExecutorFunc
}

// NewForeachExecutor creates a new foreach executor.
func NewForeachExecutor(stepExecutor StepExecutorFunc) *ForeachExecutor {
	return &ForeachExecutor{
		stepExecutor: stepExecutor,
	}
}

// Execute executes a foreach loop.
func (e *ForeachExecutor) Execute(ctx context.Context, config *ForeachConfig, execCtx *FlowExecutionContext) (*ForeachOutput, error) {
	output := &ForeachOutput{
		Iterations:    0,
		StepsExecuted: make([]string, 0),
	}

	// 解析 items 表达式
	items, err := e.resolveItems(config.Items, execCtx)
	if err != nil {
		output.TerminatedBy = "error"
		return output, err
	}

	output.TotalItems = len(items)

	for i, item := range items {
		// 检查上下文取消
		select {
		case <-ctx.Done():
			output.TerminatedBy = "context_cancelled"
			return output, ctx.Err()
		default:
		}

		// 设置循环变量
		execCtx.SetVariable(config.ItemVar, item)
		if config.IndexVar != "" {
			execCtx.SetVariable(config.IndexVar, i)
		}

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

// resolveItems 解析 items 表达式
func (e *ForeachExecutor) resolveItems(itemsExpr string, execCtx *FlowExecutionContext) ([]any, error) {
	// 简单的变量引用解析 ${varName}
	if len(itemsExpr) > 3 && itemsExpr[0:2] == "${" && itemsExpr[len(itemsExpr)-1] == '}' {
		varName := itemsExpr[2 : len(itemsExpr)-1]
		if execCtx != nil {
			if val, ok := execCtx.GetVariable(varName); ok {
				return toSlice(val)
			}
		}
		return nil, fmt.Errorf("variable '%s' not found", varName)
	}

	// 如果不是变量引用，尝试直接使用
	return nil, fmt.Errorf("items expression must be a variable reference like ${varName}")
}

// toSlice 将任意值转换为切片
func toSlice(v any) ([]any, error) {
	if v == nil {
		return []any{}, nil
	}

	// 如果已经是 []any
	if slice, ok := v.([]any); ok {
		return slice, nil
	}

	// 使用反射处理其他切片类型
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, fmt.Errorf("items must be a slice or array, got %T", v)
	}

	result := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		result[i] = rv.Index(i).Interface()
	}
	return result, nil
}

// executeLoopBody 执行循环体
func (e *ForeachExecutor) executeLoopBody(ctx context.Context, steps []types.Step, execCtx *FlowExecutionContext, output *ForeachOutput, loopLabel string) error {
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
