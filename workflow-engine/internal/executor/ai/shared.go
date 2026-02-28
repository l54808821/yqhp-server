package ai

import (
	"context"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"

	"yqhp/workflow-engine/internal/executor"
)

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

	return openai.NewChatModel(ctx, chatConfig)
}

// getMCPProxyBaseURL 获取 MCP 代理服务地址
func getMCPProxyBaseURL(config *AIConfig) string {
	if config.MCPProxyBaseURL != "" {
		return config.MCPProxyBaseURL
	}
	if envURL := os.Getenv("MCP_PROXY_BASE_URL"); envURL != "" {
		return envURL
	}
	return defaultMCPProxyBaseURL
}

// createMCPClient 创建 MCP 远程客户端（如果有 MCP 服务器配置）
func createMCPClient(config *AIConfig) *executor.MCPRemoteClient {
	if len(config.MCPServerIDs) > 0 {
		return executor.NewMCPRemoteClient(getMCPProxyBaseURL(config))
	}
	return nil
}

// hasToolsConfig 检查配置中是否启用了工具
func hasToolsConfig(config *AIConfig) bool {
	return len(config.Tools) > 0 || len(config.MCPServerIDs) > 0 ||
		config.Interactive || len(config.Skills) > 0 || len(config.KnowledgeBases) > 0
}
