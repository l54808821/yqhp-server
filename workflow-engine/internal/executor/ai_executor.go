package executor

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/pkg/types"
)

const (
	// AIExecutorType AI 执行器类型标识符
	AIExecutorType = "ai"

	// AI 调用默认超时时间
	defaultAITimeout = 5 * time.Minute
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
	Provider           string                `json:"provider"`
	Model              string                `json:"model"`
	APIKey             string                `json:"api_key"`
	BaseURL            string                `json:"base_url,omitempty"`
	APIVersion         string                `json:"api_version,omitempty"`
	Temperature        *float32              `json:"temperature,omitempty"`
	MaxTokens          *int                  `json:"max_tokens,omitempty"`
	TopP               *float32              `json:"top_p,omitempty"`
	PresencePenalty    *float32              `json:"presence_penalty,omitempty"`
	SystemPrompt       string                `json:"system_prompt,omitempty"`
	Prompt             string                `json:"prompt"`
	Streaming          bool                  `json:"streaming"`
	Interactive        bool                  `json:"interactive"`
	InteractionType    types.InteractionType `json:"interaction_type,omitempty"`
	InteractionPrompt  string                `json:"interaction_prompt,omitempty"`
	InteractionOptions []string              `json:"interaction_options,omitempty"`
	InteractionTimeout int                   `json:"interaction_timeout,omitempty"`
	InteractionDefault string                `json:"interaction_default,omitempty"`
	Timeout            int                   `json:"timeout,omitempty"` // AI 调用超时（秒），0 使用默认值
}

// AIOutput AI 节点输出
type AIOutput struct {
	Content          string `json:"content"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Model            string `json:"model"`
	FinishReason     string `json:"finish_reason"`
	SystemPrompt     string `json:"system_prompt,omitempty"`
	Prompt           string `json:"prompt"`
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

	if config.Streaming && aiCallback != nil {
		output, err = e.executeStream(ctx, chatModel, messages, step.ID, config, aiCallback)
	} else {
		output, err = e.executeNonStream(ctx, chatModel, messages, config)
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

	if config.Interactive && aiCallback != nil {
		interactionResult, err := e.handleInteraction(ctx, step.ID, config, output.Content, aiCallback)
		if err != nil {
			return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "交互处理失败", err)), nil
		}
		if interactionResult != nil {
			output.Content = fmt.Sprintf("%s\n\n[用户响应: %s]", output.Content, interactionResult.Value)
		}
	}

	result := CreateSuccessResult(step.ID, startTime, output)
	result.Metrics["ai_prompt_tokens"] = float64(output.PromptTokens)
	result.Metrics["ai_completion_tokens"] = float64(output.CompletionTokens)
	result.Metrics["ai_total_tokens"] = float64(output.TotalTokens)

	return result, nil
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

func (e *AIExecutor) buildMessages(config *AIConfig) []*schema.Message {
	var messages []*schema.Message
	if config.SystemPrompt != "" {
		messages = append(messages, schema.SystemMessage(config.SystemPrompt))
	}
	messages = append(messages, schema.UserMessage(config.Prompt))
	return messages
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

func (e *AIExecutor) handleInteraction(ctx context.Context, stepID string, config *AIConfig, aiContent string, callback types.AICallback) (*types.InteractionResponse, error) {
	request := &types.InteractionRequest{
		Type:         config.InteractionType,
		Prompt:       config.InteractionPrompt,
		DefaultValue: config.InteractionDefault,
		Timeout:      config.InteractionTimeout,
	}

	if config.InteractionType == types.InteractionTypeSelect && len(config.InteractionOptions) > 0 {
		request.Options = make([]types.InteractionOption, len(config.InteractionOptions))
		for i, opt := range config.InteractionOptions {
			request.Options[i] = types.InteractionOption{Value: opt, Label: opt}
		}
	}

	if request.Timeout <= 0 {
		request.Timeout = 300
	}

	return callback.OnAIInteractionRequired(ctx, stepID, request)
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
	if interactionType, ok := config["interaction_type"].(string); ok {
		aiConfig.InteractionType = types.InteractionType(interactionType)
	}
	if interactionPrompt, ok := config["interaction_prompt"].(string); ok {
		aiConfig.InteractionPrompt = interactionPrompt
	}
	if interactionOptions, ok := config["interaction_options"].([]any); ok {
		for _, opt := range interactionOptions {
			if s, ok := opt.(string); ok {
				aiConfig.InteractionOptions = append(aiConfig.InteractionOptions, s)
			}
		}
	}
	if interactionTimeout, ok := config["interaction_timeout"].(float64); ok {
		aiConfig.InteractionTimeout = int(interactionTimeout)
	}
	if interactionDefault, ok := config["interaction_default"].(string); ok {
		aiConfig.InteractionDefault = interactionDefault
	}
	if timeout, ok := config["timeout"].(float64); ok {
		aiConfig.Timeout = int(timeout)
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
	config.InteractionPrompt = resolver.ResolveString(config.InteractionPrompt, evalCtx)
	config.InteractionDefault = resolver.ResolveString(config.InteractionDefault, evalCtx)
	config.BaseURL = resolver.ResolveString(config.BaseURL, evalCtx)

	return config
}

func init() {
	MustRegister(NewAIExecutor())
}
