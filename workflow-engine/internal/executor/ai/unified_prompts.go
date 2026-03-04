package ai

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// buildPlanningPrompt 构建规划阶段的提示词
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
请调用 create_plan 工具来提交你的计划。如果无法使用该工具，请严格按以下 JSON 数组格式输出：

[
  {"step": 1, "task": "具体任务描述"},
  {"step": 2, "task": "具体任务描述"}
]`)
	return sb.String()
}

// buildUnifiedMessages 构建消息列表
func buildUnifiedMessages(systemPrompt string, chatHistory []*schema.Message, config *AIConfig) []*schema.Message {
	var messages []*schema.Message

	if systemPrompt != "" {
		messages = append(messages, schema.SystemMessage(systemPrompt))
	}

	messages = append(messages, chatHistory...)
	messages = append(messages, schema.UserMessage(config.Prompt))

	return messages
}

// buildPlanStepPrompt 构建 Plan 步骤执行提示词
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

// buildSynthesisPrompt 构建 Plan 汇总阶段提示词
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

// buildHistorySummary 构建对话历史摘要
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
