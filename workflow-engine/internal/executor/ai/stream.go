package ai

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/pkg/types"
)

// executeNonStream 非流式调用 LLM
func (e *AIExecutor) executeNonStream(ctx context.Context, chatModel model.ChatModel, messages []*schema.Message, config *AIConfig) (*AIOutput, error) {
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

// executeStream 流式调用 LLM
func (e *AIExecutor) executeStream(ctx context.Context, chatModel model.ChatModel, messages []*schema.Message, stepID string, config *AIConfig, callback types.AICallback) (*AIOutput, error) {
	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var contentBuilder strings.Builder
	var index int
	var finishReason string
	var usage *schema.TokenUsage

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

// executeStreamWithTools 流式模式下带工具的 LLM 调用
func (e *AIExecutor) executeStreamWithTools(ctx context.Context, chatModel model.ChatModel, messages []*schema.Message, tools []*schema.ToolInfo, stepID string, config *AIConfig, callback types.AICallback) (*schema.Message, error) {
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

		// 流式输出文本内容
		if chunk.Content != "" && callback != nil {
			callback.OnAIChunk(ctx, stepID, chunk.Content, chunkIndex)
			chunkIndex++
		}

		chunks = append(chunks, chunk)
	}

	// 合并所有 chunks 为完整消息
	if len(chunks) == 0 {
		return &schema.Message{Role: schema.Assistant}, nil
	}

	merged, err := schema.ConcatMessages(chunks)
	if err != nil {
		return nil, fmt.Errorf("合并流式消息失败: %w", err)
	}

	return merged, nil
}
