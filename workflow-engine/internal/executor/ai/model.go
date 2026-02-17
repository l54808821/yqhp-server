package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// interactiveSystemInstruction 当启用人机交互时追加到系统提示词中的指令
const interactiveSystemInstruction = `

[人机交互规则]
你可以使用 human_interaction 工具与用户进行实时交互。遵守以下规则：

1. 交互方式：当你需要用户确认、输入信息或做出选择时，必须调用 human_interaction 工具，禁止在回复文本中提问或等待用户回复。
   - 需要用户确认（是/否）：type 设为 "confirm"
   - 需要用户自由输入文本：type 设为 "input"
   - 需要用户从固定选项中选择：type 设为 "select"，并提供 options（用户界面会自动附带一个"其他"选项供用户自由输入，你不需要在 options 中手动添加"其他"）

2. 信息充分性：如果任务需要多项用户输入，请逐一通过工具询问，确保收集到所有必要信息后再开始生成最终内容。不要跳过问题，也不要替用户假设未提供的信息。

3. 最终输出：当所有必要信息收集完毕后，直接输出完整的最终内容。不要输出概要、大纲或摘要，除非用户明确要求。`

// createChatModel 创建 LLM 聊天模型
func (e *AIExecutor) createChatModel(ctx context.Context, config *AIConfig) (model.ChatModel, error) {
	chatConfig := &openai.ChatModelConfig{
		Model:  config.Model,
		APIKey: config.APIKey,
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		switch config.Provider {
		case "openai":
			baseURL = "https://api.openai.com/v1"
		case "deepseek":
			baseURL = "https://api.deepseek.com/v1"
		case "azure":
			chatConfig.ByAzure = true
			if config.APIVersion == "" {
				chatConfig.APIVersion = "2024-06-01"
			} else {
				chatConfig.APIVersion = config.APIVersion
			}
		}
	}
	if baseURL != "" {
		chatConfig.BaseURL = baseURL
	}

	if config.Temperature != nil {
		chatConfig.Temperature = config.Temperature
	}
	if config.MaxTokens != nil {
		chatConfig.MaxTokens = config.MaxTokens
	}
	if config.TopP != nil {
		chatConfig.TopP = config.TopP
	}
	if config.PresencePenalty != nil {
		chatConfig.PresencePenalty = config.PresencePenalty
	}

	return openai.NewChatModel(ctx, chatConfig)
}

// buildMessages 构建 LLM 消息列表
func (e *AIExecutor) buildMessages(config *AIConfig) []*schema.Message {
	var messages []*schema.Message

	systemPrompt := config.SystemPrompt
	if config.Interactive {
		systemPrompt += interactiveSystemInstruction
	}

	// 当挂载了 Skill 时，追加 Skill 能力说明
	if len(config.Skills) > 0 {
		systemPrompt += buildSkillInstruction(config.Skills)
	}

	if systemPrompt != "" {
		messages = append(messages, schema.SystemMessage(systemPrompt))
	}
	messages = append(messages, schema.UserMessage(config.Prompt))
	return messages
}

// buildSkillInstruction 构建 Skill 能力说明，追加到系统提示词中
func buildSkillInstruction(skills []*SkillInfo) string {
	var sb strings.Builder
	sb.WriteString("\n\n[专业能力（Skill）]\n")
	sb.WriteString("你拥有以下专业能力，当用户的问题需要某个专业领域的深度分析时，请调用对应的工具获取专业结果：\n\n")

	for _, skill := range skills {
		toolName := skillToolPrefix + sanitizeToolName(skill.Name)
		sb.WriteString(fmt.Sprintf("- %s: %s\n", toolName, skill.Description))
	}

	sb.WriteString("\n调用 Skill 时，请在 task 参数中提供完整的上下文信息，以便专家给出准确的结果。")
	return sb.String()
}
