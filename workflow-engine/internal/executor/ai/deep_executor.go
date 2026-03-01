package ai

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

const DeepAgentExecutorType = "ai_deep_agent"

// DeepAgentExecutor 深度代理执行器：deep.New() 规划驱动 + 上下文隔离
type DeepAgentExecutor struct {
	*executor.BaseExecutor
}

func NewDeepAgentExecutor() *DeepAgentExecutor {
	return &DeepAgentExecutor{
		BaseExecutor: executor.NewBaseExecutor(DeepAgentExecutorType),
	}
}

func (e *DeepAgentExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

func (e *DeepAgentExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
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
		timeout = 15 * time.Minute
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

	// 创建子 Agent
	var subAgents []adk.Agent
	for _, nodeID := range config.SubAgentNodeIDs {
		subConfig := resolveSubAgentConfigFromCtx(nodeID, execCtx)
		if subConfig == nil {
			continue
		}

		subModel, subErr := createChatModelFromConfig(ctx, subConfig.AIConfig)
		if subErr != nil {
			continue
		}

		subAgent, subErr := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
			Name:        subConfig.agentName(),
			Description: subConfig.agentDescription(),
			Instruction: subConfig.SystemPrompt,
			Model:       subModel,
		})
		if subErr != nil {
			continue
		}
		subAgents = append(subAgents, subAgent)
	}

	if len(subAgents) == 0 {
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewConfigError("DeepAgent 需要至少配置一个子 Agent", nil)), nil
	}

	maxIter := config.MaxIterations
	if maxIter <= 0 {
		maxIter = 100
	}

	deepAgent, err := deep.New(ctx, &deep.Config{
		Name:         "deep_agent",
		ChatModel:    chatModel,
		SubAgents:    subAgents,
		MaxIteration: maxIter,
	})
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewExecutionError(step.ID, "创建 DeepAgent 失败", err)), nil
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:          deepAgent,
		EnableStreaming: config.Streaming && aiCallback != nil,
	})

	iter := runner.Query(ctx, config.Prompt)
	chunkIndex := 0
	output := &AIOutput{Model: config.Model}

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			output.SystemPrompt = config.SystemPrompt
			output.Prompt = config.Prompt
			if config.Streaming && aiCallback != nil {
				aiCallback.OnAIComplete(ctx, step.ID, &types.AIResult{
					Content: output.Content, PromptTokens: output.PromptTokens,
					CompletionTokens: output.CompletionTokens, TotalTokens: output.TotalTokens,
				})
			}
			result := executor.CreateSuccessResult(step.ID, startTime, output)
			result.Status = types.ResultStatusFailed
			result.Error = executor.NewExecutionError(step.ID, "DeepAgent 执行中断", event.Err)
			result.Metrics["ai_prompt_tokens"] = float64(output.PromptTokens)
			result.Metrics["ai_completion_tokens"] = float64(output.CompletionTokens)
			result.Metrics["ai_total_tokens"] = float64(output.TotalTokens)
			return result, nil
		}

		if aiCallback != nil {
			if tc, ok2 := aiCallback.(types.AIThinkingCallback); ok2 {
				tc.OnAIThinking(ctx, step.ID, 0, "["+event.AgentName+"] 执行中...")
			}
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			mo := event.Output.MessageOutput
			if mo.IsStreaming && aiCallback != nil {
				var sb strings.Builder
				for {
					msg, recvErr := mo.MessageStream.Recv()
					if recvErr == io.EOF {
						break
					}
					if recvErr != nil {
						break
					}
					if msg.Content != "" {
						sb.WriteString(msg.Content)
						aiCallback.OnAIChunk(ctx, step.ID, msg.Content, chunkIndex)
						chunkIndex++
					}
				}
				output.Content = sb.String()
			} else {
				msg, getErr := mo.GetMessage()
				if getErr == nil {
					output.Content = msg.Content
					if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
						output.PromptTokens += msg.ResponseMeta.Usage.PromptTokens
						output.CompletionTokens += msg.ResponseMeta.Usage.CompletionTokens
						output.TotalTokens += msg.ResponseMeta.Usage.TotalTokens
					}
				}
			}
		}
	}

	if config.Streaming && aiCallback != nil {
		aiCallback.OnAIComplete(ctx, step.ID, &types.AIResult{
			Content:          output.Content,
			PromptTokens:     output.PromptTokens,
			CompletionTokens: output.CompletionTokens,
			TotalTokens:      output.TotalTokens,
		})
	}

	output.SystemPrompt = config.SystemPrompt
	output.Prompt = config.Prompt

	result := executor.CreateSuccessResult(step.ID, startTime, output)
	result.Metrics["ai_prompt_tokens"] = float64(output.PromptTokens)
	result.Metrics["ai_completion_tokens"] = float64(output.CompletionTokens)
	result.Metrics["ai_total_tokens"] = float64(output.TotalTokens)
	return result, nil
}

// resolveSubAgentConfigFromCtx 包级函数版本
func resolveSubAgentConfigFromCtx(nodeID string, execCtx *executor.ExecutionContext) *subAgentConfig {
	if execCtx == nil || execCtx.Variables == nil {
		return nil
	}
	key := "__sub_agent_config__" + nodeID
	configRaw, ok := execCtx.Variables[key]
	if !ok {
		return nil
	}
	configMap, ok := configRaw.(map[string]any)
	if !ok {
		return nil
	}
	aiConfig, err := parseAIConfig(configMap)
	if err != nil {
		return nil
	}
	sac := &subAgentConfig{AIConfig: aiConfig}
	if name, ok := configMap["agent_name"].(string); ok {
		sac.name = name
	}
	if desc, ok := configMap["agent_description"].(string); ok {
		sac.description = desc
	}
	return sac
}

func (e *DeepAgentExecutor) Cleanup(ctx context.Context) error { return nil }
