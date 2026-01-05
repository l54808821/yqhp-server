package response

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSuccessResponse(t *testing.T) {
	data := map[string]any{"key": "value"}
	resp := NewSuccessResponse(200, data, 100*time.Millisecond)

	assert.Equal(t, StatusSuccess, resp.Status)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, data, resp.Data)
	assert.Equal(t, 100*time.Millisecond, resp.Duration)
	assert.True(t, resp.IsSuccess())
	assert.False(t, resp.IsError())
}

func TestNewErrorResponse(t *testing.T) {
	err := errors.New("test error")
	resp := NewErrorResponse(500, err, 50*time.Millisecond)

	assert.Equal(t, StatusError, resp.Status)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, "test error", resp.Error)
	assert.Equal(t, 50*time.Millisecond, resp.Duration)
	assert.False(t, resp.IsSuccess())
	assert.True(t, resp.IsError())
}

func TestNewTimeoutResponse(t *testing.T) {
	resp := NewTimeoutResponse(30 * time.Second)

	assert.Equal(t, StatusTimeout, resp.Status)
	assert.Equal(t, "request timeout", resp.Error)
	assert.Equal(t, 30*time.Second, resp.Duration)
	assert.True(t, resp.IsTimeout())
}

func TestFromHTTPResponse(t *testing.T) {
	// Create mock HTTP response
	httpResp := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Custom":     []string{"value"},
		},
	}
	body := []byte(`{"name":"test","count":42}`)

	resp := FromHTTPResponse(httpResp, body, 100*time.Millisecond)

	assert.Equal(t, StatusSuccess, resp.Status)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.GetHeader("Content-Type"))
	assert.Equal(t, "value", resp.GetHeader("X-Custom"))
	assert.Equal(t, 100*time.Millisecond, resp.Duration)

	// Check JSON parsing
	jsonData := resp.GetJSON()
	require.NotNil(t, jsonData)
	assert.Equal(t, "test", jsonData["name"])
	assert.Equal(t, float64(42), jsonData["count"])
}

func TestFromHTTPResponse_ErrorStatus(t *testing.T) {
	httpResp := &http.Response{
		StatusCode: 404,
		Header:     http.Header{},
	}
	body := []byte(`{"error":"not found"}`)

	resp := FromHTTPResponse(httpResp, body, 50*time.Millisecond)

	assert.Equal(t, StatusError, resp.Status)
	assert.Equal(t, 404, resp.StatusCode)
}

func TestFromHTTPResponse_NonJSON(t *testing.T) {
	httpResp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
	}
	body := []byte("plain text response")

	resp := FromHTTPResponse(httpResp, body, 50*time.Millisecond)

	assert.Equal(t, "plain text response", resp.Data)
	assert.Equal(t, "plain text response", resp.GetString())
}

func TestFromHTTPResponse_JSONArray(t *testing.T) {
	httpResp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
	}
	body := []byte(`[{"id":1},{"id":2},{"id":3}]`)

	resp := FromHTTPResponse(httpResp, body, 50*time.Millisecond)

	list := resp.GetList()
	require.NotNil(t, list)
	assert.Len(t, list, 3)
}

func TestUnifiedResponse_GetString(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		expected string
	}{
		{"nil data", nil, ""},
		{"string data", "hello", "hello"},
		{"map data", map[string]any{"key": "value"}, `{"key":"value"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &UnifiedResponse{Data: tt.data}
			assert.Equal(t, tt.expected, resp.GetString())
		})
	}
}

func TestUnifiedResponse_Metadata(t *testing.T) {
	resp := NewSuccessResponse(200, nil, 0)

	resp.SetMetadata("request_id", "abc123")
	resp.SetMetadata("retry_count", 3)

	assert.Equal(t, "abc123", resp.GetMetadata("request_id"))
	assert.Equal(t, 3, resp.GetMetadata("retry_count"))
	assert.Nil(t, resp.GetMetadata("nonexistent"))
}

func TestUnifiedResponse_ToMap(t *testing.T) {
	resp := NewSuccessResponse(200, map[string]any{"key": "value"}, 100*time.Millisecond)
	resp.Headers["Content-Type"] = "application/json"

	m := resp.ToMap()

	assert.Equal(t, "success", m["status"])
	assert.Equal(t, 200, m["status_code"])
	assert.Equal(t, int64(100), m["duration"])
	assert.NotNil(t, m["data"])
	assert.NotNil(t, m["headers"])
}

func TestBuilder(t *testing.T) {
	resp := NewBuilder().
		WithStatus(StatusSuccess).
		WithStatusCode(201).
		WithData(map[string]any{"id": 1}).
		WithHeader("Location", "/api/resource/1").
		WithDuration(50*time.Millisecond).
		WithMetadata("created", true).
		Build()

	assert.Equal(t, StatusSuccess, resp.Status)
	assert.Equal(t, 201, resp.StatusCode)
	assert.Equal(t, "/api/resource/1", resp.GetHeader("Location"))
	assert.Equal(t, 50*time.Millisecond, resp.Duration)
	assert.Equal(t, true, resp.GetMetadata("created"))
}

func TestBuilder_WithError(t *testing.T) {
	resp := NewBuilder().
		WithStatusCode(500).
		WithError("internal server error").
		Build()

	assert.Equal(t, StatusError, resp.Status)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, "internal server error", resp.Error)
}

func TestQuickHelpers(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		resp := Success(map[string]any{"ok": true})
		assert.Equal(t, StatusSuccess, resp.Status)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Error", func(t *testing.T) {
		resp := Error(400, "bad request")
		assert.Equal(t, StatusError, resp.Status)
		assert.Equal(t, 400, resp.StatusCode)
		assert.Equal(t, "bad request", resp.Error)
	})

	t.Run("Timeout", func(t *testing.T) {
		resp := Timeout(30 * time.Second)
		assert.Equal(t, StatusTimeout, resp.Status)
		assert.Equal(t, 30*time.Second, resp.Duration)
	})
}

func TestUnifiedResponse_String(t *testing.T) {
	resp := NewSuccessResponse(200, nil, 100*time.Millisecond)
	s := resp.String()
	assert.Contains(t, s, "success")
	assert.Contains(t, s, "200")
}
