package logic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// AiChatLogic AI对话逻辑
type AiChatLogic struct {
	ctx context.Context
}

// NewAiChatLogic 创建AI对话逻辑
func NewAiChatLogic(ctx context.Context) *AiChatLogic {
	return &AiChatLogic{ctx: ctx}
}

// ChatMessage 对话消息
type ChatMessage struct {
	Role    string `json:"role"`    // system, user, assistant
	Content string `json:"content"` // 消息内容
}

// ChatRequest 对话请求
type ChatRequest struct {
	Messages    []ChatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
}

// ChatStreamCallback SSE 流式回调
type ChatStreamCallback func(data string) error

// ChatStream 流式对话（调用 OpenAI 兼容 API）
func (l *AiChatLogic) ChatStream(apiBaseURL, apiKey, modelID string, req *ChatRequest, callback ChatStreamCallback) error {
	// 构建 OpenAI 兼容请求
	openaiReq := map[string]interface{}{
		"model":    modelID,
		"messages": req.Messages,
		"stream":   true,
	}
	if req.Temperature != nil {
		openaiReq["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		openaiReq["max_tokens"] = *req.MaxTokens
	}

	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		return errors.New("请求序列化失败")
	}

	// 拼接 API URL
	url := strings.TrimRight(apiBaseURL, "/") + "/chat/completions"

	httpReq, err := http.NewRequestWithContext(l.ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("请求模型API失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("模型API返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	// 读取 SSE 流
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// 跳过空行
		if line == "" {
			continue
		}

		// 跳过非 data 行
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// [DONE] 表示结束
		if data == "[DONE]" {
			if err := callback("data: [DONE]\n\n"); err != nil {
				return err
			}
			break
		}

		// 透传 SSE 数据给前端
		if err := callback("data: " + data + "\n\n"); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取SSE流失败: %w", err)
	}

	return nil
}
