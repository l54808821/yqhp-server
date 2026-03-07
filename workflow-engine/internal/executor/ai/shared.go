package ai

import (
	"context"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

// ========== Context 辅助函数 ==========

type execCtxKeyType struct{}

var execCtxKey = execCtxKeyType{}

// WithExecCtx 将 ExecutionContext 注入到 context 中
func WithExecCtx(ctx context.Context, execCtx *executor.ExecutionContext) context.Context {
	return context.WithValue(ctx, execCtxKey, execCtx)
}

type aiCallbackKeyType struct{}

var aiCallbackKey = aiCallbackKeyType{}

// WithAICallback 将 AIStreamCallback 注入到 context 中
func WithAICallback(ctx context.Context, cb types.AIStreamCallback) context.Context {
	return context.WithValue(ctx, aiCallbackKey, cb)
}

// createChatModelFromConfig 根据 AIConfig 创建 Eino ChatModel（包级函数，供各执行器复用）
func createChatModelFromConfig(ctx context.Context, config *AIConfig) (einomodel.ToolCallingChatModel, error) {
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

	logger.Debug("[ModelCreate] 创建模型, provider=%s, model=%s, baseURL=%s, streaming=%v",
		config.Provider, config.Model, baseURL, config.Streaming)

	chatModel, err := openai.NewChatModel(ctx, chatConfig)
	if err != nil {
		logger.Debug("[ModelCreate] 模型创建失败, provider=%s, model=%s: %v", config.Provider, config.Model, err)
		return nil, err
	}
	return chatModel, nil
}

// getQdrantHost 获取 Qdrant 服务地址
func getQdrantHost(config *AIConfig) string {
	if config.QdrantHost != "" {
		return config.QdrantHost
	}
	if envURL := os.Getenv("QDRANT_HOST"); envURL != "" {
		return envURL
	}
	return defaultQdrantHost
}

// getGuluHost 获取 Gulu 服务地址
func getGuluHost(config *AIConfig) string {
	if config.GuluHost != "" {
		return config.GuluHost
	}
	if envURL := os.Getenv("GULU_HOST"); envURL != "" {
		return envURL
	}
	return defaultGuluHost
}


// jsonSchemaMapToParams 将 JSON Schema map 转换为 schema.ParameterInfo map
func jsonSchemaMapToParams(schemaMap map[string]any) map[string]*schema.ParameterInfo {
	props, ok := schemaMap["properties"].(map[string]any)
	if !ok {
		return nil
	}

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
		if paramInfo.Type == schema.Object {
			if subProps := jsonSchemaMapToParams(prop); subProps != nil {
				paramInfo.SubParams = subProps
			}
		}
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
