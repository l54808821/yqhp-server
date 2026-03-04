package ai

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/pkg/types"
)

const createPlanToolName = "create_plan"

// PlanAgent 实现 规划 → 逐步执行 → 汇总 的 Agent 模式。
// 1. 让 LLM 通过 create_plan 工具输出结构化计划
// 2. 逐步执行每个步骤（每步使用 mini ReAct 循环）
// 3. 汇总所有步骤结果生成最终回答
type PlanAgent struct{}

func NewPlanAgent() *PlanAgent {
	return &PlanAgent{}
}

func (a *PlanAgent) Mode() AgentMode {
	return AgentModePlan
}

func (a *PlanAgent) Run(ctx context.Context, req *AgentRequest) (*AIOutput, error) {
	return a.RunWithReason(ctx, req, "用户指定 Plan 模式")
}

// RunWithReason 带原因说明的 Plan 执行（由 Router 调用时传入原因）
func (a *PlanAgent) RunWithReason(ctx context.Context, req *AgentRequest, reason string) (*AIOutput, error) {
	output := &AIOutput{
		Model: req.Config.Model,
		AgentTrace: &AgentTrace{
			Mode: string(AgentModePlan),
			Plan: &PlanTrace{Reason: reason},
		},
	}

	// 1. 规划阶段
	steps, err := a.planPhase(ctx, req, output)
	if err != nil {
		return nil, fmt.Errorf("规划阶段失败: %w", err)
	}

	tasks := stepsToTasks(steps)
	maxSteps := req.Config.MaxPlanSteps
	if maxSteps <= 0 {
		maxSteps = defaultMaxPlanSteps
	}
	if len(tasks) > maxSteps {
		tasks = tasks[:maxSteps]
	}

	output.AgentTrace.Plan.PlanText = formatPlanText(tasks)
	for i, t := range tasks {
		output.AgentTrace.Plan.Steps = append(output.AgentTrace.Plan.Steps, PlanStep{
			Index:  i + 1,
			Task:   t,
			Status: "pending",
		})
	}

	a.notifyPlanStarted(ctx, req, reason, tasks)

	// 2. 逐步执行
	stepResults := a.executeSteps(ctx, req, output, tasks)

	// 3. 汇总阶段
	if err := a.synthesisPhase(ctx, req, output, tasks, stepResults); err != nil {
		return nil, err
	}

	if req.Callbacks.Plan != nil {
		req.Callbacks.Plan.OnAIPlanCompleted(ctx, req.StepID, output.Content)
	}

	return output, nil
}

// planPhase 规划阶段：让 LLM 生成执行计划
func (a *PlanAgent) planPhase(ctx context.Context, req *AgentRequest, output *AIOutput) ([]PlanStepDef, error) {
	if req.Callbacks.Thinking != nil {
		req.Callbacks.Thinking.OnAIThinking(ctx, req.StepID, 0, "正在制定执行计划...")
	}

	planTool := createPlanToolInfo()
	planMessages := make([]*schema.Message, len(req.Messages))
	copy(planMessages, req.Messages)
	planMessages = append(planMessages, schema.UserMessage(buildPlanningPrompt(req.Config.Skills)))

	// 优先使用 tool_call 方式获取结构化输出
	resp, err := callLLM(ctx, req.ChatModel, planMessages, []*schema.ToolInfo{planTool}, req.Config, req.StepID, nil)
	if err != nil {
		return nil, err
	}
	updateTokenUsage(output, resp)

	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		output.AgentTrace.Plan.PlanningTokens = &TokenUsage{
			PromptTokens:     resp.ResponseMeta.Usage.PromptTokens,
			CompletionTokens: resp.ResponseMeta.Usage.CompletionTokens,
			TotalTokens:      resp.ResponseMeta.Usage.TotalTokens,
		}
	}

	// 尝试从 tool_call 解析
	if steps, ok := parsePlanFromToolCall(resp.ToolCalls); ok {
		return steps, nil
	}

	// 回退到文本解析
	return parsePlanFromText(resp.Content), nil
}

// executeSteps 逐步执行计划中的每个步骤
func (a *PlanAgent) executeSteps(ctx context.Context, req *AgentRequest, output *AIOutput, tasks []string) []string {
	historySummary := buildHistorySummary(req.Messages)

	var stepResults []string
	for i, task := range tasks {
		if ctx.Err() != nil {
			break
		}

		output.AgentTrace.Plan.Steps[i].Status = "running"
		a.notifyStepUpdate(ctx, req, i+1, "running", "")

		stepPrompt := buildPlanStepPrompt(req.Config.Prompt, tasks, i, stepResults)
		userContent := stepPrompt
		if historySummary != "" {
			userContent = historySummary + "\n" + stepPrompt
		}
		stepMessages := []*schema.Message{
			schema.SystemMessage(req.Config.SystemPrompt),
			schema.UserMessage(userContent),
		}

		stepToolDefs := filterToolDefsForStep(req.AllToolDefs, task, req.Config.Skills)
		stepSchemaTools := toSchemaTools(stepToolDefs)

		stepReq := &AgentRequest{
			Config:       req.Config,
			ChatModel:    req.ChatModel,
			Messages:     stepMessages,
			ToolRegistry: req.ToolRegistry,
			SchemaTools:  stepSchemaTools,
			AllToolDefs:  stepToolDefs,
			StepID:       req.StepID,
			ExecCtx:      req.ExecCtx,
			Callbacks:    req.Callbacks,
			MaxRounds:    5, // Plan 步骤内的 ReAct 轮次限制
		}

		stepOutput, stepErr := a.executeMiniReAct(ctx, stepReq, i+1)
		if stepErr != nil {
			output.AgentTrace.Plan.Steps[i].Status = "failed"
			output.AgentTrace.Plan.Steps[i].Result = stepErr.Error()
			stepResults = append(stepResults, fmt.Sprintf("步骤 %d 失败: %s", i+1, stepErr.Error()))
			a.notifyStepUpdate(ctx, req, i+1, "failed", stepErr.Error())
			continue
		}

		mergeTokenUsage(output, stepOutput)
		output.AgentTrace.Plan.Steps[i].Status = "completed"
		output.AgentTrace.Plan.Steps[i].Result = stepOutput.Content
		output.AgentTrace.Plan.Steps[i].ToolCalls = stepOutput.ToolCalls
		output.AgentTrace.Plan.Steps[i].PromptTokens = stepOutput.PromptTokens
		output.AgentTrace.Plan.Steps[i].CompletionTokens = stepOutput.CompletionTokens
		output.AgentTrace.Plan.Steps[i].TotalTokens = stepOutput.TotalTokens

		resultContent := stepOutput.Content
		for _, tc := range stepOutput.ToolCalls {
			if tc.Result != "" && !tc.IsError {
				resultContent += fmt.Sprintf("\n\n[工具 %s 的原始输出]:\n%s", tc.ToolName, tc.Result)
			}
		}
		stepResults = append(stepResults, resultContent)

		a.notifyStepUpdate(ctx, req, i+1, "completed", stepOutput.Content)
	}

	return stepResults
}

// executeMiniReAct 为 Plan 步骤执行简化的 ReAct 循环
func (a *PlanAgent) executeMiniReAct(ctx context.Context, req *AgentRequest, planStepIndex int) (*AIOutput, error) {
	output := &AIOutput{Model: req.Config.Model}

	maxRounds := req.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 5
	}

	messages := make([]*schema.Message, len(req.Messages))
	copy(messages, req.Messages)
	toolTimeout := getToolTimeout(req.Config)

	if len(req.SchemaTools) == 0 {
		resp, err := callLLM(ctx, req.ChatModel, messages, nil, req.Config, req.StepID, nil)
		if err != nil {
			return nil, err
		}
		output.Content = resp.Content
		updateTokenUsage(output, resp)
		return output, nil
	}

	for round := 1; round <= maxRounds; round++ {
		resp, err := callLLM(ctx, req.ChatModel, messages, req.SchemaTools, req.Config, req.StepID, nil)
		if err != nil {
			return nil, err
		}
		updateTokenUsage(output, resp)

		if len(resp.ToolCalls) == 0 {
			output.Content = resp.Content
			return output, nil
		}

		messages = append(messages, resp)
		toolResults := executeToolsConcurrently(
			ctx, resp.ToolCalls, round, req.ExecCtx, req.ToolRegistry,
			req.StepID, req.Callbacks, planStepIndex, toolTimeout,
		)

		for _, r := range toolResults {
			messages = append(messages, schema.ToolMessage(r.result.GetLLMContent(), r.tc.ID))
			output.ToolCalls = append(output.ToolCalls, r.record)
		}
	}

	resp, err := callLLM(ctx, req.ChatModel, messages, nil, req.Config, req.StepID, nil)
	if err != nil {
		log.Printf("[WARN] miniReAct 轮次耗尽后最终回复生成失败: %v", err)
		return output, fmt.Errorf("最终回复生成失败: %w", err)
	}
	output.Content = resp.Content
	updateTokenUsage(output, resp)
	return output, nil
}

// synthesisPhase 汇总阶段
func (a *PlanAgent) synthesisPhase(ctx context.Context, req *AgentRequest, output *AIOutput, tasks []string, stepResults []string) error {
	synthesisPrompt := buildSynthesisPrompt(req.Config.Prompt, tasks, stepResults)
	synthMessages := []*schema.Message{
		schema.SystemMessage(req.Config.SystemPrompt),
		schema.UserMessage(synthesisPrompt),
	}

	resp, err := callLLM(ctx, req.ChatModel, synthMessages, nil, req.Config, req.StepID, req.Callbacks.AI)
	if err != nil {
		return fmt.Errorf("汇总阶段失败: %w", err)
	}

	updateTokenUsage(output, resp)
	output.Content = resp.Content
	output.AgentTrace.Plan.Synthesis = output.Content

	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		output.AgentTrace.Plan.SynthesisTokens = &TokenUsage{
			PromptTokens:     resp.ResponseMeta.Usage.PromptTokens,
			CompletionTokens: resp.ResponseMeta.Usage.CompletionTokens,
			TotalTokens:      resp.ResponseMeta.Usage.TotalTokens,
		}
	}

	if req.Callbacks.AI != nil {
		req.Callbacks.AI.OnAIComplete(ctx, req.StepID, toAIResult(output))
	}

	return nil
}

// --- 通知辅助 ---

func (a *PlanAgent) notifyPlanStarted(ctx context.Context, req *AgentRequest, reason string, tasks []string) {
	if req.Callbacks.Thinking != nil {
		var stepList strings.Builder
		for i, s := range tasks {
			stepList.WriteString(fmt.Sprintf("\n%d. %s", i+1, s))
		}
		req.Callbacks.Thinking.OnAIThinking(ctx, req.StepID, 0,
			fmt.Sprintf("计划制定完成，共 %d 个步骤%s", len(tasks), stepList.String()))
	}

	if req.Callbacks.Plan != nil {
		planSteps := make([]types.PlanStepInfo, len(tasks))
		for i, s := range tasks {
			planSteps[i] = types.PlanStepInfo{Index: i + 1, Task: s}
		}
		req.Callbacks.Plan.OnAIPlanStarted(ctx, req.StepID, reason, planSteps)
	}
}

func (a *PlanAgent) notifyStepUpdate(ctx context.Context, req *AgentRequest, stepIndex int, status string, result string) {
	if req.Callbacks.Thinking != nil && status == "running" {
		if stepIndex <= len(req.Config.Skills) {
			// 仅在 running 时通知
		}
		req.Callbacks.Thinking.OnAIThinking(ctx, req.StepID, 0,
			fmt.Sprintf("执行步骤 %d...", stepIndex))
	}
	if req.Callbacks.Plan != nil {
		req.Callbacks.Plan.OnAIPlanStepUpdate(ctx, req.StepID, stepIndex, status, result)
	}
}

// --- 工具辅助 ---

// createPlanToolInfo 返回 create_plan 工具定义，用于让 LLM 通过 tool_call 输出结构化计划
func createPlanToolInfo() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: createPlanToolName,
		Desc: "创建执行计划，将复杂任务分解为有序的执行步骤。每个步骤应独立且具体。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"steps": {
				Type:     schema.Array,
				Desc:     "计划步骤列表，按执行顺序排列",
				Required: true,
				ElemInfo: &schema.ParameterInfo{
					Type: schema.Object,
					SubParams: map[string]*schema.ParameterInfo{
						"step": {
							Type:     schema.Integer,
							Desc:     "步骤编号",
							Required: true,
						},
						"task": {
							Type:     schema.String,
							Desc:     "步骤任务描述",
							Required: true,
						},
					},
				},
			},
		}),
	}
}

// filterOutCreatePlan 从工具列表中移除 create_plan 工具
func filterOutCreatePlan(tools []*schema.ToolInfo) []*schema.ToolInfo {
	var filtered []*schema.ToolInfo
	for _, t := range tools {
		if t.Name != createPlanToolName {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// filterToolDefsForStep 根据步骤描述过滤相关工具（智能匹配）
func filterToolDefsForStep(allDefs []*types.ToolDefinition, task string, skills []*SkillInfo) []*types.ToolDefinition {
	if len(allDefs) <= 5 {
		return allDefs
	}

	taskLower := strings.ToLower(task)

	var matched []*types.ToolDefinition
	matchedNames := make(map[string]bool)

	for _, def := range allDefs {
		nameLower := strings.ToLower(def.Name)
		descLower := strings.ToLower(def.Description)

		// 工具名或描述与任务有关键词重叠
		if strings.Contains(taskLower, nameLower) ||
			hasKeywordOverlap(taskLower, descLower) {
			matched = append(matched, def)
			matchedNames[def.Name] = true
		}
	}

	// 如果匹配到的工具太少，返回全部（避免遗漏）
	if len(matched) < 2 {
		return allDefs
	}

	// 始终包含基础工具
	essentialTools := []string{"bing_search", "google_search", "web_fetch", "code_execute", "shell_exec"}
	for _, def := range allDefs {
		if !matchedNames[def.Name] {
			for _, essential := range essentialTools {
				if def.Name == essential {
					matched = append(matched, def)
					break
				}
			}
		}
	}

	return matched
}

func hasKeywordOverlap(a, b string) bool {
	keywords := strings.Fields(a)
	for _, kw := range keywords {
		if len(kw) >= 3 && strings.Contains(b, kw) {
			return true
		}
	}
	return false
}

func formatPlanText(tasks []string) string {
	var sb strings.Builder
	for i, t := range tasks {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, t))
	}
	return sb.String()
}

func toAIResult(output *AIOutput) *types.AIResult {
	return &types.AIResult{
		Content:          output.Content,
		PromptTokens:     output.PromptTokens,
		CompletionTokens: output.CompletionTokens,
		TotalTokens:      output.TotalTokens,
	}
}
