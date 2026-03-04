package ai

import (
	"encoding/json"
	"fmt"
	"strings"

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
	if c.EnablePlanMode == nil {
		t := true
		c.EnablePlanMode = &t
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

// extractChatHistory 从执行上下文中提取多轮对话历史（支持纯文本和多模态消息）
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
		content := m["content"]

		switch role {
		case "user":
			msg := buildUserMessage(content)
			if msg != nil {
				messages = append(messages, msg)
			}
		case "assistant":
			if s, ok := content.(string); ok && s != "" {
				messages = append(messages, schema.AssistantMessage(s, nil))
			}
		}
	}
	return messages
}

// buildUserMessage 根据 content 类型构建用户消息（支持纯文本和多模态 ContentPart 数组）
func buildUserMessage(content interface{}) *schema.Message {
	if content == nil {
		return nil
	}

	if s, ok := content.(string); ok && s != "" {
		return schema.UserMessage(s)
	}

	parts, ok := content.([]interface{})
	if !ok || len(parts) == 0 {
		return nil
	}

	msg := &schema.Message{Role: schema.User}
	for _, part := range parts {
		p, ok := part.(map[string]interface{})
		if !ok {
			continue
		}
		partType, _ := p["type"].(string)
		switch partType {
		case "text":
			text, _ := p["text"].(string)
			if text != "" {
				msg.UserInputMultiContent = append(msg.UserInputMultiContent, schema.MessageInputPart{
					Type: schema.ChatMessagePartTypeText,
					Text: text,
				})
			}
		case "image_url":
			imgMap, _ := p["image_url"].(map[string]interface{})
			if imgMap != nil {
				url, _ := imgMap["url"].(string)
				if url != "" {
					msg.UserInputMultiContent = append(msg.UserInputMultiContent, schema.MessageInputPart{
						Type: schema.ChatMessagePartTypeImageURL,
						Image: &schema.MessageInputImage{
							MessagePartCommon: schema.MessagePartCommon{URL: &url},
						},
					})
				}
			}
		case "input_audio":
			audioMap, _ := p["input_audio"].(map[string]interface{})
			if audioMap != nil {
				url, _ := audioMap["url"].(string)
				if url != "" {
					msg.UserInputMultiContent = append(msg.UserInputMultiContent, schema.MessageInputPart{
						Type: schema.ChatMessagePartTypeAudioURL,
						Audio: &schema.MessageInputAudio{
							MessagePartCommon: schema.MessagePartCommon{URL: &url},
						},
					})
				}
			}
		case "video_url":
			videoMap, _ := p["video_url"].(map[string]interface{})
			if videoMap != nil {
				url, _ := videoMap["url"].(string)
				if url != "" {
					msg.UserInputMultiContent = append(msg.UserInputMultiContent, schema.MessageInputPart{
						Type: schema.ChatMessagePartTypeVideoURL,
						Video: &schema.MessageInputVideo{
							MessagePartCommon: schema.MessagePartCommon{URL: &url},
						},
					})
				}
			}
		case "file_url":
			fileMap, _ := p["file_url"].(map[string]interface{})
			if fileMap != nil {
				url, _ := fileMap["url"].(string)
				name, _ := fileMap["name"].(string)
				if url != "" {
					msg.UserInputMultiContent = append(msg.UserInputMultiContent, schema.MessageInputPart{
						Type: schema.ChatMessagePartTypeFileURL,
						File: &schema.MessageInputFile{
							MessagePartCommon: schema.MessagePartCommon{URL: &url},
							Name:              name,
						},
					})
				}
			}
		}
	}

	if len(msg.UserInputMultiContent) == 0 {
		return nil
	}
	return msg
}

// extractMultimodalTextContent 从多模态内容中提取纯文本部分（用于 Plan 模式等需要文本的场景）
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

// applyUserMessage 如果执行上下文中有 __user_message__，用它覆盖 config.Prompt
// 支持纯文本字符串和多模态 ContentPart 数组
func applyUserMessage(config *AIConfig, execCtx *executor.ExecutionContext) {
	if execCtx == nil || execCtx.Variables == nil {
		return
	}
	userMsg := execCtx.Variables["__user_message__"]
	if userMsg == nil {
		return
	}

	if s, ok := userMsg.(string); ok && s != "" {
		config.Prompt = s
		config.PromptMultiContent = nil
		return
	}

	if parts, ok := userMsg.([]interface{}); ok && len(parts) > 0 {
		config.PromptMultiContent = parts
		config.Prompt = extractMultimodalTextContent(userMsg)
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
	config.QdrantHost = resolver.ResolveString(config.QdrantHost, evalCtx)
	config.GuluHost = resolver.ResolveString(config.GuluHost, evalCtx)

	return config
}
