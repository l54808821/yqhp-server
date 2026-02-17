package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// executeReflection 执行 Reflection 模式
// 流程：起草 → (审视 → 改进) × N → 最终输出
func (e *AIExecutor) executeReflection(
	ctx context.Context,
	chatModel model.ChatModel,
	messages []*schema.Message,
	config *AIConfig,
	stepID string,
	execCtx *executor.ExecutionContext,
	aiCallback types.AICallback,
) (*AIOutput, error) {
	output := &AIOutput{
		Model: config.Model,
		AgentTrace: &AgentTrace{
			Mode:       "reflection",
			Reflection: &ReflectionTrace{},
		},
	}

	trace := output.AgentTrace.Reflection

	maxRounds := config.MaxReflectionRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxReflectionRounds
	}

	// ========== Phase 1: 起草 ==========
	// 通知前端：起草阶段
	if aiCallback != nil {
		if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
			tc.OnAIThinking(ctx, stepID, 0, "[Draft] 生成初稿...")
		}
	}

	var currentDraft string

	if e.hasTools(config) {
		draftOutput, err := e.executeWithTools(ctx, chatModel, messages, config, stepID, execCtx, aiCallback)
		if err != nil {
			return nil, fmt.Errorf("起草阶段失败: %w", err)
		}
		currentDraft = draftOutput.Content
		e.accumulateTokensFromOutput(output, draftOutput)
		output.ToolCalls = draftOutput.ToolCalls
	} else {
		if config.Streaming && aiCallback != nil {
			draftOutput, err := e.executeStream(ctx, chatModel, messages, stepID, config, aiCallback)
			if err != nil {
				return nil, fmt.Errorf("起草阶段失败: %w", err)
			}
			currentDraft = draftOutput.Content
			e.accumulateTokensFromOutput(output, draftOutput)
		} else {
			draftResp, err := chatModel.Generate(ctx, messages)
			if err != nil {
				return nil, fmt.Errorf("起草阶段失败: %w", err)
			}
			currentDraft = draftResp.Content
			e.accumulateTokens(output, draftResp)
		}
	}

	// ========== Phase 2: 审视-改进循环 ==========
	for round := 1; round <= maxRounds; round++ {
		// --- 审视阶段 ---
		if aiCallback != nil {
			if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
				tc.OnAIThinking(ctx, stepID, round, fmt.Sprintf("[Critique Round %d] 审视当前回答...", round))
			}
		}

		critiquePrompt := fmt.Sprintf(reflectionCritiqueInstruction, config.Prompt, currentDraft)
		critiqueMessages := []*schema.Message{
			schema.SystemMessage(config.SystemPrompt),
			schema.UserMessage(critiquePrompt),
		}

		critiqueResp, err := chatModel.Generate(ctx, critiqueMessages)
		if err != nil {
			return nil, fmt.Errorf("审视阶段失败 (round %d): %w", round, err)
		}
		e.accumulateTokens(output, critiqueResp)

		critique := critiqueResp.Content

		// 通知前端：审视结果
		if aiCallback != nil {
			if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
				tc.OnAIThinking(ctx, stepID, round, fmt.Sprintf("[Critique Result]\n%s", critique))
			}
		}

		// 记录本轮
		reflRound := ReflectionRound{
			Round:    round,
			Draft:    currentDraft,
			Critique: critique,
		}
		trace.Rounds = append(trace.Rounds, reflRound)

		// 如果审视结果为 LGTM，说明无需改进
		if isLGTM(critique) {
			break
		}

		// --- 改进阶段 ---
		if aiCallback != nil {
			if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
				tc.OnAIThinking(ctx, stepID, round, fmt.Sprintf("[Improve Round %d] 根据审视意见改进...", round))
			}
		}

		improvePrompt := fmt.Sprintf(reflectionImproveInstruction, config.Prompt, currentDraft, critique)
		improveMessages := []*schema.Message{
			schema.SystemMessage(config.SystemPrompt),
			schema.UserMessage(improvePrompt),
		}

		if config.Streaming && aiCallback != nil && round == maxRounds {
			// 最后一轮改进使用流式输出
			improvedOutput, improveErr := e.executeStream(ctx, chatModel, improveMessages, stepID, config, aiCallback)
			if improveErr != nil {
				return nil, fmt.Errorf("改进阶段失败 (round %d): %w", round, improveErr)
			}
			currentDraft = improvedOutput.Content
			e.accumulateTokensFromOutput(output, improvedOutput)
		} else {
			improvedResp, improveErr := chatModel.Generate(ctx, improveMessages)
			if improveErr != nil {
				return nil, fmt.Errorf("改进阶段失败 (round %d): %w", round, improveErr)
			}
			currentDraft = improvedResp.Content
			e.accumulateTokens(output, improvedResp)
		}
	}

	// 设置最终输出
	output.Content = currentDraft
	trace.FinalAnswer = currentDraft

	return output, nil
}

// isLGTM 判断审视结果是否表示无需改进
func isLGTM(critique string) bool {
	trimmed := strings.TrimSpace(strings.ToUpper(critique))
	return trimmed == "LGTM" ||
		strings.HasPrefix(trimmed, "LGTM") ||
		strings.Contains(trimmed, "无需改进") ||
		strings.Contains(trimmed, "已经足够好")
}
