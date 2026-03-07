package ai

import (
	"fmt"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// PromptBuilder 动态提示词构建器
type PromptBuilder struct {
	config *AIConfig
	tools  []*types.ToolDefinition
}

func NewPromptBuilder(config *AIConfig, tools []*types.ToolDefinition) *PromptBuilder {
	return &PromptBuilder{
		config: config,
		tools:  tools,
	}
}

// Build 构建完整的系统提示词
func (pb *PromptBuilder) Build() string {
	var sections []string

	if pb.config.SystemPrompt != "" {
		sections = append(sections, pb.config.SystemPrompt)
	}

	if len(pb.tools) > 0 {
		sections = append(sections, agentInstruction)
	}

	if pb.config.Interactive {
		sections = append(sections, interactiveInstruction)
	}

	sections = append(sections, pb.buildDynamicContext())

	return strings.Join(sections, "\n")
}

func (pb *PromptBuilder) buildDynamicContext() string {
	now := time.Now()
	return fmt.Sprintf("\n[当前环境]\n- 当前时间: %s\n- 时区: %s",
		now.Format("2006-01-02 15:04:05"),
		now.Location().String(),
	)
}

const agentInstruction = `
[工作模式]
你是一个智能助手，能够使用工具完成各类任务。

- 简单问题直接回答，不要调用不必要的工具
- 复杂任务使用 todo_write 工具规划后逐步执行
- 多个独立的工具调用应同时发起以提高效率
- 工具失败时分析原因，尝试替代方案而不是简单重试
- 收集到足够信息后直接给出完整回答
- 避免重复调用已成功返回结果的工具`

const interactiveInstruction = `
[人机交互]
你可以使用 human_interaction 工具与用户交互：
- 需要确认（是/否）：type 设为 "confirm"
- 需要用户输入文本：type 设为 "input"
- 需要从选项中选择：type 设为 "select"，并提供 options`
