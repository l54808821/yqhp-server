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

// extractUserInputFiles 在变量解析之前从执行上下文中提取 userinput.files，
// 同时将上下文中的 userinput.files 置空，避免 ${userinput.files} 被变量解析器
// 替换为原始 JSON 文本混入 prompt。
func extractUserInputFiles(execCtx *executor.ExecutionContext) []interface{} {
	if execCtx == nil || execCtx.Variables == nil {
		return nil
	}

	raw := execCtx.Variables["userinput.files"]
	if raw == nil {
		return nil
	}

	execCtx.Variables["userinput.files"] = ""

	var files []interface{}
	switch v := raw.(type) {
	case []interface{}:
		files = v
	case string:
		if v == "" || v == "[]" {
			return nil
		}
		if err := json.Unmarshal([]byte(v), &files); err != nil {
			return nil
		}
	default:
		return nil
	}

	if len(files) == 0 {
		return nil
	}
	return files
}

// applyUserInputFiles 将预先提取的用户上传文件与已解析的 Prompt 文本
// 一起构建为多模态 PromptMultiContent，使大模型能够识别文件内容。
func applyUserInputFiles(config *AIConfig, files []interface{}) {
	if len(files) == 0 {
		return
	}

	var parts []interface{}
	if config.Prompt != "" {
		parts = append(parts, map[string]interface{}{"type": "text", "text": config.Prompt})
	}

	for _, f := range files {
		fm, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		fileType, _ := fm["type"].(string)
		url, _ := fm["url"].(string)
		if url == "" {
			continue
		}
		switch fileType {
		case "image":
			parts = append(parts, map[string]interface{}{
				"type":      "image_url",
				"image_url": map[string]interface{}{"url": url},
			})
		case "audio":
			parts = append(parts, map[string]interface{}{
				"type":        "input_audio",
				"input_audio": map[string]interface{}{"url": url},
			})
		case "video":
			parts = append(parts, map[string]interface{}{
				"type":      "video_url",
				"video_url": map[string]interface{}{"url": url},
			})
		default:
			name, _ := fm["name"].(string)
			parts = append(parts, map[string]interface{}{
				"type":     "file_url",
				"file_url": map[string]interface{}{"url": url, "name": name},
			})
		}
	}

	if len(parts) > 0 {
		config.PromptMultiContent = parts
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
	config.Prompt = strings.TrimSpace(resolver.ResolveString(config.Prompt, evalCtx))
	config.BaseURL = resolver.ResolveString(config.BaseURL, evalCtx)
	config.QdrantHost = resolver.ResolveString(config.QdrantHost, evalCtx)
	config.GuluHost = resolver.ResolveString(config.GuluHost, evalCtx)

	return config
}
