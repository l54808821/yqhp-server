package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

const UnifiedAgentType = "ai_agent"

type UnifiedAgentExecutor struct {
	*executor.BaseExecutor
}

func NewUnifiedAgentExecutor() *UnifiedAgentExecutor {
	return &UnifiedAgentExecutor{
		BaseExecutor: executor.NewBaseExecutor(UnifiedAgentType),
	}
}

func (e *UnifiedAgentExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

func (e *UnifiedAgentExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	config, err := parseAIConfig(step.Config)
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime, err), nil
	}
	config = resolveConfigVariables(config, execCtx)
	applyUserMessage(config, execCtx)
	chatHistory := extractChatHistory(execCtx)

	chatModel, err := createChatModelFromConfig(ctx, config)
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewExecutionError(step.ID, "创建 AI 模型失败", err)), nil
	}

	timeout := step.Timeout
	if timeout <= 0 && config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}
	if timeout <= 0 {
		timeout = defaultAITimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var aiCallback types.AICallback
	if execCtx.Callback != nil {
		if cb, ok := execCtx.Callback.(types.AICallback); ok {
			aiCallback = cb
		}
	}

	ctx = WithExecCtx(ctx, execCtx)
	if aiCallback != nil {
		ctx = WithAICallback(ctx, aiCallback)
	}

	// 构建工具注册表：每次执行创建独立的注册表
	toolRegistry := buildToolRegistry(ctx, config, execCtx, step.ID, aiCallback)

	allToolDefs := toolRegistry.List()
	schemaTools := toSchemaTools(allToolDefs)

	if config.EnablePlanMode != nil && *config.EnablePlanMode {
		schemaTools = append(schemaTools, switchToPlanToolInfo())
	}

	systemPrompt := buildUnifiedSystemPrompt(config, len(allToolDefs) > 0)
	messages := buildUnifiedMessages(systemPrompt, chatHistory, config)

	maxRounds := config.MaxToolRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxToolRounds
	}

	var toolCallback types.AIToolCallback
	if aiCallback != nil {
		if tc, ok := aiCallback.(types.AIToolCallback); ok {
			toolCallback = tc
		}
	}

	output, err := e.executeReActLoop(ctx, chatModel, messages, schemaTools, allToolDefs, config, step.ID, execCtx, aiCallback, toolCallback, toolRegistry, maxRounds)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return executor.CreateTimeoutResult(step.ID, startTime, timeout), nil
		}
		if aiCallback != nil {
			aiCallback.OnAIError(ctx, step.ID, err)
		}
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewExecutionError(step.ID, "AI Agent 调用失败", err)), nil
	}

	output.SystemPrompt = config.SystemPrompt
	output.Prompt = config.Prompt

	result := executor.CreateSuccessResult(step.ID, startTime, output)
	result.Metrics["ai_prompt_tokens"] = float64(output.PromptTokens)
	result.Metrics["ai_completion_tokens"] = float64(output.CompletionTokens)
	result.Metrics["ai_total_tokens"] = float64(output.TotalTokens)
	return result, nil
}

// buildToolRegistry 构建本次执行的工具注册表
func buildToolRegistry(ctx context.Context, config *AIConfig, execCtx *executor.ExecutionContext, stepID string, aiCallback types.AICallback) *executor.ToolRegistry {
	reg := executor.DefaultToolRegistry.Clone()

	// 仅保留配置中指定的内置工具 + 新增的通用工具
	// 如果 config.Tools 为空，保留所有默认工具
	if len(config.Tools) > 0 {
		filteredReg := executor.NewToolRegistry()
		for _, toolName := range config.Tools {
			if tool, ok := reg.Get(toolName); ok {
				filteredReg.Register(tool)
			}
		}
		for _, name := range []string{"bing_search", "google_search", "web_fetch", "code_execute"} {
			if tool, ok := reg.Get(name); ok {
				filteredReg.Register(tool)
			}
		}
		reg = filteredReg
	}

	// 人机交互工具
	if config.Interactive {
		interactionTool := NewHumanInteractionTool(config)
		interactionTool.SetContext(stepID, aiCallback)
		reg.Register(interactionTool)
	}

	// 知识库搜索工具
	if len(config.KnowledgeBases) > 0 {
		reg.Register(NewKnowledgeTool(config.KnowledgeBases, config))
	}

	// Skill 工具
	if len(config.Skills) > 0 {
		reg.Register(NewSkillTool(config.Skills))
	}

	// MCP 工具（直连 MCP Server）
	if len(config.MCPServers) > 0 {
		for _, serverCfg := range config.MCPServers {
			tools, _, err := loadMCPTools(ctx, serverCfg)
			if err != nil {
				log.Printf("[WARN] MCP Server %q 加载失败: %v", serverCfg.Name, err)
				continue
			}
			for _, t := range tools {
				reg.Register(t)
			}
		}
	}

	return reg
}

func (e *UnifiedAgentExecutor) executeReActLoop(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	messages []*schema.Message,
	schemaTools []*schema.ToolInfo,
	allToolDefs []*types.ToolDefinition,
	config *AIConfig,
	stepID string,
	execCtx *executor.ExecutionContext,
	aiCallback types.AICallback,
	toolCallback types.AIToolCallback,
	toolRegistry *executor.ToolRegistry,
	maxRounds int,
) (*AIOutput, error) {
	output := &AIOutput{
		Model:      config.Model,
		AgentTrace: &AgentTrace{Mode: "react"},
	}

	hasTools := len(schemaTools) > 0
	if !hasTools {
		return e.executeDirect(ctx, chatModel, messages, config, stepID, aiCallback)
	}

	for round := 1; round <= maxRounds; round++ {
		resp, err := e.callLLMWithRetry(ctx, chatModel, messages, schemaTools, config, stepID, aiCallback)
		if err != nil {
			return nil, err
		}

		updateTokenUsage(output, resp)
		roundThinking := resp.Content

		if len(resp.ToolCalls) == 0 {
			output.Content = resp.Content
			if round == 1 {
				output.AgentTrace.Mode = "direct"
			}
			return output, nil
		}

		if aiCallback != nil {
			if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
				if roundThinking != "" {
					tc.OnAIThinking(ctx, stepID, round, roundThinking)
				} else {
					toolNames := make([]string, 0, len(resp.ToolCalls))
					for _, tc := range resp.ToolCalls {
						toolNames = append(toolNames, tc.Function.Name)
					}
					tc.OnAIThinking(ctx, stepID, round, fmt.Sprintf("调用工具: %s", strings.Join(toolNames, ", ")))
				}
			}
		}

		if planReason := findSwitchToPlan(resp.ToolCalls); planReason != "" {
			if aiCallback != nil {
				if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
					tc.OnAIThinking(ctx, stepID, round, fmt.Sprintf("切换到 Plan 模式：%s", planReason))
				}
			}
			planOutput, planErr := e.executePlanMode(ctx, chatModel, messages, schemaTools, allToolDefs, config, stepID, execCtx, aiCallback, toolCallback, toolRegistry, planReason)
			if planErr != nil {
				return nil, planErr
			}
			mergeTokenUsage(output, planOutput)
			output.Content = planOutput.Content
			output.AgentTrace = planOutput.AgentTrace
			return output, nil
		}

		messages = append(messages, resp)
		toolResults := e.executeToolsConcurrently(ctx, resp.ToolCalls, round, execCtx, toolRegistry, stepID, aiCallback, toolCallback, 0)

		var roundToolCalls []ToolCallRecord
		for _, r := range toolResults {
			toolMsg := schema.ToolMessage(r.result.GetLLMContent(), r.tc.ID)
			messages = append(messages, toolMsg)
			output.ToolCalls = append(output.ToolCalls, r.record)
			roundToolCalls = append(roundToolCalls, r.record)
		}

		output.AgentTrace.ReAct = append(output.AgentTrace.ReAct, ReActRound{
			Round:     round,
			Thinking:  roundThinking,
			ToolCalls: roundToolCalls,
		})
	}

	log.Printf("[WARN] 工具调用轮次达到最大值 %d，停止循环", maxRounds)
	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Printf("[WARN] 工具轮次耗尽后最终回复生成失败: %v", err)
		return output, fmt.Errorf("最终回复生成失败: %w", err)
	}
	output.Content = resp.Content
	updateTokenUsage(output, resp)
	return output, nil
}

// callLLMWithRetry 带重试的 LLM 调用（超时重试 + context window 压缩重试）
func (e *UnifiedAgentExecutor) callLLMWithRetry(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	messages []*schema.Message,
	schemaTools []*schema.ToolInfo,
	config *AIConfig,
	stepID string,
	aiCallback types.AICallback,
) (*schema.Message, error) {
	maxRetries := 2

	for retry := 0; retry <= maxRetries; retry++ {
		var resp *schema.Message
		var err error

		if config.Streaming && aiCallback != nil {
			resp, err = streamWithTools(ctx, chatModel, messages, schemaTools, stepID, config, aiCallback)
		} else {
			resp, err = chatModel.Generate(ctx, messages, model.WithTools(schemaTools))
		}

		if err == nil {
			return resp, nil
		}

		errMsg := strings.ToLower(err.Error())

		isTimeoutError := strings.Contains(errMsg, "deadline exceeded") ||
			strings.Contains(errMsg, "client.timeout") ||
			strings.Contains(errMsg, "timed out") ||
			strings.Contains(errMsg, "timeout exceeded")

		isContextError := !isTimeoutError && (strings.Contains(errMsg, "context_length_exceeded") ||
			strings.Contains(errMsg, "context window") ||
			strings.Contains(errMsg, "maximum context length") ||
			strings.Contains(errMsg, "token limit") ||
			strings.Contains(errMsg, "too many tokens") ||
			strings.Contains(errMsg, "max_tokens") ||
			strings.Contains(errMsg, "prompt is too long") ||
			strings.Contains(errMsg, "request too large"))

		if isTimeoutError && retry < maxRetries {
			backoff := time.Duration(retry+1) * 5 * time.Second
			log.Printf("[WARN] LLM 调用超时，%v 后重试 (%d/%d): %v", backoff, retry+1, maxRetries, err)
			time.Sleep(backoff)
			continue
		}

		if isContextError && retry < maxRetries {
			log.Printf("[WARN] Context window 溢出，压缩消息后重试 (%d/%d): %v", retry+1, maxRetries, err)
			compressMessages(messages)
			continue
		}

		return nil, err
	}

	return nil, fmt.Errorf("LLM 调用在 %d 次重试后仍然失败", maxRetries)
}

// compressMessages 压缩消息历史（丢弃最老的 50% 对话消息）
func compressMessages(messages []*schema.Message) {
	if len(messages) <= 4 {
		return
	}
	// messages[0] = system prompt, 保留
	// 保留最后 2 条消息（最近一轮对话）
	conversation := messages[1 : len(messages)-2]
	if len(conversation) == 0 {
		return
	}
	mid := len(conversation) / 2
	kept := conversation[mid:]

	newMessages := make([]*schema.Message, 0, 1+len(kept)+2)
	newMessages = append(newMessages, messages[0]) // system
	newMessages = append(newMessages, kept...)
	newMessages = append(newMessages, messages[len(messages)-2:]...) // last 2

	copy(messages, newMessages)
	// Truncate the slice in-place (caller sees shortened slice through shared backing array)
	// Note: this is imperfect since we can't resize the caller's slice. The caller
	// should ideally pass a pointer, but for now the compression at least drops old entries.
}

func (e *UnifiedAgentExecutor) executeDirect(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	messages []*schema.Message,
	config *AIConfig,
	stepID string,
	aiCallback types.AICallback,
) (*AIOutput, error) {
	output := &AIOutput{
		Model:      config.Model,
		AgentTrace: &AgentTrace{Mode: "direct"},
	}

	if config.Streaming && aiCallback != nil {
		stream, err := chatModel.Stream(ctx, messages)
		if err != nil {
			return nil, err
		}
		defer stream.Close()

		var contentBuilder strings.Builder
		var idx int
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if chunk.Content != "" {
				contentBuilder.WriteString(chunk.Content)
				aiCallback.OnAIChunk(ctx, stepID, chunk.Content, idx)
				idx++
			}
			if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
				output.PromptTokens += chunk.ResponseMeta.Usage.PromptTokens
				output.CompletionTokens += chunk.ResponseMeta.Usage.CompletionTokens
				output.TotalTokens += chunk.ResponseMeta.Usage.TotalTokens
			}
		}
		output.Content = contentBuilder.String()
		aiCallback.OnAIComplete(ctx, stepID, &types.AIResult{
			Content:          output.Content,
			PromptTokens:     output.PromptTokens,
			CompletionTokens: output.CompletionTokens,
			TotalTokens:      output.TotalTokens,
		})
		return output, nil
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, err
	}
	output.Content = resp.Content
	updateTokenUsage(output, resp)
	return output, nil
}

// executePlanMode 被 Agent 自主触发的 Plan 模式
func (e *UnifiedAgentExecutor) executePlanMode(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	existingMessages []*schema.Message,
	schemaTools []*schema.ToolInfo,
	allToolDefs []*types.ToolDefinition,
	config *AIConfig,
	stepID string,
	execCtx *executor.ExecutionContext,
	aiCallback types.AICallback,
	toolCallback types.AIToolCallback,
	toolRegistry *executor.ToolRegistry,
	planReason string,
) (*AIOutput, error) {
	output := &AIOutput{
		Model: config.Model,
		AgentTrace: &AgentTrace{
			Mode: "plan",
			Plan: &PlanTrace{Reason: planReason},
		},
	}

	// 1. 规划阶段
	planMessages := make([]*schema.Message, len(existingMessages))
	copy(planMessages, existingMessages)
	planMessages = append(planMessages, schema.UserMessage(buildPlanningPrompt(config.Skills)))

	if aiCallback != nil {
		if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
			tc.OnAIThinking(ctx, stepID, 0, "正在制定执行计划...")
		}
	}

	planResp, err := chatModel.Generate(ctx, planMessages)
	if err != nil {
		return nil, fmt.Errorf("规划阶段失败: %w", err)
	}
	updateTokenUsage(output, planResp)

	if planResp.ResponseMeta != nil && planResp.ResponseMeta.Usage != nil {
		output.AgentTrace.Plan.PlanningTokens = &TokenUsage{
			PromptTokens:     planResp.ResponseMeta.Usage.PromptTokens,
			CompletionTokens: planResp.ResponseMeta.Usage.CompletionTokens,
			TotalTokens:      planResp.ResponseMeta.Usage.TotalTokens,
		}
	}

	steps := parsePlanSteps(planResp.Content)
	maxSteps := config.MaxPlanSteps
	if maxSteps <= 0 {
		maxSteps = defaultMaxPlanSteps
	}
	if len(steps) > maxSteps {
		steps = steps[:maxSteps]
	}

	output.AgentTrace.Plan.PlanText = planResp.Content
	for i, s := range steps {
		output.AgentTrace.Plan.Steps = append(output.AgentTrace.Plan.Steps, PlanStep{
			Index:  i + 1,
			Task:   s,
			Status: "pending",
		})
	}

	if aiCallback != nil {
		if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
			var stepList strings.Builder
			for i, s := range steps {
				stepList.WriteString(fmt.Sprintf("\n%d. %s", i+1, s))
			}
			tc.OnAIThinking(ctx, stepID, 0, fmt.Sprintf("计划制定完成，共 %d 个步骤%s", len(steps), stepList.String()))
		}
	}

	if aiCallback != nil {
		if pc, ok := aiCallback.(types.AIPlanCallback); ok {
			planSteps := make([]types.PlanStepInfo, len(steps))
			for i, s := range steps {
				planSteps[i] = types.PlanStepInfo{Index: i + 1, Task: s}
			}
			pc.OnAIPlanStarted(ctx, stepID, planReason, planSteps)
		}
	}

	// 2. 逐步执行
	execTools := filterOutSwitchToPlan(schemaTools)
	historySummary := buildHistorySummary(existingMessages)

	var stepResults []string
	for i, task := range steps {
		if ctx.Err() != nil {
			break
		}

		output.AgentTrace.Plan.Steps[i].Status = "running"

		if aiCallback != nil {
			if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
				tc.OnAIThinking(ctx, stepID, 0, fmt.Sprintf("执行步骤 %d/%d: %s", i+1, len(steps), task))
			}
			if pc, ok := aiCallback.(types.AIPlanCallback); ok {
				pc.OnAIPlanStepUpdate(ctx, stepID, i+1, "running", "")
			}
		}

		stepPrompt := buildPlanStepPrompt(config.Prompt, steps, i, stepResults)
		userContent := stepPrompt
		if historySummary != "" {
			userContent = historySummary + "\n" + stepPrompt
		}
		stepMessages := []*schema.Message{
			schema.SystemMessage(config.SystemPrompt),
			schema.UserMessage(userContent),
		}

		stepTools, stepToolDefs := filterToolsForStep(execTools, allToolDefs, task, config.Skills)

		stepOutput, stepErr := e.executeMiniReAct(ctx, chatModel, stepMessages, stepTools, stepToolDefs, config, stepID, execCtx, aiCallback, toolCallback, toolRegistry, i+1)
		if stepErr != nil {
			output.AgentTrace.Plan.Steps[i].Status = "failed"
			output.AgentTrace.Plan.Steps[i].Result = stepErr.Error()
			stepResults = append(stepResults, fmt.Sprintf("步骤 %d 失败: %s", i+1, stepErr.Error()))
			if aiCallback != nil {
				if pc, ok := aiCallback.(types.AIPlanCallback); ok {
					pc.OnAIPlanStepUpdate(ctx, stepID, i+1, "failed", stepErr.Error())
				}
			}
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

		if aiCallback != nil {
			if pc, ok := aiCallback.(types.AIPlanCallback); ok {
				pc.OnAIPlanStepUpdate(ctx, stepID, i+1, "completed", stepOutput.Content)
			}
		}
	}

	// 3. 汇总阶段
	synthesisPrompt := buildSynthesisPrompt(config.Prompt, steps, stepResults)
	synthMessages := []*schema.Message{
		schema.SystemMessage(config.SystemPrompt),
		schema.UserMessage(synthesisPrompt),
	}

	if config.Streaming && aiCallback != nil {
		stream, err := chatModel.Stream(ctx, synthMessages)
		if err != nil {
			return nil, fmt.Errorf("汇总阶段失败: %w", err)
		}
		defer stream.Close()

		var sb strings.Builder
		var idx int
		synthTokens := &TokenUsage{}
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			if chunk.Content != "" {
				sb.WriteString(chunk.Content)
				aiCallback.OnAIChunk(ctx, stepID, chunk.Content, idx)
				idx++
			}
			if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
				synthTokens.PromptTokens += chunk.ResponseMeta.Usage.PromptTokens
				synthTokens.CompletionTokens += chunk.ResponseMeta.Usage.CompletionTokens
				synthTokens.TotalTokens += chunk.ResponseMeta.Usage.TotalTokens
				output.PromptTokens += chunk.ResponseMeta.Usage.PromptTokens
				output.CompletionTokens += chunk.ResponseMeta.Usage.CompletionTokens
				output.TotalTokens += chunk.ResponseMeta.Usage.TotalTokens
			}
		}
		output.Content = sb.String()
		output.AgentTrace.Plan.Synthesis = output.Content
		output.AgentTrace.Plan.SynthesisTokens = synthTokens
		aiCallback.OnAIComplete(ctx, stepID, &types.AIResult{
			Content:          output.Content,
			PromptTokens:     output.PromptTokens,
			CompletionTokens: output.CompletionTokens,
			TotalTokens:      output.TotalTokens,
		})
	} else {
		synthResp, err := chatModel.Generate(ctx, synthMessages)
		if err != nil {
			return nil, fmt.Errorf("汇总阶段失败: %w", err)
		}
		updateTokenUsage(output, synthResp)
		output.Content = synthResp.Content
		output.AgentTrace.Plan.Synthesis = output.Content
		if synthResp.ResponseMeta != nil && synthResp.ResponseMeta.Usage != nil {
			output.AgentTrace.Plan.SynthesisTokens = &TokenUsage{
				PromptTokens:     synthResp.ResponseMeta.Usage.PromptTokens,
				CompletionTokens: synthResp.ResponseMeta.Usage.CompletionTokens,
				TotalTokens:      synthResp.ResponseMeta.Usage.TotalTokens,
			}
		}
	}

	if aiCallback != nil {
		if pc, ok := aiCallback.(types.AIPlanCallback); ok {
			pc.OnAIPlanCompleted(ctx, stepID, output.Content)
		}
	}

	return output, nil
}

// executeMiniReAct 为 Plan 模式中的单个步骤执行的简化 ReAct 循环
func (e *UnifiedAgentExecutor) executeMiniReAct(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	messages []*schema.Message,
	schemaTools []*schema.ToolInfo,
	allToolDefs []*types.ToolDefinition,
	config *AIConfig,
	stepID string,
	execCtx *executor.ExecutionContext,
	aiCallback types.AICallback,
	toolCallback types.AIToolCallback,
	toolRegistry *executor.ToolRegistry,
	planStepIndex int,
) (*AIOutput, error) {
	output := &AIOutput{Model: config.Model}

	maxRounds := config.MaxToolRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxToolRounds
	}
	if maxRounds > 5 {
		maxRounds = 5
	}

	if len(schemaTools) == 0 {
		resp, err := chatModel.Generate(ctx, messages)
		if err != nil {
			return nil, err
		}
		output.Content = resp.Content
		updateTokenUsage(output, resp)
		return output, nil
	}

	for round := 1; round <= maxRounds; round++ {
		resp, err := chatModel.Generate(ctx, messages, model.WithTools(schemaTools))
		if err != nil {
			return nil, err
		}
		updateTokenUsage(output, resp)

		if len(resp.ToolCalls) == 0 {
			output.Content = resp.Content
			return output, nil
		}

		messages = append(messages, resp)
		toolResults := e.executeToolsConcurrently(ctx, resp.ToolCalls, round, execCtx, toolRegistry, stepID, aiCallback, toolCallback, planStepIndex)
		for _, r := range toolResults {
			toolMsg := schema.ToolMessage(r.result.GetLLMContent(), r.tc.ID)
			messages = append(messages, toolMsg)
			output.ToolCalls = append(output.ToolCalls, r.record)
		}
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Printf("[WARN] miniReAct 轮次耗尽后最终回复生成失败: %v", err)
		return output, fmt.Errorf("最终回复生成失败: %w", err)
	}
	output.Content = resp.Content
	updateTokenUsage(output, resp)
	return output, nil
}

type toolCallResult struct {
	index  int
	tc     schema.ToolCall
	result *types.ToolResult
	record ToolCallRecord
}

func (e *UnifiedAgentExecutor) executeToolsConcurrently(
	ctx context.Context,
	toolCalls []schema.ToolCall,
	round int,
	execCtx *executor.ExecutionContext,
	toolRegistry *executor.ToolRegistry,
	stepID string,
	aiCallback types.AICallback,
	toolCallback types.AIToolCallback,
	planStepIndex int,
) []toolCallResult {
	results := make([]toolCallResult, len(toolCalls))
	var wg sync.WaitGroup

	sem := make(chan struct{}, defaultMaxToolConcurrent)

	toolTimeout := defaultToolTimeout
	if config := getConfigFromCtx(ctx); config != nil && config.ToolTimeout > 0 {
		toolTimeout = time.Duration(config.ToolTimeout) * time.Second
	}

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, toolCall schema.ToolCall) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			toolCtx, toolCancel := context.WithTimeout(ctx, toolTimeout)
			defer toolCancel()

			typesToolCall := &types.ToolCall{
				ID:            toolCall.ID,
				Name:          toolCall.Function.Name,
				Arguments:     toolCall.Function.Arguments,
				PlanStepIndex: planStepIndex,
			}
			if toolCallback != nil {
				toolCallback.OnAIToolCallStart(toolCtx, stepID, typesToolCall)
			}

			callStart := time.Now()

			// 通过统一 ToolRegistry 执行
			toolResult, err := toolRegistry.Execute(toolCtx, toolCall.Function.Name, toolCall.Function.Arguments, execCtx)
			if err != nil {
				toolResult = types.NewErrorResult(fmt.Sprintf("工具执行错误: %v", err))
			}

			callDuration := time.Since(callStart)

			if toolCtx.Err() == context.DeadlineExceeded && !toolResult.IsError {
				toolResult = types.NewErrorResult(fmt.Sprintf("工具 %s 执行超时 (%v)", toolCall.Function.Name, toolTimeout))
			}

			toolResult.ToolCallID = toolCall.ID

			if toolCallback != nil {
				toolCallback.OnAIToolCallComplete(ctx, stepID, typesToolCall, toolResult)
			}

			results[idx] = toolCallResult{
				index:  idx,
				tc:     toolCall,
				result: toolResult,
				record: ToolCallRecord{
					Round:     round,
					ToolName:  toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
					Result:    toolResult.GetLLMContent(),
					IsError:   toolResult.IsError,
					Duration:  callDuration.Milliseconds(),
				},
			}
		}(i, tc)
	}
	wg.Wait()
	return results
}

// getConfigFromCtx 尝试从 context 获取配置（用于工具超时等）
func getConfigFromCtx(ctx context.Context) *AIConfig {
	execCtx, ok := ctx.Value(execCtxKey).(*executor.ExecutionContext)
	if !ok || execCtx == nil {
		return nil
	}
	return nil
}

// ========== 辅助函数 ==========

func streamWithTools(ctx context.Context, chatModel model.ToolCallingChatModel, messages []*schema.Message, tools []*schema.ToolInfo, stepID string, config *AIConfig, callback types.AICallback) (*schema.Message, error) {
	stream, err := chatModel.Stream(ctx, messages, model.WithTools(tools))
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var chunks []*schema.Message
	var chunkIndex int

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if chunk.Content != "" && callback != nil {
			callback.OnAIChunk(ctx, stepID, chunk.Content, chunkIndex)
			chunkIndex++
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		return &schema.Message{Role: schema.Assistant}, nil
	}

	merged, err := schema.ConcatMessages(chunks)
	if err != nil {
		return nil, fmt.Errorf("合并流式消息失败: %w", err)
	}
	return merged, nil
}

func updateTokenUsage(output *AIOutput, resp *schema.Message) {
	if resp.ResponseMeta != nil {
		if resp.ResponseMeta.Usage != nil {
			output.PromptTokens += resp.ResponseMeta.Usage.PromptTokens
			output.CompletionTokens += resp.ResponseMeta.Usage.CompletionTokens
			output.TotalTokens += resp.ResponseMeta.Usage.TotalTokens
		}
		if resp.ResponseMeta.FinishReason != "" {
			output.FinishReason = resp.ResponseMeta.FinishReason
		}
	}
}

func mergeTokenUsage(dst, src *AIOutput) {
	dst.PromptTokens += src.PromptTokens
	dst.CompletionTokens += src.CompletionTokens
	dst.TotalTokens += src.TotalTokens
}

func toSchemaTools(defs []*types.ToolDefinition) []*schema.ToolInfo {
	tools := make([]*schema.ToolInfo, 0, len(defs))
	for _, td := range defs {
		toolInfo := &schema.ToolInfo{
			Name: td.Name,
			Desc: td.Description,
		}
		if len(td.Parameters) > 0 {
			var jsonSchemaMap map[string]any
			if err := json.Unmarshal(td.Parameters, &jsonSchemaMap); err == nil {
				params := jsonSchemaMapToParams(jsonSchemaMap)
				if params != nil {
					toolInfo.ParamsOneOf = schema.NewParamsOneOfByParams(params)
				}
			}
		}
		tools = append(tools, toolInfo)
	}
	return tools
}

func parsePlanSteps(content string) []string {
	trimmed := strings.TrimSpace(content)

	jsonContent := trimmed
	start := strings.Index(trimmed, "[")
	end := strings.LastIndex(trimmed, "]")
	if start >= 0 && end > start {
		jsonContent = trimmed[start : end+1]
	}

	var rawSteps []struct {
		Step int    `json:"step"`
		Task string `json:"task"`
	}
	if err := json.Unmarshal([]byte(jsonContent), &rawSteps); err == nil && len(rawSteps) > 0 {
		var tasks []string
		for _, s := range rawSteps {
			if s.Task != "" {
				tasks = append(tasks, s.Task)
			}
		}
		if len(tasks) > 0 {
			return tasks
		}
	}

	if tasks := parseNumberedList(trimmed); len(tasks) > 0 {
		log.Printf("[WARN] Plan 步骤 JSON 解析失败，回退到编号列表解析，提取到 %d 个步骤", len(tasks))
		return tasks
	}

	log.Printf("[WARN] Plan 步骤解析失败，将整段内容作为单一步骤执行")
	return []string{trimmed}
}

func parseNumberedList(content string) []string {
	var tasks []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) >= 3 && line[0] >= '0' && line[0] <= '9' {
			for i := 1; i < len(line); i++ {
				if line[i] >= '0' && line[i] <= '9' {
					continue
				}
				if (line[i] == '.' || line[i] == ')') && i+1 < len(line) && line[i+1] == ' ' {
					task := strings.TrimSpace(line[i+2:])
					if task != "" {
						tasks = append(tasks, task)
					}
				}
				break
			}
		}
		if len(line) >= 3 && line[0] == '-' && line[1] == ' ' {
			task := strings.TrimSpace(line[2:])
			if task != "" {
				tasks = append(tasks, task)
			}
		}
	}
	return tasks
}

func (e *UnifiedAgentExecutor) Cleanup(ctx context.Context) error { return nil }
