package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	pkgExecutor "yqhp/workflow-engine/pkg/executor"
	"yqhp/workflow-engine/pkg/types"
)

const (
	// AIExecutorType AI 执行器类型标识符
	AIExecutorType = "ai"

	// AI 调用默认超时时间
	defaultAITimeout = 5 * time.Minute

	// defaultMaxToolRounds 默认最大工具调用轮次
	defaultMaxToolRounds = 10

	// defaultMCPProxyBaseURL 默认 MCP 代理服务地址
	defaultMCPProxyBaseURL = "http://localhost:8080"
)

// AIExecutor AI 节点执行器
type AIExecutor struct {
	*BaseExecutor
}

// NewAIExecutor 创建 AI 执行器
func NewAIExecutor() *AIExecutor {
	return &AIExecutor{
		BaseExecutor: NewBaseExecutor(AIExecutorType),
	}
}

// AIConfig AI 节点配置
type AIConfig struct {
	Provider        string   `json:"provider"`
	Model           string   `json:"model"`
	APIKey          string   `json:"api_key"`
	BaseURL         string   `json:"base_url,omitempty"`
	APIVersion      string   `json:"api_version,omitempty"`
	Temperature     *float32 `json:"temperature,omitempty"`
	MaxTokens       *int     `json:"max_tokens,omitempty"`
	TopP            *float32 `json:"top_p,omitempty"`
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`
	SystemPrompt    string   `json:"system_prompt,omitempty"`
	Prompt          string   `json:"prompt"`
	Streaming       bool     `json:"streaming"`
	Interactive     bool     `json:"interactive"`                  // 启用后，AI 可通过 human_interaction 工具主动请求用户交互
	InteractionTimeout int  `json:"interaction_timeout,omitempty"` // 交互超时时间（秒），0 使用默认 300s
	Timeout         int      `json:"timeout,omitempty"`            // AI 调用超时（秒），0 使用默认值
	Tools           []string `json:"tools,omitempty"`              // 启用的内置工具名称列表
	MCPServerIDs    []int64  `json:"mcp_server_ids,omitempty"`     // 引用的 MCP 服务器 ID 列表
	MaxToolRounds   int      `json:"max_tool_rounds,omitempty"`    // 最大工具调用轮次，默认 10
	MCPProxyBaseURL string   `json:"mcp_proxy_base_url,omitempty"` // MCP 代理服务地址
	Skills          []*SkillInfo `json:"skills,omitempty"`         // 挂载的 Skill 列表（由 gulu 层注入）
}

// SkillInfo Skill 能力信息（由 gulu 层从数据库查询后注入到 config）
type SkillInfo struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
}

// skillToolPrefix Skill 工具名称前缀
const skillToolPrefix = "skill__"

// humanInteractionToolName 人机交互工具名称
const humanInteractionToolName = "human_interaction"

// humanInteractionToolDef 返回人机交互工具的定义
func humanInteractionToolDef() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        humanInteractionToolName,
		Description: "当你需要用户确认、输入信息或从选项中选择时，调用此工具与用户进行交互。用户将看到你提供的提示并作出响应。仅在确实需要人类介入时使用。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"type": {
					"type": "string",
					"enum": ["confirm", "input", "select"],
					"description": "交互类型：confirm（确认/拒绝）、input（自由文本输入）、select（从选项中选择）"
				},
				"prompt": {
					"type": "string",
					"description": "展示给用户的提示信息，应清晰说明需要用户做什么"
				},
				"options": {
					"type": "array",
					"items": { "type": "string" },
					"description": "当 type 为 select 时，提供给用户的选项列表"
				},
				"default_value": {
					"type": "string",
					"description": "超时时使用的默认值"
				}
			},
			"required": ["type", "prompt"]
		}`),
	}
}

// AIOutput AI 节点输出
type AIOutput struct {
	Content          string                  `json:"content"`
	PromptTokens     int                     `json:"prompt_tokens"`
	CompletionTokens int                     `json:"completion_tokens"`
	TotalTokens      int                     `json:"total_tokens"`
	Model            string                  `json:"model"`
	FinishReason     string                  `json:"finish_reason"`
	SystemPrompt     string                  `json:"system_prompt,omitempty"`
	Prompt           string                  `json:"prompt"`
	ToolCalls        []ToolCallRecord        `json:"tool_calls,omitempty"`    // 所有工具调用记录
	ConsoleLogs      []types.ConsoleLogEntry `json:"console_logs,omitempty"` // 后置处理器产生的日志
}

// ToolCallRecord 工具调用记录
type ToolCallRecord struct {
	Round     int    `json:"round"`       // 第几轮
	ToolName  string `json:"tool_name"`   // 工具名称
	Arguments string `json:"arguments"`   // 调用参数
	Result    string `json:"result"`      // 执行结果
	IsError   bool   `json:"is_error"`    // 是否出错
	Duration  int64  `json:"duration_ms"` // 耗时（毫秒）
}

// Init 初始化 AI 执行器
func (e *AIExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

// Execute 执行 AI 节点
func (e *AIExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	config, err := e.parseConfig(step.Config)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	config = e.resolveVariables(config, execCtx)

	chatModel, err := e.createChatModel(ctx, config)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "创建 AI 模型失败", err)), nil
	}

	messages := e.buildMessages(config)

	timeout := step.Timeout
	if timeout <= 0 && config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}
	if timeout <= 0 {
		timeout = defaultAITimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var output *AIOutput
	var aiCallback types.AICallback
	if execCtx.Callback != nil {
		if cb, ok := execCtx.Callback.(types.AICallback); ok {
			aiCallback = cb
		}
	}

	// Task 9.4: 无工具模式向后兼容 - 当 tools 和 mcp_server_ids 均为空时，保持原有行为
	if e.hasTools(config) {
		// Task 9.3 & 9.6: 有工具配置时，进入 Tool Call Loop
		output, err = e.executeWithTools(ctx, chatModel, messages, config, step.ID, execCtx, aiCallback)
		// 工具调用路径完成后，发送 ai_complete 事件通知前端流式结束
		if err == nil && output != nil && aiCallback != nil && config.Streaming {
			aiCallback.OnAIComplete(ctx, step.ID, &types.AIResult{
				Content:          output.Content,
				PromptTokens:     output.PromptTokens,
				CompletionTokens: output.CompletionTokens,
				TotalTokens:      output.TotalTokens,
			})
		}
	} else {
		// 无工具模式：保持原有行为
		if config.Streaming && aiCallback != nil {
			output, err = e.executeStream(ctx, chatModel, messages, step.ID, config, aiCallback)
		} else {
			output, err = e.executeNonStream(ctx, chatModel, messages, config)
		}
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return CreateTimeoutResult(step.ID, startTime, timeout), nil
		}
		if aiCallback != nil {
			aiCallback.OnAIError(ctx, step.ID, err)
		}
		return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "AI 调用失败", err)), nil
	}

	// 将解析后的 prompt 写入输出，方便调试时查看实际输入
	output.SystemPrompt = config.SystemPrompt
	output.Prompt = config.Prompt

	// 执行后置处理器（extract_param、js_script、assertion 等）
	e.executePostProcessors(ctx, step, execCtx, output, startTime)

	result := CreateSuccessResult(step.ID, startTime, output)
	result.Metrics["ai_prompt_tokens"] = float64(output.PromptTokens)
	result.Metrics["ai_completion_tokens"] = float64(output.CompletionTokens)
	result.Metrics["ai_total_tokens"] = float64(output.TotalTokens)

	return result, nil
}

// executePostProcessors 执行 AI 节点的后置处理器。
// 将 AIOutput 转换为类似 HTTP 响应的 response 格式，复用 ProcessorExecutor 执行后置处理器。
func (e *AIExecutor) executePostProcessors(ctx context.Context, step *types.Step, execCtx *ExecutionContext, output *AIOutput, startTime time.Time) {
	if len(step.PostProcessors) == 0 || execCtx == nil {
		return
	}

	// 准备变量上下文
	variables := make(map[string]interface{})
	envVars := make(map[string]interface{})
	if execCtx.Variables != nil {
		for k, v := range execCtx.Variables {
			variables[k] = v
		}
	}

	procExecutor := pkgExecutor.NewProcessorExecutor(variables, envVars)

	// 将 AI 输出转换为 response 格式，使后置处理器可以通过 jsonpath、js_script 等方式提取数据
	// body 字段兼容 HTTP 的 extract_param（jsonpath 从 body 提取），content 作为语义别名
	toolCallsJSON := "[]"
	if len(output.ToolCalls) > 0 {
		if data, err := json.Marshal(output.ToolCalls); err == nil {
			toolCallsJSON = string(data)
		}
	}

	// 构建 response body：如果 AI 回复本身是合法 JSON，直接用原始内容；
	// 否则将完整输出包装为 JSON 对象，方便 jsonpath 提取
	responseBody := output.Content
	var jsonTest json.RawMessage
	if json.Unmarshal([]byte(output.Content), &jsonTest) != nil {
		// content 不是合法 JSON，包装为 JSON 对象
		wrapped := map[string]interface{}{
			"content":           output.Content,
			"model":             output.Model,
			"finish_reason":     output.FinishReason,
			"prompt_tokens":     output.PromptTokens,
			"completion_tokens": output.CompletionTokens,
			"total_tokens":      output.TotalTokens,
		}
		if data, err := json.Marshal(wrapped); err == nil {
			responseBody = string(data)
		}
	}

	procExecutor.SetResponse(map[string]interface{}{
		"body":              responseBody,
		"content":           output.Content,
		"model":             output.Model,
		"finish_reason":     output.FinishReason,
		"prompt_tokens":     output.PromptTokens,
		"completion_tokens": output.CompletionTokens,
		"total_tokens":      output.TotalTokens,
		"tool_calls":        toolCallsJSON,
		"duration":          time.Since(startTime).Milliseconds(),
	})

	postLogs := procExecutor.ExecuteProcessors(ctx, step.PostProcessors, "post")
	execCtx.AppendLogs(postLogs)

	// 追踪变量变更
	for _, entry := range postLogs {
		if entry.Type != types.LogTypeProcessor || entry.Processor == nil {
			continue
		}
		pOutput := entry.Processor.Output
		if pOutput == nil {
			continue
		}
		if entry.Processor.Type == "set_variable" || entry.Processor.Type == "extract_param" {
			varName, _ := pOutput["variableName"].(string)
			if varName == "" {
				continue
			}
			scope, _ := pOutput["scope"].(string)
			if scope == "" {
				scope = "temp"
			}
			source, _ := pOutput["source"].(string)
			if source == "" {
				source = entry.Processor.Type
			}
			execCtx.AppendLog(types.NewVariableChangeEntry(types.VariableChangeInfo{
				Name:     varName,
				OldValue: pOutput["oldValue"],
				NewValue: pOutput["value"],
				Scope:    scope,
				Source:   source,
			}))
			if scope == "env" {
				execCtx.MarkAsEnvVar(varName)
			}
		}
		if entry.Processor.Type == "js_script" {
			if varChanges, ok := pOutput["varChanges"].([]map[string]any); ok {
				for _, change := range varChanges {
					name, _ := change["name"].(string)
					if name == "" {
						continue
					}
					s, _ := change["scope"].(string)
					src, _ := change["source"].(string)
					execCtx.AppendLog(types.NewVariableChangeEntry(types.VariableChangeInfo{
						Name:     name,
						OldValue: change["oldValue"],
						NewValue: change["newValue"],
						Scope:    s,
						Source:   src,
					}))
					if s == "env" {
						execCtx.MarkAsEnvVar(name)
					}
				}
			}
		}
	}

	// 将处理器产生的变量变更回写到执行上下文
	if execCtx.Variables != nil {
		for k, v := range procExecutor.GetVariables() {
			execCtx.Variables[k] = v
		}
	}

	// 收集后置处理器日志到 AIOutput，方便前端展示
	allLogs := execCtx.FlushLogs()
	if len(allLogs) > 0 {
		output.ConsoleLogs = allLogs
	}
}

// Cleanup 清理资源
func (e *AIExecutor) Cleanup(ctx context.Context) error {
	return nil
}

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

func (e *AIExecutor) buildMessages(config *AIConfig) []*schema.Message {
	var messages []*schema.Message

	systemPrompt := config.SystemPrompt
	if config.Interactive {
		systemPrompt += interactiveSystemInstruction
	}

	// 当挂载了 Skill 时，追加 Skill 能力说明
	if len(config.Skills) > 0 {
		systemPrompt += e.buildSkillInstruction(config.Skills)
	}

	if systemPrompt != "" {
		messages = append(messages, schema.SystemMessage(systemPrompt))
	}
	messages = append(messages, schema.UserMessage(config.Prompt))
	return messages
}

// buildSkillInstruction 构建 Skill 能力说明，追加到系统提示词中
func (e *AIExecutor) buildSkillInstruction(skills []*SkillInfo) string {
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

// humanInteractionArgs 人机交互工具参数
type humanInteractionArgs struct {
	Type         string   `json:"type"`
	Prompt       string   `json:"prompt"`
	Options      []string `json:"options,omitempty"`
	DefaultValue string   `json:"default_value,omitempty"`
}

// executeHumanInteraction 执行人机交互工具调用
// 解析 AI 模型传入的参数，通过 SSE 回调发送交互请求给用户，阻塞等待响应后返回结果给 AI。
func (e *AIExecutor) executeHumanInteraction(ctx context.Context, arguments string, stepID string, config *AIConfig, callback types.AICallback) *types.ToolResult {
	var args humanInteractionArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("参数解析失败: %v", err),
		}
	}

	if args.Type == "" {
		args.Type = "confirm"
	}
	if args.Prompt == "" {
		return &types.ToolResult{
			IsError: true,
			Content: "缺少必填参数: prompt",
		}
	}

	// 构建交互请求
	request := &types.InteractionRequest{
		Type:         types.InteractionType(args.Type),
		Prompt:       args.Prompt,
		DefaultValue: args.DefaultValue,
		Timeout:      config.InteractionTimeout,
	}

	// 处理 select 类型的选项
	if args.Type == "select" && len(args.Options) > 0 {
		request.Options = make([]types.InteractionOption, len(args.Options))
		for i, opt := range args.Options {
			request.Options[i] = types.InteractionOption{Value: opt, Label: opt}
		}
	}

	if request.Timeout <= 0 {
		request.Timeout = 300
	}

	// 通过回调发送交互请求并等待用户响应
	resp, err := callback.OnAIInteractionRequired(ctx, stepID, request)
	if err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("交互处理失败: %v", err),
		}
	}

	if resp == nil || resp.Skipped {
		defaultVal := args.DefaultValue
		if defaultVal == "" {
			defaultVal = "(用户未响应)"
		}
		return &types.ToolResult{
			IsError: false,
			Content: fmt.Sprintf(`{"skipped": true, "value": %q}`, defaultVal),
		}
	}

	return &types.ToolResult{
		IsError: false,
		Content: fmt.Sprintf(`{"skipped": false, "value": %q}`, resp.Value),
	}
}

// executeSkillCall 执行 Skill 调用 -- 使用 Skill 的系统提示词发起子 LLM 调用
func (e *AIExecutor) executeSkillCall(ctx context.Context, skill *SkillInfo, arguments string, config *AIConfig) *types.ToolResult {
	var args struct {
		Task string `json:"task"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("Skill 参数解析失败: %v", err),
		}
	}

	if args.Task == "" {
		return &types.ToolResult{
			IsError: true,
			Content: "Skill 调用缺少 task 参数",
		}
	}

	// 使用 Skill 的系统提示词 + 用户传入的 task 构建消息
	messages := []*schema.Message{
		schema.SystemMessage(skill.SystemPrompt),
		schema.UserMessage(args.Task),
	}

	// 复用同一个模型创建子调用
	chatModel, err := e.createChatModel(ctx, config)
	if err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("Skill 创建模型失败: %v", err),
		}
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("Skill 执行失败: %v", err),
		}
	}

	return &types.ToolResult{
		IsError: false,
		Content: resp.Content,
	}
}

func (e *AIExecutor) parseConfig(config map[string]any) (*AIConfig, error) {
	aiConfig := &AIConfig{Provider: "openai", Streaming: false}

	if provider, ok := config["provider"].(string); ok {
		aiConfig.Provider = provider
	}
	if m, ok := config["model"].(string); ok {
		aiConfig.Model = m
	} else {
		return nil, NewConfigError("AI 节点需要配置 'model'", nil)
	}
	if apiKey, ok := config["api_key"].(string); ok {
		aiConfig.APIKey = apiKey
	} else {
		return nil, NewConfigError("AI 节点需要配置 'api_key'", nil)
	}
	if baseURL, ok := config["base_url"].(string); ok {
		aiConfig.BaseURL = baseURL
	}
	if apiVersion, ok := config["api_version"].(string); ok {
		aiConfig.APIVersion = apiVersion
	}
	if temp, ok := config["temperature"].(float64); ok {
		t := float32(temp)
		aiConfig.Temperature = &t
	}
	if maxTokens, ok := config["max_tokens"].(float64); ok {
		m := int(maxTokens)
		aiConfig.MaxTokens = &m
	}
	if topP, ok := config["top_p"].(float64); ok {
		t := float32(topP)
		aiConfig.TopP = &t
	}
	if pp, ok := config["presence_penalty"].(float64); ok {
		p := float32(pp)
		aiConfig.PresencePenalty = &p
	}
	if systemPrompt, ok := config["system_prompt"].(string); ok {
		aiConfig.SystemPrompt = systemPrompt
	}
	if prompt, ok := config["prompt"].(string); ok {
		aiConfig.Prompt = prompt
	} else {
		return nil, NewConfigError("AI 节点需要配置 'prompt'", nil)
	}
	if streaming, ok := config["streaming"].(bool); ok {
		aiConfig.Streaming = streaming
	}
	if interactive, ok := config["interactive"].(bool); ok {
		aiConfig.Interactive = interactive
	}
	if interactionTimeout, ok := config["interaction_timeout"].(float64); ok {
		aiConfig.InteractionTimeout = int(interactionTimeout)
	}
	if timeout, ok := config["timeout"].(float64); ok {
		aiConfig.Timeout = int(timeout)
	}

	// 解析工具相关配置 (Task 9.1)
	if tools, ok := config["tools"].([]any); ok {
		for _, t := range tools {
			if s, ok := t.(string); ok {
				aiConfig.Tools = append(aiConfig.Tools, s)
			}
		}
	}
	if mcpServerIDs, ok := config["mcp_server_ids"].([]any); ok {
		for _, id := range mcpServerIDs {
			if f, ok := id.(float64); ok {
				aiConfig.MCPServerIDs = append(aiConfig.MCPServerIDs, int64(f))
			}
		}
	}
	if maxToolRounds, ok := config["max_tool_rounds"].(float64); ok {
		aiConfig.MaxToolRounds = int(maxToolRounds)
	}
	if mcpProxyBaseURL, ok := config["mcp_proxy_base_url"].(string); ok {
		aiConfig.MCPProxyBaseURL = mcpProxyBaseURL
	}

	// 解析 Skill 列表（由 gulu 层注入的完整 Skill 信息）
	if skills, ok := config["skills"].([]any); ok {
		for _, s := range skills {
			if skillMap, ok := s.(map[string]any); ok {
				skill := &SkillInfo{}
				if id, ok := skillMap["id"].(float64); ok {
					skill.ID = int64(id)
				}
				if name, ok := skillMap["name"].(string); ok {
					skill.Name = name
				}
				if desc, ok := skillMap["description"].(string); ok {
					skill.Description = desc
				}
				if sp, ok := skillMap["system_prompt"].(string); ok {
					skill.SystemPrompt = sp
				}
				if skill.Name != "" && skill.SystemPrompt != "" {
					aiConfig.Skills = append(aiConfig.Skills, skill)
				}
			}
		}
	}

	return aiConfig, nil
}

func (e *AIExecutor) resolveVariables(config *AIConfig, execCtx *ExecutionContext) *AIConfig {
	if execCtx == nil {
		return config
	}

	evalCtx := execCtx.ToEvaluationContext()
	resolver := GetVariableResolver()
	config.APIKey = resolver.ResolveString(config.APIKey, evalCtx)
	config.SystemPrompt = resolver.ResolveString(config.SystemPrompt, evalCtx)
	config.Prompt = resolver.ResolveString(config.Prompt, evalCtx)
	config.BaseURL = resolver.ResolveString(config.BaseURL, evalCtx)
	config.MCPProxyBaseURL = resolver.ResolveString(config.MCPProxyBaseURL, evalCtx)

	return config
}

// hasTools 检查配置中是否启用了工具 (Task 9.4)
func (e *AIExecutor) hasTools(config *AIConfig) bool {
	return len(config.Tools) > 0 || len(config.MCPServerIDs) > 0 || config.Interactive || len(config.Skills) > 0
}

// getMCPProxyBaseURL 获取 MCP 代理服务地址
func (e *AIExecutor) getMCPProxyBaseURL(config *AIConfig) string {
	if config.MCPProxyBaseURL != "" {
		return config.MCPProxyBaseURL
	}
	if envURL := os.Getenv("MCP_PROXY_BASE_URL"); envURL != "" {
		return envURL
	}
	return defaultMCPProxyBaseURL
}

// collectToolDefinitions 收集所有工具定义（内置工具 + MCP 工具 + 人机交互工具）(Task 9.5, 9.6)
func (e *AIExecutor) collectToolDefinitions(ctx context.Context, config *AIConfig, mcpClient *MCPRemoteClient) ([]*types.ToolDefinition, error) {
	var allDefs []*types.ToolDefinition

	// 收集内置工具定义，跳过未知工具名称并记录警告 (Task 9.5)
	for _, toolName := range config.Tools {
		if DefaultToolRegistry.Has(toolName) {
			tool, _ := DefaultToolRegistry.Get(toolName)
			allDefs = append(allDefs, tool.Definition())
		} else {
			log.Printf("[WARN] 未知的内置工具名称，已跳过: %s", toolName)
		}
	}

	// 如果启用了人机交互，注入 human_interaction 工具
	if config.Interactive {
		allDefs = append(allDefs, humanInteractionToolDef())
	}

	// 收集 MCP 工具定义 (Task 9.6)
	if mcpClient != nil {
		for _, serverID := range config.MCPServerIDs {
			tools, err := mcpClient.GetTools(ctx, serverID)
			if err != nil {
				log.Printf("[WARN] 获取 MCP 服务器 %d 工具列表失败，已跳过: %v", serverID, err)
				continue
			}
			allDefs = append(allDefs, tools...)
		}
	}

	// 收集 Skill 工具定义
	for _, skill := range config.Skills {
		allDefs = append(allDefs, skillToToolDef(skill))
	}

	return allDefs, nil
}

// skillToToolDef 将 Skill 转换为工具定义，使 AI 可以像调用工具一样调用 Skill
func skillToToolDef(skill *SkillInfo) *types.ToolDefinition {
	toolName := skillToolPrefix + sanitizeToolName(skill.Name)
	return &types.ToolDefinition{
		Name:        toolName,
		Description: fmt.Sprintf("[Skill] %s", skill.Description),
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"task": {
					"type": "string",
					"description": "需要该专家处理的具体任务内容，请提供完整上下文"
				}
			},
			"required": ["task"]
		}`),
	}
}

// sanitizeToolName 将中文/特殊字符的 Skill 名称转换为合法的工具名称
func sanitizeToolName(name string) string {
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		} else if r >= 0x4e00 && r <= 0x9fff {
			result.WriteRune(r)
		} else {
			result.WriteRune('_')
		}
	}
	s := result.String()
	if s == "" {
		s = "unnamed"
	}
	return s
}

// findSkillByToolName 根据工具名称查找 Skill
func findSkillByToolName(toolName string, skills []*SkillInfo) *SkillInfo {
	for _, skill := range skills {
		if skillToolPrefix+sanitizeToolName(skill.Name) == toolName {
			return skill
		}
	}
	return nil
}

// toSchemaTools 将 ToolDefinition 列表转换为 eino schema 格式
func (e *AIExecutor) toSchemaTools(defs []*types.ToolDefinition) []*schema.ToolInfo {
	tools := make([]*schema.ToolInfo, 0, len(defs))
	for _, td := range defs {
		var params map[string]*schema.ParameterInfo
		// 尝试将 JSON Schema 解析为 map，然后用 NewParamsOneOfByJSONSchema
		// 但 ToolInfo 使用 ParamsOneOf，最简单的方式是用 jsonschema
		toolInfo := &schema.ToolInfo{
			Name: td.Name,
			Desc: td.Description,
		}
		if len(td.Parameters) > 0 {
			var jsonSchemaMap map[string]any
			if err := json.Unmarshal(td.Parameters, &jsonSchemaMap); err == nil {
				// 将 JSON Schema map 转换为 ParameterInfo
				params = jsonSchemaMapToParams(jsonSchemaMap)
				if params != nil {
					toolInfo.ParamsOneOf = schema.NewParamsOneOfByParams(params)
				}
			}
		}
		tools = append(tools, toolInfo)
	}
	return tools
}

// jsonSchemaMapToParams 将 JSON Schema map 转换为 schema.ParameterInfo map
func jsonSchemaMapToParams(schemaMap map[string]any) map[string]*schema.ParameterInfo {
	props, ok := schemaMap["properties"].(map[string]any)
	if !ok {
		return nil
	}

	// 获取 required 字段列表
	requiredSet := make(map[string]bool)
	if required, ok := schemaMap["required"].([]any); ok {
		for _, r := range required {
			if s, ok := r.(string); ok {
				requiredSet[s] = true
			}
		}
	}

	params := make(map[string]*schema.ParameterInfo, len(props))
	for name, propRaw := range props {
		prop, ok := propRaw.(map[string]any)
		if !ok {
			continue
		}
		paramInfo := &schema.ParameterInfo{
			Required: requiredSet[name],
		}
		if t, ok := prop["type"].(string); ok {
			paramInfo.Type = schema.DataType(t)
		}
		if desc, ok := prop["description"].(string); ok {
			paramInfo.Desc = desc
		}
		if enumVals, ok := prop["enum"].([]any); ok {
			for _, ev := range enumVals {
				if s, ok := ev.(string); ok {
					paramInfo.Enum = append(paramInfo.Enum, s)
				}
			}
		}
		// 处理嵌套 object
		if paramInfo.Type == schema.Object {
			if subProps := jsonSchemaMapToParams(prop); subProps != nil {
				paramInfo.SubParams = subProps
			}
		}
		// 处理 array 的 items
		if paramInfo.Type == schema.Array {
			if items, ok := prop["items"].(map[string]any); ok {
				elemInfo := &schema.ParameterInfo{}
				if t, ok := items["type"].(string); ok {
					elemInfo.Type = schema.DataType(t)
				}
				if desc, ok := items["description"].(string); ok {
					elemInfo.Desc = desc
				}
				paramInfo.ElemInfo = elemInfo
			}
		}
		params[name] = paramInfo
	}
	return params
}

// executeWithTools 带工具的 AI 执行（Tool Call Loop）(Task 9.3, 9.6)
func (e *AIExecutor) executeWithTools(ctx context.Context, chatModel model.ChatModel, messages []*schema.Message, config *AIConfig, stepID string, execCtx *ExecutionContext, aiCallback types.AICallback) (*AIOutput, error) {
	// 创建 MCP 远程客户端（如果有 MCP 服务器配置）
	var mcpClient *MCPRemoteClient
	if len(config.MCPServerIDs) > 0 {
		mcpClient = NewMCPRemoteClient(e.getMCPProxyBaseURL(config))
	}

	// 收集所有工具定义
	allToolDefs, err := e.collectToolDefinitions(ctx, config, mcpClient)
	if err != nil {
		return nil, fmt.Errorf("收集工具定义失败: %w", err)
	}

	// 如果没有有效的工具定义，回退到无工具模式
	if len(allToolDefs) == 0 {
		if config.Streaming && aiCallback != nil {
			return e.executeStream(ctx, chatModel, messages, stepID, config, aiCallback)
		}
		return e.executeNonStream(ctx, chatModel, messages, config)
	}

	// 转换为 eino schema 格式
	schemaTools := e.toSchemaTools(allToolDefs)

	// 确定最大轮次
	maxRounds := config.MaxToolRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxToolRounds
	}

	// 获取 AIToolCallback（如果实现了）
	var toolCallback types.AIToolCallback
	if aiCallback != nil {
		if tc, ok := aiCallback.(types.AIToolCallback); ok {
			toolCallback = tc
		}
	}

	return e.executeToolCallLoop(ctx, chatModel, messages, schemaTools, allToolDefs, config, stepID, execCtx, aiCallback, toolCallback, mcpClient, maxRounds)
}

// executeToolCallLoop 执行工具调用循环 (Task 9.3)
func (e *AIExecutor) executeToolCallLoop(
	ctx context.Context,
	chatModel model.ChatModel,
	messages []*schema.Message,
	schemaTools []*schema.ToolInfo,
	allToolDefs []*types.ToolDefinition,
	config *AIConfig,
	stepID string,
	execCtx *ExecutionContext,
	aiCallback types.AICallback,
	toolCallback types.AIToolCallback,
	mcpClient *MCPRemoteClient,
	maxRounds int,
) (*AIOutput, error) {
	output := &AIOutput{
		Model: config.Model,
	}

	for round := 1; round <= maxRounds; round++ {
		// 调用 LLM（带工具定义）
		var resp *schema.Message
		var err error

		if config.Streaming && aiCallback != nil {
			resp, err = e.executeStreamWithTools(ctx, chatModel, messages, schemaTools, stepID, config, aiCallback)
		} else {
			resp, err = chatModel.Generate(ctx, messages, model.WithTools(schemaTools))
		}
		if err != nil {
			return nil, err
		}

		// 更新 token 使用信息
		if resp.ResponseMeta != nil {
			if resp.ResponseMeta.Usage != nil {
				output.PromptTokens += resp.ResponseMeta.Usage.PromptTokens
				output.CompletionTokens += resp.ResponseMeta.Usage.CompletionTokens
				output.TotalTokens += resp.ResponseMeta.Usage.TotalTokens
			}
			if resp.ResponseMeta.FinishReason != "" {
				output.FinishReason = resp.ResponseMeta.FinishReason
			}
		}

		// 如果没有工具调用，返回最终结果
		if len(resp.ToolCalls) == 0 {
			output.Content = resp.Content
			return output, nil
		}

		// 将 assistant 消息（含 ToolCalls）追加到消息列表
		messages = append(messages, resp)

		// 并行执行所有工具调用
		type toolCallResult struct {
			index  int
			tc     schema.ToolCall
			result *types.ToolResult
			record ToolCallRecord
		}

		results := make([]toolCallResult, len(resp.ToolCalls))
		var wg sync.WaitGroup

		for i, tc := range resp.ToolCalls {
			wg.Add(1)
			go func(idx int, toolCall schema.ToolCall) {
				defer wg.Done()

				// 通知工具调用开始
				typesToolCall := &types.ToolCall{
					ID:        toolCall.ID,
					Name:      toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
				}
				if toolCallback != nil {
					toolCallback.OnAIToolCallStart(ctx, stepID, typesToolCall)
				}

				callStart := time.Now()
				toolResult := e.executeSingleToolCall(ctx, toolCall, execCtx, mcpClient, config.MCPServerIDs, allToolDefs, stepID, config, aiCallback)
				callDuration := time.Since(callStart)

				// 通知工具调用完成
				if toolCallback != nil {
					toolCallback.OnAIToolCallComplete(ctx, stepID, typesToolCall, toolResult)
				}

				results[idx] = toolCallResult{
					index:  idx,
					tc:     toolCall,
					result: toolResult,
					record: ToolCallRecord{
						Round:     round,
						ToolName:  toolCall.Function.Name,
						Arguments: toolCall.Function.Arguments,
						Result:    toolResult.Content,
						IsError:   toolResult.IsError,
						Duration:  callDuration.Milliseconds(),
					},
				}
			}(i, tc)
		}
		wg.Wait()

		// 按顺序追加工具结果消息并记录
		for _, r := range results {
			toolMsg := schema.ToolMessage(r.result.Content, r.tc.ID)
			messages = append(messages, toolMsg)
			output.ToolCalls = append(output.ToolCalls, r.record)
		}
	}

	// 达到最大轮次限制，进行最后一次调用（不带工具）获取最终回答
	log.Printf("[WARN] 工具调用轮次达到最大值 %d，停止循环", maxRounds)
	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		// 如果最后一次调用失败，返回已有内容
		return output, nil
	}
	output.Content = resp.Content
	if resp.ResponseMeta != nil {
		if resp.ResponseMeta.Usage != nil {
			output.PromptTokens += resp.ResponseMeta.Usage.PromptTokens
			output.CompletionTokens += resp.ResponseMeta.Usage.CompletionTokens
			output.TotalTokens += resp.ResponseMeta.Usage.TotalTokens
		}
		if resp.ResponseMeta.FinishReason != "" {
			output.FinishReason = resp.ResponseMeta.FinishReason
		}
	}
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

// executeSingleToolCall 执行单个工具调用 (Task 9.3)
func (e *AIExecutor) executeSingleToolCall(
	ctx context.Context,
	tc schema.ToolCall,
	execCtx *ExecutionContext,
	mcpClient *MCPRemoteClient,
	mcpServerIDs []int64,
	allToolDefs []*types.ToolDefinition,
	stepID string,
	config *AIConfig,
	aiCallback types.AICallback,
) *types.ToolResult {
	toolName := tc.Function.Name

	// 检查是否为人机交互工具
	if toolName == humanInteractionToolName {
		if aiCallback == nil {
			return &types.ToolResult{
				ToolCallID: tc.ID,
				Content:    "人机交互不可用：缺少回调接口",
				IsError:    true,
			}
		}
		result := e.executeHumanInteraction(ctx, tc.Function.Arguments, stepID, config, aiCallback)
		result.ToolCallID = tc.ID
		return result
	}

	// 检查是否为 Skill 工具调用
	if strings.HasPrefix(toolName, skillToolPrefix) {
		skill := findSkillByToolName(toolName, config.Skills)
		if skill != nil {
			result := e.executeSkillCall(ctx, skill, tc.Function.Arguments, config)
			result.ToolCallID = tc.ID
			return result
		}
	}

	// 检查是否为内置工具
	if DefaultToolRegistry.Has(toolName) {
		tool, _ := DefaultToolRegistry.Get(toolName)
		result, err := tool.Execute(ctx, tc.Function.Arguments, execCtx)
		if err != nil {
			return &types.ToolResult{
				ToolCallID: tc.ID,
				Content:    fmt.Sprintf("内置工具执行错误: %v", err),
				IsError:    true,
			}
		}
		result.ToolCallID = tc.ID
		return result
	}

	// 检查是否为 MCP 工具
	if mcpClient != nil && len(mcpServerIDs) > 0 {
		// 查找该工具属于哪个 MCP 服务器
		serverID := e.findMCPServerForTool(ctx, toolName, mcpServerIDs, mcpClient)
		if serverID > 0 {
			result, err := mcpClient.CallTool(ctx, serverID, toolName, tc.Function.Arguments)
			if err != nil {
				return &types.ToolResult{
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("MCP 工具调用失败: %v", err),
					IsError:    true,
				}
			}
			result.ToolCallID = tc.ID
			return result
		}
	}

	// 未知工具
	return &types.ToolResult{
		ToolCallID: tc.ID,
		Content:    fmt.Sprintf("未知工具: %s", toolName),
		IsError:    true,
	}
}

// findMCPServerForTool 查找工具所属的 MCP 服务器 ID
func (e *AIExecutor) findMCPServerForTool(ctx context.Context, toolName string, mcpServerIDs []int64, mcpClient *MCPRemoteClient) int64 {
	for _, serverID := range mcpServerIDs {
		tools, err := mcpClient.GetTools(ctx, serverID)
		if err != nil {
			continue
		}
		for _, t := range tools {
			if t.Name == toolName {
				return serverID
			}
		}
	}
	return 0
}

func init() {
	MustRegister(NewAIExecutor())
}
