package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

// KnowledgeTool knowledge_search 工具，实现统一 Tool 接口
type KnowledgeTool struct {
	knowledgeBases []*KnowledgeBaseInfo
	config         *AIConfig
}

func NewKnowledgeTool(kbs []*KnowledgeBaseInfo, config *AIConfig) *KnowledgeTool {
	return &KnowledgeTool{knowledgeBases: kbs, config: config}
}

func (t *KnowledgeTool) Definition() *types.ToolDefinition {
	var kbNames []string
	for _, kb := range t.knowledgeBases {
		kbNames = append(kbNames, kb.Name)
	}
	kbList := strings.Join(kbNames, "、")
	return &types.ToolDefinition{
		Name:        "knowledge_search",
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

func (t *KnowledgeTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("知识库检索参数解析失败: %v", err)), nil
	}
	if args.Query == "" {
		return types.NewErrorResult("检索查询内容不能为空"), nil
	}
	topK := args.TopK
	if topK <= 0 {
		topK = 5
	}

	var allResults []knowledgeChunk
	qdrantHost := getQdrantHost(t.config)
	guluHost := getGuluHost(t.config)
	for _, kb := range t.knowledgeBases {
		if kb.QdrantCollection != "" {
			results := searchQdrant(ctx, kb, args.Query, topK, qdrantHost)
			allResults = append(allResults, results...)
		}
		if kb.Type == "graph" {
			graphResults := searchGraph(ctx, kb, args.Query, topK, guluHost)
			allResults = append(allResults, graphResults...)
		}
	}

	if len(allResults) == 0 {
		return types.NewToolResult("未找到与查询相关的知识库内容。"), nil
	}
	if len(allResults) > topK {
		allResults = allResults[:topK]
	}

	var sb strings.Builder
	for i, chunk := range allResults {
		sb.WriteString(fmt.Sprintf("[%d] (来源: %s, 相关度: %.2f)\n%s\n\n", i+1, chunk.Source, chunk.Score, chunk.Content))
	}
	return types.NewToolResult(sb.String()), nil
}

// ========== 以下为知识库搜索的辅助函数 ==========

type knowledgeChunk struct {
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	Source     string  `json:"source"`
	DocumentID int64   `json:"document_id"`
	ChunkIndex int     `json:"chunk_index"`
}

func searchQdrant(ctx context.Context, kb *KnowledgeBaseInfo, query string, topK int, qdrantHost string) []knowledgeChunk {
	queryVector, err := callEmbeddingAPI(ctx, kb.EmbeddingBaseURL, kb.EmbeddingAPIKey, kb.EmbeddingModel, query)
	if err != nil {
		logger.Warn("[Knowledge] 知识库 %s 的查询向量化失败: %v", kb.Name, err)
		return nil
	}

	hits, err := searchQdrantREST(ctx, qdrantHost, kb.QdrantCollection, queryVector, topK, float32(kb.ScoreThreshold))
	if err != nil {
		logger.Warn("[Knowledge] 知识库 %s 向量搜索失败: %v", kb.Name, err)
		return nil
	}

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

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(httpReq)
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

func searchGraph(ctx context.Context, kb *KnowledgeBaseInfo, query string, topK int, guluHost string) []knowledgeChunk {
	reqBody := map[string]interface{}{
		"query":          query,
		"top_k":          topK,
		"retrieval_mode": "graph",
	}

	bodyBytes, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/api/knowledge-bases/%d/search", guluHost, kb.ID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		logger.Warn("[Knowledge] 图知识库 %s 搜索请求创建失败: %v", kb.Name, err)
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(httpReq)
	if err != nil {
		logger.Warn("[Knowledge] 图知识库 %s 搜索失败: %v", kb.Name, err)
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		logger.Warn("[Knowledge] 图知识库 %s 搜索返回错误: %s", kb.Name, string(body))
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
