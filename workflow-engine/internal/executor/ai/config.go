package ai

import "yqhp/workflow-engine/internal/executor"

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
	SystemPrompt      string        `json:"system_prompt,omitempty"`
	Prompt            string        `json:"prompt"`
	PromptMultiContent []interface{} `json:"-"` // 运行时填充，多模态用户消息的原始 ContentPart 数组
	Streaming         bool          `json:"streaming"`
	Timeout         int      `json:"timeout,omitempty"`

	// ===== 工具配置 =====
	Tools              []string             `json:"tools,omitempty"`
	MCPServers         []*MCPServerConfig   `json:"mcp_servers,omitempty"`
	MaxToolRounds      int                  `json:"max_tool_rounds,omitempty"`
	Interactive        bool                 `json:"interactive"`
	InteractionTimeout int                  `json:"interaction_timeout,omitempty"`
	Skills             []*SkillInfo         `json:"skills,omitempty"`
	KnowledgeBases     []*KnowledgeBaseInfo `json:"knowledge_bases,omitempty"`
	KBTopK             int                  `json:"kb_top_k,omitempty"`
	KBScoreThreshold   float32              `json:"kb_score_threshold,omitempty"`

	// ===== Plan 模式配置 =====
	EnablePlanMode *bool `json:"enable_plan_mode,omitempty"`
	MaxPlanSteps   int   `json:"max_plan_steps,omitempty"`

	// ===== 自我验证配置 =====
	EnableSelfVerify *bool `json:"enable_self_verify,omitempty"`

	// ===== Fallback 配置 =====
	FallbackModels []FallbackModelConfig `json:"fallback_models,omitempty"`

	// ===== 基础设施地址 =====
	QdrantHost  string `json:"qdrant_host,omitempty"`
	GuluHost    string `json:"gulu_host,omitempty"`
	ToolTimeout int    `json:"tool_timeout,omitempty"`
}

// FallbackModelConfig 备选模型配置
type FallbackModelConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url,omitempty"`
}

// SkillInfo Skill 能力信息
type SkillInfo struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
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

// MCPServerConfig MCP 服务器连接配置（由 gulu handler 从数据库解析注入）
type MCPServerConfig struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Timeout   int               `json:"timeout,omitempty"`
}

func parseConfig(config map[string]any) (*AIConfig, error) {
	return parseAIConfig(config)
}

func resolveVariables(config *AIConfig, execCtx *executor.ExecutionContext) *AIConfig {
	return resolveConfigVariables(config, execCtx)
}
