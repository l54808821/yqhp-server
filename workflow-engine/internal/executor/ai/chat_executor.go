package ai

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

const ChatExecutorType = "ai_chat"

// ChatExecutor 基础对话执行器：单轮 LLM 调用 + 可选工具（MaxIterations=1）
type ChatExecutor struct {
	*executor.BaseExecutor
}

func NewChatExecutor() *ChatExecutor {
	return &ChatExecutor{
		BaseExecutor: executor.NewBaseExecutor(ChatExecutorType),
	}
}

func (e *ChatExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

func (e *ChatExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	config, err := parseAIConfig(step.Config)
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime, err), nil
	}
	config = resolveConfigVariables(config, execCtx)

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

	// 收集工具
	mcpClient := createMCPClient(config)
	einoTools := CollectEinoTools(ctx, config, step.ID, mcpClient)

	var output *AIOutput

	if len(einoTools) > 0 {
		// 有工具时：使用 ChatModelAgent 但 MaxIterations=1（单轮）
		agent, agentErr := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
			Name:          "chat",
			Description:   "基础对话，单轮调用",
			Instruction:   config.SystemPrompt,
			Model:         chatModel,
			MaxIterations: 1,
			ToolsConfig: adk.ToolsConfig{
				ToolsNodeConfig: compose.ToolsNodeConfig{
					Tools: einoTools,
				},
			},
		})
		if agentErr != nil {
			return executor.CreateFailedResult(step.ID, startTime,
				executor.NewExecutionError(step.ID, "创建 Agent 失败", agentErr)), nil
		}

		output, err = e.runAgent(ctx, agent, config, step.ID, aiCallback)
	} else {
		// 无工具时：直接调用 ChatModel
		output, err = e.runDirect(ctx, chatModel, config, step.ID, aiCallback)
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return executor.CreateTimeoutResult(step.ID, startTime, timeout), nil
		}
		if aiCallback != nil {
			aiCallback.OnAIError(ctx, step.ID, err)
		}
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewExecutionError(step.ID, "AI 调用失败", err)), nil
	}

	output.SystemPrompt = config.SystemPrompt
	output.Prompt = config.Prompt

	result := executor.CreateSuccessResult(step.ID, startTime, output)
	result.Metrics["ai_prompt_tokens"] = float64(output.PromptTokens)
	result.Metrics["ai_completion_tokens"] = float64(output.CompletionTokens)
	result.Metrics["ai_total_tokens"] = float64(output.TotalTokens)
	return result, nil
}

func (e *ChatExecutor) runAgent(ctx context.Context, agent adk.Agent, config *AIConfig, stepID string, aiCallback types.AICallback) (*AIOutput, error) {
	input := &adk.AgentInput{
		Messages:       []*schema.Message{schema.UserMessage(config.Prompt)},
		EnableStreaming: config.Streaming && aiCallback != nil,
	}

	iter := agent.Run(ctx, input)
	output := &AIOutput{Model: config.Model}
	chunkIdx := 0

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return nil, event.Err
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			mo := event.Output.MessageOutput
			if mo.IsStreaming && aiCallback != nil {
				var contentBuilder strings.Builder
				for {
					msg, err := mo.MessageStream.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
						return nil, err
					}
					if msg.Content != "" {
						contentBuilder.WriteString(msg.Content)
						aiCallback.OnAIChunk(ctx, stepID, msg.Content, chunkIdx)
						chunkIdx++
					}
				}
				output.Content = contentBuilder.String()
			} else {
				msg, err := mo.GetMessage()
				if err != nil {
					return nil, err
				}
				output.Content = msg.Content
				if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
					output.PromptTokens += msg.ResponseMeta.Usage.PromptTokens
					output.CompletionTokens += msg.ResponseMeta.Usage.CompletionTokens
					output.TotalTokens += msg.ResponseMeta.Usage.TotalTokens
				}
			}
		}
	}

	if config.Streaming && aiCallback != nil {
		aiCallback.OnAIComplete(ctx, stepID, &types.AIResult{
			Content:          output.Content,
			PromptTokens:     output.PromptTokens,
			CompletionTokens: output.CompletionTokens,
			TotalTokens:      output.TotalTokens,
		})
	}

	return output, nil
}

func (e *ChatExecutor) runDirect(ctx context.Context, chatModel einomodel.ToolCallingChatModel, config *AIConfig, stepID string, aiCallback types.AICallback) (*AIOutput, error) {
	messages := buildSimpleMessages(config)

	if config.Streaming && aiCallback != nil {
		stream, err := chatModel.Stream(ctx, messages)
		if err == nil {
			return executeStreamDirect(ctx, stream, stepID, config, aiCallback)
		}
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, err
	}

	output := &AIOutput{
		Content:      resp.Content,
		Model:        config.Model,
		FinishReason: string(resp.ResponseMeta.FinishReason),
	}
	if resp.ResponseMeta.Usage != nil {
		output.PromptTokens = resp.ResponseMeta.Usage.PromptTokens
		output.CompletionTokens = resp.ResponseMeta.Usage.CompletionTokens
		output.TotalTokens = resp.ResponseMeta.Usage.TotalTokens
	}
	return output, nil
}

func buildSimpleMessages(config *AIConfig) []*schema.Message {
	var messages []*schema.Message
	if config.SystemPrompt != "" {
		messages = append(messages, schema.SystemMessage(config.SystemPrompt))
	}
	messages = append(messages, schema.UserMessage(config.Prompt))
	return messages
}

func executeStreamDirect(ctx context.Context, stream *schema.StreamReader[*schema.Message], stepID string, config *AIConfig, callback types.AICallback) (*AIOutput, error) {
	defer stream.Close()

	var contentBuilder strings.Builder
	var finishReason string
	var usage *schema.TokenUsage
	index := 0

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
			callback.OnAIChunk(ctx, stepID, chunk.Content, index)
			index++
		}
		if chunk.ResponseMeta != nil {
			if chunk.ResponseMeta.FinishReason != "" {
				finishReason = string(chunk.ResponseMeta.FinishReason)
			}
			if chunk.ResponseMeta.Usage != nil {
				usage = chunk.ResponseMeta.Usage
			}
		}
	}

	output := &AIOutput{
		Content:      contentBuilder.String(),
		Model:        config.Model,
		FinishReason: finishReason,
	}
	if usage != nil {
		output.PromptTokens = usage.PromptTokens
		output.CompletionTokens = usage.CompletionTokens
		output.TotalTokens = usage.TotalTokens
	}

	callback.OnAIComplete(ctx, stepID, &types.AIResult{
		Content:          output.Content,
		PromptTokens:     output.PromptTokens,
		CompletionTokens: output.CompletionTokens,
		TotalTokens:      output.TotalTokens,
	})

	return output, nil
}

func (e *ChatExecutor) Cleanup(ctx context.Context) error { return nil }
