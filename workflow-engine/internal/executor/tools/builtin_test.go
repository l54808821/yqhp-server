package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yqhp/workflow-engine/internal/executor"
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

	var schema map[string]any
	if err := json.Unmarshal(def.Parameters, &schema); err != nil {
		t.Errorf("parameters is not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("expected schema type 'object', got %v", schema["type"])
	}
}

func TestHTTPTool_Execute_GET(t *testing.T) {
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
	result, err := tool.Execute(context.Background(), `{"method":"GET","url":"http://127.0.0.1:1"}`, nil)

	if err != nil {
		t.Fatalf("should not return Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for connection refused")
	}
}

func TestHTTPTool_ImplementsToolInterface(t *testing.T) {
	var _ executor.Tool = (*HTTPTool)(nil)
}

// ==================== VarReadTool Tests ====================

func TestVarReadTool_Execute_Success(t *testing.T) {
	tool := &VarReadTool{}
	execCtx := executor.NewExecutionContext()
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

func TestVarReadTool_ImplementsToolInterface(t *testing.T) {
	var _ executor.Tool = (*VarReadTool)(nil)
}

// ==================== VarWriteTool Tests ====================

func TestVarWriteTool_Execute_Success(t *testing.T) {
	tool := &VarWriteTool{}
	execCtx := executor.NewExecutionContext()

	result, err := tool.Execute(context.Background(), `{"name":"key","value":"val"}`, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}

	val, ok := execCtx.GetVariable("key")
	if !ok {
		t.Fatal("expected variable 'key' to exist")
	}
	if val != "val" {
		t.Errorf("expected variable value 'val', got %v", val)
	}
}

func TestVarWriteTool_ImplementsToolInterface(t *testing.T) {
	var _ executor.Tool = (*VarWriteTool)(nil)
}

// ==================== JSONParseTool Tests ====================

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
	b, _ := json.Marshal(jsonStr)
	escaped := string(b[1 : len(b)-1])
	args := `{"json_string": "` + escaped + `", "path": "data.items.1.name"}`

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

func TestJSONParseTool_ImplementsToolInterface(t *testing.T) {
	var _ executor.Tool = (*JSONParseTool)(nil)
}

// ==================== RegisterAll 测试 ====================

func TestRegisterAll(t *testing.T) {
	registry := executor.NewToolRegistry()
	RegisterAll(registry)

	expectedTools := []string{
		"http_request", "var_read", "var_write", "json_parse",
		"web_search", "web_read",
		"code_execute", "shell_exec",
	}
	for _, name := range expectedTools {
		if !registry.Has(name) {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}
