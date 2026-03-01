package ai

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

const ReflectionExecutorType = "ai_reflection"

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

	output := &AIOutput{
		Model: config.Model,
		AgentTrace: &AgentTrace{
			Mode:       "reflection",
			Reflection: &ReflectionTrace{},
		},
	}
	reflTrace := output.AgentTrace.Reflection
	chunkIndex := 0

	// ========== Phase 1: 生成初稿 ==========
	if aiCallback != nil {
		if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
			tc.OnAIThinking(ctx, step.ID, 0, "[Draft] 生成初稿...")
		}
	}

	draftMessages := []*schema.Message{
		schema.SystemMessage(config.SystemPrompt),
		schema.UserMessage(config.Prompt),
	}

	var currentDraft string
	if config.Streaming && aiCallback != nil {
		stream, streamErr := chatModel.Stream(ctx, draftMessages)
		if streamErr != nil {
			return executor.CreateFailedResult(step.ID, startTime,
				executor.NewExecutionError(step.ID, "生成初稿失败", streamErr)), nil
		}
		var sb strings.Builder
		for {
			chunk, recvErr := stream.Recv()
			if recvErr == io.EOF {
				break
			}
			if recvErr != nil {
				break
			}
			if chunk.Content != "" {
				sb.WriteString(chunk.Content)
				aiCallback.OnAIChunk(ctx, step.ID, chunk.Content, chunkIndex)
				chunkIndex++
			}
			if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
				output.PromptTokens += chunk.ResponseMeta.Usage.PromptTokens
				output.CompletionTokens += chunk.ResponseMeta.Usage.CompletionTokens
				output.TotalTokens += chunk.ResponseMeta.Usage.TotalTokens
			}
		}
		stream.Close()
		currentDraft = sb.String()
	} else {
		resp, genErr := chatModel.Generate(ctx, draftMessages)
		if genErr != nil {
			return executor.CreateFailedResult(step.ID, startTime,
				executor.NewExecutionError(step.ID, "生成初稿失败", genErr)), nil
		}
		currentDraft = resp.Content
		if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
			output.PromptTokens += resp.ResponseMeta.Usage.PromptTokens
			output.CompletionTokens += resp.ResponseMeta.Usage.CompletionTokens
			output.TotalTokens += resp.ResponseMeta.Usage.TotalTokens
		}
	}
	output.Content = currentDraft

	critiqueInstruction := config.CritiquePrompt
	if critiqueInstruction == "" {
		critiqueInstruction = `你是一个严格的质量审查专家。请审视提供的内容，从准确性、完整性、清晰度、逻辑性和相关性等角度进行评估。
如果内容已足够好，无需改进，请只输出 "LGTM"（不含引号）。
否则请列出具体的改进建议，每条建议要明确指出问题和改进方向。`
	}

	improveInstruction := config.ImprovePrompt
	if improveInstruction == "" {
		improveInstruction = "你是一个内容改进专家。请根据审视意见对内容进行针对性改进，输出改进后的完整内容。"
	}

	// ========== Phase 2: 审视-改进循环 ==========
	for round := 1; round <= maxRounds; round++ {
		if ctx.Err() != nil {
			break
		}

		// --- 审视 ---
		if aiCallback != nil {
			if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
				tc.OnAIThinking(ctx, step.ID, round, fmt.Sprintf("[Critique Round %d] 审视中...", round))
			}
		}

		critiquePrompt := fmt.Sprintf("原始问题：\n%s\n\n待审视的回答：\n%s", config.Prompt, currentDraft)
		critiqueMessages := []*schema.Message{
			schema.SystemMessage(critiqueInstruction),
			schema.UserMessage(critiquePrompt),
		}

		critiqueResp, critiqueErr := chatModel.Generate(ctx, critiqueMessages)
		if critiqueErr != nil {
			break
		}
		critique := critiqueResp.Content
		if critiqueResp.ResponseMeta != nil && critiqueResp.ResponseMeta.Usage != nil {
			output.PromptTokens += critiqueResp.ResponseMeta.Usage.PromptTokens
			output.CompletionTokens += critiqueResp.ResponseMeta.Usage.CompletionTokens
			output.TotalTokens += critiqueResp.ResponseMeta.Usage.TotalTokens
		}

		if aiCallback != nil {
			if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
				tc.OnAIThinking(ctx, step.ID, round, fmt.Sprintf("[Critique Result]\n%s", critique))
			}
		}

		reflTrace.Rounds = append(reflTrace.Rounds, ReflectionRound{
			Round:    round,
			Draft:    currentDraft,
			Critique: critique,
		})

		if isLGTMResponse(critique) {
			break
		}

		// --- 改进 ---
		if aiCallback != nil {
			if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
				tc.OnAIThinking(ctx, step.ID, round, fmt.Sprintf("[Improve Round %d] 改进中...", round))
			}
		}

		improvePrompt := fmt.Sprintf("原始问题：\n%s\n\n当前回答：\n%s\n\n审视意见：\n%s\n\n请输出改进后的完整回答。", config.Prompt, currentDraft, critique)
		improveMessages := []*schema.Message{
			schema.SystemMessage(improveInstruction),
			schema.UserMessage(improvePrompt),
		}

		isLastRound := round == maxRounds
		if config.Streaming && aiCallback != nil && isLastRound {
			stream, streamErr := chatModel.Stream(ctx, improveMessages)
			if streamErr != nil {
				break
			}
			var sb strings.Builder
			for {
				chunk, recvErr := stream.Recv()
				if recvErr == io.EOF {
					break
				}
				if recvErr != nil {
					break
				}
				if chunk.Content != "" {
					sb.WriteString(chunk.Content)
					aiCallback.OnAIChunk(ctx, step.ID, chunk.Content, chunkIndex)
					chunkIndex++
				}
				if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
					output.PromptTokens += chunk.ResponseMeta.Usage.PromptTokens
					output.CompletionTokens += chunk.ResponseMeta.Usage.CompletionTokens
					output.TotalTokens += chunk.ResponseMeta.Usage.TotalTokens
				}
			}
			stream.Close()
			currentDraft = sb.String()
		} else {
			improveResp, improveErr := chatModel.Generate(ctx, improveMessages)
			if improveErr != nil {
				break
			}
			currentDraft = improveResp.Content
			if improveResp.ResponseMeta != nil && improveResp.ResponseMeta.Usage != nil {
				output.PromptTokens += improveResp.ResponseMeta.Usage.PromptTokens
				output.CompletionTokens += improveResp.ResponseMeta.Usage.CompletionTokens
				output.TotalTokens += improveResp.ResponseMeta.Usage.TotalTokens
			}
		}
		output.Content = currentDraft
	}

	output.SystemPrompt = config.SystemPrompt
	output.Prompt = config.Prompt
	reflTrace.FinalAnswer = output.Content

	if config.Streaming && aiCallback != nil {
		aiCallback.OnAIComplete(ctx, step.ID, &types.AIResult{
			Content:          output.Content,
			PromptTokens:     output.PromptTokens,
			CompletionTokens: output.CompletionTokens,
			TotalTokens:      output.TotalTokens,
		})
	}

	result := executor.CreateSuccessResult(step.ID, startTime, output)
	result.Metrics["ai_prompt_tokens"] = float64(output.PromptTokens)
	result.Metrics["ai_completion_tokens"] = float64(output.CompletionTokens)
	result.Metrics["ai_total_tokens"] = float64(output.TotalTokens)
	return result, nil
}

func isLGTMResponse(critique string) bool {
	trimmed := strings.TrimSpace(strings.ToUpper(critique))
	return trimmed == "LGTM" ||
		strings.HasPrefix(trimmed, "LGTM") ||
		strings.Contains(trimmed, "无需改进") ||
		strings.Contains(trimmed, "已经足够好")
}

func (e *ReflectionExecutor) Cleanup(ctx context.Context) error { return nil }
