package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// hasTools 检查配置中是否启用了工具
func (e *AIExecutor) hasTools(config *AIConfig) bool {
	return len(config.Tools) > 0 || len(config.MCPServerIDs) > 0 || config.Interactive || len(config.Skills) > 0 || len(config.KnowledgeBases) > 0
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

// collectToolDefinitions 收集所有工具定义（内置工具 + MCP 工具 + 人机交互工具 + Skill 工具）
func (e *AIExecutor) collectToolDefinitions(ctx context.Context, config *AIConfig, mcpClient *executor.MCPRemoteClient) ([]*types.ToolDefinition, error) {
	var allDefs []*types.ToolDefinition

	// 收集内置工具定义，跳过未知工具名称并记录警告
	for _, toolName := range config.Tools {
		if executor.DefaultToolRegistry.Has(toolName) {
			tool, _ := executor.DefaultToolRegistry.Get(toolName)
			allDefs = append(allDefs, tool.Definition())
		} else {
			log.Printf("[WARN] 未知的内置工具名称，已跳过: %s", toolName)
		}
	}

	// 如果启用了人机交互，注入 human_interaction 工具
	if config.Interactive {
		allDefs = append(allDefs, humanInteractionToolDef())
	}

	// 收集 MCP 工具定义
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

	// 收集知识库检索工具定义
	if len(config.KnowledgeBases) > 0 {
		var kbNames []string
		for _, kb := range config.KnowledgeBases {
			kbNames = append(kbNames, kb.Name)
		}
		allDefs = append(allDefs, knowledgeSearchToolDef(kbNames))
	}

	return allDefs, nil
}

// toSchemaTools 将 ToolDefinition 列表转换为 eino schema 格式
func (e *AIExecutor) toSchemaTools(defs []*types.ToolDefinition) []*schema.ToolInfo {
	tools := make([]*schema.ToolInfo, 0, len(defs))
	for _, td := range defs {
		toolInfo := &schema.ToolInfo{
			Name: td.Name,
			Desc: td.Description,
		}
		if len(td.Parameters) > 0 {
			var jsonSchemaMap map[string]any
			if err := json.Unmarshal(td.Parameters, &jsonSchemaMap); err == nil {
				params := jsonSchemaMapToParams(jsonSchemaMap)
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

// executeWithTools 带工具的 AI 执行（Tool Call Loop）
func (e *AIExecutor) executeWithTools(ctx context.Context, chatModel model.ChatModel, messages []*schema.Message, config *AIConfig, stepID string, execCtx *executor.ExecutionContext, aiCallback types.AICallback) (*AIOutput, error) {
	// 创建 MCP 远程客户端（如果有 MCP 服务器配置）
	var mcpClient *executor.MCPRemoteClient
	if len(config.MCPServerIDs) > 0 {
		mcpClient = executor.NewMCPRemoteClient(e.getMCPProxyBaseURL(config))
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

// executeToolCallLoop 执行工具调用循环
func (e *AIExecutor) executeToolCallLoop(
	ctx context.Context,
	chatModel model.ChatModel,
	messages []*schema.Message,
	schemaTools []*schema.ToolInfo,
	allToolDefs []*types.ToolDefinition,
	config *AIConfig,
	stepID string,
	execCtx *executor.ExecutionContext,
	aiCallback types.AICallback,
	toolCallback types.AIToolCallback,
	mcpClient *executor.MCPRemoteClient,
	maxRounds int,
) (*AIOutput, error) {
	output := &AIOutput{
		Model: config.Model,
	}

	// 初始化 AgentTrace（ReAct 模式或默认工具调用模式）
	if config.AgentMode == "react" {
		output.AgentTrace = &AgentTrace{Mode: "react"}
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

		// 捕获本轮的 thinking（模型在决定调用工具前的推理内容）
		roundThinking := resp.Content

		// 如果没有工具调用，返回最终结果
		if len(resp.ToolCalls) == 0 {
			output.Content = resp.Content
			return output, nil
		}

		// 通知 thinking（流式模式下实时推送推理过程）
		if roundThinking != "" && aiCallback != nil {
			if tc, ok := aiCallback.(types.AIThinkingCallback); ok {
				tc.OnAIThinking(ctx, stepID, round, roundThinking)
			}
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
		var roundToolCalls []ToolCallRecord
		for _, r := range results {
			toolMsg := schema.ToolMessage(r.result.Content, r.tc.ID)
			messages = append(messages, toolMsg)
			output.ToolCalls = append(output.ToolCalls, r.record)
			roundToolCalls = append(roundToolCalls, r.record)
		}

		// 追加到 AgentTrace（ReAct 模式）
		if output.AgentTrace != nil && output.AgentTrace.Mode == "react" {
			output.AgentTrace.ReAct = append(output.AgentTrace.ReAct, ReActRound{
				Round:     round,
				Thinking:  roundThinking,
				ToolCalls: roundToolCalls,
			})
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

// executeSingleToolCall 执行单个工具调用
func (e *AIExecutor) executeSingleToolCall(
	ctx context.Context,
	tc schema.ToolCall,
	execCtx *executor.ExecutionContext,
	mcpClient *executor.MCPRemoteClient,
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

	// 检查是否为知识库检索工具
	if toolName == knowledgeSearchToolName && len(config.KnowledgeBases) > 0 {
		result := e.executeKnowledgeSearch(ctx, tc.Function.Arguments, config.KnowledgeBases)
		result.ToolCallID = tc.ID
		return result
	}

	// 检查是否为 Skill 工具调用
	if len(config.Skills) > 0 && len(toolName) > len(skillToolPrefix) && toolName[:len(skillToolPrefix)] == skillToolPrefix {
		skill := findSkillByToolName(toolName, config.Skills)
		if skill != nil {
			result := e.executeSkillCall(ctx, skill, tc.Function.Arguments, config)
			result.ToolCallID = tc.ID
			return result
		}
	}

	// 检查是否为内置工具
	if executor.DefaultToolRegistry.Has(toolName) {
		tool, _ := executor.DefaultToolRegistry.Get(toolName)
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
func (e *AIExecutor) findMCPServerForTool(ctx context.Context, toolName string, mcpServerIDs []int64, mcpClient *executor.MCPRemoteClient) int64 {
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
