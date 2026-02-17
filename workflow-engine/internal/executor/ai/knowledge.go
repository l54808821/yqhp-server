package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"yqhp/workflow-engine/pkg/types"
)

// KnowledgeBaseInfo 知识库信息（由 gulu 层从数据库查询后注入到 config）
type KnowledgeBaseInfo struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	Type             string  `json:"type"`              // normal / graph
	QdrantCollection string  `json:"qdrant_collection"` // Qdrant collection 名称
	Neo4jDatabase    string  `json:"neo4j_database"`    // Neo4j database 名称
	EmbeddingModel   string  `json:"embedding_model"`   // 嵌入模型名称
	EmbeddingModelID int64   `json:"embedding_model_id"`
	TopK             int     `json:"top_k"`
	ScoreThreshold   float64 `json:"score_threshold"`
	// 嵌入模型的 API 配置（由 gulu 层从 t_ai_model 解析后注入）
	EmbeddingProvider string `json:"embedding_provider"`
	EmbeddingAPIKey   string `json:"embedding_api_key"`
	EmbeddingBaseURL  string `json:"embedding_base_url"`
	EmbeddingDimension int   `json:"embedding_dimension"`
}

// knowledgeSearchToolName 知识库检索工具名称
const knowledgeSearchToolName = "knowledge_search"

// knowledgeSearchToolDef 知识库检索工具定义
func knowledgeSearchToolDef(kbNames []string) *types.ToolDefinition {
	kbList := strings.Join(kbNames, "、")
	return &types.ToolDefinition{
		Name:        knowledgeSearchToolName,
		Description: fmt.Sprintf("[知识库检索] 从以下知识库中检索相关信息：%s。当你需要获取更精确的知识来回答用户问题时，可调用此工具。", kbList),
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "检索查询内容，尽量用简洁且有针对性的关键词或短句"
				},
				"top_k": {
					"type": "integer",
					"description": "返回结果数量，默认为 5"
				}
			},
			"required": ["query"]
		}`),
	}
}

// retrieveKnowledge 检索知识库并返回相关上下文（上下文注入模式）
// 在 AI 调用前执行，将检索到的知识注入到系统提示词中
func (e *AIExecutor) retrieveKnowledge(ctx context.Context, query string, knowledgeBases []*KnowledgeBaseInfo, topK int) string {
	if len(knowledgeBases) == 0 || query == "" {
		return ""
	}

	if topK <= 0 {
		topK = 5
	}

	var allResults []knowledgeChunk

	for _, kb := range knowledgeBases {
		if kb.Type == "normal" && kb.QdrantCollection != "" {
			results := e.searchQdrant(ctx, kb, query, topK)
			allResults = append(allResults, results...)
		}
		// TODO: Phase 3 - 图知识库检索
	}

	if len(allResults) == 0 {
		return ""
	}

	// 按相关度排序，截取 topK 个结果
	if len(allResults) > topK {
		allResults = allResults[:topK]
	}

	// 格式化为上下文文本
	var sb strings.Builder
	sb.WriteString("以下是从知识库中检索到的相关参考资料，请结合这些信息来回答用户的问题：\n\n")
	for i, chunk := range allResults {
		sb.WriteString(fmt.Sprintf("--- 参考 %d (来源: %s, 相关度: %.2f) ---\n", i+1, chunk.Source, chunk.Score))
		sb.WriteString(chunk.Content)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// executeKnowledgeSearch 执行知识库检索工具调用
func (e *AIExecutor) executeKnowledgeSearch(ctx context.Context, arguments string, knowledgeBases []*KnowledgeBaseInfo) *types.ToolResult {
	var args struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("知识库检索参数解析失败: %v", err),
		}
	}

	if args.Query == "" {
		return &types.ToolResult{
			IsError: true,
			Content: "检索查询内容不能为空",
		}
	}

	topK := args.TopK
	if topK <= 0 {
		topK = 5
	}

	var allResults []knowledgeChunk
	for _, kb := range knowledgeBases {
		if kb.Type == "normal" && kb.QdrantCollection != "" {
			results := e.searchQdrant(ctx, kb, args.Query, topK)
			allResults = append(allResults, results...)
		}
	}

	if len(allResults) == 0 {
		return &types.ToolResult{
			IsError: false,
			Content: "未找到与查询相关的知识库内容。",
		}
	}

	if len(allResults) > topK {
		allResults = allResults[:topK]
	}

	// 格式化结果
	var sb strings.Builder
	for i, chunk := range allResults {
		sb.WriteString(fmt.Sprintf("[%d] (来源: %s, 相关度: %.2f)\n%s\n\n", i+1, chunk.Source, chunk.Score, chunk.Content))
	}

	return &types.ToolResult{
		IsError: false,
		Content: sb.String(),
	}
}

// searchQdrant 搜索 Qdrant 向量数据库
func (e *AIExecutor) searchQdrant(ctx context.Context, kb *KnowledgeBaseInfo, query string, topK int) []knowledgeChunk {
	// TODO: 实际实现需要:
	// 1. 调用嵌入模型将 query 转为向量
	// 2. 调用 Qdrant 进行相似度搜索
	// 3. 返回结果

	// 当前占位实现
	fmt.Printf("[INFO] 知识库检索: kb=%s, collection=%s, query=%s, topK=%d\n",
		kb.Name, kb.QdrantCollection, query, topK)

	return nil
}

// knowledgeChunk 知识片段
type knowledgeChunk struct {
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	Source     string  `json:"source"`
	DocumentID int64   `json:"document_id"`
	ChunkIndex int     `json:"chunk_index"`
}

// buildKnowledgeInstruction 构建知识库能力说明，追加到系统提示词中
func buildKnowledgeInstruction(kbs []*KnowledgeBaseInfo) string {
	var sb strings.Builder
	sb.WriteString("\n\n[知识库]\n")
	sb.WriteString("你已接入以下知识库，可随时通过 knowledge_search 工具检索更精确的信息：\n\n")

	for _, kb := range kbs {
		typeLabel := "向量知识库"
		if kb.Type == "graph" {
			typeLabel = "图知识库"
		}
		sb.WriteString(fmt.Sprintf("- %s (%s)\n", kb.Name, typeLabel))
	}

	sb.WriteString("\n当用户的问题可能需要专业知识或事实依据时，请主动使用 knowledge_search 工具检索。")
	return sb.String()
}
