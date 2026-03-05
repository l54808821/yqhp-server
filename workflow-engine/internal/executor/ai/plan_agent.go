package ai

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

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
	output := &AIOutput{
		Model: req.Config.Model,
		AgentTrace: &AgentTrace{
			Mode: string(AgentModePlan),
			Plan: &PlanTrace{Reason: reason},
		},
	}

	// 1. 规划阶段
	stepDefs, err := a.planPhase(ctx, req, output)
	if err != nil {
		return nil, fmt.Errorf("规划阶段失败: %w", err)
	}

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

	a.notifyPlanStarted(ctx, req, reason, tasks)

	// 2. 按依赖关系执行（支持并行）
	stepResults := a.executeStepsWithDeps(ctx, req, output, tasks, stepDefs)

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

// executeStepsWithDeps 根据依赖关系调度步骤执行，无依赖的步骤并行执行
func (a *PlanAgent) executeStepsWithDeps(ctx context.Context, req *AgentRequest, output *AIOutput, tasks []string, stepDefs []PlanStepDef) []string {
	hasDeps := false
	for _, sd := range stepDefs {
		if len(sd.DependsOn) > 0 {
			hasDeps = true
			break
		}
	}

	// 如果没有任何依赖声明，回退到原有的顺序执行
	if !hasDeps {
		return a.executeSteps(ctx, req, output, tasks)
	}

	stepResults := make([]string, len(tasks))
	completed := make([]bool, len(tasks))
	var mu sync.Mutex

	// 构建依赖图：stepIndex -> 依赖的 stepIndex 列表
	depMap := make(map[int][]int)
	for i, sd := range stepDefs {
		for _, dep := range sd.DependsOn {
			depIdx := dep - 1 // 步骤编号从 1 开始
			if depIdx >= 0 && depIdx < len(tasks) {
				depMap[i] = append(depMap[i], depIdx)
			}
		}
	}

	// 按批次执行：每批找出所有依赖已满足的步骤，并行执行
	for {
		if ctx.Err() != nil {
			break
		}

		// 找出当前可执行的步骤
		var readyIndices []int
		for i := range tasks {
			if completed[i] {
				continue
			}
			depsOK := true
			for _, depIdx := range depMap[i] {
				if !completed[depIdx] {
					depsOK = false
					break
				}
			}
			if depsOK {
				readyIndices = append(readyIndices, i)
			}
		}

		if len(readyIndices) == 0 {
			break
		}

		if len(readyIndices) == 1 {
			// 单个步骤直接执行
			idx := readyIndices[0]
			result := a.executeSingleStep(ctx, req, output, tasks, idx, stepResults)
			stepResults[idx] = result
			completed[idx] = true
		} else {
			// 多个步骤并行执行
			logger.Debug("[Plan] 并行执行 %d 个步骤: %v", len(readyIndices), readyIndices)
			if req.Callbacks.Thinking != nil {
				stepNums := make([]string, len(readyIndices))
				for i, idx := range readyIndices {
					stepNums[i] = fmt.Sprintf("%d", idx+1)
				}
				req.Callbacks.Thinking.OnAIThinking(ctx, req.StepID, 0,
					fmt.Sprintf("并行执行步骤 %s ...", strings.Join(stepNums, ", ")))
			}

			var wg sync.WaitGroup
			for _, idx := range readyIndices {
				wg.Add(1)
				go func(stepIdx int) {
					defer wg.Done()
					result := a.executeSingleStep(ctx, req, output, tasks, stepIdx, stepResults)
					mu.Lock()
					stepResults[stepIdx] = result
					completed[stepIdx] = true
					mu.Unlock()
				}(idx)
			}
			wg.Wait()
		}
	}

	return stepResults
}

// executeSingleStep 执行单个计划步骤
func (a *PlanAgent) executeSingleStep(ctx context.Context, req *AgentRequest, output *AIOutput, tasks []string, idx int, stepResults []string) string {
	output.AgentTrace.Plan.Steps[idx].Status = "running"
	a.notifyStepUpdate(ctx, req, idx+1, "running", "")

	historySummary := buildHistorySummary(req.Messages)
	stepPrompt := buildPlanStepPrompt(req.Config.Prompt, tasks, idx, stepResults)
	userContent := stepPrompt
	if historySummary != "" {
		userContent = historySummary + "\n" + stepPrompt
	}
	stepMessages := []*schema.Message{
		schema.SystemMessage(req.Config.SystemPrompt),
		schema.UserMessage(userContent),
	}

	task := tasks[idx]
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

	stepOutput, stepErr := a.executeMiniReAct(ctx, stepReq, idx+1)
	if stepErr != nil {
		output.AgentTrace.Plan.Steps[idx].Status = "failed"
		output.AgentTrace.Plan.Steps[idx].Result = stepErr.Error()
		a.notifyStepUpdate(ctx, req, idx+1, "failed", stepErr.Error())
		return fmt.Sprintf("步骤 %d 失败: %s", idx+1, stepErr.Error())
	}

	mergeTokenUsage(output, stepOutput)
	output.AgentTrace.Plan.Steps[idx].Status = "completed"
	output.AgentTrace.Plan.Steps[idx].Result = stepOutput.Content
	output.AgentTrace.Plan.Steps[idx].ToolCalls = stepOutput.ToolCalls
	output.AgentTrace.Plan.Steps[idx].PromptTokens = stepOutput.PromptTokens
	output.AgentTrace.Plan.Steps[idx].CompletionTokens = stepOutput.CompletionTokens
	output.AgentTrace.Plan.Steps[idx].TotalTokens = stepOutput.TotalTokens

	resultContent := stepOutput.Content
	for _, tc := range stepOutput.ToolCalls {
		if tc.Result != "" && !tc.IsError {
			resultContent += fmt.Sprintf("\n\n[工具 %s 的原始输出]:\n%s", tc.ToolName, tc.Result)
		}
	}

	a.notifyStepUpdate(ctx, req, idx+1, "completed", stepOutput.Content)
	return resultContent
}

// executeSteps 逐步执行计划中的每个步骤，支持动态重新规划（无依赖时的顺序执行模式）
func (a *PlanAgent) executeSteps(ctx context.Context, req *AgentRequest, output *AIOutput, tasks []string) []string {
	historySummary := buildHistorySummary(req.Messages)

	var stepResults []string
	consecutiveFailures := 0

	for i := 0; i < len(tasks); i++ {
		if ctx.Err() != nil {
			break
		}

		task := tasks[i]
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
			MaxRounds:    5,
		}

		stepOutput, stepErr := a.executeMiniReAct(ctx, stepReq, i+1)
		if stepErr != nil {
			output.AgentTrace.Plan.Steps[i].Status = "failed"
			output.AgentTrace.Plan.Steps[i].Result = stepErr.Error()
			stepResults = append(stepResults, fmt.Sprintf("步骤 %d 失败: %s", i+1, stepErr.Error()))
			a.notifyStepUpdate(ctx, req, i+1, "failed", stepErr.Error())
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

		resultContent := stepOutput.Content
		for _, tc := range stepOutput.ToolCalls {
			if tc.Result != "" && !tc.IsError {
				resultContent += fmt.Sprintf("\n\n[工具 %s 的原始输出]:\n%s", tc.ToolName, tc.Result)
			}
		}
		stepResults = append(stepResults, resultContent)

		a.notifyStepUpdate(ctx, req, i+1, "completed", stepOutput.Content)

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

	replanPrompt := buildReplanPrompt(req.Config.Prompt, currentTasks, completedIndex, stepResults, remainingTasks)
	replanMessages := []*schema.Message{
		schema.SystemMessage(req.Config.SystemPrompt),
		schema.UserMessage(replanPrompt),
	}

	planTool := createPlanToolInfo()
	resp, err := callLLM(ctx, req.ChatModel, replanMessages, []*schema.ToolInfo{planTool}, req.Config, req.StepID, nil)
	if err != nil {
		logger.Warn("[Plan] 重新规划调用失败: %v", err)
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
		if req.Callbacks.Thinking != nil {
			req.Callbacks.Thinking.OnAIThinking(ctx, req.StepID, 0,
				fmt.Sprintf("根据执行结果调整计划，剩余步骤从 %d 调整为 %d", len(remainingTasks), len(newRemainingTasks)))
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
		Desc: "创建执行计划，将复杂任务分解为有序的执行步骤。每个步骤应独立且具体。支持通过 depends_on 声明步骤间依赖关系，无依赖的步骤将并行执行。",
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
						"depends_on": {
							Type: schema.Array,
							Desc: "依赖的步骤编号列表。如果为空或不设置，表示该步骤没有依赖，可以与其他无依赖步骤并行执行",
							ElemInfo: &schema.ParameterInfo{
								Type: schema.Integer,
							},
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
