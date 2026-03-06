package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/pkg/logger"
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
	logger.Debug("[Plan] 开始执行, model=%s, stepID=%s, reason=%s, tools数量=%d",
		req.Config.Model, req.StepID, reason, len(req.SchemaTools))
	planStartTime := time.Now()

	output := &AIOutput{
		Model: req.Config.Model,
		AgentTrace: &AgentTrace{
			Mode: string(AgentModePlan),
			Plan: &PlanTrace{Reason: reason},
		},
	}

	// 1. 规划阶段
	logger.Debug("[Plan] === 规划阶段开始, stepID=%s ===", req.StepID)
	stepDefs, err := a.planPhase(ctx, req, output)
	if err != nil {
		logger.Debug("[Plan] 规划阶段失败, stepID=%s, 耗时=%v: %v", req.StepID, time.Since(planStartTime), err)
		return nil, fmt.Errorf("规划阶段失败: %w", err)
	}
	logger.Debug("[Plan] 规划阶段完成, stepID=%s, 生成 %d 个步骤", req.StepID, len(stepDefs))

	maxSteps := req.Config.MaxPlanSteps
	if maxSteps <= 0 {
		maxSteps = defaultMaxPlanSteps
	}
	if len(stepDefs) > maxSteps {
		stepDefs = stepDefs[:maxSteps]
	}

	tasks := stepsToTasks(stepDefs)
	output.AgentTrace.Plan.PlanText = formatPlanText(tasks)
	for i, t := range tasks {
		output.AgentTrace.Plan.Steps = append(output.AgentTrace.Plan.Steps, PlanStep{
			Index:  i + 1,
			Task:   t,
			Status: "pending",
		})
	}

	a.notifyPlanStarted(ctx, req, output, reason, tasks)

	// 2. 逐步执行（支持动态 replan）
	logger.Debug("[Plan] === 执行阶段开始, stepID=%s, 步骤数=%d ===", req.StepID, len(tasks))
	execStartTime := time.Now()
	stepResults := a.executeSteps(ctx, req, output, tasks)
	logger.Debug("[Plan] === 执行阶段完成, stepID=%s, 步骤数=%d, 结果数=%d, 耗时=%v ===",
		req.StepID, len(tasks), len(stepResults), time.Since(execStartTime))

	// 3. 汇总阶段
	logger.Debug("[Plan] === 汇总阶段开始, stepID=%s ===", req.StepID)
	if err := a.synthesisPhase(ctx, req, output, tasks, stepResults); err != nil {
		logger.Debug("[Plan] 汇总阶段失败, stepID=%s: %v", req.StepID, err)
		return nil, err
	}

	logger.Debug("[Plan] 执行全部完成, stepID=%s, 总耗时=%v, content长度=%d, 累计tokens=%d",
		req.StepID, time.Since(planStartTime), len([]rune(output.Content)), output.TotalTokens)

	if req.Callbacks.Stream != nil {
		req.Callbacks.Stream.OnAIPlanUpdate(ctx, req.StepID, output.planBlockID, &types.PlanUpdate{
			Action:    types.PlanActionCompleted,
			Synthesis: output.Content,
		})
	}

	return output, nil
}

// planPhase 规划阶段：让 LLM 生成执行计划
func (a *PlanAgent) planPhase(ctx context.Context, req *AgentRequest, output *AIOutput) ([]PlanStepDef, error) {
	planTool := createPlanToolInfo()
	planMessages := make([]*schema.Message, len(req.Messages))
	copy(planMessages, req.Messages)
	planMessages = append(planMessages, schema.UserMessage(buildPlanningPrompt(req.Config.Skills)))

	logger.Debug("[Plan] planPhase: 调用 LLM 生成计划, messages数=%d, stepID=%s", len(planMessages), req.StepID)

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
		logger.Debug("[Plan] planPhase: LLM 返回, toolCalls=%d, content长度=%d, promptTokens=%d, completionTokens=%d",
			len(resp.ToolCalls), len([]rune(resp.Content)),
			resp.ResponseMeta.Usage.PromptTokens, resp.ResponseMeta.Usage.CompletionTokens)
	}

	if steps, ok := parsePlanFromToolCall(resp.ToolCalls); ok {
		logger.Debug("[Plan] planPhase: 从 tool_call 解析出 %d 个步骤", len(steps))
		return steps, nil
	}

	steps := parsePlanFromText(resp.Content)
	logger.Debug("[Plan] planPhase: 从文本解析出 %d 个步骤 (回退模式)", len(steps))
	return steps, nil
}

// executeSteps 逐步执行计划中的每个步骤，支持动态重新规划
func (a *PlanAgent) executeSteps(ctx context.Context, req *AgentRequest, output *AIOutput, tasks []string) []string {
	historySummary := buildHistorySummary(req.Messages)

	var stepResults []string
	consecutiveFailures := 0

	for i := 0; i < len(tasks); i++ {
		if ctx.Err() != nil {
			logger.Debug("[Plan] 执行被取消, stepID=%s, 已完成 %d/%d 步", req.StepID, i, len(tasks))
			break
		}

		task := tasks[i]
		logger.Debug("[Plan] 开始执行步骤 %d/%d: %s, stepID=%s", i+1, len(tasks), truncateForLog(task, 100), req.StepID)
		stepStartTime := time.Now()
		output.AgentTrace.Plan.Steps[i].Status = "running"
		a.notifyStepUpdate(ctx, req, output, i+1, "running", "")

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
			MaxRounds:    5,
		}

		stepOutput, stepErr := a.executeMiniReAct(ctx, stepReq, i+1)
		if stepErr != nil {
			logger.Debug("[Plan] 步骤 %d/%d 执行失败, 耗时=%v: %v", i+1, len(tasks), time.Since(stepStartTime), stepErr)
			output.AgentTrace.Plan.Steps[i].Status = "failed"
			output.AgentTrace.Plan.Steps[i].Result = stepErr.Error()
			stepResults = append(stepResults, fmt.Sprintf("步骤 %d 失败: %s", i+1, stepErr.Error()))
			a.notifyStepUpdate(ctx, req, output, i+1, "failed", stepErr.Error())
			consecutiveFailures++

			// 连续失败时尝试重新规划剩余步骤
			if consecutiveFailures >= 2 && i < len(tasks)-1 {
				newTasks := a.tryReplan(ctx, req, output, tasks, i, stepResults)
				if newTasks != nil {
					tasks = newTasks
					a.rebuildPlanSteps(output, tasks, i+1)
				}
				consecutiveFailures = 0
			}
			continue
		}

		consecutiveFailures = 0
		mergeTokenUsage(output, stepOutput)
		output.AgentTrace.Plan.Steps[i].Status = "completed"
		output.AgentTrace.Plan.Steps[i].Result = stepOutput.Content
		output.AgentTrace.Plan.Steps[i].ToolCalls = stepOutput.ToolCalls
		output.AgentTrace.Plan.Steps[i].PromptTokens = stepOutput.PromptTokens
		output.AgentTrace.Plan.Steps[i].CompletionTokens = stepOutput.CompletionTokens
		output.AgentTrace.Plan.Steps[i].TotalTokens = stepOutput.TotalTokens

		logger.Debug("[Plan] 步骤 %d/%d 执行完成, 耗时=%v, content长度=%d, toolCalls=%d, tokens=%d",
			i+1, len(tasks), time.Since(stepStartTime), len([]rune(stepOutput.Content)),
			len(stepOutput.ToolCalls), stepOutput.TotalTokens)

		resultContent := stepOutput.Content
		for _, tc := range stepOutput.ToolCalls {
			if tc.Result != "" && !tc.IsError {
				resultContent += fmt.Sprintf("\n\n[工具 %s 的原始输出]:\n%s", tc.ToolName, tc.Result)
			}
		}
		stepResults = append(stepResults, resultContent)

		a.notifyStepUpdate(ctx, req, output, i+1, "completed", stepOutput.Content)

		// 执行过半后，检查是否需要根据中间结果调整后续计划
		if a.shouldCheckReplan(i, len(tasks), stepOutput.Content) {
			newTasks := a.tryReplan(ctx, req, output, tasks, i, stepResults)
			if newTasks != nil {
				tasks = newTasks
				a.rebuildPlanSteps(output, tasks, i+1)
			}
		}
	}

	return stepResults
}

// shouldCheckReplan 判断是否需要触发重新规划检查
func (a *PlanAgent) shouldCheckReplan(currentIndex int, totalSteps int, stepResult string) bool {
	if totalSteps <= 3 {
		return false
	}
	halfwayPoint := totalSteps / 2
	return currentIndex == halfwayPoint-1
}

// tryReplan 尝试重新规划剩余步骤
func (a *PlanAgent) tryReplan(ctx context.Context, req *AgentRequest, output *AIOutput, currentTasks []string, completedIndex int, stepResults []string) []string {
	if ctx.Err() != nil {
		return nil
	}

	remainingTasks := currentTasks[completedIndex+1:]
	if len(remainingTasks) == 0 {
		return nil
	}

	logger.Debug("[Plan] tryReplan: 开始重新规划, completedIndex=%d, 剩余步骤=%d, stepID=%s",
		completedIndex, len(remainingTasks), req.StepID)

	replanPrompt := buildReplanPrompt(req.Config.Prompt, currentTasks, completedIndex, stepResults, remainingTasks)
	replanMessages := []*schema.Message{
		schema.SystemMessage(req.Config.SystemPrompt),
		schema.UserMessage(replanPrompt),
	}

	planTool := createPlanToolInfo()
	resp, err := callLLM(ctx, req.ChatModel, replanMessages, []*schema.ToolInfo{planTool}, req.Config, req.StepID, nil)
	if err != nil {
		logger.Warn("[Plan] 重新规划 LLM 调用失败, stepID=%s: %v", req.StepID, err)
		return nil
	}
	updateTokenUsage(output, resp)

	// 尝试从 tool_call 解析新计划
	if steps, ok := parsePlanFromToolCall(resp.ToolCalls); ok && len(steps) > 0 {
		newRemainingTasks := stepsToTasks(steps)
		// 保留已完成步骤 + 新的剩余步骤
		result := make([]string, completedIndex+1, completedIndex+1+len(newRemainingTasks))
		copy(result, currentTasks[:completedIndex+1])
		result = append(result, newRemainingTasks...)

		logger.Debug("[Plan] 重新规划成功: 剩余步骤从 %d 调整为 %d", len(remainingTasks), len(newRemainingTasks))
		if req.Callbacks.Stream != nil {
			newSteps := make([]types.PlanStepInfo, len(newRemainingTasks))
			for i, t := range newRemainingTasks {
				newSteps[i] = types.PlanStepInfo{Index: completedIndex + 2 + i, Task: t}
			}
			req.Callbacks.Stream.OnAIPlanUpdate(ctx, req.StepID, output.planBlockID, &types.PlanUpdate{
				Action:        types.PlanActionModified,
				FromStepIndex: completedIndex + 2,
				Reason:        fmt.Sprintf("根据执行结果调整计划，剩余步骤从 %d 调整为 %d", len(remainingTasks), len(newRemainingTasks)),
				NewSteps:      newSteps,
			})
		}
		return result
	}

	// 回退到文本解析
	if steps := parsePlanFromText(resp.Content); len(steps) > 1 {
		newRemainingTasks := stepsToTasks(steps)
		result := make([]string, completedIndex+1, completedIndex+1+len(newRemainingTasks))
		copy(result, currentTasks[:completedIndex+1])
		result = append(result, newRemainingTasks...)
		return result
	}

	return nil
}

// rebuildPlanSteps 重建 PlanTrace 中的步骤信息（用于重新规划后更新）
func (a *PlanAgent) rebuildPlanSteps(output *AIOutput, tasks []string, fromIndex int) {
	// 截断到已有步骤
	if fromIndex < len(output.AgentTrace.Plan.Steps) {
		output.AgentTrace.Plan.Steps = output.AgentTrace.Plan.Steps[:fromIndex]
	}
	// 追加新步骤
	for i := fromIndex; i < len(tasks); i++ {
		output.AgentTrace.Plan.Steps = append(output.AgentTrace.Plan.Steps, PlanStep{
			Index:  i + 1,
			Task:   tasks[i],
			Status: "pending",
		})
	}
	output.AgentTrace.Plan.PlanText = formatPlanText(tasks)
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

	logger.Debug("[Plan-MiniReAct] 步骤 %d 开始, tools数量=%d, maxRounds=%d, stepID=%s",
		planStepIndex, len(req.SchemaTools), maxRounds, req.StepID)

	if len(req.SchemaTools) == 0 {
		logger.Debug("[Plan-MiniReAct] 步骤 %d 无工具, 直接调用 LLM", planStepIndex)
		resp, err := callLLM(ctx, req.ChatModel, messages, nil, req.Config, req.StepID, nil)
		if err != nil {
			return nil, err
		}
		output.Content = resp.Content
		updateTokenUsage(output, resp)
		logger.Debug("[Plan-MiniReAct] 步骤 %d 完成(无工具), content长度=%d, tokens=%d",
			planStepIndex, len([]rune(output.Content)), output.TotalTokens)
		return output, nil
	}

	for round := 1; round <= maxRounds; round++ {
		logger.Debug("[Plan-MiniReAct] 步骤 %d, 第 %d/%d 轮, messages数=%d",
			planStepIndex, round, maxRounds, len(messages))
		resp, err := callLLM(ctx, req.ChatModel, messages, req.SchemaTools, req.Config, req.StepID, nil)
		if err != nil {
			logger.Debug("[Plan-MiniReAct] 步骤 %d, 第 %d 轮 LLM 调用失败: %v", planStepIndex, round, err)
			return nil, err
		}
		updateTokenUsage(output, resp)

		if len(resp.ToolCalls) == 0 {
			logger.Debug("[Plan-MiniReAct] 步骤 %d, 第 %d 轮 LLM 无工具调用, content长度=%d",
				planStepIndex, round, len([]rune(resp.Content)))
			output.Content = resp.Content
			return output, nil
		}

		toolNames := make([]string, 0, len(resp.ToolCalls))
		for _, tc := range resp.ToolCalls {
			toolNames = append(toolNames, tc.Function.Name)
		}
		logger.Debug("[Plan-MiniReAct] 步骤 %d, 第 %d 轮 LLM 返回 %d 个工具调用: %s",
			planStepIndex, round, len(resp.ToolCalls), strings.Join(toolNames, ", "))

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

	logger.Warn("[Plan-MiniReAct] 步骤 %d 轮次耗尽(%d), 生成最终回复, stepID=%s", planStepIndex, maxRounds, req.StepID)
	resp, err := callLLM(ctx, req.ChatModel, messages, nil, req.Config, req.StepID, nil)
	if err != nil {
		logger.Warn("[Plan-MiniReAct] 步骤 %d 最终回复生成失败: %v", planStepIndex, err)
		return output, fmt.Errorf("最终回复生成失败: %w", err)
	}
	output.Content = resp.Content
	updateTokenUsage(output, resp)
	logger.Debug("[Plan-MiniReAct] 步骤 %d 完成, content长度=%d, 累计tokens=%d",
		planStepIndex, len([]rune(output.Content)), output.TotalTokens)
	return output, nil
}

// synthesisPhase 汇总阶段
func (a *PlanAgent) synthesisPhase(ctx context.Context, req *AgentRequest, output *AIOutput, tasks []string, stepResults []string) error {
	synthesisPrompt := buildSynthesisPrompt(req.Config.Prompt, tasks, stepResults)
	synthMessages := []*schema.Message{
		schema.SystemMessage(req.Config.SystemPrompt),
		schema.UserMessage(synthesisPrompt),
	}

	logger.Debug("[Plan] synthesisPhase: 调用 LLM 进行汇总, 步骤数=%d, 结果数=%d, messages数=%d, stepID=%s",
		len(tasks), len(stepResults), len(synthMessages), req.StepID)

	resp, err := callLLM(ctx, req.ChatModel, synthMessages, nil, req.Config, req.StepID, req.Callbacks)
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
		logger.Debug("[Plan] synthesisPhase: 汇总完成, content长度=%d, promptTokens=%d, completionTokens=%d, stepID=%s",
			len([]rune(output.Content)), resp.ResponseMeta.Usage.PromptTokens,
			resp.ResponseMeta.Usage.CompletionTokens, req.StepID)
	} else {
		logger.Debug("[Plan] synthesisPhase: 汇总完成, content长度=%d, stepID=%s",
			len([]rune(output.Content)), req.StepID)
	}

	if req.Callbacks.Stream != nil {
		req.Callbacks.Stream.OnMessageComplete(ctx, req.StepID, toAIResult(output))
	}

	return nil
}

// --- 通知辅助 ---

func (a *PlanAgent) notifyPlanStarted(ctx context.Context, req *AgentRequest, output *AIOutput, reason string, tasks []string) {
	if req.Callbacks.Stream == nil {
		return
	}
	planBlockID := req.Callbacks.BlockID.Next()
	output.planBlockID = planBlockID

	planSteps := make([]types.PlanStepInfo, len(tasks))
	for i, s := range tasks {
		planSteps[i] = types.PlanStepInfo{Index: i + 1, Task: s}
	}
	req.Callbacks.Stream.OnAIPlanUpdate(ctx, req.StepID, planBlockID, &types.PlanUpdate{
		Action: types.PlanActionStarted,
		Reason: reason,
		Steps:  planSteps,
	})
}

func (a *PlanAgent) notifyStepUpdate(ctx context.Context, req *AgentRequest, output *AIOutput, stepIndex int, status string, result string) {
	if req.Callbacks.Stream == nil {
		return
	}
	update := &types.PlanUpdate{
		Action:    types.PlanActionStepUpdate,
		StepIndex: stepIndex,
		Status:    status,
	}
	if result != "" {
		if status == "failed" {
			update.Error = result
		} else {
			update.Result = result
		}
	}
	req.Callbacks.Stream.OnAIPlanUpdate(ctx, req.StepID, output.planBlockID, update)
}

// --- 工具辅助 ---

// createPlanToolInfo 返回 create_plan 工具定义，用于让 LLM 通过 tool_call 输出结构化计划
func createPlanToolInfo() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: createPlanToolName,
		Desc: "创建执行计划，将复杂任务分解为有序的执行步骤。每个步骤应独立且具体，按逻辑顺序排列。",
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
	r := &types.AIResult{
		Content:          output.Content,
		PromptTokens:     output.PromptTokens,
		CompletionTokens: output.CompletionTokens,
		TotalTokens:      output.TotalTokens,
		Model:            output.Model,
		FinishReason:     output.FinishReason,
	}
	return r
}
