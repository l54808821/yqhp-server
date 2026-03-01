package ai

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/compose"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

const PlanExecuteExecutorType = "ai_plan_execute"

// PlanExecuteExecutor 规划执行执行器：Planner + Executor + Replanner
type PlanExecuteExecutor struct {
	*executor.BaseExecutor
}

func NewPlanExecuteExecutor() *PlanExecuteExecutor {
	return &PlanExecuteExecutor{
		BaseExecutor: executor.NewBaseExecutor(PlanExecuteExecutorType),
	}
}

func (e *PlanExecuteExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

func (e *PlanExecuteExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
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
		timeout = 10 * time.Minute
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

	plannerInstruction := config.PlannerPrompt
	if plannerInstruction == "" {
		plannerInstruction = "你是一个任务规划专家。请根据用户的需求，制定一个详细的分步执行计划。每个步骤应独立且具体。"
	}

	plannerAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "planner",
		Description: "制定详细的分步执行计划",
		Instruction: plannerInstruction,
		Model:       chatModel,
	})
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewExecutionError(step.ID, "创建 Planner Agent 失败", err)), nil
	}

	executorAgentConfig := &adk.ChatModelAgentConfig{
		Name:        "executor",
		Description: "执行计划中的具体步骤",
		Instruction: config.SystemPrompt,
		Model:       chatModel,
	}
	if len(einoTools) > 0 {
		executorAgentConfig.ToolsConfig = adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: einoTools,
			},
		}
	}
	executorAgent, err := adk.NewChatModelAgent(ctx, executorAgentConfig)
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewExecutionError(step.ID, "创建 Executor Agent 失败", err)), nil
	}

	peConfig := &planexecute.Config{
		Planner:  plannerAgent,
		Executor: executorAgent,
	}

	if config.EnableReplanner {
		replannerAgent, replanErr := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
			Name:        "replanner",
			Description: "评估执行进度并决定是否调整计划",
			Instruction: "你是一个项目评估专家。根据当前执行进度和结果，决定是继续执行剩余步骤、调整计划、还是完成任务。",
			Model:       chatModel,
		})
		if replanErr != nil {
			return executor.CreateFailedResult(step.ID, startTime,
				executor.NewExecutionError(step.ID, "创建 Replanner Agent 失败", replanErr)), nil
		}
		peConfig.Replanner = replannerAgent
	}

	peAgent, err := planexecute.New(ctx, peConfig)
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewExecutionError(step.ID, "创建 Plan-Execute Agent 失败", err)), nil
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:          peAgent,
		EnableStreaming: config.Streaming && aiCallback != nil,
	})

	iter := runner.Query(ctx, config.Prompt)
	chunkIndex := 0
	output := &AIOutput{
		Model: config.Model,
		AgentTrace: &AgentTrace{
			Mode:           "plan_and_execute",
			PlanAndExecute: &PlanExecTrace{},
		},
	}

	var lastErr error
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			lastErr = event.Err
			break
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
					msg, err := mo.MessageStream.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
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
				msg, err := mo.GetMessage()
				if err == nil {
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

	output.SystemPrompt = config.SystemPrompt
	output.Prompt = config.Prompt

	if config.Streaming && aiCallback != nil {
		aiCallback.OnAIComplete(ctx, step.ID, &types.AIResult{
			Content:          output.Content,
			PromptTokens:     output.PromptTokens,
			CompletionTokens: output.CompletionTokens,
			TotalTokens:      output.TotalTokens,
		})
	}

	// 即使出错，也返回已收集的部分内容
	result := executor.CreateSuccessResult(step.ID, startTime, output)
	if lastErr != nil {
		result.Status = types.ResultStatusFailed
		result.Error = executor.NewExecutionError(step.ID, "Plan-Execute 执行中断", lastErr)
	}
	result.Metrics["ai_prompt_tokens"] = float64(output.PromptTokens)
	result.Metrics["ai_completion_tokens"] = float64(output.CompletionTokens)
	result.Metrics["ai_total_tokens"] = float64(output.TotalTokens)
	return result, nil
}

func (e *PlanExecuteExecutor) Cleanup(ctx context.Context) error { return nil }
