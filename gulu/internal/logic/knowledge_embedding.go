package logic

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
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
	BaseURL        string
	APIKey         string
	Model          string
	Timeout        time.Duration
	SupportInputType bool // 是否在 API 请求中发送 input_type 字段（Qwen3-Embedding 等模型需要显式启用）
}

// NewEmbeddingClient 创建嵌入模型客户端
func NewEmbeddingClient(baseURL, apiKey, model string) *EmbeddingClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &EmbeddingClient{
		BaseURL:          baseURL,
		APIKey:           apiKey,
		Model:            model,
		Timeout:          60 * time.Second,
		SupportInputType: false, // 默认不发送，避免破坏不支持该字段的本地部署
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

// TextEmbedPurpose 文本 Embedding 用途（影响部分模型的向量质量）
type TextEmbedPurpose string

const (
	EmbedPurposeDocument TextEmbedPurpose = "document" // 索引阶段，文档内容
	EmbedPurposeQuery    TextEmbedPurpose = "query"    // 检索阶段，查询文本
)

// embeddingRequest OpenAI Embedding API 请求
type embeddingRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"` // string, []string, or []map for multimodal
}

// embeddingRequestWithInputType 带 input_type 的请求（Qwen3-Embedding 等模型专用）
// 仅在知识库明确配置了 input_type 支持时才使用
type embeddingRequestWithInputType struct {
	Model     string      `json:"model"`
	Input     interface{} `json:"input"`
	InputType string      `json:"input_type,omitempty"`
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

// EmbedTexts 批量生成文档 Embedding（索引阶段使用）
func (c *EmbeddingClient) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	return c.embedTextsWithPurpose(ctx, texts, EmbedPurposeDocument)
}

// EmbedTextAsQuery 生成查询 Embedding（检索阶段使用）
// 对 Qwen3-Embedding 等支持 input_type 的模型效果更好
func (c *EmbeddingClient) EmbedTextAsQuery(ctx context.Context, text string) ([]float32, error) {
	results, err := c.embedTextsWithPurpose(ctx, []string{text}, EmbedPurposeQuery)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("Embedding 结果为空")
	}
	return results[0], nil
}

// embedTextsWithPurpose 带用途参数的批量 Embedding 实现
func (c *EmbeddingClient) embedTextsWithPurpose(ctx context.Context, texts []string, purpose TextEmbedPurpose) ([][]float32, error) {
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

		embeddings, err := c.callEmbeddingAPI(ctx, batch, string(purpose))
		if err != nil {
			return nil, fmt.Errorf("Embedding API 调用失败 (batch %d-%d): %w", i, end, err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// EmbedText 生成单条文档 Embedding（索引阶段使用）
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

// doHTTPPost 发送 JSON POST 请求并返回响应体（提取公共 HTTP 逻辑）
func (c *EmbeddingClient) doHTTPPost(ctx context.Context, url string, reqBody interface{}, timeout time.Duration) ([]byte, error) {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("请求序列化失败: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回错误 (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// callMultimodalEmbeddingAPI 调用多模态 Embedding API
func (c *EmbeddingClient) callMultimodalEmbeddingAPI(ctx context.Context, inputs []MultimodalInput) ([][]float32, error) {
	inputItems := make([]map[string]string, len(inputs))
	for i, inp := range inputs {
		switch inp.Type {
		case EmbeddingInputImage:
			b64 := base64.StdEncoding.EncodeToString(inp.ImageData)
			mimeType := detectImageMime(inp.ImageData)
			inputItems[i] = map[string]string{"image": fmt.Sprintf("data:%s;base64,%s", mimeType, b64)}
		default:
			inputItems[i] = map[string]string{"text": inp.Text}
		}
	}

	respBody, err := c.doHTTPPost(ctx, c.BaseURL+"/embeddings",
		embeddingRequest{Model: c.Model, Input: inputItems}, c.Timeout*2)
	if err != nil {
		return nil, err
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
			embeddings[d.Index] = normalizeVector(d.Embedding)
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
// inputType：传 "document" 或 "query"，仅在 SupportInputType=true 时才发送
func (c *EmbeddingClient) callEmbeddingAPI(ctx context.Context, texts []string, inputType string) ([][]float32, error) {
	var reqBody interface{}
	if c.SupportInputType && inputType != "" {
		reqBody = embeddingRequestWithInputType{Model: c.Model, Input: texts, InputType: inputType}
	} else {
		reqBody = embeddingRequest{Model: c.Model, Input: texts}
	}

	respBody, err := c.doHTTPPost(ctx, c.BaseURL+"/embeddings", reqBody, c.Timeout)
	if err != nil {
		return nil, err
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
			embeddings[d.Index] = normalizeVector(d.Embedding)
		}
	}
	for i, emb := range embeddings {
		if emb == nil {
			return nil, fmt.Errorf("缺少第 %d 条文本的 Embedding 结果", i)
		}
	}
	return embeddings, nil
}

// normalizeVector 对向量做 L2 归一化（与 Dify 行为保持一致）
// 对于 Qdrant Cosine 距离，归一化后向量长度统一，相似度计算更稳定
func normalizeVector(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	norm := float32(math.Sqrt(sum))
	if norm < 1e-10 {
		return v
	}
	result := make([]float32, len(v))
	for i, x := range v {
		result[i] = x / norm
	}
	return result
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
