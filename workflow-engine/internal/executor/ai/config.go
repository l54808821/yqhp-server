package ai

import (
	"yqhp/workflow-engine/internal/executor"
)

// AIConfig 统一 AI Agent 配置
type AIConfig struct {
	// ===== 基础配置 =====
	Provider        string   `json:"provider"`
	Model           string   `json:"model"`
	APIKey          string   `json:"api_key"`
	BaseURL         string   `json:"base_url,omitempty"`
	APIVersion      string   `json:"api_version,omitempty"`
	Temperature     *float32 `json:"temperature,omitempty"`
	MaxTokens       *int     `json:"max_tokens,omitempty"`
	TopP            *float32 `json:"top_p,omitempty"`
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`
	SystemPrompt    string   `json:"system_prompt,omitempty"`
	Prompt          string   `json:"prompt"`
	Streaming       bool     `json:"streaming"`
	Timeout         int      `json:"timeout,omitempty"`

	// ===== 工具配置 =====
	Tools              []string             `json:"tools,omitempty"`
	MCPServerIDs       []int64              `json:"mcp_server_ids,omitempty"`
	MaxToolRounds      int                  `json:"max_tool_rounds,omitempty"`
	MCPProxyBaseURL    string               `json:"mcp_proxy_base_url,omitempty"`
	Interactive        bool                 `json:"interactive"`
	InteractionTimeout int                  `json:"interaction_timeout,omitempty"`
	Skills             []*SkillInfo         `json:"skills,omitempty"`
	KnowledgeBases     []*KnowledgeBaseInfo `json:"knowledge_bases,omitempty"`
	KBTopK             int                  `json:"kb_top_k,omitempty"`
	KBScoreThreshold   float32              `json:"kb_score_threshold,omitempty"`

	// ===== Plan 模式配置 =====
	EnablePlanMode bool `json:"enable_plan_mode,omitempty"`
	MaxPlanSteps   int  `json:"max_plan_steps,omitempty"`

	// ===== 兼容旧版字段（解析时保留，运行时忽略）=====
	AgentMode string `json:"agent_mode,omitempty"`
}

// SkillInfo Skill 能力信息
type SkillInfo struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
}

// KnowledgeBaseInfo 知识库信息
type KnowledgeBaseInfo struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Type               string  `json:"type"`
	QdrantCollection   string  `json:"qdrant_collection,omitempty"`
	Neo4jDatabase      string  `json:"neo4j_database,omitempty"`
	EmbeddingModel     string  `json:"embedding_model,omitempty"`
	EmbeddingModelID   int64   `json:"embedding_model_id,omitempty"`
	TopK               int     `json:"top_k,omitempty"`
	ScoreThreshold     float64 `json:"score_threshold,omitempty"`
	EmbeddingProvider  string  `json:"embedding_provider,omitempty"`
	EmbeddingAPIKey    string  `json:"embedding_api_key,omitempty"`
	EmbeddingBaseURL   string  `json:"embedding_base_url,omitempty"`
	EmbeddingDimension int     `json:"embedding_dimension,omitempty"`
}

// parseConfig 从 map 解析配置（兼容旧版）
func parseConfig(config map[string]any) (*AIConfig, error) {
	return parseAIConfig(config)
}

// resolveVariables 解析配置中的变量引用
func resolveVariables(config *AIConfig, execCtx *executor.ExecutionContext) *AIConfig {
	return resolveConfigVariables(config, execCtx)
}
