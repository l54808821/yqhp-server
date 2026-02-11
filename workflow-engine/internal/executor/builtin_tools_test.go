package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPTool_Definition(t *testing.T) {
	tool := &HTTPTool{}
	def := tool.Definition()

	if def.Name != "http_request" {
		t.Errorf("expected name 'http_request', got %q", def.Name)
	}
	if def.Description == "" {
		t.Error("expected non-empty description")
	}
	if len(def.Parameters) == 0 {
		t.Error("expected non-empty parameters JSON Schema")
	}

	// 验证 Parameters 是合法 JSON
	var schema map[string]any
	if err := json.Unmarshal(def.Parameters, &schema); err != nil {
		t.Errorf("parameters is not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("expected schema type 'object', got %v", schema["type"])
	}
}

func TestHTTPTool_Execute_GET(t *testing.T) {
	// 启动测试 HTTP 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("X-Test", "hello")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"ok"}`))
	}))
	defer server.Close()

	tool := &HTTPTool{}
	args := `{"method":"GET","url":"` + server.URL + `"}`
	result, err := tool.Execute(context.Background(), args, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	var resp httpResponseResult
	if err := json.Unmarshal([]byte(result.Content), &resp); err != nil {
		t.Fatalf("failed to parse result content: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if resp.Body != `{"message":"ok"}` {
		t.Errorf("unexpected body: %s", resp.Body)
	}
	if resp.Headers["X-Test"] != "hello" {
		t.Errorf("expected header X-Test=hello, got %q", resp.Headers["X-Test"])
	}
}

func TestHTTPTool_Execute_POST_WithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":1}`))
	}))
	defer server.Close()

	tool := &HTTPTool{}
	args := `{"method":"POST","url":"` + server.URL + `","headers":{"Content-Type":"application/json"},"body":"{\"name\":\"test\"}"}`
	result, err := tool.Execute(context.Background(), args, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	var resp httpResponseResult
	json.Unmarshal([]byte(result.Content), &resp)
	if resp.StatusCode != 201 {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}
}

func TestHTTPTool_Execute_PUT_DELETE(t *testing.T) {
	for _, method := range []string{"PUT", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != method {
					t.Errorf("expected %s, got %s", method, r.Method)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()

			tool := &HTTPTool{}
			args := `{"method":"` + method + `","url":"` + server.URL + `"}`
			result, err := tool.Execute(context.Background(), args, nil)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.IsError {
				t.Fatalf("expected success, got error: %s", result.Content)
			}

			var resp httpResponseResult
			json.Unmarshal([]byte(result.Content), &resp)
			if resp.StatusCode != 200 {
				t.Errorf("expected status 200, got %d", resp.StatusCode)
			}
		})
	}
}

func TestHTTPTool_Execute_InvalidJSON(t *testing.T) {
	tool := &HTTPTool{}
	result, err := tool.Execute(context.Background(), "not-json", nil)

	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for invalid JSON")
	}
}

func TestHTTPTool_Execute_MissingMethod(t *testing.T) {
	tool := &HTTPTool{}
	result, err := tool.Execute(context.Background(), `{"url":"http://example.com"}`, nil)

	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for missing method")
	}
}

func TestHTTPTool_Execute_MissingURL(t *testing.T) {
	tool := &HTTPTool{}
	result, err := tool.Execute(context.Background(), `{"method":"GET"}`, nil)

	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for missing URL")
	}
}

func TestHTTPTool_Execute_UnsupportedMethod(t *testing.T) {
	tool := &HTTPTool{}
	result, err := tool.Execute(context.Background(), `{"method":"PATCH","url":"http://example.com"}`, nil)

	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for unsupported method")
	}
}

func TestHTTPTool_Execute_InvalidURL(t *testing.T) {
	tool := &HTTPTool{}
	result, err := tool.Execute(context.Background(), `{"method":"GET","url":"://bad-url"}`, nil)

	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for invalid URL")
	}
}

func TestHTTPTool_Execute_ConnectionRefused(t *testing.T) {
	tool := &HTTPTool{}
	// 使用一个不可达的地址
	result, err := tool.Execute(context.Background(), `{"method":"GET","url":"http://127.0.0.1:1"}`, nil)

	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for connection refused")
	}
}

func TestHTTPTool_Execute_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 不响应，让 context 取消生效
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	tool := &HTTPTool{}
	result, err := tool.Execute(ctx, `{"method":"GET","url":"`+server.URL+`"}`, nil)

	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for cancelled context")
	}
}

func TestHTTPTool_ImplementsToolInterface(t *testing.T) {
	// 编译时验证 HTTPTool 实现了 Tool 接口
	var _ Tool = (*HTTPTool)(nil)
}

// ==================== VarReadTool Tests ====================

func TestVarReadTool_Definition(t *testing.T) {
	tool := &VarReadTool{}
	def := tool.Definition()

	if def.Name != "var_read" {
		t.Errorf("expected name 'var_read', got %q", def.Name)
	}
	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	var schema map[string]any
	if err := json.Unmarshal(def.Parameters, &schema); err != nil {
		t.Errorf("parameters is not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("expected schema type 'object', got %v", schema["type"])
	}
}

func TestVarReadTool_Execute_Success(t *testing.T) {
	tool := &VarReadTool{}
	execCtx := NewExecutionContext()
	execCtx.SetVariable("greeting", "hello world")

	result, err := tool.Execute(context.Background(), `{"name":"greeting"}`, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(result.Content), &resp); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if resp["value"] != "hello world" {
		t.Errorf("expected value 'hello world', got %v", resp["value"])
	}
}

func TestVarReadTool_Execute_NumericValue(t *testing.T) {
	tool := &VarReadTool{}
	execCtx := NewExecutionContext()
	execCtx.SetVariable("count", 42)

	result, err := tool.Execute(context.Background(), `{"name":"count"}`, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	var resp map[string]any
	json.Unmarshal([]byte(result.Content), &resp)
	// JSON numbers are float64
	if resp["value"] != float64(42) {
		t.Errorf("expected value 42, got %v", resp["value"])
	}
}

func TestVarReadTool_Execute_VariableNotFound(t *testing.T) {
	tool := &VarReadTool{}
	execCtx := NewExecutionContext()

	result, err := tool.Execute(context.Background(), `{"name":"nonexistent"}`, execCtx)
	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for nonexistent variable")
	}
}

func TestVarReadTool_Execute_NilContext(t *testing.T) {
	tool := &VarReadTool{}

	result, err := tool.Execute(context.Background(), `{"name":"foo"}`, nil)
	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for nil context")
	}
}

func TestVarReadTool_Execute_InvalidJSON(t *testing.T) {
	tool := &VarReadTool{}
	execCtx := NewExecutionContext()

	result, err := tool.Execute(context.Background(), "not-json", execCtx)
	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for invalid JSON")
	}
}

func TestVarReadTool_Execute_MissingName(t *testing.T) {
	tool := &VarReadTool{}
	execCtx := NewExecutionContext()

	result, err := tool.Execute(context.Background(), `{}`, execCtx)
	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for missing name")
	}
}

func TestVarReadTool_ImplementsToolInterface(t *testing.T) {
	var _ Tool = (*VarReadTool)(nil)
}

// ==================== VarWriteTool Tests ====================

func TestVarWriteTool_Definition(t *testing.T) {
	tool := &VarWriteTool{}
	def := tool.Definition()

	if def.Name != "var_write" {
		t.Errorf("expected name 'var_write', got %q", def.Name)
	}
	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	var schema map[string]any
	if err := json.Unmarshal(def.Parameters, &schema); err != nil {
		t.Errorf("parameters is not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("expected schema type 'object', got %v", schema["type"])
	}
}

func TestVarWriteTool_Execute_Success(t *testing.T) {
	tool := &VarWriteTool{}
	execCtx := NewExecutionContext()

	result, err := tool.Execute(context.Background(), `{"name":"key","value":"val"}`, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	var resp map[string]any
	json.Unmarshal([]byte(result.Content), &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp["success"])
	}

	// 验证变量已写入
	val, ok := execCtx.GetVariable("key")
	if !ok {
		t.Fatal("expected variable 'key' to exist")
	}
	if val != "val" {
		t.Errorf("expected variable value 'val', got %v", val)
	}
}

func TestVarWriteTool_Execute_NumericValue(t *testing.T) {
	tool := &VarWriteTool{}
	execCtx := NewExecutionContext()

	result, err := tool.Execute(context.Background(), `{"name":"count","value":99}`, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	val, ok := execCtx.GetVariable("count")
	if !ok {
		t.Fatal("expected variable 'count' to exist")
	}
	// JSON 反序列化后数字为 float64
	if val != float64(99) {
		t.Errorf("expected 99, got %v", val)
	}
}

func TestVarWriteTool_Execute_NilContext(t *testing.T) {
	tool := &VarWriteTool{}

	result, err := tool.Execute(context.Background(), `{"name":"key","value":"val"}`, nil)
	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for nil context")
	}
}

func TestVarWriteTool_Execute_InvalidJSON(t *testing.T) {
	tool := &VarWriteTool{}
	execCtx := NewExecutionContext()

	result, err := tool.Execute(context.Background(), "not-json", execCtx)
	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for invalid JSON")
	}
}

func TestVarWriteTool_Execute_MissingName(t *testing.T) {
	tool := &VarWriteTool{}
	execCtx := NewExecutionContext()

	result, err := tool.Execute(context.Background(), `{"value":"val"}`, execCtx)
	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for missing name")
	}
}

func TestVarWriteTool_Execute_OverwriteExisting(t *testing.T) {
	tool := &VarWriteTool{}
	execCtx := NewExecutionContext()
	execCtx.SetVariable("key", "old")

	result, err := tool.Execute(context.Background(), `{"name":"key","value":"new"}`, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	val, _ := execCtx.GetVariable("key")
	if val != "new" {
		t.Errorf("expected 'new', got %v", val)
	}
}

func TestVarWriteTool_ImplementsToolInterface(t *testing.T) {
	var _ Tool = (*VarWriteTool)(nil)
}

// ==================== VarRead + VarWrite 往返测试 ====================

func TestVarReadWrite_RoundTrip(t *testing.T) {
	readTool := &VarReadTool{}
	writeTool := &VarWriteTool{}
	execCtx := NewExecutionContext()

	// 写入变量
	writeResult, err := writeTool.Execute(context.Background(), `{"name":"rt_key","value":"rt_value"}`, execCtx)
	if err != nil {
		t.Fatalf("write unexpected error: %v", err)
	}
	if writeResult.IsError {
		t.Fatalf("write failed: %s", writeResult.Content)
	}

	// 读取变量
	readResult, err := readTool.Execute(context.Background(), `{"name":"rt_key"}`, execCtx)
	if err != nil {
		t.Fatalf("read unexpected error: %v", err)
	}
	if readResult.IsError {
		t.Fatalf("read failed: %s", readResult.Content)
	}

	var resp map[string]any
	json.Unmarshal([]byte(readResult.Content), &resp)
	if resp["value"] != "rt_value" {
		t.Errorf("round-trip failed: expected 'rt_value', got %v", resp["value"])
	}
}

// ==================== JSONParseTool 测试 ====================

func TestJSONParseTool_Definition(t *testing.T) {
	tool := &JSONParseTool{}
	def := tool.Definition()

	if def.Name != "json_parse" {
		t.Errorf("expected name 'json_parse', got %q", def.Name)
	}
	if def.Description == "" {
		t.Error("expected non-empty description")
	}
	if len(def.Parameters) == 0 {
		t.Error("expected non-empty parameters JSON Schema")
	}

	// 验证 Parameters 是合法 JSON
	var schema map[string]any
	if err := json.Unmarshal(def.Parameters, &schema); err != nil {
		t.Errorf("parameters is not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("expected schema type 'object', got %v", schema["type"])
	}
}

func TestJSONParseTool_Execute_SimpleObject(t *testing.T) {
	tool := &JSONParseTool{}
	args := `{"json_string": "{\"name\":\"alice\",\"age\":30}"}`

	result, err := tool.Execute(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result.Content), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if parsed["name"] != "alice" {
		t.Errorf("expected name 'alice', got %v", parsed["name"])
	}
}

func TestJSONParseTool_Execute_WithPath(t *testing.T) {
	tool := &JSONParseTool{}
	jsonStr := `{"data":{"items":[{"name":"first"},{"name":"second"}]}}`
	args := `{"json_string": "` + escapeJSONString(jsonStr) + `", "path": "data.items.1.name"}`

	result, err := tool.Execute(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	if result.Content != `"second"` {
		t.Errorf("expected '\"second\"', got %s", result.Content)
	}
}

func TestJSONParseTool_Execute_ArrayIndex(t *testing.T) {
	tool := &JSONParseTool{}
	args := `{"json_string": "[10, 20, 30]", "path": "1"}`

	result, err := tool.Execute(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	if result.Content != "20" {
		t.Errorf("expected '20', got %s", result.Content)
	}
}

func TestJSONParseTool_Execute_InvalidJSON(t *testing.T) {
	tool := &JSONParseTool{}
	args := `{"json_string": "not valid json"}`

	result, err := tool.Execute(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for invalid JSON")
	}
}

func TestJSONParseTool_Execute_PathNotFound(t *testing.T) {
	tool := &JSONParseTool{}
	args := `{"json_string": "{\"a\":1}", "path": "b"}`

	result, err := tool.Execute(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for path not found")
	}
}

func TestJSONParseTool_Execute_ArrayIndexOutOfRange(t *testing.T) {
	tool := &JSONParseTool{}
	args := `{"json_string": "[1,2,3]", "path": "5"}`

	result, err := tool.Execute(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for array index out of range")
	}
}

func TestJSONParseTool_Execute_InvalidArguments(t *testing.T) {
	tool := &JSONParseTool{}

	result, err := tool.Execute(context.Background(), "not json", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for invalid arguments")
	}
}

func TestJSONParseTool_Execute_EmptyJSONString(t *testing.T) {
	tool := &JSONParseTool{}
	args := `{"json_string": ""}`

	result, err := tool.Execute(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for empty json_string")
	}
}

func TestJSONParseTool_Execute_NavigateOnPrimitive(t *testing.T) {
	tool := &JSONParseTool{}
	args := `{"json_string": "{\"a\":\"hello\"}", "path": "a.b"}`

	result, err := tool.Execute(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true when navigating on a primitive value")
	}
}

func TestJSONParseTool_Execute_NestedPath(t *testing.T) {
	tool := &JSONParseTool{}
	jsonStr := `{"a":{"b":{"c":"deep"}}}`
	args := `{"json_string": "` + escapeJSONString(jsonStr) + `", "path": "a.b.c"}`

	result, err := tool.Execute(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	if result.Content != `"deep"` {
		t.Errorf("expected '\"deep\"', got %s", result.Content)
	}
}

func TestJSONParseTool_ImplementsToolInterface(t *testing.T) {
	var _ Tool = (*JSONParseTool)(nil)
}

// escapeJSONString 转义 JSON 字符串中的双引号，用于构造测试参数
func escapeJSONString(s string) string {
	b, _ := json.Marshal(s)
	// 去掉首尾的双引号
	return string(b[1 : len(b)-1])
}

// ==================== init() 注册测试 ====================

func TestInit_AllBuiltinToolsRegistered(t *testing.T) {
	expectedTools := []string{"http_request", "var_read", "var_write", "json_parse"}

	for _, name := range expectedTools {
		if !DefaultToolRegistry.Has(name) {
			t.Errorf("expected built-in tool %q to be registered in DefaultToolRegistry", name)
		}
	}
}

func TestInit_RegisteredToolsAreRetrievable(t *testing.T) {
	expectedTools := map[string]Tool{
		"http_request": &HTTPTool{},
		"var_read":     &VarReadTool{},
		"var_write":    &VarWriteTool{},
		"json_parse":   &JSONParseTool{},
	}

	for name := range expectedTools {
		tool, ok := DefaultToolRegistry.Get(name)
		if !ok {
			t.Errorf("expected tool %q to be retrievable from DefaultToolRegistry", name)
			continue
		}
		def := tool.Definition()
		if def.Name != name {
			t.Errorf("expected tool definition name %q, got %q", name, def.Name)
		}
	}
}

func TestInit_RegisteredToolsInList(t *testing.T) {
	defs := DefaultToolRegistry.List()

	nameSet := make(map[string]bool)
	for _, def := range defs {
		nameSet[def.Name] = true
	}

	expectedTools := []string{"http_request", "var_read", "var_write", "json_parse"}
	for _, name := range expectedTools {
		if !nameSet[name] {
			t.Errorf("expected tool %q to appear in DefaultToolRegistry.List()", name)
		}
	}
}
