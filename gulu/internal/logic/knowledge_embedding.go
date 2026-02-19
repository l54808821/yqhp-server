package logic

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// -----------------------------------------------
// Embedding API 客户端
// 支持纯文本和多模态（文本+图片）嵌入
// 兼容 OpenAI /v1/embeddings 接口和 Jina 多模态接口
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

// EmbeddingInputType 嵌入输入类型
type EmbeddingInputType string

const (
	EmbeddingInputText  EmbeddingInputType = "text"
	EmbeddingInputImage EmbeddingInputType = "image"
)

// MultimodalInput 多模态输入项
type MultimodalInput struct {
	Type      EmbeddingInputType `json:"-"`
	Text      string             `json:"text,omitempty"`
	ImageData []byte             `json:"-"`
}

// embeddingRequest OpenAI Embedding API 请求
type embeddingRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"` // string, []string, or []map for multimodal
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
func (c *EmbeddingClient) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

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

// EmbedImage 生成单张图片的 Embedding（多模态模型）
// imageData: 图片的原始字节数据
func (c *EmbeddingClient) EmbedImage(ctx context.Context, imageData []byte) ([]float32, error) {
	results, err := c.EmbedMultimodal(ctx, []MultimodalInput{
		{Type: EmbeddingInputImage, ImageData: imageData},
	})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("图片 Embedding 结果为空")
	}
	return results[0], nil
}

// EmbedImages 批量生成图片 Embedding
func (c *EmbeddingClient) EmbedImages(ctx context.Context, images [][]byte) ([][]float32, error) {
	if len(images) == 0 {
		return nil, nil
	}

	inputs := make([]MultimodalInput, len(images))
	for i, img := range images {
		inputs[i] = MultimodalInput{Type: EmbeddingInputImage, ImageData: img}
	}

	return c.EmbedMultimodal(ctx, inputs)
}

// EmbedMultimodal 批量生成多模态 Embedding（混合文本和图片）
// 使用 Jina/CLIP 兼容的多模态 API 格式:
// input: [{"text": "..."}, {"image": "base64:..."}]
func (c *EmbeddingClient) EmbedMultimodal(ctx context.Context, inputs []MultimodalInput) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	batchSize := 10
	var allEmbeddings [][]float32

	for i := 0; i < len(inputs); i += batchSize {
		end := i + batchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		batch := inputs[i:end]

		embeddings, err := c.callMultimodalEmbeddingAPI(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("多模态 Embedding API 调用失败 (batch %d-%d): %w", i, end, err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// callMultimodalEmbeddingAPI 调用多模态 Embedding API
// 兼容 Jina CLIP、Google Vertex AI 等多模态嵌入模型 API 格式
func (c *EmbeddingClient) callMultimodalEmbeddingAPI(ctx context.Context, inputs []MultimodalInput) ([][]float32, error) {
	inputItems := make([]map[string]string, len(inputs))
	for i, inp := range inputs {
		switch inp.Type {
		case EmbeddingInputImage:
			b64 := base64.StdEncoding.EncodeToString(inp.ImageData)
			mimeType := detectImageMime(inp.ImageData)
			inputItems[i] = map[string]string{
				"image": fmt.Sprintf("data:%s;base64,%s", mimeType, b64),
			}
		default:
			inputItems[i] = map[string]string{
				"text": inp.Text,
			}
		}
	}

	reqBody := embeddingRequest{
		Model: c.Model,
		Input: inputItems,
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

	httpClient := &http.Client{Timeout: c.Timeout * 2}
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
		return nil, fmt.Errorf("多模态 Embedding API 返回错误 (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}

	if embResp.Error != nil {
		return nil, fmt.Errorf("多模态 Embedding API 错误: %s", embResp.Error.Message)
	}

	embeddings := make([][]float32, len(inputs))
	for _, d := range embResp.Data {
		if d.Index < len(embeddings) {
			embeddings[d.Index] = d.Embedding
		}
	}

	for i, emb := range embeddings {
		if emb == nil {
			return nil, fmt.Errorf("缺少第 %d 项的多模态 Embedding 结果", i)
		}
	}

	return embeddings, nil
}

// callEmbeddingAPI 调用纯文本 Embedding API
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

	embeddings := make([][]float32, len(texts))
	for _, d := range embResp.Data {
		if d.Index < len(embeddings) {
			embeddings[d.Index] = d.Embedding
		}
	}

	for i, emb := range embeddings {
		if emb == nil {
			return nil, fmt.Errorf("缺少第 %d 条文本的 Embedding 结果", i)
		}
	}

	return embeddings, nil
}

// detectImageMime 检测图片 MIME 类型
func detectImageMime(data []byte) string {
	if len(data) < 4 {
		return "application/octet-stream"
	}
	if data[0] == 0xFF && data[1] == 0xD8 {
		return "image/jpeg"
	}
	if data[0] == 0x89 && string(data[1:4]) == "PNG" {
		return "image/png"
	}
	if string(data[:4]) == "GIF8" {
		return "image/gif"
	}
	if string(data[:4]) == "RIFF" && len(data) > 8 && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	if len(data) > 2 && data[0] == 0x42 && data[1] == 0x4D {
		return "image/bmp"
	}
	if strings.HasPrefix(string(data), "<svg") || strings.HasPrefix(string(data), "<?xml") {
		return "image/svg+xml"
	}
	return "image/png"
}
