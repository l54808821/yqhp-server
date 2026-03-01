package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// execCtxKey 用于在 context.Context 中传递 ExecutionContext
type execCtxKeyType struct{}

var execCtxKey = execCtxKeyType{}

// WithExecCtx 将 ExecutionContext 注入到 context 中
func WithExecCtx(ctx context.Context, execCtx *executor.ExecutionContext) context.Context {
	return context.WithValue(ctx, execCtxKey, execCtx)
}

// GetExecCtx 从 context 中提取 ExecutionContext
func GetExecCtx(ctx context.Context) *executor.ExecutionContext {
	if v := ctx.Value(execCtxKey); v != nil {
		if ec, ok := v.(*executor.ExecutionContext); ok {
			return ec
		}
	}
	return nil
}

// aiCallbackKey 用于在 context.Context 中传递 AICallback
type aiCallbackKeyType struct{}

var aiCallbackKey = aiCallbackKeyType{}

// WithAICallback 将 AICallback 注入到 context 中
func WithAICallback(ctx context.Context, cb types.AICallback) context.Context {
	return context.WithValue(ctx, aiCallbackKey, cb)
}

// GetAICallback 从 context 中提取 AICallback
func GetAICallback(ctx context.Context) types.AICallback {
	if v := ctx.Value(aiCallbackKey); v != nil {
		if cb, ok := v.(types.AICallback); ok {
			return cb
		}
	}
	return nil
}

// ========== 内置工具适配为 Eino InvokableTool ==========

// builtinToolAdapter 将旧的 executor.Tool 适配为 Eino InvokableTool
type builtinToolAdapter struct {
	wrapped executor.Tool
}

func (a *builtinToolAdapter) Info(_ context.Context) (*schema.ToolInfo, error) {
	def := a.wrapped.Definition()
	toolInfo := &schema.ToolInfo{
		Name: def.Name,
		Desc: def.Description,
	}
	if len(def.Parameters) > 0 {
		var schemaMap map[string]any
		if err := json.Unmarshal(def.Parameters, &schemaMap); err == nil {
			params := jsonSchemaMapToParams(schemaMap)
			if params != nil {
				toolInfo.ParamsOneOf = schema.NewParamsOneOfByParams(params)
			}
		}
	}
	return toolInfo, nil
}

func (a *builtinToolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	execCtx := GetExecCtx(ctx)
	result, err := a.wrapped.Execute(ctx, argumentsInJSON, execCtx)
	if err != nil {
		return "", err
	}
	if result.IsError {
		return result.Content, fmt.Errorf("tool error: %s", result.Content)
	}
	return result.Content, nil
}

// AdaptBuiltinTool 将旧的 executor.Tool 适配为 Eino InvokableTool
func AdaptBuiltinTool(t executor.Tool) tool.InvokableTool {
	return &builtinToolAdapter{wrapped: t}
}

// ========== MCP 工具适配为 Eino InvokableTool ==========

// mcpToolAdapter 将 MCP 远程工具适配为 Eino InvokableTool
type mcpToolAdapter struct {
	def       *types.ToolDefinition
	mcpClient *executor.MCPRemoteClient
	serverIDs []int64
}

func (a *mcpToolAdapter) Info(_ context.Context) (*schema.ToolInfo, error) {
	toolInfo := &schema.ToolInfo{
		Name: a.def.Name,
		Desc: a.def.Description,
	}
	if len(a.def.Parameters) > 0 {
		var schemaMap map[string]any
		if err := json.Unmarshal(a.def.Parameters, &schemaMap); err == nil {
			params := jsonSchemaMapToParams(schemaMap)
			if params != nil {
				toolInfo.ParamsOneOf = schema.NewParamsOneOfByParams(params)
			}
		}
	}
	return toolInfo, nil
}

func (a *mcpToolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	for _, serverID := range a.serverIDs {
		tools, err := a.mcpClient.GetTools(ctx, serverID)
		if err != nil {
			continue
		}
		for _, t := range tools {
			if t.Name == a.def.Name {
				result, err := a.mcpClient.CallTool(ctx, serverID, a.def.Name, argumentsInJSON)
				if err != nil {
					return "", fmt.Errorf("MCP 工具调用失败: %w", err)
				}
				if result.IsError {
					return result.Content, fmt.Errorf("MCP tool error: %s", result.Content)
				}
				return result.Content, nil
			}
		}
	}
	return "", fmt.Errorf("MCP 工具 %q 未找到", a.def.Name)
}

// ========== Skill 工具适配为 Eino InvokableTool ==========

// skillToolAdapter 将 Skill 适配为 Eino InvokableTool
type skillToolAdapter struct {
	skill         *SkillInfo
	createModelFn func(ctx context.Context) (*aiModelInstance, error)
}

type aiModelInstance struct {
	config *AIConfig
}

func (a *skillToolAdapter) Info(_ context.Context) (*schema.ToolInfo, error) {
	toolName := skillToolPrefix + sanitizeToolName(a.skill.Name)
	return &schema.ToolInfo{
		Name: toolName,
		Desc: fmt.Sprintf("[Skill] %s", a.skill.Description),
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"task": {
				Type:     schema.String,
				Desc:     "需要该专家处理的具体任务内容，请提供完整上下文",
				Required: true,
			},
		}),
	}, nil
}

func (a *skillToolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args struct {
		Task string `json:"task"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("Skill 参数解析失败: %w", err)
	}
	if args.Task == "" {
		return "", fmt.Errorf("Skill 调用缺少 task 参数")
	}

	messages := []*schema.Message{
		schema.SystemMessage(a.skill.SystemPrompt),
		schema.UserMessage(args.Task),
	}

	mi, err := a.createModelFn(ctx)
	if err != nil {
		return "", fmt.Errorf("Skill 创建模型失败: %w", err)
	}

	chatModel, err := createChatModelFromConfig(ctx, mi.config)
	if err != nil {
		return "", fmt.Errorf("Skill 创建模型失败: %w", err)
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("Skill 执行失败: %w", err)
	}

	return resp.Content, nil
}

// ========== 知识库检索工具适配为 Eino InvokableTool ==========

type knowledgeSearchInput struct {
	Query string `json:"query" jsonschema:"description=检索查询内容"`
	TopK  int    `json:"top_k,omitempty" jsonschema:"description=返回结果数量"`
}

// NewKnowledgeSearchTool 创建知识库检索 Eino 工具
func NewKnowledgeSearchTool(knowledgeBases []*KnowledgeBaseInfo) tool.InvokableTool {
	var kbNames []string
	for _, kb := range knowledgeBases {
		kbNames = append(kbNames, kb.Name)
	}
	kbList := strings.Join(kbNames, "、")

	t, err := utils.InferTool(
		knowledgeSearchToolName,
		fmt.Sprintf("[知识库检索] 从以下知识库中检索相关信息：%s", kbList),
		func(ctx context.Context, input *knowledgeSearchInput) (string, error) {
			if input.Query == "" {
				return "", fmt.Errorf("检索查询内容不能为空")
			}
			topK := input.TopK
			if topK <= 0 {
				topK = 5
			}

			var allResults []knowledgeChunk
			for _, kb := range knowledgeBases {
				if kb.QdrantCollection != "" {
					results := searchQdrant(ctx, kb, input.Query, topK)
					allResults = append(allResults, results...)
				}
				if kb.Type == "graph" {
					graphResults := searchGraph(ctx, kb, input.Query, topK)
					allResults = append(allResults, graphResults...)
				}
			}

			if len(allResults) == 0 {
				return "未找到与查询相关的知识库内容。", nil
			}
			if len(allResults) > topK {
				allResults = allResults[:topK]
			}

			var sb strings.Builder
			for i, chunk := range allResults {
				sb.WriteString(fmt.Sprintf("[%d] (来源: %s, 相关度: %.2f)\n%s\n\n", i+1, chunk.Source, chunk.Score, chunk.Content))
			}
			return sb.String(), nil
		},
	)
	if err != nil {
		log.Printf("[WARN] 创建知识库检索工具失败: %v", err)
		return nil
	}
	return t
}

// ========== 人机交互工具适配为 Eino InvokableTool ==========

type humanInteractionInput struct {
	Type         string   `json:"type" jsonschema:"description=交互类型: confirm/input/select,enum=confirm,enum=input,enum=select"`
	Prompt       string   `json:"prompt" jsonschema:"description=展示给用户的提示信息"`
	Options      []string `json:"options,omitempty" jsonschema:"description=select 类型的选项列表"`
	DefaultValue string   `json:"default_value,omitempty" jsonschema:"description=超时时使用的默认值"`
}

// NewHumanInteractionTool 创建人机交互 Eino 工具
func NewHumanInteractionTool(stepID string, config *AIConfig) tool.InvokableTool {
	t, err := utils.InferTool(
		humanInteractionToolName,
		"当你需要用户确认、输入信息或从选项中选择时，调用此工具与用户进行交互。",
		func(ctx context.Context, input *humanInteractionInput) (string, error) {
			callback := GetAICallback(ctx)
			if callback == nil {
				return "", fmt.Errorf("人机交互不可用：缺少回调接口")
			}

			if input.Type == "" {
				input.Type = "confirm"
			}
			if input.Prompt == "" {
				return "", fmt.Errorf("缺少必填参数: prompt")
			}

			request := &types.InteractionRequest{
				Type:         types.InteractionType(input.Type),
				Prompt:       input.Prompt,
				DefaultValue: input.DefaultValue,
				Timeout:      config.InteractionTimeout,
			}

			if input.Type == "select" && len(input.Options) > 0 {
				request.Options = make([]types.InteractionOption, len(input.Options))
				for i, opt := range input.Options {
					request.Options[i] = types.InteractionOption{Value: opt, Label: opt}
				}
			}

			if request.Timeout <= 0 {
				request.Timeout = 300
			}

			resp, err := callback.OnAIInteractionRequired(ctx, stepID, request)
			if err != nil {
				return "", fmt.Errorf("交互处理失败: %w", err)
			}

			if resp == nil || resp.Skipped {
				defaultVal := input.DefaultValue
				if defaultVal == "" {
					defaultVal = "(用户未响应)"
				}
				return fmt.Sprintf(`{"skipped": true, "value": %q}`, defaultVal), nil
			}

			return fmt.Sprintf(`{"skipped": false, "value": %q}`, resp.Value), nil
		},
	)
	if err != nil {
		log.Printf("[WARN] 创建人机交互工具失败: %v", err)
		return nil
	}
	return t
}

// ========== 工具收集器 ==========

// CollectEinoTools 收集所有 Eino 工具
func CollectEinoTools(ctx context.Context, config *AIConfig, stepID string, mcpClient *executor.MCPRemoteClient) []tool.BaseTool {
	var allTools []tool.BaseTool

	// 收集内置工具
	for _, toolName := range config.Tools {
		if executor.DefaultToolRegistry.Has(toolName) {
			t, _ := executor.DefaultToolRegistry.Get(toolName)
			allTools = append(allTools, AdaptBuiltinTool(t))
		} else {
			log.Printf("[WARN] 未知的内置工具名称，已跳过: %s", toolName)
		}
	}

	// 收集 MCP 工具
	if mcpClient != nil {
		for _, serverID := range config.MCPServerIDs {
			tools, err := mcpClient.GetTools(ctx, serverID)
			if err != nil {
				log.Printf("[WARN] 获取 MCP 服务器 %d 工具列表失败: %v", serverID, err)
				continue
			}
			for _, def := range tools {
				allTools = append(allTools, &mcpToolAdapter{
					def:       def,
					mcpClient: mcpClient,
					serverIDs: config.MCPServerIDs,
				})
			}
		}
	}

	// 收集 Skill 工具
	for _, skill := range config.Skills {
		allTools = append(allTools, &skillToolAdapter{
			skill: skill,
			createModelFn: func(ctx context.Context) (*aiModelInstance, error) {
				return &aiModelInstance{config: config}, nil
			},
		})
	}

	// 收集知识库检索工具
	if len(config.KnowledgeBases) > 0 {
		kbTool := NewKnowledgeSearchTool(config.KnowledgeBases)
		if kbTool != nil {
			allTools = append(allTools, kbTool)
		}
	}

	// 人机交互工具
	if config.Interactive {
		hiTool := NewHumanInteractionTool(stepID, config)
		if hiTool != nil {
			allTools = append(allTools, hiTool)
		}
	}

	return allTools
}
