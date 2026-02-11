package types

import (
	"encoding/json"
	"testing"
)

func TestToOpenAIFunction(t *testing.T) {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string"}
		},
		"required": ["url"]
	}`)

	td := &ToolDefinition{
		Name:        "http_request",
		Description: "发送 HTTP 请求到指定 URL",
		Parameters:  params,
	}

	result := td.ToOpenAIFunction()

	if result.Type != "function" {
		t.Errorf("expected type 'function', got %q", result.Type)
	}
	if result.Function.Name != "http_request" {
		t.Errorf("expected name 'http_request', got %q", result.Function.Name)
	}
	if result.Function.Description != "发送 HTTP 请求到指定 URL" {
		t.Errorf("expected description mismatch, got %q", result.Function.Description)
	}

	// 验证 JSON 序列化输出格式
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if m["type"] != "function" {
		t.Errorf("serialized type should be 'function', got %v", m["type"])
	}

	fn, ok := m["function"].(map[string]interface{})
	if !ok {
		t.Fatal("serialized 'function' field missing or wrong type")
	}
	if fn["name"] != "http_request" {
		t.Errorf("serialized function.name mismatch: %v", fn["name"])
	}
	if fn["description"] != "发送 HTTP 请求到指定 URL" {
		t.Errorf("serialized function.description mismatch: %v", fn["description"])
	}
	if fn["parameters"] == nil {
		t.Error("serialized function.parameters should not be nil")
	}
}

func TestToOpenAIFunction_NilParameters(t *testing.T) {
	td := &ToolDefinition{
		Name:        "simple_tool",
		Description: "无参数工具",
		Parameters:  nil,
	}

	result := td.ToOpenAIFunction()

	if result.Type != "function" {
		t.Errorf("expected type 'function', got %q", result.Type)
	}
	if result.Function.Name != "simple_tool" {
		t.Errorf("expected name 'simple_tool', got %q", result.Function.Name)
	}
}
