package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yqhp/workflow-engine/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper to create a standard proxy response JSON
func proxySuccessJSON(data interface{}) []byte {
	d, _ := json.Marshal(data)
	resp := proxyResponse{Code: 0, Message: "success", Data: d}
	b, _ := json.Marshal(resp)
	return b
}

func proxyErrorJSON(code int, msg string) []byte {
	resp := proxyResponse{Code: code, Message: msg}
	b, _ := json.Marshal(resp)
	return b
}

func TestMCPRemoteClient_GetTools_Success(t *testing.T) {
	tools := []*types.ToolDefinition{
		{Name: "tool_a", Description: "Tool A", Parameters: json.RawMessage(`{"type":"object"}`)},
		{Name: "tool_b", Description: "Tool B", Parameters: json.RawMessage(`{"type":"object"}`)},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/mcp-proxy/tools", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, http.MethodPost, r.Method)

		var req map[string]int64
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, int64(42), req["server_id"])

		w.Header().Set("Content-Type", "application/json")
		w.Write(proxySuccessJSON(getToolsData{Tools: tools}))
	}))
	defer server.Close()

	client := NewMCPRemoteClient(server.URL)
	result, err := client.GetTools(context.Background(), 42)

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "tool_a", result[0].Name)
	assert.Equal(t, "tool_b", result[1].Name)
}

func TestMCPRemoteClient_GetTools_ProxyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(proxyErrorJSON(-1, "服务器未连接"))
	}))
	defer server.Close()

	client := NewMCPRemoteClient(server.URL)
	_, err := client.GetTools(context.Background(), 1)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "服务器未连接")
}

func TestMCPRemoteClient_GetTools_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewMCPRemoteClient(server.URL)
	_, err := client.GetTools(context.Background(), 1)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestMCPRemoteClient_CallTool_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/mcp-proxy/call-tool", r.URL.Path)

		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, float64(5), req["server_id"])
		assert.Equal(t, "my_tool", req["tool_name"])
		assert.Equal(t, `{"key":"val"}`, req["arguments"])

		result := types.ToolResult{
			ToolCallID: "call_123",
			Content:    "result data",
			IsError:    false,
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(proxySuccessJSON(result))
	}))
	defer server.Close()

	client := NewMCPRemoteClient(server.URL)
	result, err := client.CallTool(context.Background(), 5, "my_tool", `{"key":"val"}`)

	require.NoError(t, err)
	assert.Equal(t, "result data", result.Content)
	assert.False(t, result.IsError)
}

func TestMCPRemoteClient_CallTool_ProxyError_ReturnsToolResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(proxyErrorJSON(-1, "工具不存在"))
	}))
	defer server.Close()

	client := NewMCPRemoteClient(server.URL)
	result, err := client.CallTool(context.Background(), 1, "bad_tool", "")

	// CallTool returns a ToolResult with IsError=true instead of an error
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "工具不存在")
}

func TestMCPRemoteClient_CallTool_HTTPError_ReturnsToolResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("bad gateway"))
	}))
	defer server.Close()

	client := NewMCPRemoteClient(server.URL)
	result, err := client.CallTool(context.Background(), 1, "tool", "")

	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "502")
}

func TestMCPRemoteClient_BaseURL_TrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/mcp-proxy/tools", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write(proxySuccessJSON(getToolsData{Tools: nil}))
	}))
	defer server.Close()

	// Pass URL with trailing slash
	client := NewMCPRemoteClient(server.URL + "/")
	result, err := client.GetTools(context.Background(), 1)

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestMCPRemoteClient_ConnectionRefused(t *testing.T) {
	client := NewMCPRemoteClient("http://127.0.0.1:1") // port 1 should refuse
	_, err := client.GetTools(context.Background(), 1)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "获取 MCP 工具列表失败")
}
