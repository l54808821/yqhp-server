package ai

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

const ReflectionExecutorType = "ai_reflection"

// ReflectionExecutor 反思迭代执行器：使用 LoopAgent 组合 Critique + Improve
type ReflectionExecutor struct {
	*executor.BaseExecutor
}

func NewReflectionExecutor() *ReflectionExecutor {
	return &ReflectionExecutor{
		BaseExecutor: executor.NewBaseExecutor(ReflectionExecutorType),
	}
}

func (e *ReflectionExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

func (e *ReflectionExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
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

	maxRounds := config.MaxReflectionRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxReflectionRounds
	}

	critiqueInstruction := config.CritiquePrompt
	if critiqueInstruction == "" {
		critiqueInstruction = `你是一个严格的质量审查专家。请审视提供的内容，从准确性、完整性、清晰度、逻辑性和相关性等角度进行评估。
如果内容已足够好，请只输出 "LGTM"。否则请列出具体的改进建议。`
	}

	improveInstruction := config.ImprovePrompt
	if improveInstruction == "" {
		improveInstruction = "你是一个内容改进专家。请根据审视意见对内容进行针对性改进，输出改进后的完整内容。"
	}

	// 使用 LoopAgent 组合 Critique + Improve
	critiqueAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "critique",
		Description: "审视内容质量并给出改进建议",
		Instruction: critiqueInstruction,
		Model:       chatModel,
	})
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewExecutionError(step.ID, "创建 Critique Agent 失败", err)), nil
	}

	improveAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "improve",
		Description: "根据审视意见改进内容",
		Instruction: improveInstruction,
		Model:       chatModel,
	})
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewExecutionError(step.ID, "创建 Improve Agent 失败", err)), nil
	}

	loopAgent, err := adk.NewLoopAgent(ctx, &adk.LoopAgentConfig{
		Name:          "reflection_loop",
		Description:   "反思迭代循环：审视 → 改进",
		SubAgents:     []adk.Agent{critiqueAgent, improveAgent},
		MaxIterations: maxRounds,
	})
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewExecutionError(step.ID, "创建 LoopAgent 失败", err)), nil
	}

	// 先用一个 SequentialAgent: Draft → LoopAgent(Critique + Improve)
	draftAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "draft",
		Description: "生成初稿",
		Instruction: config.SystemPrompt,
		Model:       chatModel,
	})
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewExecutionError(step.ID, "创建 Draft Agent 失败", err)), nil
	}

	seqAgent, err := adk.NewSequentialAgent(ctx, &adk.SequentialAgentConfig{
		Name:        "reflection_pipeline",
		Description: "反思流水线：初稿 → 审视改进循环",
		SubAgents:   []adk.Agent{draftAgent, loopAgent},
	})
	if err != nil {
		return executor.CreateFailedResult(step.ID, startTime,
			executor.NewExecutionError(step.ID, "创建 SequentialAgent 失败", err)), nil
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:          seqAgent,
		EnableStreaming: config.Streaming && aiCallback != nil,
	})

	iter := runner.Query(ctx, config.Prompt)
	output := &AIOutput{
		Model: config.Model,
		AgentTrace: &AgentTrace{
			Mode:       "reflection",
			Reflection: &ReflectionTrace{},
		},
	}

	reflTrace := output.AgentTrace.Reflection
	currentRound := 0
	chunkIndex := 0
	var lastDraft string
	var lastCritique string

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return executor.CreateFailedResult(step.ID, startTime,
				executor.NewExecutionError(step.ID, "Reflection 执行失败", event.Err)), nil
		}

		if aiCallback != nil {
			if tc, ok2 := aiCallback.(types.AIThinkingCallback); ok2 {
				label := event.AgentName
				switch event.AgentName {
				case "draft":
					label = "[Draft] 生成初稿..."
				case "critique":
					label = "[Critique] 审视中..."
				case "improve":
					label = "[Improve] 改进中..."
				}
				tc.OnAIThinking(ctx, step.ID, currentRound, label)
			}
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			mo := event.Output.MessageOutput
			var content string
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
				content = sb.String()
			} else {
				msg, getErr := mo.GetMessage()
				if getErr == nil {
					content = msg.Content
					if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
						output.PromptTokens += msg.ResponseMeta.Usage.PromptTokens
						output.CompletionTokens += msg.ResponseMeta.Usage.CompletionTokens
						output.TotalTokens += msg.ResponseMeta.Usage.TotalTokens
					}
				}
			}

			output.Content = content

			switch event.AgentName {
			case "draft":
				lastDraft = content
			case "critique":
				lastCritique = content
			case "improve":
				currentRound++
				reflTrace.Rounds = append(reflTrace.Rounds, ReflectionRound{
					Round:    currentRound,
					Draft:    lastDraft,
					Critique: lastCritique,
				})
				lastDraft = content
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
	output.AgentTrace.Reflection.FinalAnswer = output.Content

	result := executor.CreateSuccessResult(step.ID, startTime, output)
	result.Metrics["ai_prompt_tokens"] = float64(output.PromptTokens)
	result.Metrics["ai_completion_tokens"] = float64(output.CompletionTokens)
	result.Metrics["ai_total_tokens"] = float64(output.TotalTokens)
	return result, nil
}

func (e *ReflectionExecutor) Cleanup(ctx context.Context) error { return nil }
