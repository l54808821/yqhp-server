package ai

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// Eino ADK requires ToolCallingChatModel, createChatModelFromConfig returns this type

const AgentExecutorType = "ai_agent"

// AgentExecutor ReAct Agent 执行器：多轮「思考 -> 行动 -> 观察」循环
type AgentExecutor struct {
	*executor.BaseExecutor
}

func NewAgentExecutor() *AgentExecutor {
	return &AgentExecutor{
		BaseExecutor: executor.NewBaseExecutor(AgentExecutorType),
	}
}

func (e *AgentExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

func (e *AgentExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
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

	mcpClient := createMCPClient(config)
	einoTools := CollectEinoTools(ctx, config, step.ID, mcpClient)

	if len(einoTools) == 0 {
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewConfigError("ReAct Agent 需要至少配置一个工具", nil)), nil
	}

	maxIterations := config.MaxToolRounds
	if maxIterations <= 0 {
		maxIterations = defaultMaxToolRounds
	}

	instruction := config.SystemPrompt + reactSystemInstruction
	if config.Interactive {
		instruction += interactiveSystemInstruction
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "agent",
		Description:   "ReAct Agent，通过多轮推理和工具调用完成任务",
		Instruction:   instruction,
		Model:         chatModel,
		MaxIterations: maxIterations,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: einoTools,
			},
		},
	})
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewExecutionError(step.ID, "创建 Agent 失败", err)), nil
	}

	output, err := runAgentWithTrace(ctx, agent, config, step.ID, aiCallback)
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

// runAgentWithTrace 运行 Agent 并收集 AgentTrace
func runAgentWithTrace(ctx context.Context, agent adk.Agent, config *AIConfig, stepID string, aiCallback types.AICallback) (*AIOutput, error) {
	input := &adk.AgentInput{
		Messages:       []*schema.Message{schema.UserMessage(config.Prompt)},
		EnableStreaming: config.Streaming && aiCallback != nil,
	}

	iter := agent.Run(ctx, input)
	output := &AIOutput{
		Model:      config.Model,
		AgentTrace: &AgentTrace{Mode: "react"},
	}

	round := 0
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
						aiCallback.OnAIChunk(ctx, stepID, msg.Content, round)
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

				// 记录工具调用到 AgentTrace
				if len(msg.ToolCalls) > 0 {
					round++
					reactRound := ReActRound{
						Round:    round,
						Thinking: msg.Content,
					}
					for _, tc := range msg.ToolCalls {
						reactRound.ToolCalls = append(reactRound.ToolCalls, ToolCallRecord{
							Round:     round,
							ToolName:  tc.Function.Name,
							Arguments: tc.Function.Arguments,
						})
					}
					output.AgentTrace.ReAct = append(output.AgentTrace.ReAct, reactRound)
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

func (e *AgentExecutor) Cleanup(ctx context.Context) error { return nil }
