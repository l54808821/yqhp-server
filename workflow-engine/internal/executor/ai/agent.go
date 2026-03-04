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

// AgentCallbacks 聚合所有回调接口，避免在每个方法签名中传递多个回调
type AgentCallbacks struct {
	AI       types.AICallback
	Tool     types.AIToolCallback
	Thinking types.AIThinkingCallback
	Plan     types.AIPlanCallback
}

// NewAgentCallbacks 从 AICallback 中提取所有可选回调接口
func NewAgentCallbacks(aiCallback types.AICallback) *AgentCallbacks {
	if aiCallback == nil {
		return &AgentCallbacks{}
	}
	cb := &AgentCallbacks{AI: aiCallback}
	if tc, ok := aiCallback.(types.AIToolCallback); ok {
		cb.Tool = tc
	}
	if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
		cb.Thinking = tc
	}
	if pc, ok := aiCallback.(types.AIPlanCallback); ok {
		cb.Plan = pc
	}
	return cb
}

// --- 共享的 LLM 调用和工具执行逻辑 ---

// callLLM 统一的 LLM 调用入口，支持流式和非流式，带重试
func callLLM(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	messages []*schema.Message,
	tools []*schema.ToolInfo,
	config *AIConfig,
	stepID string,
	aiCallback types.AICallback,
) (*schema.Message, error) {
	maxRetries := 2

	for retry := 0; retry <= maxRetries; retry++ {
		var resp *schema.Message
		var err error

		if config.Streaming && aiCallback != nil && len(tools) > 0 {
			resp, err = streamWithToolsCalls(ctx, chatModel, messages, tools, stepID, config, aiCallback)
		} else if config.Streaming && aiCallback != nil {
			resp, err = streamGenerate(ctx, chatModel, messages, stepID, aiCallback)
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
			log.Printf("[WARN] LLM 调用超时，%v 后重试 (%d/%d): %v", backoff, retry+1, maxRetries, err)
			time.Sleep(backoff)
			continue
		}

		if errClass == llmErrorContextWindow && retry < maxRetries {
			log.Printf("[WARN] Context window 溢出，压缩消息后重试 (%d/%d): %v", retry+1, maxRetries, err)
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

// compressMessagesCopy 压缩消息历史，返回新 slice（修复原 compressMessages 的 bug）
func compressMessagesCopy(messages []*schema.Message) []*schema.Message {
	if len(messages) <= 4 {
		return messages
	}
	// messages[0] = system prompt, 保留
	// 保留最后 2 条消息（最近一轮对话）
	conversation := messages[1 : len(messages)-2]
	if len(conversation) == 0 {
		return messages
	}
	mid := len(conversation) / 2
	kept := conversation[mid:]

	result := make([]*schema.Message, 0, 1+len(kept)+2)
	result = append(result, messages[0])
	result = append(result, kept...)
	result = append(result, messages[len(messages)-2:]...)
	return result
}

// streamWithToolsCalls 流式调用 LLM（带工具），收集 chunk 后合并为完整消息
func streamWithToolsCalls(ctx context.Context, chatModel model.ToolCallingChatModel, messages []*schema.Message, tools []*schema.ToolInfo, stepID string, config *AIConfig, callback types.AICallback) (*schema.Message, error) {
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

// streamGenerate 流式调用 LLM（无工具），直接流式输出
func streamGenerate(ctx context.Context, chatModel model.ToolCallingChatModel, messages []*schema.Message, stepID string, callback types.AICallback) (*schema.Message, error) {
	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var contentBuilder strings.Builder
	var idx int
	resp := &schema.Message{Role: schema.Assistant}

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
			callback.OnAIChunk(ctx, stepID, chunk.Content, idx)
			idx++
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

// executeToolsConcurrently 并发执行工具调用
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
			if callbacks.Tool != nil {
				callbacks.Tool.OnAIToolCallStart(toolCtx, stepID, typesToolCall)
			}

			callStart := time.Now()

			toolResult, err := toolRegistry.Execute(toolCtx, toolCall.Function.Name, toolCall.Function.Arguments, execCtx)
			if err != nil {
				toolResult = types.NewErrorResult(fmt.Sprintf("工具执行错误: %v", err))
			}

			callDuration := time.Since(callStart)

			if toolCtx.Err() == context.DeadlineExceeded && !toolResult.IsError {
				toolResult = types.NewErrorResult(fmt.Sprintf("工具 %s 执行超时 (%v)", toolCall.Function.Name, toolTimeout))
			}

			toolResult.ToolCallID = toolCall.ID

			if callbacks.Tool != nil {
				callbacks.Tool.OnAIToolCallComplete(ctx, stepID, typesToolCall, toolResult)
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

// appendToolResults 将工具执行结果追加到消息列表和输出中
func appendToolResults(messages []*schema.Message, output *AIOutput, results []toolCallResult) []*schema.Message {
	for _, r := range results {
		toolMsg := schema.ToolMessage(r.result.GetLLMContent(), r.tc.ID)
		messages = append(messages, toolMsg)
		output.ToolCalls = append(output.ToolCalls, r.record)
	}
	return messages
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
