package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// ========== Web 搜索工具 ==========

type WebSearchTool struct{}

func (t *WebSearchTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "web_search",
		Description: "在互联网上搜索信息。当你需要最新信息、事实验证、或回答需要实时数据的问题时使用。返回搜索结果的标题、摘要和链接。",
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
				}
			},
			"required": ["query"]
		}`),
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
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

	results, err := duckDuckGoSearch(ctx, args.Query, args.MaxResults)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("搜索失败: %v", err)), nil
	}
	if len(results) == 0 {
		return types.NewToolResult("未找到相关搜索结果。"), nil
	}

	var sb strings.Builder
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("[%d] %s\n%s\nURL: %s\n\n", i+1, r.Title, r.Snippet, r.URL))
	}
	return types.NewToolResult(sb.String()), nil
}

type searchResult struct {
	Title   string
	Snippet string
	URL     string
}

func duckDuckGoSearch(ctx context.Context, query string, maxResults int) ([]searchResult, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; YQHPBot/1.0)")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, err
	}

	return parseDDGHTML(string(body), maxResults), nil
}

func parseDDGHTML(html string, maxResults int) []searchResult {
	var results []searchResult
	remaining := html

	for len(results) < maxResults {
		linkStart := strings.Index(remaining, `class="result__a"`)
		if linkStart < 0 {
			break
		}
		remaining = remaining[linkStart:]

		hrefIdx := strings.Index(remaining, `href="`)
		if hrefIdx < 0 {
			break
		}
		remaining = remaining[hrefIdx+6:]
		hrefEnd := strings.Index(remaining, `"`)
		if hrefEnd < 0 {
			break
		}
		rawURL := remaining[:hrefEnd]
		remaining = remaining[hrefEnd:]

		linkURL := rawURL
		if strings.Contains(rawURL, "uddg=") {
			if u, err := url.Parse(rawURL); err == nil {
				if uddg := u.Query().Get("uddg"); uddg != "" {
					linkURL = uddg
				}
			}
		}

		tagEnd := strings.Index(remaining, ">")
		if tagEnd < 0 {
			break
		}
		remaining = remaining[tagEnd+1:]
		closeTag := strings.Index(remaining, "</a>")
		if closeTag < 0 {
			break
		}
		title := stripTags(remaining[:closeTag])
		remaining = remaining[closeTag:]

		snippet := ""
		snippetStart := strings.Index(remaining, `class="result__snippet"`)
		if snippetStart >= 0 {
			snippetHTML := remaining[snippetStart:]
			tagStart := strings.Index(snippetHTML, ">")
			if tagStart >= 0 {
				snippetHTML = snippetHTML[tagStart+1:]
				tagClose := strings.Index(snippetHTML, "</")
				if tagClose >= 0 {
					snippet = stripTags(snippetHTML[:tagClose])
				}
			}
		}

		if title != "" && linkURL != "" {
			results = append(results, searchResult{
				Title:   strings.TrimSpace(title),
				Snippet: strings.TrimSpace(snippet),
				URL:     linkURL,
			})
		}
	}
	return results
}

func stripTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, ch := range s {
		if ch == '<' {
			inTag = true
		} else if ch == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(ch)
		}
	}
	return strings.TrimSpace(result.String())
}

// ========== Web 内容抓取工具 ==========

type WebFetchTool struct{}

func (t *WebFetchTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "web_fetch",
		Description: "获取指定 URL 的网页内容。用于读取网页文章、API 文档、或任何公开可访问的 URL 内容。返回纯文本格式。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": {
					"type": "string",
					"description": "要获取内容的 URL"
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

func (t *WebFetchTool) Execute(ctx context.Context, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error) {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, args.URL, nil)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("创建请求失败: %v", err)), nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; YQHPBot/1.0)")
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(args.MaxLength)+1024))
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("读取响应失败: %v", err)), nil
	}

	content := string(body)
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		content = htmlToText(content)
	}

	if len(content) > args.MaxLength {
		content = content[:args.MaxLength] + "\n...(内容已截断)"
	}

	return types.NewToolResult(content), nil
}

func htmlToText(html string) string {
	for _, tag := range []string{"script", "style", "noscript"} {
		for {
			startTag := strings.Index(strings.ToLower(html), "<"+tag)
			if startTag < 0 {
				break
			}
			endTag := strings.Index(strings.ToLower(html[startTag:]), "</"+tag+">")
			if endTag < 0 {
				html = html[:startTag]
				break
			}
			html = html[:startTag] + html[startTag+endTag+len("</"+tag+">"):]
		}
	}

	text := stripTags(html)
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "\n")
}

// ========== 代码执行工具 ==========

type CodeExecuteTool struct{}

func (t *CodeExecuteTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "code_execute",
		Description: "执行代码片段。支持 Python 和 JavaScript (Node.js)。用于数据计算、格式转换、文本处理等场景。代码在沙箱中运行，有 30 秒超时限制。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"language": {
					"type": "string",
					"enum": ["python", "javascript"],
					"description": "编程语言"
				},
				"code": {
					"type": "string",
					"description": "要执行的代码"
				}
			},
			"required": ["language", "code"]
		}`),
	}
}

func (t *CodeExecuteTool) Execute(ctx context.Context, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Language string `json:"language"`
		Code     string `json:"code"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Language == "" {
		return types.NewErrorResult("缺少必填参数: language"), nil
	}
	if args.Code == "" {
		return types.NewErrorResult("缺少必填参数: code"), nil
	}

	var cmd string
	var cmdArgs []string
	switch args.Language {
	case "python":
		cmd = "python3"
		cmdArgs = []string{"-c", args.Code}
	case "javascript":
		cmd = "node"
		cmdArgs = []string{"-e", args.Code}
	default:
		return types.NewErrorResult(fmt.Sprintf("不支持的语言: %s，仅支持 python 和 javascript", args.Language)), nil
	}

	import_ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	import_cmd := execCommandContext(import_ctx, cmd, cmdArgs...)
	output, err := import_cmd.CombinedOutput()
	if err != nil {
		result := string(output)
		if result == "" {
			result = err.Error()
		}
		return types.NewErrorResult(fmt.Sprintf("执行失败:\n%s", result)), nil
	}

	return types.NewToolResult(string(output)), nil
}

func init() {
	RegisterTool(&WebSearchTool{})
	RegisterTool(&WebFetchTool{})
	RegisterTool(&CodeExecuteTool{})
}
