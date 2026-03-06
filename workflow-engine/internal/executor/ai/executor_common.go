package ai

import (
	"context"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

// mcpCloser 记录需要在执行结束后关闭的 MCP 连接
type mcpCloser struct {
	name   string
	client *mcpclient.Client
}

func closeMCPClients(clients []mcpCloser) {
	for _, c := range clients {
		if err := c.client.Close(); err != nil {
			logger.Warn("[MCPTool] 关闭 MCP Server %q 连接失败: %v", c.name, err)
		}
	}
}

// preparedRequest 封装构建好的 Agent 请求和相关资源
type preparedRequest struct {
	ctx        context.Context
	cancel     context.CancelFunc
	config     *AIConfig
	agentReq   *AgentRequest
	mcpClients []mcpCloser
	startTime  time.Time
}

// buildAgentRequest 统一构建 Agent 请求（所有 Executor 共用）
func buildAgentRequest(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext, mode AgentMode) (*preparedRequest, context.CancelFunc, error) {
	startTime := time.Now()
	logger.Debug("[AgentBuild] 开始构建请求, stepID=%s, stepType=%s, mode=%s", step.ID, step.Type, mode)

	config, err := parseAIConfig(step.Config)
	if err != nil {
		logger.Debug("[AgentBuild] 解析 AI 配置失败, stepID=%s: %v", step.ID, err)
		return nil, func() {}, err
	}
	logger.Debug("[AgentBuild] AI 配置: provider=%s, model=%s, streaming=%v, tools=%v, mcpServers=%d, skills=%d, knowledgeBases=%d, stepID=%s",
		config.Provider, config.Model, config.Streaming, config.Tools, len(config.MCPServers), len(config.Skills), len(config.KnowledgeBases), step.ID)

	userInputFiles := extractUserInputFiles(execCtx)
	config = resolveConfigVariables(config, execCtx)
	applyUserInputFiles(config, userInputFiles)
	chatHistory := extractChatHistory(execCtx)

	chatModel, err := createChatModelFromConfig(ctx, config)
	if err != nil {
		logger.Debug("[AgentBuild] 创建 AI 模型失败, provider=%s, model=%s, stepID=%s: %v",
			config.Provider, config.Model, step.ID, err)
		return nil, func() {}, executor.NewExecutionError(step.ID, "创建 AI 模型失败", err)
	}
	logger.Debug("[AgentBuild] AI 模型创建成功, provider=%s, model=%s", config.Provider, config.Model)

	timeout := step.Timeout
	if timeout <= 0 && config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}
	if timeout <= 0 {
		timeout = defaultAITimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)

	callbacks := NewAgentCallbacks(execCtx.Callback, step.ID)

	ctx = WithExecCtx(ctx, execCtx)
	if callbacks.Stream != nil {
		ctx = WithAICallback(ctx, callbacks.Stream)
	}

	// 构建工具注册表
	toolRegistry, mcpClients := buildToolRegistry(ctx, config, execCtx, step.ID, callbacks.Stream)

	allToolDefs := toolRegistry.List()
	schemaTools := toSchemaTools(allToolDefs)

	toolNames := make([]string, 0, len(allToolDefs))
	for _, td := range allToolDefs {
		toolNames = append(toolNames, td.Name)
	}
	logger.Debug("[AgentBuild] 工具注册表构建完成, 工具数=%d, tools=%v, mcpClients=%d, stepID=%s",
		len(allToolDefs), toolNames, len(mcpClients), step.ID)

	// 使用 PromptBuilder 构建系统提示词
	pb := NewPromptBuilder(config, allToolDefs, mode)
	systemPrompt := pb.Build()

	messages := buildUnifiedMessages(systemPrompt, chatHistory, config)
	logger.Debug("[AgentBuild] 消息构建完成, messages数=%d, systemPrompt长度=%d, chatHistory数=%d, stepID=%s",
		len(messages), len([]rune(systemPrompt)), len(chatHistory), step.ID)

	maxRounds := config.MaxToolRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxToolRounds
	}

	agentReq := &AgentRequest{
		Config:       config,
		ChatModel:    chatModel,
		Messages:     messages,
		ToolRegistry: toolRegistry,
		SchemaTools:  schemaTools,
		AllToolDefs:  allToolDefs,
		StepID:       step.ID,
		ExecCtx:      execCtx,
		Callbacks:    callbacks,
		MaxRounds:    maxRounds,
	}

	cleanup := func() {
		cancel()
		if len(mcpClients) > 0 {
			closeMCPClients(mcpClients)
		}
	}

	return &preparedRequest{
		ctx:        ctx,
		cancel:     cancel,
		config:     config,
		agentReq:   agentReq,
		mcpClients: mcpClients,
		startTime:  startTime,
	}, cleanup, nil
}

// handleAgentError 统一处理 Agent 执行错误
func handleAgentError(step *types.Step, req *preparedRequest, err error) (*types.StepResult, error) {
	if req.ctx.Err() == context.DeadlineExceeded {
		timeout := step.Timeout
		if timeout <= 0 && req.config.Timeout > 0 {
			timeout = time.Duration(req.config.Timeout) * time.Second
		}
		if timeout <= 0 {
			timeout = defaultAITimeout
		}
		return executor.CreateTimeoutResult(step.ID, req.startTime, timeout), nil
	}
	if req.agentReq.Callbacks.Stream != nil {
		req.agentReq.Callbacks.Stream.OnAIError(req.ctx, step.ID, err)
	}
	return executor.CreateFailedResult(step.ID, req.startTime,
		executor.NewExecutionError(step.ID, "AI Agent 调用失败", err)), nil
}

// buildAgentResult 统一构建 Agent 执行结果
func buildAgentResult(step *types.Step, req *preparedRequest, output *AIOutput) (*types.StepResult, error) {
	output.SystemPrompt = req.config.SystemPrompt
	output.Prompt = req.config.Prompt

	// 执行后置处理器
	executePostProcessors(req.ctx, step, req.agentReq.ExecCtx, output, req.startTime)

	result := executor.CreateSuccessResult(step.ID, req.startTime, output)
	result.Metrics["ai_prompt_tokens"] = float64(output.PromptTokens)
	result.Metrics["ai_completion_tokens"] = float64(output.CompletionTokens)
	result.Metrics["ai_total_tokens"] = float64(output.TotalTokens)
	return result, nil
}

// buildToolRegistry 构建本次执行的工具注册表
func buildToolRegistry(ctx context.Context, config *AIConfig, execCtx *executor.ExecutionContext, stepID string, aiCallback types.AIStreamCallback) (*executor.ToolRegistry, []mcpCloser) {
	reg := executor.DefaultToolRegistry.Clone()

	if len(config.Tools) > 0 {
		filteredReg := executor.NewToolRegistry()
		for _, toolName := range config.Tools {
			if tool, ok := reg.Get(toolName); ok {
				filteredReg.Register(tool)
			}
		}
		// 始终保留基础联网和代码工具
		for _, name := range []string{"bing_search", "google_search", "web_fetch", "code_execute"} {
			if tool, ok := reg.Get(name); ok {
				filteredReg.Register(tool)
			}
		}
		reg = filteredReg
	}

	if config.Interactive {
		interactionTool := NewHumanInteractionTool(config)
		interactionTool.SetContext(stepID, aiCallback)
		reg.Register(interactionTool)
	}

	if len(config.KnowledgeBases) > 0 {
		reg.Register(NewKnowledgeTool(config.KnowledgeBases, config))
	}

	if len(config.Skills) > 0 {
		reg.Register(NewSkillTool(config.Skills))
	}

	var mcpClients []mcpCloser
	if len(config.MCPServers) > 0 {
		for _, serverCfg := range config.MCPServers {
			logger.Debug("[AgentBuild] 加载 MCP Server %q, transport=%s", serverCfg.Name, serverCfg.Transport)
			tools, cli, err := loadMCPTools(ctx, serverCfg)
			if err != nil {
				logger.Warn("[AgentBuild] MCP Server %q 加载失败: %v", serverCfg.Name, err)
				continue
			}
			mcpClients = append(mcpClients, mcpCloser{name: serverCfg.Name, client: cli})
			mcpToolNames := make([]string, 0, len(tools))
			for _, t := range tools {
				reg.Register(t)
				mcpToolNames = append(mcpToolNames, t.Definition().Name)
			}
			logger.Debug("[AgentBuild] MCP Server %q 加载成功, 注册 %d 个工具: %v", serverCfg.Name, len(tools), mcpToolNames)
		}
	}

	return reg, mcpClients
}
