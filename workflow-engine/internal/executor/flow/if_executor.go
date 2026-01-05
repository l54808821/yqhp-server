// Package flow provides flow control executors for workflow engine v2.
package flow

import (
	"context"
	"time"

	"yqhp/workflow-engine/internal/expression"
	"yqhp/workflow-engine/pkg/types"
)

const (
	// IfExecutorType is the type identifier for if executor.
	IfExecutorType = "if"
)

// ElseIf 条件分支
type ElseIf struct {
	Condition string       `yaml:"condition" json:"condition"`
	Steps     []types.Step `yaml:"steps" json:"steps"`
}

// IfConfig If 步骤配置
type IfConfig struct {
	Condition string       `yaml:"condition" json:"condition"`
	Then      []types.Step `yaml:"then" json:"then"`
	ElseIf    []ElseIf     `yaml:"else_if,omitempty" json:"else_if,omitempty"`
	Else      []types.Step `yaml:"else,omitempty" json:"else,omitempty"`
}

// IfOutput If 步骤输出
type IfOutput struct {
	Condition     string   `json:"condition"`
	Result        bool     `json:"result"`
	BranchTaken   string   `json:"branch_taken"`
	BranchIndex   int      `json:"branch_index"`
	StepsExecuted []string `json:"steps_executed"`
}

// IfExecutor executes if-then-else-if-else conditional logic.
type IfExecutor struct {
	evaluator    expression.ExpressionEvaluator
	stepExecutor StepExecutorFunc
}

// StepExecutorFunc 步骤执行函数类型
type StepExecutorFunc func(ctx context.Context, step *types.Step, execCtx *FlowExecutionContext) (*types.StepResult, error)

// FlowExecutionContext 流程执行上下文
type FlowExecutionContext struct {
	Variables map[string]any
	Results   map[string]*types.StepResult
}

// NewFlowExecutionContext 创建新的流程执行上下文
func NewFlowExecutionContext() *FlowExecutionContext {
	return &FlowExecutionContext{
		Variables: make(map[string]any),
		Results:   make(map[string]*types.StepResult),
	}
}

// SetVariable 设置变量
func (c *FlowExecutionContext) SetVariable(name string, value any) {
	c.Variables[name] = value
}

// GetVariable 获取变量
func (c *FlowExecutionContext) GetVariable(name string) (any, bool) {
	val, ok := c.Variables[name]
	return val, ok
}

// SetResult 设置步骤结果
func (c *FlowExecutionContext) SetResult(stepID string, result *types.StepResult) {
	c.Results[stepID] = result
}

// NewIfExecutor creates a new if executor.
func NewIfExecutor(stepExecutor StepExecutorFunc) *IfExecutor {
	return &IfExecutor{
		evaluator:    expression.NewEvaluator(),
		stepExecutor: stepExecutor,
	}
}

// Execute executes an if step.
func (e *IfExecutor) Execute(ctx context.Context, config *IfConfig, execCtx *FlowExecutionContext) (*IfOutput, error) {
	startTime := time.Now()
	_ = startTime // 用于计时

	// 构建评估上下文
	evalCtx := e.buildEvaluationContext(execCtx)

	// 评估主条件
	result, err := e.evaluator.EvaluateString(config.Condition, evalCtx)
	if err != nil {
		return nil, err
	}

	output := &IfOutput{
		Condition:     config.Condition,
		Result:        result,
		StepsExecuted: make([]string, 0),
	}

	// 如果主条件为真，执行 then 分支
	if result {
		output.BranchTaken = "then"
		output.BranchIndex = 0
		err = e.executeBranch(ctx, config.Then, execCtx, output)
		return output, err
	}

	// 评估 else_if 分支
	for i, elseIf := range config.ElseIf {
		elseIfResult, err := e.evaluator.EvaluateString(elseIf.Condition, evalCtx)
		if err != nil {
			return nil, err
		}

		if elseIfResult {
			output.BranchTaken = "else_if"
			output.BranchIndex = i + 1
			output.Condition = elseIf.Condition
			output.Result = true
			err = e.executeBranch(ctx, elseIf.Steps, execCtx, output)
			return output, err
		}
	}

	// 执行 else 分支
	if len(config.Else) > 0 {
		output.BranchTaken = "else"
		output.BranchIndex = len(config.ElseIf) + 1
		output.Result = false
		err = e.executeBranch(ctx, config.Else, execCtx, output)
		return output, err
	}

	// 没有分支被执行
	output.BranchTaken = "none"
	output.BranchIndex = -1
	return output, nil
}

// executeBranch 执行分支步骤
func (e *IfExecutor) executeBranch(ctx context.Context, steps []types.Step, execCtx *FlowExecutionContext, output *IfOutput) error {
	for i := range steps {
		step := &steps[i]

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
func (e *IfExecutor) buildEvaluationContext(execCtx *FlowExecutionContext) *expression.EvaluationContext {
	evalCtx := expression.NewEvaluationContext()

	if execCtx == nil {
		return evalCtx
	}

	// 复制变量
	for k, v := range execCtx.Variables {
		evalCtx.Set(k, v)
	}

	// 转换步骤结果
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
