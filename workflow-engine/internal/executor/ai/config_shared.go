package ai

import (
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/internal/executor"
)

// parseAIConfig 从 map 解析 AI 节点配置
func parseAIConfig(config map[string]any) (*AIConfig, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, executor.NewConfigError(fmt.Sprintf("AI 配置序列化失败: %v", err), err)
	}

	aiConfig := &AIConfig{}
	if err := json.Unmarshal(data, aiConfig); err != nil {
		return nil, executor.NewConfigError(fmt.Sprintf("AI 配置解析失败: %v", err), err)
	}

	aiConfig.applyDefaults()

	if err := aiConfig.Validate(); err != nil {
		return nil, err
	}

	return aiConfig, nil
}

// applyDefaults 设置默认值
func (c *AIConfig) applyDefaults() {
	if c.Provider == "" {
		c.Provider = "openai"
	}
}

// Validate 校验必填字段和值范围
func (c *AIConfig) Validate() error {
	if c.Model == "" {
		return executor.NewConfigError("AI 节点需要配置 'model'", nil)
	}
	if c.APIKey == "" {
		return executor.NewConfigError("AI 节点需要配置 'api_key'", nil)
	}
	if c.Prompt == "" {
		return executor.NewConfigError("AI 节点需要配置 'prompt'", nil)
	}
	if c.Temperature != nil {
		if *c.Temperature < 0 || *c.Temperature > 2 {
			return executor.NewConfigError(fmt.Sprintf("temperature 应在 0~2 之间，当前值: %.2f", *c.Temperature), nil)
		}
	}
	if c.TopP != nil {
		if *c.TopP < 0 || *c.TopP > 1 {
			return executor.NewConfigError(fmt.Sprintf("top_p 应在 0~1 之间，当前值: %.2f", *c.TopP), nil)
		}
	}
	return nil
}

// extractChatHistory 从执行上下文中提取多轮对话历史
func extractChatHistory(execCtx *executor.ExecutionContext) []*schema.Message {
	if execCtx == nil || execCtx.Variables == nil {
		return nil
	}
	history, ok := execCtx.Variables["__chat_history__"]
	if !ok {
		return nil
	}
	historySlice, ok := history.([]interface{})
	if !ok {
		return nil
	}
	var messages []*schema.Message
	for _, item := range historySlice {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		if content == "" {
			continue
		}
		switch role {
		case "user":
			messages = append(messages, schema.UserMessage(content))
		case "assistant":
			messages = append(messages, schema.AssistantMessage(content, nil))
		}
	}
	return messages
}

// applyUserMessage 如果执行上下文中有 __user_message__，用它覆盖 config.Prompt
func applyUserMessage(config *AIConfig, execCtx *executor.ExecutionContext) {
	if execCtx == nil || execCtx.Variables == nil {
		return
	}
	if userMsg, ok := execCtx.Variables["__user_message__"].(string); ok && userMsg != "" {
		config.Prompt = userMsg
	}
}

// resolveConfigVariables 解析配置中的变量引用
func resolveConfigVariables(config *AIConfig, execCtx *executor.ExecutionContext) *AIConfig {
	if execCtx == nil {
		return config
	}

	evalCtx := execCtx.ToEvaluationContext()
	resolver := executor.GetVariableResolver()
	config.APIKey = resolver.ResolveString(config.APIKey, evalCtx)
	config.SystemPrompt = resolver.ResolveString(config.SystemPrompt, evalCtx)
	config.Prompt = resolver.ResolveString(config.Prompt, evalCtx)
	config.BaseURL = resolver.ResolveString(config.BaseURL, evalCtx)
	config.MCPProxyBaseURL = resolver.ResolveString(config.MCPProxyBaseURL, evalCtx)
	config.QdrantHost = resolver.ResolveString(config.QdrantHost, evalCtx)
	config.GuluHost = resolver.ResolveString(config.GuluHost, evalCtx)

	return config
}
