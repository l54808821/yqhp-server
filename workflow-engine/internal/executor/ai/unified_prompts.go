package ai

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

const unifiedBaseInstruction = `
[工作模式]
你是一个智能助手，能够自主分析问题并选择最佳策略来完成任务。

1. 简单问题：直接回答，不需要调用工具
2. 需要信息或操作：使用可用工具获取信息或执行操作（思考 → 行动 → 观察循环）
3. 复杂多步任务：当你判断任务需要多个步骤的系统性规划时，调用 switch_to_plan 工具

[ReAct 推理规则]
- 在每次调用工具之前，必须先输出你的思考过程
- 思考应包含：对当前情况的分析、选择该工具的原因、预期结果
- 当所有必要信息收集完毕后，直接输出最终的完整回答`

const planModeInstruction = `

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

const planningPrompt = `请为当前任务制定一个分步执行计划。

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

func buildUnifiedSystemPrompt(config *AIConfig, hasTools bool) string {
	var sb strings.Builder

	if config.SystemPrompt != "" {
		sb.WriteString(config.SystemPrompt)
	}

	if hasTools {
		sb.WriteString(unifiedBaseInstruction)
	}

	if config.EnablePlanMode && hasTools {
		sb.WriteString(planModeInstruction)
	}

	if config.Interactive {
		sb.WriteString(interactiveInstruction)
	}

	if len(config.Skills) > 0 {
		sb.WriteString(buildSkillInstruction(config.Skills))
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
	sb.WriteString("只需输出执行结果，不要重复计划内容。")

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

func buildSkillInstruction(skills []*SkillInfo) string {
	var sb strings.Builder
	sb.WriteString("\n\n[专业能力（Skill）]\n")
	sb.WriteString("你拥有以下专业能力，当用户的问题需要某个专业领域的深度分析时，请调用对应的工具：\n\n")

	for _, skill := range skills {
		toolName := skillToolPrefix + sanitizeToolName(skill.Name)
		sb.WriteString(fmt.Sprintf("- %s: %s\n", toolName, skill.Description))
	}

	sb.WriteString("\n调用 Skill 时，请在 task 参数中提供完整的上下文信息。")
	return sb.String()
}
