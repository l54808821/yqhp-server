package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

const reflectionConsecFailLimit = 2

// AgentRequest 封装 Agent 执行所需的全部输入
type AgentRequest struct {
	Config       *AIConfig
	ChatModel    model.ToolCallingChatModel
	Messages     []*schema.Message
	ToolRegistry *executor.ToolRegistry
	SchemaTools  []*schema.ToolInfo
	AllToolDefs  []*types.ToolDefinition
	StepID       string
	ExecCtx      *executor.ExecutionContext
	Callbacks    *AgentCallbacks
	MaxRounds    int
}

// AgentCallbacks 聚合 AI 流式回调和 blockID 生成器
type AgentCallbacks struct {
	Stream  types.AIStreamCallback
	BlockID *BlockIDGenerator
}

func NewAgentCallbacks(callback types.ExecutionCallback, stepID string) *AgentCallbacks {
	cb := &AgentCallbacks{
		BlockID: NewBlockIDGenerator(stepID),
	}
	if callback != nil {
		if sc, ok := callback.(types.AIStreamCallback); ok {
			cb.Stream = sc
		}
	}
	return cb
}

// BlockIDGenerator 生成同一 step 内唯一的 block ID
type BlockIDGenerator struct {
	prefix string
	seq    int64
}

func NewBlockIDGenerator(stepID string) *BlockIDGenerator {
	prefix := stepID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	return &BlockIDGenerator{prefix: prefix}
}

func (g *BlockIDGenerator) Next() string {
	return fmt.Sprintf("blk_%s_%d", g.prefix, atomic.AddInt64(&g.seq, 1))
}

// RunAgent 唯一的 Agent 执行入口，实现 ReAct 循环（Reason -> Act -> Observe）
// 无工具时自然退化为单次 LLM 调用（第一轮无 tool_calls 直接返回）
func RunAgent(ctx context.Context, req *AgentRequest) (*AIOutput, error) {
	logger.Debug("[Agent] 开始执行, model=%s, stepID=%s, tools数量=%d, maxRounds=%d",
		req.Config.Model, req.StepID, len(req.SchemaTools), req.MaxRounds)
	startTime := time.Now()

	output := &AIOutput{
		Model:      req.Config.Model,
		AgentTrace: &AgentTrace{Mode: "react"},
	}

	maxRounds := req.MaxRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxToolRounds
	}

	messages := make([]*schema.Message, len(req.Messages))
	copy(messages, req.Messages)
	toolTimeout := getToolTimeout(req.Config)
	consecutiveFailures := 0

	for round := 1; round <= maxRounds; round++ {
		logger.Debug("[Agent] ===== 第 %d/%d 轮开始 (stepID=%s, 当前messages数=%d) =====",
			round, maxRounds, req.StepID, len(messages))

		resp, err := callLLM(ctx, req.ChatModel, messages, req.SchemaTools, req.Config, req.StepID, req.Callbacks)

		if err != nil {
			logger.Debug("[Agent] 第 %d 轮 LLM 调用失败, 总耗时=%v: %v", round, time.Since(startTime), err)
			return nil, err
		}

		updateTokenUsage(output, resp)
		roundThinking := strings.TrimSpace(resp.Content)
		if roundThinking == "" {
			roundThinking = strings.TrimSpace(resp.ReasoningContent)
		}

		if len(resp.ToolCalls) == 0 {
			logger.Debug("[Agent] 第 %d 轮 LLM 未返回工具调用，直接输出 (长度=%d), 总耗时=%v",
				round, len([]rune(resp.Content)), time.Since(startTime))
			output.Content = resp.Content
			if req.Callbacks.Stream != nil {
				req.Callbacks.Stream.OnMessageComplete(ctx, req.StepID, toAIResult(output))
			}
			return output, nil
		}

		toolNames := make([]string, 0, len(resp.ToolCalls))
		for _, tc := range resp.ToolCalls {
			toolNames = append(toolNames, tc.Function.Name)
		}
		logger.Debug("[Agent] 第 %d 轮 LLM 返回 %d 个工具调用: %s",
			round, len(resp.ToolCalls), strings.Join(toolNames, ", "))

		if roundThinking != "" && req.Callbacks.Stream != nil {
			thinkBlockID := req.Callbacks.BlockID.Next()
			req.Callbacks.Stream.OnAIThinking(ctx, req.StepID, thinkBlockID, roundThinking)
			req.Callbacks.Stream.OnAIThinkingComplete(ctx, req.StepID, thinkBlockID)
		}

		messages = append(messages, resp)
		toolResults := executeToolsConcurrently(
			ctx, resp.ToolCalls, round, req.ExecCtx, req.ToolRegistry,
			req.StepID, req.Callbacks, toolTimeout,
		)

		var roundToolCalls []ToolCallRecord
		roundHasFailure := false
		for _, r := range toolResults {
			messages = append(messages, schema.ToolMessage(r.result.GetLLMContent(), r.tc.ID))
			output.ToolCalls = append(output.ToolCalls, r.record)
			roundToolCalls = append(roundToolCalls, r.record)
			if r.record.IsError {
				roundHasFailure = true
			}
		}

		if roundHasFailure {
			consecutiveFailures++
		} else {
			consecutiveFailures = 0
		}

		if consecutiveFailures >= reflectionConsecFailLimit {
			reflectionPrompt := buildReflectionPrompt(round, consecutiveFailures, roundToolCalls)
			messages = append(messages, schema.UserMessage(reflectionPrompt))
			logger.Debug("[Agent] 第 %d 轮触发反思 (连续失败=%d)", round, consecutiveFailures)
		}

		output.AgentTrace.Rounds = append(output.AgentTrace.Rounds, AgentRound{
			Round:     round,
			Thinking:  roundThinking,
			ToolCalls: roundToolCalls,
		})
	}

	logger.Warn("[Agent] 轮次达到最大值 %d，生成最终回复 (stepID=%s)", maxRounds, req.StepID)
	resp, err := callLLM(ctx, req.ChatModel, messages, nil, req.Config, req.StepID, nil)
	if err != nil {
		logger.Debug("[Agent] 最终回复生成失败, stepID=%s: %v", req.StepID, err)
		return output, fmt.Errorf("最终回复生成失败: %w", err)
	}
	output.Content = resp.Content
	updateTokenUsage(output, resp)
	logger.Debug("[Agent] 执行完成, stepID=%s, 总轮次=%d, 总耗时=%v, tokens=%d",
		req.StepID, maxRounds, time.Since(startTime), output.TotalTokens)
	if req.Callbacks.Stream != nil {
		req.Callbacks.Stream.OnMessageComplete(ctx, req.StepID, toAIResult(output))
	}
	return output, nil
}

// buildReflectionPrompt 连续工具失败后引导 LLM 反思
func buildReflectionPrompt(round int, consecutiveFailures int, latestToolCalls []ToolCallRecord) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[反思] 已执行 %d 轮，最近 %d 轮工具调用失败。\n\n", round, consecutiveFailures))

	sb.WriteString("最近的工具调用：\n")
	for _, tc := range latestToolCalls {
		status := "成功"
		if tc.IsError {
			status = "失败"
		}
		sb.WriteString(fmt.Sprintf("- %s [%s]: %s\n", tc.ToolName, status, truncateRunes(tc.Result, 200)))
	}

	sb.WriteString("\n请分析失败原因，尝试替代方案。如果已有足够信息，请直接给出最终回答。")
	return sb.String()
}

// --- LLM 调用 ---

func callLLM(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	messages []*schema.Message,
	tools []*schema.ToolInfo,
	config *AIConfig,
	stepID string,
	callbacks *AgentCallbacks,
) (*schema.Message, error) {
	maxRetries := 2
	hasStream := config.Streaming && callbacks != nil && callbacks.Stream != nil

	for retry := 0; retry <= maxRetries; retry++ {
		var resp *schema.Message
		var err error

		logger.Debug("[LLM] 调用, model=%s, streaming=%v, tools=%d, messages=%d, stepID=%s",
			config.Model, config.Streaming, len(tools), len(messages), stepID)

		callStart := time.Now()

		if hasStream && len(tools) > 0 {
			resp, err = streamWithToolsCalls(ctx, chatModel, messages, tools, stepID, callbacks)
		} else if hasStream {
			resp, err = streamGenerate(ctx, chatModel, messages, stepID, callbacks)
		} else if len(tools) > 0 {
			resp, err = chatModel.Generate(ctx, messages, model.WithTools(tools))
		} else {
			resp, err = chatModel.Generate(ctx, messages)
		}

		callDuration := time.Since(callStart)

		if err == nil {
			logger.Debug("[LLM] 完成, 耗时=%v, content长度=%d, toolCalls=%d, stepID=%s",
				callDuration, len([]rune(resp.Content)), len(resp.ToolCalls), stepID)
			return resp, nil
		}

		errClass := classifyLLMError(err)

		if errClass == llmErrorTimeout && retry < maxRetries {
			backoff := time.Duration(retry+1) * 5 * time.Second
			logger.Warn("[LLM] 超时，%v 后重试 (%d/%d): %v", backoff, retry+1, maxRetries, err)
			time.Sleep(backoff)
			continue
		}

		if errClass == llmErrorContextWindow && retry < maxRetries {
			logger.Warn("[LLM] Context window 溢出，压缩后重试 (%d/%d)", retry+1, maxRetries)
			messages = compressMessagesCopy(messages)
			continue
		}

		return nil, err
	}

	return nil, fmt.Errorf("LLM 调用在 %d 次重试后仍然失败", maxRetries)
}

type llmErrorClass int

const (
	llmErrorUnknown llmErrorClass = iota
	llmErrorTimeout
	llmErrorContextWindow
	llmErrorRateLimit
	llmErrorAuth
)

func classifyLLMError(err error) llmErrorClass {
	if err == nil {
		return llmErrorUnknown
	}
	errMsg := strings.ToLower(err.Error())

	timeoutKeywords := []string{"deadline exceeded", "client.timeout", "timed out", "timeout exceeded"}
	for _, kw := range timeoutKeywords {
		if strings.Contains(errMsg, kw) {
			return llmErrorTimeout
		}
	}

	contextKeywords := []string{
		"context_length_exceeded", "context window", "maximum context length",
		"token limit", "too many tokens", "max_tokens",
		"prompt is too long", "request too large",
	}
	for _, kw := range contextKeywords {
		if strings.Contains(errMsg, kw) {
			return llmErrorContextWindow
		}
	}

	if strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "429") {
		return llmErrorRateLimit
	}
	if strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "401") || strings.Contains(errMsg, "invalid api key") {
		return llmErrorAuth
	}

	return llmErrorUnknown
}

func compressMessagesCopy(messages []*schema.Message) []*schema.Message {
	if len(messages) <= 4 {
		return messages
	}

	keepLast := 4
	if len(messages) < keepLast+2 {
		keepLast = 2
	}

	middleMessages := messages[1 : len(messages)-keepLast]
	if len(middleMessages) == 0 {
		return messages
	}

	summary := summarizeMessages(middleMessages)

	result := make([]*schema.Message, 0, 2+keepLast)
	result = append(result, messages[0])
	result = append(result, schema.UserMessage(fmt.Sprintf("[以下是之前对话的摘要]\n%s", summary)))
	result = append(result, messages[len(messages)-keepLast:]...)
	return result
}

func summarizeMessages(messages []*schema.Message) string {
	var sb strings.Builder
	sb.WriteString("对话历史摘要：\n")

	toolCallSummary := make(map[string]int)
	var keyExchanges []string

	for _, msg := range messages {
		switch msg.Role {
		case schema.User:
			content := msg.Content
			if len([]rune(content)) > 200 {
				content = string([]rune(content)[:200]) + "..."
			}
			if content != "" {
				keyExchanges = append(keyExchanges, fmt.Sprintf("用户: %s", content))
			}
		case schema.Assistant:
			content := msg.Content
			if len([]rune(content)) > 200 {
				content = string([]rune(content)[:200]) + "..."
			}
			if content != "" {
				keyExchanges = append(keyExchanges, fmt.Sprintf("助手: %s", content))
			}
			for _, tc := range msg.ToolCalls {
				toolCallSummary[tc.Function.Name]++
			}
		case schema.Tool:
			content := msg.Content
			if len([]rune(content)) > 150 {
				content = string([]rune(content)[:150]) + "..."
			}
			if content != "" {
				keyExchanges = append(keyExchanges, fmt.Sprintf("工具结果: %s", content))
			}
		}
	}

	if len(toolCallSummary) > 0 {
		sb.WriteString("已调用的工具: ")
		toolParts := make([]string, 0, len(toolCallSummary))
		for name, count := range toolCallSummary {
			if count > 1 {
				toolParts = append(toolParts, fmt.Sprintf("%s(×%d)", name, count))
			} else {
				toolParts = append(toolParts, name)
			}
		}
		sb.WriteString(strings.Join(toolParts, ", "))
		sb.WriteString("\n\n")
	}

	maxExchanges := 10
	if len(keyExchanges) > maxExchanges {
		keyExchanges = keyExchanges[len(keyExchanges)-maxExchanges:]
	}
	for _, exchange := range keyExchanges {
		sb.WriteString(exchange)
		sb.WriteString("\n")
	}

	result := sb.String()
	maxRunes := 2000
	if len([]rune(result)) > maxRunes {
		result = string([]rune(result)[:maxRunes]) + "..."
	}
	return result
}

// --- 流式调用 ---

func streamWithToolsCalls(ctx context.Context, chatModel model.ToolCallingChatModel, messages []*schema.Message, tools []*schema.ToolInfo, stepID string, callbacks *AgentCallbacks) (*schema.Message, error) {
	stream, err := chatModel.Stream(ctx, messages, model.WithTools(tools))
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var chunks []*schema.Message
	textBlockID := callbacks.BlockID.Next()
	thinkBlockID := ""
	hasThinking := false

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if chunk.ReasoningContent != "" {
			if thinkBlockID == "" {
				thinkBlockID = callbacks.BlockID.Next()
			}
			callbacks.Stream.OnAIThinking(ctx, stepID, thinkBlockID, chunk.ReasoningContent)
			hasThinking = true
		}
		if chunk.Content != "" {
			callbacks.Stream.OnAIChunk(ctx, stepID, textBlockID, chunk.Content)
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		return &schema.Message{Role: schema.Assistant}, nil
	}

	merged, err := schema.ConcatMessages(chunks)
	logger.Debug("[LLM Stream] 收到结果：content=%s, reasoning=%s", merged.Content, merged.ReasoningContent)

	if err != nil {
		return nil, fmt.Errorf("合并流式消息失败: %w", err)
	}

	if hasThinking {
		callbacks.Stream.OnAIThinkingComplete(ctx, stepID, thinkBlockID)
		merged.ReasoningContent = ""
	}

	return merged, nil
}

func streamGenerate(ctx context.Context, chatModel model.ToolCallingChatModel, messages []*schema.Message, stepID string, callbacks *AgentCallbacks) (*schema.Message, error) {
	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var contentBuilder strings.Builder
	resp := &schema.Message{Role: schema.Assistant}
	textBlockID := callbacks.BlockID.Next()

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
			callbacks.Stream.OnAIChunk(ctx, stepID, textBlockID, chunk.Content)
		}
		if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
			if resp.ResponseMeta == nil {
				resp.ResponseMeta = &schema.ResponseMeta{Usage: &schema.TokenUsage{}}
			}
			resp.ResponseMeta.Usage.PromptTokens += chunk.ResponseMeta.Usage.PromptTokens
			resp.ResponseMeta.Usage.CompletionTokens += chunk.ResponseMeta.Usage.CompletionTokens
			resp.ResponseMeta.Usage.TotalTokens += chunk.ResponseMeta.Usage.TotalTokens
		}
	}
	resp.Content = contentBuilder.String()
	return resp, nil
}

// --- 工具执行 ---

type toolCallResult struct {
	index  int
	tc     schema.ToolCall
	result *types.ToolResult
	record ToolCallRecord
}

var toolFallbackMap = map[string]string{}

const maxToolRetries = 1

func isRetryableError(err error, toolResult *types.ToolResult) bool {
	if err == nil && toolResult != nil && !toolResult.IsError {
		return false
	}
	errMsg := ""
	if err != nil {
		errMsg = strings.ToLower(err.Error())
	} else if toolResult != nil {
		errMsg = strings.ToLower(toolResult.GetLLMContent())
	}
	retryableKeywords := []string{"timeout", "timed out", "connection refused", "connection reset",
		"temporary failure", "service unavailable", "503", "502", "429", "rate limit"}
	for _, kw := range retryableKeywords {
		if strings.Contains(errMsg, kw) {
			return true
		}
	}
	return false
}

func executeToolsConcurrently(
	ctx context.Context,
	toolCalls []schema.ToolCall,
	round int,
	execCtx *executor.ExecutionContext,
	toolRegistry *executor.ToolRegistry,
	stepID string,
	callbacks *AgentCallbacks,
	toolTimeout time.Duration,
) []toolCallResult {
	results := make([]toolCallResult, len(toolCalls))
	var wg sync.WaitGroup

	sem := make(chan struct{}, defaultMaxToolConcurrent)

	if toolTimeout <= 0 {
		toolTimeout = defaultToolTimeout
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
				ID:        toolCall.ID,
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			}
			toolBlockID := callbacks.BlockID.Next()
			if callbacks.Stream != nil {
				callbacks.Stream.OnAIToolCallStart(toolCtx, stepID, toolBlockID, typesToolCall)
			}

			callStart := time.Now()
			toolName := toolCall.Function.Name

			toolResult, err := toolRegistry.Execute(toolCtx, toolName, toolCall.Function.Arguments, execCtx)
			if err != nil {
				toolResult = types.NewErrorResult(fmt.Sprintf("工具执行错误: %v", err))
			}

			if toolCtx.Err() == context.DeadlineExceeded && !toolResult.IsError {
				toolResult = types.NewErrorResult(fmt.Sprintf("工具 %s 执行超时 (%v)", toolName, toolTimeout))
			}

			if toolResult.IsError && isRetryableError(err, toolResult) {
				for retry := 0; retry < maxToolRetries; retry++ {
					time.Sleep(time.Duration(retry+1) * 2 * time.Second)
					retryCtx, retryCancel := context.WithTimeout(ctx, toolTimeout)
					toolResult, err = toolRegistry.Execute(retryCtx, toolName, toolCall.Function.Arguments, execCtx)
					retryCancel()
					if err != nil {
						toolResult = types.NewErrorResult(fmt.Sprintf("工具执行错误: %v", err))
					}
					if !toolResult.IsError {
						break
					}
				}
			}

			if toolResult.IsError {
				if fallbackTool, ok := toolFallbackMap[toolName]; ok {
					if toolRegistry.Has(fallbackTool) {
						fallbackCtx, fallbackCancel := context.WithTimeout(ctx, toolTimeout)
						fallbackResult, fallbackErr := toolRegistry.Execute(fallbackCtx, fallbackTool, toolCall.Function.Arguments, execCtx)
						fallbackCancel()
						if fallbackErr == nil && !fallbackResult.IsError {
							toolResult = fallbackResult
							toolResult.ForLLM = fmt.Sprintf("[原工具 %s 失败，已切换到 %s]\n%s", toolName, fallbackTool, fallbackResult.GetLLMContent())
						}
					}
				}
			}

			callDuration := time.Since(callStart)
			toolResult.ToolCallID = toolCall.ID
			typesToolCall.DurationMs = callDuration.Milliseconds()

			if callbacks.Stream != nil {
				callbacks.Stream.OnAIToolCallComplete(ctx, stepID, toolBlockID, typesToolCall, toolResult)
			}

			results[idx] = toolCallResult{
				index:  idx,
				tc:     toolCall,
				result: toolResult,
				record: ToolCallRecord{
					Round:     round,
					ToolName:  toolName,
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

// --- Token 统计 ---

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

// --- Schema 转换 ---

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

func toAIResult(output *AIOutput) *types.AIResult {
	return &types.AIResult{
		Content:          output.Content,
		PromptTokens:     output.PromptTokens,
		CompletionTokens: output.CompletionTokens,
		TotalTokens:      output.TotalTokens,
		Model:            output.Model,
		FinishReason:     output.FinishReason,
	}
}

func getToolTimeout(config *AIConfig) time.Duration {
	if config != nil && config.ToolTimeout > 0 {
		return time.Duration(config.ToolTimeout) * time.Second
	}
	return defaultToolTimeout
}

func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}

func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
