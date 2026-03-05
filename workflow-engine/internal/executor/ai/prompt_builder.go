package ai

import (
	"fmt"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// PromptBuilder 分层动态提示词构建器
// 提示词结构：Identity → Capabilities → Tool Guidance → Constraints → Dynamic Context
type PromptBuilder struct {
	config  *AIConfig
	tools   []*types.ToolDefinition
	mode    AgentMode
}

// NewPromptBuilder 创建提示词构建器
func NewPromptBuilder(config *AIConfig, tools []*types.ToolDefinition, mode AgentMode) *PromptBuilder {
	return &PromptBuilder{
		config: config,
		tools:  tools,
		mode:   mode,
	}
}

// Build 构建完整的系统提示词
func (pb *PromptBuilder) Build() string {
	var sections []string

	// 用户自定义系统提示词（最高优先级）
	if pb.config.SystemPrompt != "" {
		sections = append(sections, pb.config.SystemPrompt)
	}

	// 根据模式构建工作模式说明
	if len(pb.tools) > 0 {
		sections = append(sections, pb.buildModeInstruction())
	}

	// 工具能力描述（动态生成）
	if len(pb.tools) > 0 {
		sections = append(sections, pb.buildToolGuidance())
	}

	// Skills 和知识库
	if len(pb.config.Skills) > 0 {
		sections = append(sections, pb.buildSkillSection())
	}
	if len(pb.config.KnowledgeBases) > 0 {
		sections = append(sections, pb.buildKnowledgeSection())
	}

	// 交互规则
	if pb.config.Interactive {
		sections = append(sections, interactiveInstruction)
	}

	// 动态上下文
	sections = append(sections, pb.buildDynamicContext())

	return strings.Join(sections, "\n")
}

// buildModeInstruction 根据 Agent 模式生成工作模式说明
func (pb *PromptBuilder) buildModeInstruction() string {
	switch pb.mode {
	case AgentModeReAct:
		return reactModeInstruction
	case AgentModePlan:
		return planOnlyModeInstruction
	case AgentModeRouter:
		enablePlan := pb.config.EnablePlanMode != nil && *pb.config.EnablePlanMode
		if enablePlan {
			return reactModeInstruction + "\n" + planModeInstruction
		}
		return reactModeInstruction
	default:
		return reactModeInstruction
	}
}

// buildToolGuidance 根据实际注册的工具动态生成工具使用指南
func (pb *PromptBuilder) buildToolGuidance() string {
	var sb strings.Builder

	// 按类别分组工具
	categories := pb.categorizeTools()

	for _, cat := range categories {
		if len(cat.tools) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n[%s]\n", cat.name))
		sb.WriteString(cat.description)
		sb.WriteString("\n")
		for _, t := range cat.tools {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
		}
	}

	return sb.String()
}

type toolCategory struct {
	name        string
	description string
	tools       []*types.ToolDefinition
}

func (pb *PromptBuilder) categorizeTools() []toolCategory {
	webTools := &toolCategory{
		name:        "联网能力",
		description: "你可以通过以下工具获取互联网上的信息。当用户的问题涉及实时信息、最新数据、或你不确定的事实时，应主动使用搜索工具。",
	}
	codeTools := &toolCategory{
		name:        "代码与命令执行",
		description: "你可以执行代码和命令来完成计算、数据处理、系统操作等任务。",
	}
	fileTools := &toolCategory{
		name:        "文件操作",
		description: "你可以读取、写入和编辑文件。",
	}
	dataTools := &toolCategory{
		name:        "数据处理",
		description: "你可以使用以下工具处理结构化数据。",
	}
	mcpTools := &toolCategory{
		name:        "MCP 外部工具",
		description: "你已接入 MCP（Model Context Protocol）外部服务提供的工具。当用户的问题涉及项目管理、工作流、执行记录等业务数据时，应优先使用这些工具获取准确信息。",
	}
	otherTools := &toolCategory{
		name:        "其他工具",
		description: "以下是其他可用工具。",
	}

	webNames := map[string]bool{"bing_search": true, "google_search": true, "web_fetch": true}
	codeNames := map[string]bool{"code_execute": true, "shell_exec": true}
	fileNames := map[string]bool{"read_file": true, "write_file": true, "edit_file": true, "list_dir": true, "append_file": true}
	dataNames := map[string]bool{"http_request": true, "json_parse": true, "var_read": true, "var_write": true}

	for _, t := range pb.tools {
		switch {
		case webNames[t.Name]:
			webTools.tools = append(webTools.tools, t)
		case codeNames[t.Name]:
			codeTools.tools = append(codeTools.tools, t)
		case fileNames[t.Name]:
			fileTools.tools = append(fileTools.tools, t)
		case dataNames[t.Name]:
			dataTools.tools = append(dataTools.tools, t)
		case strings.HasPrefix(t.Name, "mcp_"):
			mcpTools.tools = append(mcpTools.tools, t)
		default:
			otherTools.tools = append(otherTools.tools, t)
		}
	}

	return []toolCategory{*webTools, *codeTools, *fileTools, *dataTools, *mcpTools, *otherTools}
}

// buildSkillSection 构建 Skills 部分
func (pb *PromptBuilder) buildSkillSection() string {
	var sb strings.Builder
	sb.WriteString("\n[可用专业能力（Skills）]\n")
	sb.WriteString("以下是你可以使用的专业能力。需要时调用 read_skill 工具加载完整指令，然后按指令使用现有工具完成任务。\n\n")
	for _, skill := range pb.config.Skills {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", skill.Name, skill.Description))
	}
	return sb.String()
}

// buildKnowledgeSection 构建知识库部分
func (pb *PromptBuilder) buildKnowledgeSection() string {
	var sb strings.Builder
	sb.WriteString("\n[知识库]\n")
	sb.WriteString("你已接入以下知识库，可随时通过 knowledge_search 工具检索更精确的信息：\n\n")
	for _, kb := range pb.config.KnowledgeBases {
		typeLabel := "向量知识库"
		if kb.Type == "graph" {
			typeLabel = "图知识库"
		}
		sb.WriteString(fmt.Sprintf("- %s (%s)\n", kb.Name, typeLabel))
	}
	sb.WriteString("\n当用户的问题可能需要专业知识或事实依据时，请主动使用 knowledge_search 工具检索。")
	return sb.String()
}

// buildDynamicContext 构建动态上下文（时间、环境等）
func (pb *PromptBuilder) buildDynamicContext() string {
	now := time.Now()
	return fmt.Sprintf("\n[当前环境]\n- 当前时间: %s\n- 时区: %s",
		now.Format("2006-01-02 15:04:05"),
		now.Location().String(),
	)
}

// --- 模式提示词常量 ---

const reactModeInstruction = `
[工作模式]
你是一个能力全面的智能助手，能够自主分析问题并选择最佳策略来完成任务。

1. 简单问题：直接回答，不需要调用工具
2. 需要信息或操作：使用可用工具获取信息或执行操作（思考 → 行动 → 观察 → 反思循环）

[结构化推理框架]
在每次调用工具之前，必须先按以下框架输出思考过程：

1. 目标：我当前要解决什么问题？用户的核心需求是什么？
2. 现状：我已经知道什么信息？还缺少什么关键信息？
3. 策略：最优的下一步行动是什么？为什么选择这个工具/方法？
4. 风险：这个行动可能失败吗？如果失败，备选方案是什么？

[ReAct 推理规则]
- 当所有必要信息收集完毕后，直接输出最终的完整回答
- 善于组合使用多个工具来完成复杂任务：如果多个工具调用之间没有依赖关系，应同时调用以提高效率
- 如果一个工具失败了，分析失败原因，尝试用其他方式达成目标，而不是简单重试
- 如果需要多次调用同一工具，确保每次调用的参数不同
- 避免重复调用已经成功返回结果的工具
- 在执行多轮后仍未完成任务时，停下来反思已有进展，评估是否需要调整策略

[自我检查]
在输出最终回答前，快速检查：
- 回答是否完整覆盖了用户的所有需求？
- 是否有遗漏的信息或未处理的边界情况？
- 对于不确定的部分，是否已明确标注？`

const planOnlyModeInstruction = `
[工作模式]
你是一个能力全面的智能助手，使用计划驱动模式完成任务。

你将收到一个需要分步执行的任务。对于每个步骤：
1. 分析当前步骤需要做什么
2. 评估完成该步骤需要哪些工具和信息
3. 使用可用工具获取信息或执行操作
4. 检验执行结果是否满足步骤要求
5. 输出该步骤的完整执行结果

[推理规则]
- 在每次调用工具之前，先按"目标 → 现状 → 策略 → 风险"框架输出思考过程
- 只关注当前步骤，不要试图一次完成所有步骤
- 如果当前步骤的前提条件发生变化（如前序步骤结果与预期不同），应主动调整执行策略
- 确保输出完整的执行结果数据，后续步骤可能需要使用`

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
