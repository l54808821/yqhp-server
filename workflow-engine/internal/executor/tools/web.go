package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

// ========== Tavily 搜索工具 ==========

type TavilySearchTool struct{}

func (t *TavilySearchTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "web_search",
		Description: "搜索互联网信息。支持中英文搜索，返回结构化的搜索结果（标题、摘要、链接、正文提取）。可通过 search_depth 控制搜索深度。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "搜索查询关键词"
				},
				"max_results": {
					"type": "integer",
					"description": "返回结果数量，默认 5，最多 10"
				},
				"search_depth": {
					"type": "string",
					"enum": ["basic", "advanced"],
					"description": "搜索深度：basic 快速搜索，advanced 深度搜索（会提取页面正文），默认 basic"
				},
				"include_answer": {
					"type": "boolean",
					"description": "是否返回 AI 生成的直接答案摘要，默认 true"
				}
			},
			"required": ["query"]
		}`),
	}
}

func (t *TavilySearchTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Query         string `json:"query"`
		MaxResults    int    `json:"max_results"`
		SearchDepth   string `json:"search_depth"`
		IncludeAnswer *bool  `json:"include_answer"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Query == "" {
		return types.NewErrorResult("缺少必填参数: query"), nil
	}
	if args.MaxResults <= 0 {
		args.MaxResults = 5
	}
	if args.MaxResults > 10 {
		args.MaxResults = 10
	}
	if args.SearchDepth == "" {
		args.SearchDepth = "basic"
	}
	includeAnswer := true
	if args.IncludeAnswer != nil {
		includeAnswer = *args.IncludeAnswer
	}

	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return types.NewErrorResult("未配置 TAVILY_API_KEY 环境变量"), nil
	}

	result, err := tavilySearch(ctx, apiKey, args.Query, args.MaxResults, args.SearchDepth, includeAnswer)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("搜索失败: %v", err)), nil
	}
	return result, nil
}

type tavilyRequest struct {
	APIKey        string `json:"api_key"`
	Query         string `json:"query"`
	MaxResults    int    `json:"max_results"`
	SearchDepth   string `json:"search_depth"`
	IncludeAnswer bool   `json:"include_answer"`
}

type tavilyResponse struct {
	Answer  string         `json:"answer"`
	Results []tavilyResult `json:"results"`
}

type tavilyResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

func tavilySearch(ctx context.Context, apiKey, query string, maxResults int, searchDepth string, includeAnswer bool) (*types.ToolResult, error) {
	reqBody := tavilyRequest{
		APIKey:        apiKey,
		Query:         query,
		MaxResults:    maxResults,
		SearchDepth:   searchDepth,
		IncludeAnswer: includeAnswer,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 Tavily API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tavily API 返回 HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tavilyResp tavilyResponse
	if err := json.Unmarshal(body, &tavilyResp); err != nil {
		return nil, fmt.Errorf("解析 Tavily 响应失败: %w", err)
	}

	var sb strings.Builder
	if tavilyResp.Answer != "" {
		sb.WriteString(fmt.Sprintf("**搜索摘要**: %s\n\n---\n\n", tavilyResp.Answer))
	}

	if len(tavilyResp.Results) == 0 {
		if tavilyResp.Answer != "" {
			return types.NewToolResult(sb.String()), nil
		}
		return types.NewToolResult("未找到相关搜索结果。"), nil
	}

	for i, r := range tavilyResp.Results {
		sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, r.Title))
		if r.Content != "" {
			content := r.Content
			if len([]rune(content)) > 500 {
				content = string([]rune(content)[:500]) + "..."
			}
			sb.WriteString(content)
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("URL: %s\n\n", r.URL))
	}

	return types.NewToolResult(sb.String()), nil
}

// ========== Jina Reader 网页读取工具 ==========

type JinaReaderTool struct{}

func (t *JinaReaderTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "web_read",
		Description: "读取指定 URL 的网页内容，返回干净的 Markdown 格式。自动处理 JavaScript 渲染的页面，提取正文内容并过滤广告。适合读取文章、API 文档、博客等。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": {
					"type": "string",
					"description": "要读取内容的网页 URL"
				},
				"max_length": {
					"type": "integer",
					"description": "最大返回字符数，默认 50000"
				}
			},
			"required": ["url"]
		}`),
	}
}

func (t *JinaReaderTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		URL       string `json:"url"`
		MaxLength int    `json:"max_length"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.URL == "" {
		return types.NewErrorResult("缺少必填参数: url"), nil
	}
	if args.MaxLength <= 0 {
		args.MaxLength = 50000
	}

	content, err := jinaRead(ctx, args.URL, args.MaxLength)
	if err != nil {
		logger.Warn("[JinaReader] Jina Reader 失败，回退到直接抓取: %v", err)
		return directFetch(ctx, args.URL, args.MaxLength)
	}

	return types.NewToolResult(content), nil
}

func jinaRead(ctx context.Context, targetURL string, maxLength int) (string, error) {
	jinaURL := "https://r.jina.ai/" + targetURL

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jinaURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Accept", "text/plain")
	req.Header.Set("X-Return-Format", "markdown")

	jinaAPIKey := os.Getenv("JINA_API_KEY")
	if jinaAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+jinaAPIKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 Jina Reader 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Jina Reader 返回 HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxLength)+1024))
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	content := string(body)
	if len(content) > maxLength {
		content = content[:maxLength] + "\n...(内容已截断)"
	}

	return content, nil
}

func directFetch(ctx context.Context, targetURL string, maxLength int) (*types.ToolResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("创建请求失败: %v", err)), nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("请求失败: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.NewErrorResult(fmt.Sprintf("HTTP 状态码: %d", resp.StatusCode)), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxLength)+1024))
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("读取响应失败: %v", err)), nil
	}

	content := string(body)
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		content = HTMLToText(content)
	}

	if len(content) > maxLength {
		content = content[:maxLength] + "\n...(内容已截断)"
	}

	return types.NewToolResult(content), nil
}
