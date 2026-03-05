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

// AgentMode 标识 Agent 运行模式
type AgentMode string

const (
	AgentModeDirect AgentMode = "direct"
	AgentModeReAct  AgentMode = "react"
	AgentModePlan   AgentMode = "plan"
	AgentModeRouter AgentMode = "router"
)

// AgentRunner 所有 Agent 模式的统一执行接口
type AgentRunner interface {
	Mode() AgentMode
	Run(ctx context.Context, req *AgentRequest) (*AIOutput, error)
}

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

// NewAgentCallbacks 从 ExecutionCallback 中提取 AIStreamCallback
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

// --- 共享的 LLM 调用和工具执行逻辑 ---

// callLLM 统一的 LLM 调用入口，支持流式和非流式，带重试
// callbacks 可为 nil（非流式场景或不需要回调时）
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

		logger.Debug("[LLM] 调用 LLM, streaming=%v, tools数量=%d, messages数量=%d, stepID=%s",
			config.Streaming, len(tools), len(messages), stepID)

		if hasStream && len(tools) > 0 {
			resp, err = streamWithToolsCalls(ctx, chatModel, messages, tools, stepID, callbacks)
		} else if hasStream {
			resp, err = streamGenerate(ctx, chatModel, messages, stepID, callbacks)
		} else if len(tools) > 0 {
			resp, err = chatModel.Generate(ctx, messages, model.WithTools(tools))
		} else {
			resp, err = chatModel.Generate(ctx, messages)
		}

		if err == nil {
			return resp, nil
		}

		errClass := classifyLLMError(err)

		if errClass == llmErrorTimeout && retry < maxRetries {
			backoff := time.Duration(retry+1) * 5 * time.Second
			logger.Warn("[LLM] 调用超时，%v 后重试 (%d/%d): %v", backoff, retry+1, maxRetries, err)
			time.Sleep(backoff)
			continue
		}

		if errClass == llmErrorContextWindow && retry < maxRetries {
			logger.Warn("[LLM] Context window 溢出，压缩消息后重试 (%d/%d): %v", retry+1, maxRetries, err)
			messages = compressMessagesCopy(messages)
			continue
		}

		return nil, err
	}

	return nil, fmt.Errorf("LLM 调用在 %d 次重试后仍然失败", maxRetries)
}

type llmErrorClass int

const (
	llmErrorUnknown       llmErrorClass = iota
	llmErrorTimeout                     // 超时
	llmErrorContextWindow               // 上下文窗口溢出
	llmErrorRateLimit                   // 速率限制
	llmErrorAuth                        // 认证失败
)

// classifyLLMError 对 LLM 错误进行分类，用于决定重试策略
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

// compressMessagesCopy 智能压缩消息历史：对中间消息生成摘要而不是简单裁剪
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
	result = append(result, messages[0]) // system prompt
	result = append(result, schema.UserMessage(fmt.Sprintf("[以下是之前对话的摘要，供你参考]\n%s", summary)))
	result = append(result, messages[len(messages)-keepLast:]...)
	return result
}

// summarizeMessages 对中间消息提取关键信息生成摘要（无需 LLM 调用，基于规则压缩）
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

// streamWithToolsCalls 流式调用 LLM（带工具），收集 chunk 后合并为完整消息
func streamWithToolsCalls(ctx context.Context, chatModel model.ToolCallingChatModel, messages []*schema.Message, tools []*schema.ToolInfo, stepID string, callbacks *AgentCallbacks) (*schema.Message, error) {
	stream, err := chatModel.Stream(ctx, messages, model.WithTools(tools))
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var chunks []*schema.Message
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
			callbacks.Stream.OnAIChunk(ctx, stepID, textBlockID, chunk.Content)
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

// streamGenerate 流式调用 LLM（无工具），直接流式输出
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

// toolFallbackMap 定义工具降级映射：当一个工具失败时，可以尝试使用备选工具
var toolFallbackMap = map[string]string{
	"google_search": "bing_search",
	"bing_search":   "google_search",
}

const maxToolRetries = 1

// isRetryableError 判断工具错误是否可以重试
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

// executeToolsConcurrently 并发执行工具调用，支持自动重试和降级
func executeToolsConcurrently(
	ctx context.Context,
	toolCalls []schema.ToolCall,
	round int,
	execCtx *executor.ExecutionContext,
	toolRegistry *executor.ToolRegistry,
	stepID string,
	callbacks *AgentCallbacks,
	planStepIndex int,
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
			ID:            toolCall.ID,
			Name:          toolCall.Function.Name,
			Arguments:     toolCall.Function.Arguments,
			PlanStepIndex: planStepIndex,
		}
		toolBlockID := callbacks.BlockID.Next()
		if callbacks.Stream != nil {
			callbacks.Stream.OnAIToolCallStart(toolCtx, stepID, toolBlockID, typesToolCall)
		}

			callStart := time.Now()
			toolName := toolCall.Function.Name
			logger.Debug("[ToolExec] 开始执行工具 [%s], round=%d, 参数=%s", toolName, round, toolCall.Function.Arguments)

			toolResult, err := toolRegistry.Execute(toolCtx, toolName, toolCall.Function.Arguments, execCtx)
			if err != nil {
				logger.Debug("[ToolExec] 工具 [%s] 执行出错: %v", toolName, err)
				toolResult = types.NewErrorResult(fmt.Sprintf("工具执行错误: %v", err))
			}

			if toolCtx.Err() == context.DeadlineExceeded && !toolResult.IsError {
				toolResult = types.NewErrorResult(fmt.Sprintf("工具 %s 执行超时 (%v)", toolName, toolTimeout))
			}

			// 可重试错误的自动重试
			if toolResult.IsError && isRetryableError(err, toolResult) {
				for retry := 0; retry < maxToolRetries; retry++ {
					logger.Debug("[ToolExec] 工具 [%s] 可重试错误，第 %d 次重试", toolName, retry+1)
					time.Sleep(time.Duration(retry+1) * 2 * time.Second)

					retryCtx, retryCancel := context.WithTimeout(ctx, toolTimeout)
					toolResult, err = toolRegistry.Execute(retryCtx, toolName, toolCall.Function.Arguments, execCtx)
					retryCancel()

					if err != nil {
						toolResult = types.NewErrorResult(fmt.Sprintf("工具执行错误: %v", err))
					}
					if !toolResult.IsError {
						logger.Debug("[ToolExec] 工具 [%s] 重试成功", toolName)
						break
					}
				}
			}

			// 降级到备选工具
			if toolResult.IsError {
				if fallbackTool, ok := toolFallbackMap[toolName]; ok {
					if toolRegistry.Has(fallbackTool) {
						logger.Debug("[ToolExec] 工具 [%s] 失败，降级到 [%s]", toolName, fallbackTool)
						fallbackCtx, fallbackCancel := context.WithTimeout(ctx, toolTimeout)
						fallbackResult, fallbackErr := toolRegistry.Execute(fallbackCtx, fallbackTool, toolCall.Function.Arguments, execCtx)
						fallbackCancel()

						if fallbackErr == nil && !fallbackResult.IsError {
							toolResult = fallbackResult
							toolResult.ForLLM = fmt.Sprintf("[原工具 %s 失败，已自动切换到 %s]\n%s", toolName, fallbackTool, fallbackResult.GetLLMContent())
							logger.Debug("[ToolExec] 降级到 [%s] 成功", fallbackTool)
						}
					}
				}
			}

			callDuration := time.Since(callStart)
			logger.Debug("[ToolExec] 工具 [%s] 执行完成, 耗时=%v, isError=%v, 结果=%s",
				toolName, callDuration, toolResult.IsError, truncateForLog(toolResult.GetLLMContent(), 500))

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

// appendToolResults 将工具执行结果追加到消息列表和输出中
func appendToolResults(messages []*schema.Message, output *AIOutput, results []toolCallResult) []*schema.Message {
	for _, r := range results {
		toolMsg := schema.ToolMessage(r.result.GetLLMContent(), r.tc.ID)
		messages = append(messages, toolMsg)
		output.ToolCalls = append(output.ToolCalls, r.record)
	}
	return messages
}

// truncateForLog 截断字符串用于日志输出，避免超长日志
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}

// --- 自我验证 ---

const selfVerifyPrompt = `请验证你即将给出的回答，按以下维度检查：

1. 完整性：回答是否覆盖了用户问题的所有方面？有没有遗漏的要点？
2. 准确性：回答中的事实和数据是否正确？是否有未经验证的假设？
3. 逻辑性：推理过程是否合理？结论是否从前提中自然得出？
4. 实用性：回答是否可操作？用户能否直接使用？

如果发现问题，请修正后输出改进版本。如果回答已经足够好，直接输出原回答即可（可做微调润色）。

你之前的回答草稿：
`

// selfVerifyWithCallbacks 对生成的回答进行自我验证
// 验证阶段使用非流式调用：草稿已流式推送，验证后内容通过 message_complete 覆盖
func selfVerifyWithCallbacks(ctx context.Context, chatModel model.ToolCallingChatModel, config *AIConfig, stepID string, originalContent string, output *AIOutput, callbacks *AgentCallbacks) string {
	if config.EnableSelfVerify == nil || !*config.EnableSelfVerify {
		return originalContent
	}
	if len([]rune(originalContent)) < 100 {
		return originalContent
	}

	logger.Debug("[SelfVerify] 开始自我验证, stepID=%s, 原始回答长度=%d", stepID, len([]rune(originalContent)))

	var verifyBlockID string
	if callbacks != nil && callbacks.Stream != nil {
		verifyBlockID = callbacks.BlockID.Next()
		callbacks.Stream.OnAIVerify(ctx, stepID, verifyBlockID, "verifying", false)
	}

	verifyMessages := []*schema.Message{
		schema.SystemMessage("你是一个严谨的回答质量检查员。"),
		schema.UserMessage(selfVerifyPrompt + originalContent),
	}

	verifyConfig := *config
	verifyConfig.Streaming = false

	resp, err := callLLM(ctx, chatModel, verifyMessages, nil, &verifyConfig, stepID, nil)
	if err != nil {
		logger.Warn("[SelfVerify] 自我验证调用失败: %v, 返回原始回答", err)
		if callbacks != nil && callbacks.Stream != nil {
			callbacks.Stream.OnAIVerify(ctx, stepID, verifyBlockID, "completed", false)
		}
		return originalContent
	}

	updateTokenUsage(output, resp)

	if resp.Content == "" {
		if callbacks != nil && callbacks.Stream != nil {
			callbacks.Stream.OnAIVerify(ctx, stepID, verifyBlockID, "completed", false)
		}
		return originalContent
	}

	verified := true
	if output.AgentTrace != nil {
		output.AgentTrace.Verified = true
	}

	if callbacks != nil && callbacks.Stream != nil {
		callbacks.Stream.OnAIVerify(ctx, stepID, verifyBlockID, "completed", verified)
	}

	logger.Debug("[SelfVerify] 自我验证完成, 原始长度=%d, 验证后长度=%d", len([]rune(originalContent)), len([]rune(resp.Content)))
	return resp.Content
}

// --- Token 统计辅助 ---

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

// --- Schema 转换辅助 ---

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

// getToolTimeout 从配置中获取工具超时时间
func getToolTimeout(config *AIConfig) time.Duration {
	if config != nil && config.ToolTimeout > 0 {
		return time.Duration(config.ToolTimeout) * time.Second
	}
	return defaultToolTimeout
}
