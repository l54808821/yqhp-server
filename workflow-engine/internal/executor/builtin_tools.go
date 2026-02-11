package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// maxResponseBodySize 响应体最大长度（10KB），超过则截断
const maxResponseBodySize = 10 * 1024

// httpToolTimeout HTTP 请求工具的默认超时时间
const httpToolTimeout = 30 * time.Second

// HTTPTool HTTP 请求工具，支持 GET、POST、PUT、DELETE 方法
type HTTPTool struct{}

// httpRequestArgs HTTP 请求工具的参数
type httpRequestArgs struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// httpResponseResult HTTP 请求工具的返回结果
type httpResponseResult struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

// Definition 返回 HTTP 请求工具的定义信息
func (t *HTTPTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "http_request",
		Description: "发送 HTTP 请求到指定 URL",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"method": {
					"type": "string",
					"enum": ["GET", "POST", "PUT", "DELETE"],
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
					"description": "请求体"
				}
			},
			"required": ["method", "url"]
		}`),
	}
}

// Execute 执行 HTTP 请求工具调用
// 解析参数 JSON，发送 HTTP 请求，返回结构化结果。
// 任何错误都通过 IsError=true 的 ToolResult 返回，不返回 Go error。
func (t *HTTPTool) Execute(ctx context.Context, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error) {
	// 解析参数
	var args httpRequestArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("参数解析失败: %v", err),
		}, nil
	}

	// 校验必填参数
	if args.Method == "" {
		return &types.ToolResult{
			IsError: true,
			Content: "缺少必填参数: method",
		}, nil
	}
	if args.URL == "" {
		return &types.ToolResult{
			IsError: true,
			Content: "缺少必填参数: url",
		}, nil
	}

	// 校验 HTTP 方法
	method := strings.ToUpper(args.Method)
	switch method {
	case "GET", "POST", "PUT", "DELETE":
		// 合法方法
	default:
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("不支持的 HTTP 方法: %s，仅支持 GET、POST、PUT、DELETE", args.Method),
		}, nil
	}

	// 构建请求体
	var bodyReader io.Reader
	if args.Body != "" {
		bodyReader = strings.NewReader(args.Body)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, method, args.URL, bodyReader)
	if err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("创建请求失败: %v", err),
		}, nil
	}

	// 设置请求头
	for key, value := range args.Headers {
		req.Header.Set(key, value)
	}

	// 发送请求，使用默认超时
	client := &http.Client{Timeout: httpToolTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("请求执行失败: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	// 读取响应体，限制最大长度
	limitedReader := io.LimitReader(resp.Body, maxResponseBodySize+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("读取响应体失败: %v", err),
		}, nil
	}

	// 截断超长响应体
	bodyStr := string(bodyBytes)
	if len(bodyBytes) > maxResponseBodySize {
		bodyStr = bodyStr[:maxResponseBodySize] + "...(已截断)"
	}

	// 提取响应头
	respHeaders := make(map[string]string)
	for key := range resp.Header {
		respHeaders[key] = resp.Header.Get(key)
	}

	// 构建结果
	result := httpResponseResult{
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       bodyStr,
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("序列化响应结果失败: %v", err),
		}, nil
	}

	return &types.ToolResult{
		IsError: false,
		Content: string(resultJSON),
	}, nil
}

// VarReadTool 变量读取工具，从 ExecutionContext 中读取变量
type VarReadTool struct{}

// varReadArgs 变量读取工具的参数
type varReadArgs struct {
	Name string `json:"name"`
}

// Definition 返回变量读取工具的定义信息
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

// Execute 执行变量读取
// 从 ExecutionContext 中读取指定名称的变量，返回 JSON 格式的结果。
// 变量不存在时返回 IsError=true 的 ToolResult。
func (t *VarReadTool) Execute(ctx context.Context, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error) {
	// 解析参数
	var args varReadArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("参数解析失败: %v", err),
		}, nil
	}

	// 校验必填参数
	if args.Name == "" {
		return &types.ToolResult{
			IsError: true,
			Content: "缺少必填参数: name",
		}, nil
	}

	// 检查执行上下文
	if execCtx == nil {
		return &types.ToolResult{
			IsError: true,
			Content: "执行上下文不可用",
		}, nil
	}

	// 读取变量
	value, ok := execCtx.GetVariable(args.Name)
	if !ok {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("变量 %q 不存在", args.Name),
		}, nil
	}

	// 构建结果
	result := map[string]any{"value": value}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("序列化结果失败: %v", err),
		}, nil
	}

	return &types.ToolResult{
		IsError: false,
		Content: string(resultJSON),
	}, nil
}

// VarWriteTool 变量写入工具，向 ExecutionContext 中写入变量
type VarWriteTool struct{}

// varWriteArgs 变量写入工具的参数
type varWriteArgs struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// Definition 返回变量写入工具的定义信息
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
					"description": "变量值"
				}
			},
			"required": ["name", "value"]
		}`),
	}
}

// Execute 执行变量写入
// 向 ExecutionContext 中写入指定名称和值的变量。
// 执行上下文为 nil 时返回 IsError=true 的 ToolResult。
func (t *VarWriteTool) Execute(ctx context.Context, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error) {
	// 检查执行上下文
	if execCtx == nil {
		return &types.ToolResult{
			IsError: true,
			Content: "执行上下文不可用",
		}, nil
	}

	// 解析参数
	var args varWriteArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("参数解析失败: %v", err),
		}, nil
	}

	// 校验必填参数
	if args.Name == "" {
		return &types.ToolResult{
			IsError: true,
			Content: "缺少必填参数: name",
		}, nil
	}

	// 写入变量
	execCtx.SetVariable(args.Name, args.Value)

	// 构建结果
	resultJSON, err := json.Marshal(map[string]bool{"success": true})
	if err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("序列化结果失败: %v", err),
		}, nil
	}

	return &types.ToolResult{
		IsError: false,
		Content: string(resultJSON),
	}, nil
}

// JSONParseTool JSON 解析工具，支持通过 path 提取嵌套值
type JSONParseTool struct{}

// jsonParseArgs JSON 解析工具的参数
type jsonParseArgs struct {
	JSONString string `json:"json_string"`
	Path       string `json:"path,omitempty"`
}

// Definition 返回 JSON 解析工具的定义信息
func (t *JSONParseTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        "json_parse",
		Description: "解析 JSON 字符串，支持通过 path 提取嵌套值",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"json_string": {
					"type": "string",
					"description": "要解析的 JSON 字符串"
				},
				"path": {
					"type": "string",
					"description": "可选的路径表达式，用点号分隔（如 data.items.0.name）"
				}
			},
			"required": ["json_string"]
		}`),
	}
}

// Execute 执行 JSON 解析
// 解析 json_string，如果提供了 path 则按点号分隔路径提取嵌套值。
// 路径中的数字段会尝试作为数组索引访问。
// 无效 JSON 或路径不存在时返回 IsError=true 的 ToolResult。
func (t *JSONParseTool) Execute(ctx context.Context, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error) {
	// 解析参数
	var args jsonParseArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("参数解析失败: %v", err),
		}, nil
	}

	// 校验必填参数
	if args.JSONString == "" {
		return &types.ToolResult{
			IsError: true,
			Content: "缺少必填参数: json_string",
		}, nil
	}

	// 解析 JSON 字符串
	var parsed any
	if err := json.Unmarshal([]byte(args.JSONString), &parsed); err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("无效的 JSON: %v", err),
		}, nil
	}

	// 如果提供了 path，按路径提取嵌套值
	if args.Path != "" {
		value, err := navigateJSONPath(parsed, args.Path)
		if err != nil {
			return &types.ToolResult{
				IsError: true,
				Content: fmt.Sprintf("路径提取失败: %v", err),
			}, nil
		}
		parsed = value
	}

	// 将结果序列化为 JSON 字符串
	resultJSON, err := json.Marshal(parsed)
	if err != nil {
		return &types.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("序列化结果失败: %v", err),
		}, nil
	}

	return &types.ToolResult{
		IsError: false,
		Content: string(resultJSON),
	}, nil
}

// navigateJSONPath 按点号分隔的路径导航 JSON 数据
// 每个路径段先尝试作为 map key 访问，再尝试作为数组索引访问
func navigateJSONPath(data any, path string) (any, error) {
	segments := strings.Split(path, ".")
	current := data

	for _, seg := range segments {
		if seg == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]any:
			// 尝试作为 map key 访问
			val, ok := v[seg]
			if !ok {
				return nil, fmt.Errorf("路径 %q 不存在", seg)
			}
			current = val
		case []any:
			// 尝试作为数组索引访问
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

// parseArrayIndex 解析数组索引字符串，校验范围
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

// init 注册所有内置工具到全局 DefaultToolRegistry。
// 包被导入时自动执行，确保内置工具在使用前已注册。
func init() {
	builtinTools := []Tool{
		&HTTPTool{},
		&VarReadTool{},
		&VarWriteTool{},
		&JSONParseTool{},
	}

	for _, tool := range builtinTools {
		if err := RegisterTool(tool); err != nil {
			panic(fmt.Sprintf("注册内置工具失败: %v", err))
		}
	}
}
