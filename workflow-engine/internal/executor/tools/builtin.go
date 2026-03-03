package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

const maxResponseBodySize = 100 * 1024 // 100KB
const httpToolTimeout = 30 * time.Second

// ========== HTTP 请求工具 ==========

type HTTPTool struct{}

type httpRequestArgs struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

type httpResponseResult struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

func (t *HTTPTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "http_request",
		Description: "发送 HTTP 请求到指定 URL，支持 GET/POST/PUT/DELETE/PATCH 方法",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"method": {
					"type": "string",
					"enum": ["GET", "POST", "PUT", "DELETE", "PATCH"],
					"description": "HTTP 方法"
				},
				"url": {
					"type": "string",
					"description": "请求 URL"
				},
				"headers": {
					"type": "object",
					"description": "请求头",
					"additionalProperties": { "type": "string" }
				},
				"body": {
					"type": "string",
					"description": "请求体（JSON 字符串）"
				}
			},
			"required": ["method", "url"]
		}`),
	}
}

func (t *HTTPTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args httpRequestArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Method == "" {
		return types.NewErrorResult("缺少必填参数: method"), nil
	}
	if args.URL == "" {
		return types.NewErrorResult("缺少必填参数: url"), nil
	}

	method := strings.ToUpper(args.Method)
	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH":
	default:
		return types.NewErrorResult(fmt.Sprintf("不支持的 HTTP 方法: %s", args.Method)), nil
	}

	var bodyReader io.Reader
	if args.Body != "" {
		bodyReader = strings.NewReader(args.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, args.URL, bodyReader)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("创建请求失败: %v", err)), nil
	}
	for key, value := range args.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: httpToolTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("请求执行失败: %v", err)), nil
	}
	defer resp.Body.Close()

	limitedReader := io.LimitReader(resp.Body, int64(maxResponseBodySize)+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("读取响应体失败: %v", err)), nil
	}

	bodyStr := string(bodyBytes)
	if len(bodyBytes) > maxResponseBodySize {
		bodyStr = bodyStr[:maxResponseBodySize] + "...(已截断)"
	}

	respHeaders := make(map[string]string)
	for key := range resp.Header {
		respHeaders[key] = resp.Header.Get(key)
	}

	result := httpResponseResult{
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       bodyStr,
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("序列化响应结果失败: %v", err)), nil
	}
	return types.NewToolResult(string(resultJSON)), nil
}

// ========== 变量读取工具 ==========

type VarReadTool struct{}

func (t *VarReadTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "var_read",
		Description: "读取工作流上下文中的变量",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "变量名"
				}
			},
			"required": ["name"]
		}`),
	}
}

func (t *VarReadTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Name == "" {
		return types.NewErrorResult("缺少必填参数: name"), nil
	}
	if execCtx == nil {
		return types.NewErrorResult("执行上下文不可用"), nil
	}

	value, ok := execCtx.GetVariable(args.Name)
	if !ok {
		return types.NewErrorResult(fmt.Sprintf("变量 %q 不存在", args.Name)), nil
	}

	resultJSON, err := json.Marshal(map[string]any{"value": value})
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("序列化结果失败: %v", err)), nil
	}
	return types.NewToolResult(string(resultJSON)), nil
}

// ========== 变量写入工具 ==========

type VarWriteTool struct{}

func (t *VarWriteTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "var_write",
		Description: "设置工作流上下文中的变量",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "变量名"
				},
				"value": {
					"description": "变量值（任意类型）"
				}
			},
			"required": ["name", "value"]
		}`),
	}
}

func (t *VarWriteTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	if execCtx == nil {
		return types.NewErrorResult("执行上下文不可用"), nil
	}

	var args struct {
		Name  string `json:"name"`
		Value any    `json:"value"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.Name == "" {
		return types.NewErrorResult("缺少必填参数: name"), nil
	}

	execCtx.SetVariable(args.Name, args.Value)
	return types.NewSilentResult(`{"success": true}`), nil
}

// ========== JSON 解析工具 ==========

type JSONParseTool struct{}

func (t *JSONParseTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "json_parse",
		Description: "解析 JSON 字符串，支持通过 path 提取嵌套值（如 data.items.0.name）",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"json_string": {
					"type": "string",
					"description": "要解析的 JSON 字符串"
				},
				"path": {
					"type": "string",
					"description": "可选的路径表达式，用点号分隔"
				}
			},
			"required": ["json_string"]
		}`),
	}
}

func (t *JSONParseTool) Execute(ctx context.Context, arguments string, execCtx *executor.ExecutionContext) (*types.ToolResult, error) {
	var args struct {
		JSONString string `json:"json_string"`
		Path       string `json:"path,omitempty"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return types.NewErrorResult(fmt.Sprintf("参数解析失败: %v", err)), nil
	}
	if args.JSONString == "" {
		return types.NewErrorResult("缺少必填参数: json_string"), nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(args.JSONString), &parsed); err != nil {
		return types.NewErrorResult(fmt.Sprintf("无效的 JSON: %v", err)), nil
	}

	if args.Path != "" {
		value, err := navigateJSONPath(parsed, args.Path)
		if err != nil {
			return types.NewErrorResult(fmt.Sprintf("路径提取失败: %v", err)), nil
		}
		parsed = value
	}

	resultJSON, err := json.Marshal(parsed)
	if err != nil {
		return types.NewErrorResult(fmt.Sprintf("序列化结果失败: %v", err)), nil
	}
	return types.NewToolResult(string(resultJSON)), nil
}

func navigateJSONPath(data any, path string) (any, error) {
	segments := strings.Split(path, ".")
	current := data

	for _, seg := range segments {
		if seg == "" {
			continue
		}
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[seg]
			if !ok {
				return nil, fmt.Errorf("路径 %q 不存在", seg)
			}
			current = val
		case []any:
			idx, err := parseArrayIndex(seg, len(v))
			if err != nil {
				return nil, fmt.Errorf("路径 %q: %v", seg, err)
			}
			current = v[idx]
		default:
			return nil, fmt.Errorf("路径 %q: 无法在 %T 类型上导航", seg, current)
		}
	}
	return current, nil
}

func parseArrayIndex(seg string, length int) (int, error) {
	idx := 0
	for _, ch := range seg {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("无效的数组索引 %q", seg)
		}
		idx = idx*10 + int(ch-'0')
	}
	if idx < 0 || idx >= length {
		return 0, fmt.Errorf("数组索引 %d 超出范围 [0, %d)", idx, length)
	}
	return idx, nil
}
