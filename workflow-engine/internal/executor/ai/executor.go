package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"

	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

const AgentType = "ai_agent"

// AgentExecutor 唯一的 AI Agent Step Executor
type AgentExecutor struct {
	*executor.BaseExecutor
}

func NewAgentExecutor() *AgentExecutor {
	return &AgentExecutor{
		BaseExecutor: executor.NewBaseExecutor(AgentType),
	}
}

func (e *AgentExecutor) Init(ctx context.Context, config map[string]any) error {
	return e.BaseExecutor.Init(ctx, config)
}

func (e *AgentExecutor) Execute(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*types.StepResult, error) {
	req, cleanup, err := buildAgentRequest(ctx, step, execCtx)
	if err != nil {
		return executor.CreateFailedResult(step.ID, time.Now(), err), nil
	}
	defer cleanup()

	output, err := RunAgent(req.ctx, req.agentReq)
	if err != nil {
		return handleAgentError(step, req, err)
	}

	return buildAgentResult(step, req, output)
}

func (e *AgentExecutor) Cleanup(ctx context.Context) error { return nil }

// --- 请求构建 ---

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

type preparedRequest struct {
	ctx        context.Context
	cancel     context.CancelFunc
	config     *AIConfig
	agentReq   *AgentRequest
	mcpClients []mcpCloser
	startTime  time.Time
}

func buildAgentRequest(ctx context.Context, step *types.Step, execCtx *executor.ExecutionContext) (*preparedRequest, context.CancelFunc, error) {
	startTime := time.Now()

	config, err := parseAIConfig(step.Config)
	if err != nil {
		return nil, func() {}, err
	}

	userInputFiles := extractUserInputFiles(execCtx)
	config = resolveConfigVariables(config, execCtx)
	applyUserInputFiles(config, userInputFiles)
	chatHistory := extractChatHistory(execCtx)

	chatModel, err := createChatModelFromConfig(ctx, config)
	if err != nil {
		return nil, func() {}, executor.NewExecutionError(step.ID, "创建 AI 模型失败", err)
	}

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

	toolRegistry, mcpClients := buildToolRegistry(ctx, config, execCtx, step.ID, callbacks.Stream)

	allToolDefs := toolRegistry.List()
	schemaTools := toSchemaTools(allToolDefs)

	pb := NewPromptBuilder(config, allToolDefs)
	systemPrompt := pb.Build()

	messages := buildMessages(systemPrompt, chatHistory, config)

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

func buildAgentResult(step *types.Step, req *preparedRequest, output *AIOutput) (*types.StepResult, error) {
	output.SystemPrompt = req.config.SystemPrompt
	output.Prompt = req.config.Prompt

	executePostProcessors(req.ctx, step, req.agentReq.ExecCtx, output, req.startTime)

	result := executor.CreateSuccessResult(step.ID, req.startTime, output)
	result.Metrics["ai_prompt_tokens"] = float64(output.PromptTokens)
	result.Metrics["ai_completion_tokens"] = float64(output.CompletionTokens)
	result.Metrics["ai_total_tokens"] = float64(output.TotalTokens)
	return result, nil
}

// --- 工具注册表 ---

func buildToolRegistry(ctx context.Context, config *AIConfig, execCtx *executor.ExecutionContext, stepID string, aiCallback types.AIStreamCallback) (*executor.ToolRegistry, []mcpCloser) {
	reg := executor.DefaultToolRegistry.Clone()

	if len(config.Tools) > 0 {
		filteredReg := executor.NewToolRegistry()
		for _, toolName := range config.Tools {
			if tool, ok := reg.Get(toolName); ok {
				filteredReg.Register(tool)
			}
		}
		for _, name := range []string{"bing_search", "google_search", "web_fetch", "code_execute"} {
			if tool, ok := reg.Get(name); ok {
				filteredReg.Register(tool)
			}
		}
		reg = filteredReg
	}

	reg.Register(NewTodoTool())

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
			tools, cli, err := loadMCPTools(ctx, serverCfg)
			if err != nil {
				logger.Warn("[AgentBuild] MCP Server %q 加载失败: %v", serverCfg.Name, err)
				continue
			}
			mcpClients = append(mcpClients, mcpCloser{name: serverCfg.Name, client: cli})
			for _, t := range tools {
				reg.Register(t)
			}
		}
	}

	return reg, mcpClients
}

// --- 消息构建 ---

func buildMessages(systemPrompt string, chatHistory []*schema.Message, config *AIConfig) []*schema.Message {
	var messages []*schema.Message

	if systemPrompt != "" {
		messages = append(messages, schema.SystemMessage(systemPrompt))
	}

	messages = append(messages, chatHistory...)

	if len(config.PromptMultiContent) > 0 {
		msg := buildUserMessage(config.PromptMultiContent)
		if msg != nil {
			messages = append(messages, msg)
		} else {
			messages = append(messages, schema.UserMessage(config.Prompt))
		}
	} else {
		messages = append(messages, schema.UserMessage(config.Prompt))
	}

	return messages
}

// extractMultimodalTextContent 从多模态内容中提取纯文本部分
func extractMultimodalTextContent(content interface{}) string {
	if s, ok := content.(string); ok {
		return s
	}
	parts, ok := content.([]interface{})
	if !ok {
		return fmt.Sprintf("%v", content)
	}
	var texts []string
	for _, part := range parts {
		p, ok := part.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _ := p["type"].(string); t == "text" {
			if text, ok := p["text"].(string); ok {
				texts = append(texts, text)
			}
		}
	}
	return strings.Join(texts, "\n")
}
