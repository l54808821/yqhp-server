package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// -----------------------------------------------
// Embedding API 客户端
// 调用 OpenAI-compatible /v1/embeddings 接口
// -----------------------------------------------

// EmbeddingClient 嵌入模型客户端
type EmbeddingClient struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

// NewEmbeddingClient 创建嵌入模型客户端
func NewEmbeddingClient(baseURL, apiKey, model string) *EmbeddingClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &EmbeddingClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		Timeout: 60 * time.Second,
	}
}

// embeddingRequest OpenAI Embedding API 请求
type embeddingRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"` // string 或 []string
}

// embeddingResponse OpenAI Embedding API 响应
type embeddingResponse struct {
	Data  []embeddingData `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type embeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbedTexts 批量生成文本 Embedding
// 输入多段文本，返回对应的向量列表
func (c *EmbeddingClient) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// 分批处理，每批最多 20 条（避免超过 API 限制）
	batchSize := 20
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		embeddings, err := c.callEmbeddingAPI(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("Embedding API 调用失败 (batch %d-%d): %w", i, end, err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// EmbedText 生成单条文本的 Embedding
func (c *EmbeddingClient) EmbedText(ctx context.Context, text string) ([]float32, error) {
	results, err := c.EmbedTexts(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("Embedding 结果为空")
	}
	return results[0], nil
}

// callEmbeddingAPI 调用 Embedding API
func (c *EmbeddingClient) callEmbeddingAPI(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := embeddingRequest{
		Model: c.Model,
		Input: texts,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("请求序列化失败: %w", err)
	}

	url := c.BaseURL + "/embeddings"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	httpClient := &http.Client{Timeout: c.Timeout}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Embedding API 返回错误 (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}

	if embResp.Error != nil {
		return nil, fmt.Errorf("Embedding API 错误: %s", embResp.Error.Message)
	}

	// 按 index 排序结果
	embeddings := make([][]float32, len(texts))
	for _, d := range embResp.Data {
		if d.Index < len(embeddings) {
			embeddings[d.Index] = d.Embedding
		}
	}

	// 验证所有位置都有结果
	for i, emb := range embeddings {
		if emb == nil {
			return nil, fmt.Errorf("缺少第 %d 条文本的 Embedding 结果", i)
		}
	}

	return embeddings, nil
}
