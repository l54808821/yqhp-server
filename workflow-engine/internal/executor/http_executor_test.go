package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

func TestHTTPExecutor_Type(t *testing.T) {
	executor := NewHTTPExecutor()
	assert.Equal(t, HTTPExecutorType, executor.Type())
}

func TestHTTPExecutor_Init(t *testing.T) {
	executor := NewHTTPExecutor()

	err := executor.Init(context.Background(), map[string]any{
		"timeout": "10s",
	})

	assert.NoError(t, err)
	assert.NotNil(t, executor.client)
}

func TestHTTPExecutor_Execute_GET(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/test", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "success"})
	}))
	defer server.Close()

	executor := NewHTTPExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-get",
		Name: "Test GET",
		Type: HTTPExecutorType,
		Config: map[string]any{
			"method": "GET",
			"url":    server.URL + "/api/test",
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)
	assert.Equal(t, "test-get", result.StepID)

	output, ok := result.Output.(*HTTPResponse)
	require.True(t, ok)
	assert.Equal(t, 200, output.StatusCode)
}

func TestHTTPExecutor_Execute_POST(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "testuser", body["username"])

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "123"})
	}))
	defer server.Close()

	executor := NewHTTPExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-post",
		Name: "Test POST",
		Type: HTTPExecutorType,
		Config: map[string]any{
			"method": "POST",
			"url":    server.URL + "/api/users",
			"headers": map[string]any{
				"Content-Type": "application/json",
			},
			"body": map[string]any{
				"username": "testuser",
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)

	output, ok := result.Output.(*HTTPResponse)
	require.True(t, ok)
	assert.Equal(t, 201, output.StatusCode)
}

func TestHTTPExecutor_Execute_WithHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom-Header"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := NewHTTPExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-headers",
		Name: "Test Headers",
		Type: HTTPExecutorType,
		Config: map[string]any{
			"method": "GET",
			"url":    server.URL + "/api/protected",
			"headers": map[string]any{
				"Authorization":   "Bearer token123",
				"X-Custom-Header": "custom-value",
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)
}

func TestHTTPExecutor_Execute_WithQueryParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "value1", r.URL.Query().Get("param1"))
		assert.Equal(t, "value2", r.URL.Query().Get("param2"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := NewHTTPExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-params",
		Name: "Test Params",
		Type: HTTPExecutorType,
		Config: map[string]any{
			"method": "GET",
			"url":    server.URL + "/api/search",
			"params": map[string]any{
				"param1": "value1",
				"param2": "value2",
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)
}

func TestHTTPExecutor_Execute_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := NewHTTPExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:      "test-timeout",
		Name:    "Test Timeout",
		Type:    HTTPExecutorType,
		Timeout: 50 * time.Millisecond,
		Config: map[string]any{
			"method": "GET",
			"url":    server.URL + "/api/slow",
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusTimeout, result.Status)
}

func TestHTTPExecutor_Execute_VariableResolution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer abc123", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := NewHTTPExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	execCtx := NewExecutionContext()
	execCtx.SetVariable("token", "abc123")
	execCtx.SetResult("login", &types.StepResult{
		StepID: "login",
		Status: types.ResultStatusSuccess,
		Output: map[string]any{"token": "abc123"},
	})

	step := &types.Step{
		ID:   "test-vars",
		Name: "Test Variables",
		Type: HTTPExecutorType,
		Config: map[string]any{
			"method": "GET",
			"url":    server.URL + "/api/protected",
			"headers": map[string]any{
				"Authorization": "Bearer ${token}",
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, execCtx)

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusSuccess, result.Status)
}

func TestHTTPExecutor_Execute_MissingURL(t *testing.T) {
	executor := NewHTTPExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-no-url",
		Name: "Test No URL",
		Type: HTTPExecutorType,
		Config: map[string]any{
			"method": "GET",
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, types.ResultStatusFailed, result.Status)
	assert.NotNil(t, result.Error)
}

func TestHTTPExecutor_Execute_Metrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": "test"}`))
	}))
	defer server.Close()

	executor := NewHTTPExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	step := &types.Step{
		ID:   "test-metrics",
		Name: "Test Metrics",
		Type: HTTPExecutorType,
		Config: map[string]any{
			"method": "GET",
			"url":    server.URL + "/api/data",
		},
	}

	result, err := executor.Execute(context.Background(), step, NewExecutionContext())

	require.NoError(t, err)
	assert.Equal(t, float64(200), result.Metrics["http_status"])
	assert.Greater(t, result.Metrics["http_response_size"], float64(0))
}

func TestHTTPExecutor_Cleanup(t *testing.T) {
	executor := NewHTTPExecutor()
	require.NoError(t, executor.Init(context.Background(), nil))

	err := executor.Cleanup(context.Background())
	assert.NoError(t, err)
}
