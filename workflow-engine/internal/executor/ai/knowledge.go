package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

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
	// 嵌入模型的 API 配置
	EmbeddingProvider  string `json:"embedding_provider"`
	EmbeddingAPIKey    string `json:"embedding_api_key"`
	EmbeddingBaseURL   string `json:"embedding_base_url"`
	EmbeddingDimension int    `json:"embedding_dimension"`
	// 多模态配置（Phase 2）
	MultimodalEnabled  bool   `json:"multimodal_enabled"`
	MultimodalModel    string `json:"multimodal_model"`
	MultimodalAPIKey   string `json:"multimodal_api_key"`
	MultimodalBaseURL  string `json:"multimodal_base_url"`
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
		if kb.QdrantCollection != "" {
			results := e.searchQdrant(ctx, kb, query, topK)
			allResults = append(allResults, results...)
		}
		if kb.Type == "graph" {
			graphResults := e.searchGraph(ctx, kb, query, topK)
			allResults = append(allResults, graphResults...)
		}
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
		if kb.QdrantCollection != "" {
			results := e.searchQdrant(ctx, kb, args.Query, topK)
			allResults = append(allResults, results...)
		}
		if kb.Type == "graph" {
			graphResults := e.searchGraph(ctx, kb, args.Query, topK)
			allResults = append(allResults, graphResults...)
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
	// 1. 调用嵌入模型将查询文本转为向量
	queryVector, err := callEmbeddingAPI(ctx, kb.EmbeddingBaseURL, kb.EmbeddingAPIKey, kb.EmbeddingModel, query)
	if err != nil {
		log.Printf("[WARN] 知识库 %s 的查询向量化失败: %v", kb.Name, err)
		return nil
	}

	// 2. 通过 Qdrant REST API 搜索（使用 HTTP 而非 gRPC，避免依赖冲突）
	qdrantHost := "http://127.0.0.1:6333"
	hits, err := searchQdrantREST(ctx, qdrantHost, kb.QdrantCollection, queryVector, topK, float32(kb.ScoreThreshold))
	if err != nil {
		log.Printf("[WARN] 知识库 %s 向量搜索失败: %v", kb.Name, err)
		return nil
	}

	// 3. 转换结果
	var chunks []knowledgeChunk
	for _, hit := range hits {
		chunks = append(chunks, knowledgeChunk{
			Content:    hit.Content,
			Score:      hit.Score,
			Source:     kb.Name,
			DocumentID: hit.DocumentID,
			ChunkIndex: hit.ChunkIndex,
		})
	}

	return chunks
}

// -----------------------------------------------
// Qdrant REST API 搜索（workflow engine 侧）
// 使用 HTTP REST API (端口 6333) 代替 gRPC，避免 genproto 依赖冲突
// -----------------------------------------------

type qdrantSearchHit struct {
	Content    string
	Score      float64
	DocumentID int64
	ChunkIndex int
}

func searchQdrantREST(ctx context.Context, qdrantHost, collection string, queryVector []float32, topK int, scoreThreshold float32) ([]qdrantSearchHit, error) {
	reqBody := map[string]interface{}{
		"query":           queryVector,
		"using":           "text",
		"limit":           topK,
		"score_threshold": scoreThreshold,
		"with_payload":    true,
	}

	bodyBytes, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/collections/%s/points/query", qdrantHost, collection)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Qdrant HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Qdrant 返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Result struct {
			Points []struct {
				Score   float64                `json:"score"`
				Payload map[string]interface{} `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("Qdrant 响应解析失败: %w", err)
	}

	var hits []qdrantSearchHit
	for _, p := range result.Result.Points {
		hit := qdrantSearchHit{Score: p.Score}
		if v, ok := p.Payload["content"].(string); ok {
			hit.Content = v
		}
		if v, ok := p.Payload["document_id"].(float64); ok {
			hit.DocumentID = int64(v)
		}
		if v, ok := p.Payload["chunk_index"].(float64); ok {
			hit.ChunkIndex = int(v)
		}
		hits = append(hits, hit)
	}

	return hits, nil
}

// -----------------------------------------------
// Embedding API 调用（workflow engine 侧）
// -----------------------------------------------

// callEmbeddingAPI 调用 OpenAI-compatible /v1/embeddings 接口
func callEmbeddingAPI(ctx context.Context, baseURL, apiKey, model, text string) ([]float32, error) {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"input": text,
	})

	url := baseURL + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Embedding HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Embedding API 错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("Embedding 响应解析失败: %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("Embedding API 错误: %s", result.Error.Message)
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("Embedding 结果为空")
	}

	return result.Data[0].Embedding, nil
}

// searchGraph 通过 gulu API 搜索图知识库
func (e *AIExecutor) searchGraph(ctx context.Context, kb *KnowledgeBaseInfo, query string, topK int) []knowledgeChunk {
	// 图知识库检索通过 gulu 的 search API（retrieval_mode=graph）
	guluHost := "http://127.0.0.1:5321"
	reqBody := map[string]interface{}{
		"query":          query,
		"top_k":          topK,
		"retrieval_mode": "graph",
	}

	bodyBytes, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/api/knowledge-bases/%d/search", guluHost, kb.ID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("[WARN] 图知识库 %s 搜索请求创建失败: %v", kb.Name, err)
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(httpReq)
	if err != nil {
		log.Printf("[WARN] 图知识库 %s 搜索失败: %v", kb.Name, err)
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("[WARN] 图知识库 %s 搜索返回错误: %s", kb.Name, string(body))
		return nil
	}

	var result struct {
		Data []struct {
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil
	}

	var chunks []knowledgeChunk
	for _, item := range result.Data {
		chunks = append(chunks, knowledgeChunk{
			Content: item.Content,
			Score:   item.Score,
			Source:  kb.Name + " (图谱)",
		})
	}
	return chunks
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
