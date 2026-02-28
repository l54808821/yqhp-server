package ai

import (
	"yqhp/workflow-engine/internal/executor"
)

// parseAIConfig 从 map 解析 AI 节点配置（包级函数，供所有执行器复用）
func parseAIConfig(config map[string]any) (*AIConfig, error) {
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

	// 工具相关配置
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

	// Agent 模式
	if agentMode, ok := config["agent_mode"].(string); ok {
		aiConfig.AgentMode = agentMode
	}
	if maxReflectionRounds, ok := config["max_reflection_rounds"].(float64); ok {
		aiConfig.MaxReflectionRounds = int(maxReflectionRounds)
	}

	// Skill 列表
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

	// 知识库列表
	if kbs, ok := config["knowledge_bases"].([]any); ok {
		for _, k := range kbs {
			if kbMap, ok := k.(map[string]any); ok {
				kbInfo := &KnowledgeBaseInfo{}
				if id, ok := kbMap["id"].(float64); ok {
					kbInfo.ID = int64(id)
				}
				if name, ok := kbMap["name"].(string); ok {
					kbInfo.Name = name
				}
				if t, ok := kbMap["type"].(string); ok {
					kbInfo.Type = t
				}
				if col, ok := kbMap["qdrant_collection"].(string); ok {
					kbInfo.QdrantCollection = col
				}
				if neo, ok := kbMap["neo4j_database"].(string); ok {
					kbInfo.Neo4jDatabase = neo
				}
				if em, ok := kbMap["embedding_model"].(string); ok {
					kbInfo.EmbeddingModel = em
				}
				if emID, ok := kbMap["embedding_model_id"].(float64); ok {
					kbInfo.EmbeddingModelID = int64(emID)
				}
				if tk, ok := kbMap["top_k"].(float64); ok {
					kbInfo.TopK = int(tk)
				}
				if st, ok := kbMap["score_threshold"].(float64); ok {
					kbInfo.ScoreThreshold = st
				}
				if ep, ok := kbMap["embedding_provider"].(string); ok {
					kbInfo.EmbeddingProvider = ep
				}
				if ek, ok := kbMap["embedding_api_key"].(string); ok {
					kbInfo.EmbeddingAPIKey = ek
				}
				if eu, ok := kbMap["embedding_base_url"].(string); ok {
					kbInfo.EmbeddingBaseURL = eu
				}
				if ed, ok := kbMap["embedding_dimension"].(float64); ok {
					kbInfo.EmbeddingDimension = int(ed)
				}
				if kbInfo.Name != "" {
					aiConfig.KnowledgeBases = append(aiConfig.KnowledgeBases, kbInfo)
				}
			}
		}
	}
	if kbTopK, ok := config["kb_top_k"].(float64); ok {
		aiConfig.KBTopK = int(kbTopK)
	}
	if kbThreshold, ok := config["kb_score_threshold"].(float64); ok {
		aiConfig.KBScoreThreshold = float32(kbThreshold)
	}

	// Plan-Execute 特有配置
	if pp, ok := config["planner_prompt"].(string); ok {
		aiConfig.PlannerPrompt = pp
	}
	if mps, ok := config["max_plan_steps"].(float64); ok {
		aiConfig.MaxPlanSteps = int(mps)
	}
	if er, ok := config["enable_replanner"].(bool); ok {
		aiConfig.EnableReplanner = er
	}

	// Reflection 特有配置
	if cp, ok := config["critique_prompt"].(string); ok {
		aiConfig.CritiquePrompt = cp
	}
	if ip, ok := config["improve_prompt"].(string); ok {
		aiConfig.ImprovePrompt = ip
	}

	// Supervisor / DeepAgent 特有配置
	if subIDs, ok := config["sub_agent_node_ids"].([]any); ok {
		for _, id := range subIDs {
			if s, ok := id.(string); ok {
				aiConfig.SubAgentNodeIDs = append(aiConfig.SubAgentNodeIDs, s)
			}
		}
	}
	if mi, ok := config["max_iterations"].(float64); ok {
		aiConfig.MaxIterations = int(mi)
	}

	// HITL 配置
	if he, ok := config["hitl_enabled"].(bool); ok {
		aiConfig.HITLEnabled = he
	}
	if hat, ok := config["hitl_approve_tools"].([]any); ok {
		for _, t := range hat {
			if s, ok := t.(string); ok {
				aiConfig.HITLApproveTools = append(aiConfig.HITLApproveTools, s)
			}
		}
	}
	if hap, ok := config["hitl_approve_plan"].(bool); ok {
		aiConfig.HITLApprovePlan = hap
	}

	return aiConfig, nil
}

// resolveConfigVariables 解析配置中的变量引用（包级函数）
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

	if config.PlannerPrompt != "" {
		config.PlannerPrompt = resolver.ResolveString(config.PlannerPrompt, evalCtx)
	}
	if config.CritiquePrompt != "" {
		config.CritiquePrompt = resolver.ResolveString(config.CritiquePrompt, evalCtx)
	}
	if config.ImprovePrompt != "" {
		config.ImprovePrompt = resolver.ResolveString(config.ImprovePrompt, evalCtx)
	}

	return config
}

