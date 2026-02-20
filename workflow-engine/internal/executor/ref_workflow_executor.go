package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

const (
	RefWorkflowExecutorType = "ref_workflow"
)

// RefWorkflowExecutor 执行引用工作流步骤。
// 从 config 中获取完整的子工作流定义，创建独立的执行上下文，
// 注入参数后执行子工作流的步骤，最后将指定结果映射回父上下文。
type RefWorkflowExecutor struct {
	*NestedExecutorBase
}

func NewRefWorkflowExecutor() *RefWorkflowExecutor {
	return &RefWorkflowExecutor{
		NestedExecutorBase: NewNestedExecutorBase(RefWorkflowExecutorType),
	}
}

func (e *RefWorkflowExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

// RefWorkflowOutput 引用工作流执行输出
type RefWorkflowOutput struct {
	WorkflowID   any            `json:"workflow_id"`
	WorkflowName string         `json:"workflow_name"`
	StepCount    int            `json:"step_count"`
	StepsExecuted []string      `json:"steps_executed"`
	Outputs      map[string]any `json:"outputs,omitempty"`
}

func (e *RefWorkflowExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	if step.Config == nil {
		return CreateFailedResult(step.ID, startTime,
			NewConfigError("引用工作流步骤缺少配置", nil)), nil
	}

	// 解析子工作流定义
	wfDef, err := e.parseWorkflowDefinition(step.Config)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// 创建独立的子执行上下文
	childCtx := NewExecutionContext()
	childCtx.WithWorkflowID(execCtx.WorkflowID)
	childCtx.WithExecutionID(execCtx.ExecutionID)
	childCtx.WithCallback(execCtx.GetCallback())

	// 注入子工作流自身的 variables
	if wfDef.Variables != nil {
		for k, v := range wfDef.Variables {
			childCtx.SetVariable(k, v)
		}
	}

	// 解析并注入参数映射（父上下文变量 -> 子上下文变量）
	if params, ok := step.Config["params"].(map[string]any); ok && len(params) > 0 {
		evalCtx := execCtx.ToEvaluationContext()
		resolver := GetVariableResolver()
		for paramName, paramValue := range params {
			if s, ok := paramValue.(string); ok {
				childCtx.SetVariable(paramName, resolver.ResolveString(s, evalCtx))
			} else {
				childCtx.SetVariable(paramName, paramValue)
			}
		}
	}

	// 执行子工作流步骤
	stepResults, execErr := e.ExecuteNestedSteps(ctx, wfDef.Steps, childCtx, step.ID, 0)

	// 收集已执行步骤 ID
	stepsExecuted := make([]string, 0, len(stepResults))
	for _, r := range stepResults {
		stepsExecuted = append(stepsExecuted, r.StepID)
	}

	// 将指定输出变量映射回父上下文
	outputs := make(map[string]any)
	if outputMappings, ok := step.Config["outputs"].(map[string]any); ok {
		for parentVar, childVarRaw := range outputMappings {
			if childVarName, ok := childVarRaw.(string); ok {
				if val, found := childCtx.GetVariable(childVarName); found {
					outputs[parentVar] = val
					execCtx.SetVariable(parentVar, val)
				}
			}
		}
	}

	// 合并子上下文日志
	execCtx.MergeLogsFrom(childCtx)

	output := &RefWorkflowOutput{
		WorkflowID:    step.Config["workflow_id"],
		WorkflowName:  stringFromConfig(step.Config, "workflow_name"),
		StepCount:     len(wfDef.Steps),
		StepsExecuted: stepsExecuted,
		Outputs:       outputs,
	}

	if execErr != nil {
		failedResult := CreateFailedResult(step.ID, startTime, execErr)
		failedResult.Output = output
		return failedResult, nil
	}

	// 检查子步骤是否有失败
	for _, r := range stepResults {
		if r.Status == types.ResultStatusFailed || r.Status == types.ResultStatusTimeout {
			failedResult := CreateFailedResult(step.ID, startTime, r.Error)
			failedResult.Output = output
			return failedResult, nil
		}
	}

	result := CreateSuccessResult(step.ID, startTime, output)
	result.Metrics["ref_workflow_step_count"] = float64(len(wfDef.Steps))
	result.Metrics["ref_workflow_steps_executed"] = float64(len(stepsExecuted))
	return result, nil
}

// subWorkflowDef 子工作流定义（从 config 中解析）
type subWorkflowDef struct {
	Variables map[string]any `json:"variables"`
	Steps     []types.Step   `json:"steps"`
}

func (e *RefWorkflowExecutor) parseWorkflowDefinition(config map[string]any) (*subWorkflowDef, error) {
	wfDefRaw, ok := config["workflow_definition"]
	if !ok {
		return nil, NewConfigError("引用工作流步骤缺少 workflow_definition", nil)
	}

	def := &subWorkflowDef{}

	switch v := wfDefRaw.(type) {
	case map[string]any:
		// 解析 variables
		if vars, ok := v["variables"].(map[string]any); ok {
			def.Variables = vars
		}
		// 解析 steps
		stepsRaw, ok := v["steps"]
		if !ok || stepsRaw == nil {
			return nil, NewConfigError("引用工作流定义中缺少 steps", nil)
		}
		stepsBytes, err := json.Marshal(stepsRaw)
		if err != nil {
			return nil, NewConfigError(fmt.Sprintf("序列化子工作流步骤失败: %v", err), err)
		}
		if err := json.Unmarshal(stepsBytes, &def.Steps); err != nil {
			return nil, NewConfigError(fmt.Sprintf("解析子工作流步骤失败: %v", err), err)
		}
	default:
		return nil, NewConfigError(fmt.Sprintf("workflow_definition 类型无效: %T", wfDefRaw), nil)
	}

	if len(def.Steps) == 0 {
		return nil, NewConfigError("引用工作流没有可执行的步骤", nil)
	}

	return def, nil
}

func stringFromConfig(config map[string]any, key string) string {
	if v, ok := config[key].(string); ok {
		return v
	}
	return ""
}

func (e *RefWorkflowExecutor) Cleanup(ctx context.Context) error {
	return nil
}

func init() {
	MustRegister(NewRefWorkflowExecutor())
}
