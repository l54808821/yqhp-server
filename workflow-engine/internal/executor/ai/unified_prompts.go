package ai

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

const unifiedBaseInstruction = `
[工作模式]
你是一个能力全面的智能助手，能够自主分析问题并选择最佳策略来完成任务。

1. 简单问题：直接回答，不需要调用工具
2. 需要信息或操作：使用可用工具获取信息或执行操作（思考 → 行动 → 观察循环）

[ReAct 推理规则]
- 在每次调用工具之前，必须先输出你的思考过程
- 思考应包含：对当前情况的分析、选择该工具的原因、预期结果
- 当所有必要信息收集完毕后，直接输出最终的完整回答
- 善于组合使用多个工具来完成复杂任务
- 如果一个工具失败了，尝试用其他方式达成目标`

const planModeInstruction = `
3. 复杂多步任务：当你判断任务需要多个步骤的系统性规划时，调用 switch_to_plan 工具

[Plan 模式触发条件]
当你判断任务符合以下条件之一时，应调用 switch_to_plan 工具：
- 任务涉及 3 个以上独立子任务
- 需要按特定顺序执行多个操作
- 任务结果相互依赖，需要全局规划
调用时请在 reason 参数中说明为什么需要规划`

const interactiveInstruction = `

[人机交互规则]
你可以使用 human_interaction 工具与用户进行实时交互。规则：
1. 需要用户确认（是/否）：type 设为 "confirm"
2. 需要用户自由输入文本：type 设为 "input"
3. 需要用户从固定选项中选择：type 设为 "select"，并提供 options
4. 如果任务需要多项用户输入，请逐一通过工具询问
5. 所有必要信息收集完毕后，直接输出完整的最终内容`

const webToolsInstruction = `

[联网能力]
你可以通过以下工具获取互联网上的信息：
- web_search：搜索互联网获取最新信息、事实验证
- web_fetch：获取指定 URL 的网页内容
当用户的问题涉及实时信息、最新数据、或你不确定的事实时，应主动使用搜索工具。`

const codeToolInstruction = `

[代码执行]
你可以使用 code_execute 工具执行 Python 或 JavaScript 代码。适用场景：
- 数学计算和数据处理
- 格式转换和文本处理
- 生成图表或数据可视化
- 验证代码逻辑`

func buildPlanningPrompt(skills []*SkillInfo) string {
	var sb strings.Builder
	sb.WriteString(`请为当前任务制定一个分步执行计划。

规则：
- 每个步骤应独立且具体，可以独立执行
- 步骤数量控制在 2-10 个之间
- 按逻辑顺序排列，后续步骤可以依赖前续步骤的结果
- 每个步骤应最多对应一个专业能力（Skill）的调用，不要在一个步骤中混合多个不同功能的操作
- 步骤描述应明确指出该步骤的输入来源（如"使用第 1 步的结果"）
`)

	if len(skills) > 0 {
		sb.WriteString("\n你可用的专业能力（Skill）：\n")
		for _, s := range skills {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
		}
		sb.WriteString("\n请根据这些能力合理拆分步骤，确保每步只使用一个 Skill，不同功能拆分到不同步骤。\n")
	}

	sb.WriteString(`
严格按以下 JSON 数组格式输出，不要输出其他任何内容

输出格式：
[
  {"step": 1, "task": "具体任务描述"},
  {"step": 2, "task": "具体任务描述"}
]`)
	return sb.String()
}

func buildUnifiedSystemPrompt(config *AIConfig, hasTools bool) string {
	var sb strings.Builder

	if config.SystemPrompt != "" {
		sb.WriteString(config.SystemPrompt)
	}

	if hasTools {
		sb.WriteString(unifiedBaseInstruction)
	}

	if config.EnablePlanMode != nil && *config.EnablePlanMode && hasTools {
		sb.WriteString(planModeInstruction)
	}

	if config.Interactive {
		sb.WriteString(interactiveInstruction)
	}

	// 如果配置了联网工具，添加联网能力说明
	if hasTools {
		sb.WriteString(webToolsInstruction)
		sb.WriteString(codeToolInstruction)
	}

	if len(config.Skills) > 0 {
		sb.WriteString(buildSkillInstruction(config.Skills))
	}

	if len(config.KnowledgeBases) > 0 {
		sb.WriteString(buildKnowledgeInstruction(config.KnowledgeBases))
	}

	return sb.String()
}

func buildUnifiedMessages(systemPrompt string, chatHistory []*schema.Message, config *AIConfig) []*schema.Message {
	var messages []*schema.Message

	if systemPrompt != "" {
		messages = append(messages, schema.SystemMessage(systemPrompt))
	}

	messages = append(messages, chatHistory...)
	messages = append(messages, schema.UserMessage(config.Prompt))

	return messages
}

func buildPlanStepPrompt(originalTask string, steps []string, currentIndex int, previousResults []string) string {
	var sb strings.Builder

	sb.WriteString("你正在执行一个多步骤计划。\n\n")
	sb.WriteString("原始任务：\n")
	sb.WriteString(originalTask)
	sb.WriteString("\n\n整体计划：\n")
	for i, s := range steps {
		status := "待执行"
		if i < currentIndex {
			status = "已完成"
		} else if i == currentIndex {
			status = "当前"
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, status, s))
	}

	if len(previousResults) > 0 {
		sb.WriteString("\n已完成步骤的结果：\n")
		for i, r := range previousResults {
			sb.WriteString(fmt.Sprintf("步骤 %d 结果：%s\n\n", i+1, r))
		}
	}

	sb.WriteString(fmt.Sprintf("\n请执行当前步骤（第 %d 步）：%s\n", currentIndex+1, steps[currentIndex]))
	sb.WriteString(`
重要要求：
- 只需输出执行结果，不要重复计划内容
- 你必须在回复中包含完整的执行结果数据（如生成的文档全文），不要只给出摘要或简短描述，后续步骤需要使用这些完整数据`)

	return sb.String()
}

func buildSynthesisPrompt(originalTask string, steps []string, stepResults []string) string {
	var sb strings.Builder

	sb.WriteString("请综合以下所有步骤的执行结果，给出最终完整的回答。\n\n")
	sb.WriteString("原始任务：\n")
	sb.WriteString(originalTask)
	sb.WriteString("\n\n各步骤执行结果：\n")

	for i, task := range steps {
		result := ""
		if i < len(stepResults) {
			result = stepResults[i]
		}
		sb.WriteString(fmt.Sprintf("步骤 %d (%s)：\n%s\n\n", i+1, task, result))
	}

	sb.WriteString("请直接输出最终答案，要求完整、准确、清晰。")

	return sb.String()
}

const maxHistorySummaryRunes = 1500

func buildHistorySummary(messages []*schema.Message) string {
	var userAssistantPairs []struct{ user, assistant string }
	var currentUser string

	for _, msg := range messages {
		switch msg.Role {
		case schema.User:
			currentUser = msg.Content
		case schema.Assistant:
			if currentUser != "" && msg.Content != "" {
				userAssistantPairs = append(userAssistantPairs, struct{ user, assistant string }{currentUser, msg.Content})
				currentUser = ""
			}
		}
	}
	if currentUser != "" {
		userAssistantPairs = append(userAssistantPairs, struct{ user, assistant string }{currentUser, ""})
	}

	if len(userAssistantPairs) == 0 {
		return ""
	}

	maxTurns := 3
	start := len(userAssistantPairs) - maxTurns
	if start < 0 {
		start = 0
	}
	recent := userAssistantPairs[start:]

	var sb strings.Builder
	sb.WriteString("[对话上下文]\n")
	for _, pair := range recent {
		userContent := truncateRunes(pair.user, 300)
		sb.WriteString(fmt.Sprintf("用户: %s\n", userContent))
		if pair.assistant != "" {
			assistantContent := truncateRunes(pair.assistant, 300)
			sb.WriteString(fmt.Sprintf("助手: %s\n", assistantContent))
		}
		sb.WriteString("\n")
	}

	result := sb.String()
	if len([]rune(result)) > maxHistorySummaryRunes {
		runes := []rune(result)
		result = string(runes[:maxHistorySummaryRunes]) + "..."
	}
	return result
}

func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

func buildSkillInstruction(skills []*SkillInfo) string {
	var sb strings.Builder
	sb.WriteString("\n\n[可用专业能力（Skills）]\n")
	sb.WriteString("以下是你可以使用的专业能力。需要时调用 read_skill 工具加载完整指令，然后按指令使用现有工具完成任务。\n\n")

	for _, skill := range skills {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", skill.Name, skill.Description))
	}
	return sb.String()
}

func buildKnowledgeInstruction(kbs []*KnowledgeBaseInfo) string {
	var sb strings.Builder
	sb.WriteString("\n\n[知识库]\n")
	sb.WriteString("你已接入以下知识库，可随时通过 knowledge_search 工具检索更精确的信息：\n\n")

	for _, kb := range kbs {
		typeLabel := "向量知识库"
		if kb.Type == "graph" {
			typeLabel = "图知识库"
		}
		sb.WriteString(fmt.Sprintf("- %s (%s)\n", kb.Name, typeLabel))
	}

	sb.WriteString("\n当用户的问题可能需要专业知识或事实依据时，请主动使用 knowledge_search 工具检索。")
	return sb.String()
}
