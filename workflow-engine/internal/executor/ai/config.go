package ai

import (
	"yqhp/workflow-engine/internal/executor"
)

// AIConfig AI 节点配置
type AIConfig struct {
	Provider           string      `json:"provider"`
	Model              string      `json:"model"`
	APIKey             string      `json:"api_key"`
	BaseURL            string      `json:"base_url,omitempty"`
	APIVersion         string      `json:"api_version,omitempty"`
	Temperature        *float32    `json:"temperature,omitempty"`
	MaxTokens          *int        `json:"max_tokens,omitempty"`
	TopP               *float32    `json:"top_p,omitempty"`
	PresencePenalty    *float32    `json:"presence_penalty,omitempty"`
	SystemPrompt       string      `json:"system_prompt,omitempty"`
	Prompt             string      `json:"prompt"`
	Streaming          bool        `json:"streaming"`
	Interactive        bool        `json:"interactive"`
	InteractionTimeout int         `json:"interaction_timeout,omitempty"`
	Timeout            int         `json:"timeout,omitempty"`
	Tools              []string    `json:"tools,omitempty"`
	MCPServerIDs       []int64     `json:"mcp_server_ids,omitempty"`
	MaxToolRounds      int         `json:"max_tool_rounds,omitempty"`
	MCPProxyBaseURL    string      `json:"mcp_proxy_base_url,omitempty"`
	Skills             []*SkillInfo `json:"skills,omitempty"`
}

// SkillInfo Skill 能力信息（由 gulu 层从数据库查询后注入到 config）
type SkillInfo struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
}

// parseConfig 从 map 解析 AI 节点配置
func (e *AIExecutor) parseConfig(config map[string]any) (*AIConfig, error) {
	aiConfig := &AIConfig{Provider: "openai", Streaming: false}

	if provider, ok := config["provider"].(string); ok {
		aiConfig.Provider = provider
	}
	if m, ok := config["model"].(string); ok {
		aiConfig.Model = m
	} else {
		return nil, executor.NewConfigError("AI 节点需要配置 'model'", nil)
	}
	if apiKey, ok := config["api_key"].(string); ok {
		aiConfig.APIKey = apiKey
	} else {
		return nil, executor.NewConfigError("AI 节点需要配置 'api_key'", nil)
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
		return nil, executor.NewConfigError("AI 节点需要配置 'prompt'", nil)
	}
	if streaming, ok := config["streaming"].(bool); ok {
		aiConfig.Streaming = streaming
	}
	if interactive, ok := config["interactive"].(bool); ok {
		aiConfig.Interactive = interactive
	}
	if interactionTimeout, ok := config["interaction_timeout"].(float64); ok {
		aiConfig.InteractionTimeout = int(interactionTimeout)
	}
	if timeout, ok := config["timeout"].(float64); ok {
		aiConfig.Timeout = int(timeout)
	}

	// 解析工具相关配置
	if tools, ok := config["tools"].([]any); ok {
		for _, t := range tools {
			if s, ok := t.(string); ok {
				aiConfig.Tools = append(aiConfig.Tools, s)
			}
		}
	}
	if mcpServerIDs, ok := config["mcp_server_ids"].([]any); ok {
		for _, id := range mcpServerIDs {
			if f, ok := id.(float64); ok {
				aiConfig.MCPServerIDs = append(aiConfig.MCPServerIDs, int64(f))
			}
		}
	}
	if maxToolRounds, ok := config["max_tool_rounds"].(float64); ok {
		aiConfig.MaxToolRounds = int(maxToolRounds)
	}
	if mcpProxyBaseURL, ok := config["mcp_proxy_base_url"].(string); ok {
		aiConfig.MCPProxyBaseURL = mcpProxyBaseURL
	}

	// 解析 Skill 列表（由 gulu 层注入的完整 Skill 信息）
	if skills, ok := config["skills"].([]any); ok {
		for _, s := range skills {
			if skillMap, ok := s.(map[string]any); ok {
				skill := &SkillInfo{}
				if id, ok := skillMap["id"].(float64); ok {
					skill.ID = int64(id)
				}
				if name, ok := skillMap["name"].(string); ok {
					skill.Name = name
				}
				if desc, ok := skillMap["description"].(string); ok {
					skill.Description = desc
				}
				if sp, ok := skillMap["system_prompt"].(string); ok {
					skill.SystemPrompt = sp
				}
				if skill.Name != "" && skill.SystemPrompt != "" {
					aiConfig.Skills = append(aiConfig.Skills, skill)
				}
			}
		}
	}

	return aiConfig, nil
}

// resolveVariables 解析配置中的变量引用
func (e *AIExecutor) resolveVariables(config *AIConfig, execCtx *executor.ExecutionContext) *AIConfig {
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

	return config
}
