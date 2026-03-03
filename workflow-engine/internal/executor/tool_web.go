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

// ========== Bing 搜索工具 ==========

type BingSearchTool struct{}

func (t *BingSearchTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "bing_search",
		Description: "使用 Bing 搜索引擎搜索互联网信息。适合中文搜索、国内信息查询。返回搜索结果的标题、摘要和链接。",
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

func (t *BingSearchTool) Execute(ctx context.Context, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error) {
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

	results, err := bingSearch(ctx, args.Query, args.MaxResults)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("Bing 搜索失败: %v", err)), nil
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

func bingSearch(ctx context.Context, query string, maxResults int) ([]searchResult, error) {
	searchURL := fmt.Sprintf("https://cn.bing.com/search?q=%s&count=%d&ensearch=0", url.QueryEscape(query), maxResults)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cookie", "ENSEARCH=BENVER=0;")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bing 返回 HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}

	return parseBingHTML(string(body), maxResults), nil
}

func parseBingHTML(html string, maxResults int) []searchResult {
	var results []searchResult
	remaining := html

	for len(results) < maxResults {
		algoIdx := strings.Index(remaining, `class="b_algo"`)
		if algoIdx < 0 {
			break
		}
		remaining = remaining[algoIdx:]

		nextAlgoIdx := strings.Index(remaining[14:], `class="b_algo"`)
		var block string
		if nextAlgoIdx >= 0 {
			block = remaining[:nextAlgoIdx+14]
		} else {
			block = remaining
		}
		remaining = remaining[14:]

		linkURL := extractFirstHref(block)
		if linkURL == "" {
			continue
		}

		title := extractBingTitle(block)
		if title == "" {
			continue
		}

		snippet := extractBingSnippet(block)

		if strings.HasPrefix(linkURL, "http") {
			results = append(results, searchResult{
				Title:   strings.TrimSpace(title),
				Snippet: strings.TrimSpace(snippet),
				URL:     linkURL,
			})
		}
	}
	return results
}

func extractFirstHref(block string) string {
	h2Idx := strings.Index(block, "<h2")
	if h2Idx >= 0 {
		h2Block := block[h2Idx:]
		h2End := strings.Index(h2Block, "</h2>")
		if h2End < 0 {
			h2End = len(h2Block)
		}
		if u := findHrefInTag(h2Block[:h2End]); u != "" {
			return u
		}
	}
	return findHrefInTag(block)
}

func findHrefInTag(s string) string {
	search := s
	for {
		idx := strings.Index(search, `href="`)
		if idx < 0 {
			return ""
		}
		val := search[idx+6:]
		end := strings.Index(val, `"`)
		if end < 0 {
			return ""
		}
		u := val[:end]
		if strings.HasPrefix(u, "http") && !strings.Contains(u, "bing.com/rs/") && !strings.HasSuffix(u, ".css") && !strings.HasSuffix(u, ".js") {
			return u
		}
		search = val[end:]
	}
}

func extractBingTitle(block string) string {
	h2Idx := strings.Index(block, "<h2")
	if h2Idx < 0 {
		return ""
	}
	h2HTML := block[h2Idx:]
	closeIdx := strings.Index(h2HTML, "</h2>")
	if closeIdx < 0 {
		return ""
	}
	return stripTags(h2HTML[:closeIdx])
}

func extractBingSnippet(block string) string {
	for _, marker := range []string{`class="b_lineclamp`, `class="b_caption"`, `class="b_paractl"`} {
		sIdx := strings.Index(block, marker)
		if sIdx < 0 {
			continue
		}
		section := block[sIdx:]

		pIdx := strings.Index(section, "<p")
		if pIdx >= 0 {
			section = section[pIdx:]
			tagEnd := strings.Index(section, ">")
			if tagEnd < 0 {
				continue
			}
			section = section[tagEnd+1:]
			pClose := strings.Index(section, "</p>")
			if pClose >= 0 {
				return stripTags(section[:pClose])
			}
		}

		tagEnd := strings.Index(section, ">")
		if tagEnd >= 0 {
			section = section[tagEnd+1:]
			closeDiv := strings.Index(section, "</div>")
			if closeDiv >= 0 && closeDiv < 2000 {
				return stripTags(section[:closeDiv])
			}
		}
	}
	return ""
}

// ========== Google 搜索工具 ==========

type GoogleSearchTool struct{}

func (t *GoogleSearchTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "google_search",
		Description: "使用 Google 搜索引擎搜索互联网信息。适合英文搜索、国际信息查询。注意：需要能访问 Google 的网络环境。返回搜索结果的标题、摘要和链接。",
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

func (t *GoogleSearchTool) Execute(ctx context.Context, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error) {
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

	results, err := googleSearch(ctx, args.Query, args.MaxResults)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("Google 搜索失败: %v", err)), nil
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

func googleSearch(ctx context.Context, query string, maxResults int) ([]searchResult, error) {
	searchURL := fmt.Sprintf("https://www.google.com/search?q=%s&num=%d&hl=zh-CN", url.QueryEscape(query), maxResults)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

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

	return parseGoogleHTML(string(body), maxResults), nil
}

func parseGoogleHTML(html string, maxResults int) []searchResult {
	var results []searchResult
	remaining := html

	for len(results) < maxResults {
		// Google 搜索结果链接在 <a href="/url?q=..." 或直接 <a href="https://..."
		var linkURL, title, snippet string

		// 查找搜索结果块（class="g" 是 Google 的标准结果块）
		gIdx := strings.Index(remaining, `class="g"`)
		if gIdx < 0 {
			break
		}
		remaining = remaining[gIdx:]

		// 提取链接
		hrefIdx := strings.Index(remaining, `<a href="`)
		if hrefIdx < 0 || hrefIdx > 500 {
			remaining = remaining[10:]
			continue
		}
		remaining = remaining[hrefIdx+9:]
		hrefEnd := strings.Index(remaining, `"`)
		if hrefEnd < 0 {
			break
		}
		rawURL := remaining[:hrefEnd]
		remaining = remaining[hrefEnd:]

		// 解析 Google 重定向 URL
		if strings.HasPrefix(rawURL, "/url?") {
			if u, err := url.Parse(rawURL); err == nil {
				if q := u.Query().Get("q"); q != "" {
					linkURL = q
				}
			}
		} else if strings.HasPrefix(rawURL, "http") {
			linkURL = rawURL
		}

		// 提取标题
		h3Idx := strings.Index(remaining[:min(len(remaining), 1000)], "<h3")
		if h3Idx >= 0 {
			h3HTML := remaining[h3Idx:]
			tagEnd := strings.Index(h3HTML, ">")
			if tagEnd >= 0 {
				h3HTML = h3HTML[tagEnd+1:]
				closeIdx := strings.Index(h3HTML, "</h3>")
				if closeIdx >= 0 {
					title = stripTags(h3HTML[:closeIdx])
				}
			}
		}

		// 提取摘要（通常在 data-sncf 或 class 含 "VwiC3b" 的 span 中）
		for _, marker := range []string{`class="VwiC3b"`, `data-sncf=`} {
			sIdx := strings.Index(remaining[:min(len(remaining), 3000)], marker)
			if sIdx >= 0 {
				sHTML := remaining[sIdx:]
				sTagEnd := strings.Index(sHTML, ">")
				if sTagEnd >= 0 {
					sHTML = sHTML[sTagEnd+1:]
					sClose := strings.Index(sHTML, "</span>")
					if sClose < 0 {
						sClose = strings.Index(sHTML, "</div>")
					}
					if sClose >= 0 && sClose < 2000 {
						snippet = stripTags(sHTML[:sClose])
					}
				}
				break
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

// ========== 共享类型和辅助函数 ==========

type searchResult struct {
	Title   string
	Snippet string
	URL     string
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
	return decodeHTMLEntities(strings.TrimSpace(result.String()))
}

func decodeHTMLEntities(s string) string {
	r := strings.NewReplacer(
		"&nbsp;", " ",
		"&ensp;", " ",
		"&emsp;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
		"&#0183;", "·",
		"&#183;", "·",
		"&middot;", "·",
		"&hellip;", "...",
		"&mdash;", "—",
		"&ndash;", "–",
		"&laquo;", "«",
		"&raquo;", "»",
	)
	return r.Replace(s)
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
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

	execCtx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	execCmd := execCommandContext(execCtx2, cmd, cmdArgs...)
	output, err := execCmd.CombinedOutput()
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
	RegisterTool(&BingSearchTool{})
	RegisterTool(&GoogleSearchTool{})
	RegisterTool(&WebFetchTool{})
	RegisterTool(&CodeExecuteTool{})
}
