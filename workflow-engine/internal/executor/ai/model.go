package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// reactSystemInstruction 当启用 ReAct 模式时追加到系统提示词中的指令
const reactSystemInstruction = `

[ReAct 推理模式]
在解决任务时，请严格遵循以下推理模式：

1. 思考（Thought）：分析当前情况，明确下一步需要做什么，评估应该使用哪个工具
2. 行动（Action）：调用合适的工具获取信息或执行操作
3. 观察（Observation）：分析工具返回的结果
4. 重复以上步骤，直到问题完全解决

重要规则：
- 在每次调用工具之前，必须先输出你的思考过程（Thought）
- 思考过程应包含：对当前情况的分析、选择该工具的原因、预期结果
- 当所有必要信息收集完毕后，输出最终的完整回答`

// planningInstruction Plan-and-Execute 模式的规划阶段提示词
const planningInstruction = `请为以下任务制定一个分步执行计划。

规则：
- 每个步骤应独立且具体，可以独立执行
- 步骤数量控制在 2-10 个之间
- 按逻辑顺序排列，后续步骤可以依赖前续步骤的结果
- 严格按以下 JSON 数组格式输出，不要输出其他任何内容

输出格式：
[
  {"step": 1, "task": "具体任务描述"},
  {"step": 2, "task": "具体任务描述"}
]`

// planStepExecutionInstruction Plan-and-Execute 模式的步骤执行提示词模板
const planStepExecutionInstruction = `你正在执行一个多步骤计划。

整体计划：
%s

当前执行：第 %d 步 - %s

%s

请执行当前步骤并提供结果。只需输出执行结果，不要重复计划内容。`

// planSynthesisInstruction Plan-and-Execute 模式的汇总阶段提示词模板
const planSynthesisInstruction = `请综合以下所有步骤的执行结果，给出最终完整的回答。

原始任务：
%s

各步骤执行结果：
%s

请直接输出最终答案，要求完整、准确、清晰。`

// reflectionCritiqueInstruction Reflection 模式的审视阶段提示词模板
const reflectionCritiqueInstruction = `请审视以下回答，从以下角度进行严格评估：

1. 准确性：信息是否正确？是否有事实错误？
2. 完整性：是否遗漏了重要内容？
3. 清晰度：表达是否清晰易懂？
4. 逻辑性：论述是否有逻辑连贯性？
5. 相关性：是否紧扣原始问题？

原始问题：
%s

待审视的回答：
%s

如果回答已经足够好，无需改进，请只输出 "LGTM"（不含引号）。
否则请列出具体的改进建议，每条建议要明确指出问题和改进方向。`

// reflectionImproveInstruction Reflection 模式的改进阶段提示词模板
const reflectionImproveInstruction = `请根据审视意见改进你的回答。

原始问题：
%s

当前回答：
%s

审视意见：
%s

请输出改进后的完整回答。注意要针对审视意见中指出的每个问题进行改进。`

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
	if config.AgentMode == "react" {
		systemPrompt += reactSystemInstruction
	}
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
