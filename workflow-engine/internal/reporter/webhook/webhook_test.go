package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"yqhp/workflow-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestMetrics() *types.Metrics {
	return &types.Metrics{
		Timestamp: time.Now(),
		StepMetrics: map[string]*types.StepMetrics{
			"step1": {
				StepID:       "step1",
				Count:        100,
				SuccessCount: 95,
				FailureCount: 5,
				Duration: &types.DurationMetrics{
					Min: 10 * time.Millisecond,
					Max: 500 * time.Millisecond,
					Avg: 100 * time.Millisecond,
					P50: 80 * time.Millisecond,
					P90: 200 * time.Millisecond,
					P95: 300 * time.Millisecond,
					P99: 450 * time.Millisecond,
				},
			},
		},
		SystemMetrics: &types.SystemMetrics{
			CPUUsage:       45.5,
			MemoryUsage:    60.2,
			GoroutineCount: 50,
		},
	}
}

func TestNew(t *testing.T) {
	// Test with nil config
	r := New(nil)
	assert.NotNil(t, r)
	assert.Equal(t, "webhook", r.Name())
	assert.Equal(t, http.MethodPost, r.config.Method)

	// Test with custom config
	config := &Config{
		URL:           "http://example.com/webhook",
		Method:        http.MethodPut,
		RetryAttempts: 5,
	}
	r = New(config)
	assert.Equal(t, "http://example.com/webhook", r.config.URL)
	assert.Equal(t, http.MethodPut, r.config.Method)
	assert.Equal(t, 5, r.config.RetryAttempts)
}

func TestReporter_Init(t *testing.T) {
	// Init without URL should fail
	config := &Config{
		URL: "",
	}
	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "URL is required")

	// Init with URL should succeed
	config = &Config{
		URL: "http://example.com/webhook",
	}
	r = New(config)

	err = r.Init(ctx, nil)
	assert.NoError(t, err)
	assert.True(t, r.initialized)

	// Double init should fail
	err = r.Init(ctx, nil)
	assert.Error(t, err)
}

func TestReporter_Report(t *testing.T) {
	var receivedBody []byte
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		URL:       server.URL,
		BatchSize: 1, // Flush immediately
		Headers: map[string]string{
			"X-Custom-Header": "test-value",
		},
		Timeout: 5 * time.Second,
	}

	r := New(config)
	ctx := context.Background()

	// Report without init should fail
	err := r.Report(ctx, createTestMetrics())
	assert.Error(t, err)

	// Init and report
	err = r.Init(ctx, nil)
	require.NoError(t, err)

	err = r.Report(ctx, createTestMetrics())
	assert.NoError(t, err)

	// Verify payload
	var batch WebhookBatchPayload
	err = json.Unmarshal(receivedBody, &batch)
	assert.NoError(t, err)
	assert.Equal(t, 1, batch.Count)
	assert.Len(t, batch.Records, 1)

	record := batch.Records[0]
	assert.Contains(t, record.StepMetrics, "step1")
	assert.Equal(t, int64(100), record.StepMetrics["step1"].Count)
	assert.NotNil(t, record.SystemMetrics)
	assert.Equal(t, 45.5, record.SystemMetrics.CPUUsage)

	// Verify headers
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))
	assert.Equal(t, "test-value", receivedHeaders.Get("X-Custom-Header"))
}

func TestReporter_ReportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	config := &Config{
		URL:           server.URL,
		BatchSize:     1,
		RetryAttempts: 0, // No retries
		Timeout:       5 * time.Second,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	err = r.Report(ctx, createTestMetrics())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestReporter_RetryLogic(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		URL:           server.URL,
		BatchSize:     1,
		RetryAttempts: 3,
		RetryDelay:    10 * time.Millisecond, // Short delay for testing
		Timeout:       5 * time.Second,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	err = r.Report(ctx, createTestMetrics())
	assert.NoError(t, err)

	// Should have made 3 requests (2 failures + 1 success)
	assert.Equal(t, int32(3), requestCount.Load())
}

func TestReporter_RetryExhausted(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	config := &Config{
		URL:           server.URL,
		BatchSize:     1,
		RetryAttempts: 2,
		RetryDelay:    10 * time.Millisecond,
		Timeout:       5 * time.Second,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	err = r.Report(ctx, createTestMetrics())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 3 attempts")

	// Should have made 3 requests (1 initial + 2 retries)
	assert.Equal(t, int32(3), requestCount.Load())
}

func TestReporter_BatchWrite(t *testing.T) {
	var requestCount atomic.Int32
	var lastBatch WebhookBatchPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &lastBatch)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		URL:       server.URL,
		BatchSize: 3, // Buffer 3 records before sending
		Timeout:   5 * time.Second,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	// Report twice (buffer not full)
	err = r.Report(ctx, createTestMetrics())
	assert.NoError(t, err)
	assert.Equal(t, int32(0), requestCount.Load())

	err = r.Report(ctx, createTestMetrics())
	assert.NoError(t, err)
	assert.Equal(t, int32(0), requestCount.Load())

	// Third report triggers flush
	err = r.Report(ctx, createTestMetrics())
	assert.NoError(t, err)
	assert.Equal(t, int32(1), requestCount.Load())

	// Verify batch
	assert.Equal(t, 3, lastBatch.Count)
	assert.Len(t, lastBatch.Records, 3)
}

func TestReporter_Flush(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		URL:       server.URL,
		BatchSize: 100, // Large buffer
		Timeout:   5 * time.Second,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	// Report without triggering auto-flush
	err = r.Report(ctx, createTestMetrics())
	assert.NoError(t, err)
	assert.Equal(t, int32(0), requestCount.Load())

	// Manual flush
	err = r.Flush(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), requestCount.Load())

	// Flush empty buffer should not send
	err = r.Flush(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), requestCount.Load())
}

func TestReporter_Close(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		URL:       server.URL,
		BatchSize: 100, // Large buffer
		Timeout:   5 * time.Second,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	// Report without triggering auto-flush
	err = r.Report(ctx, createTestMetrics())
	assert.NoError(t, err)
	assert.Equal(t, int32(0), requestCount.Load())

	// Close should flush
	err = r.Close(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), requestCount.Load())
	assert.False(t, r.initialized)
}

func TestReporter_ConvertMetrics(t *testing.T) {
	r := New(nil)
	metrics := createTestMetrics()

	payload := r.convertMetrics(metrics)

	assert.NotNil(t, payload)
	assert.Contains(t, payload.StepMetrics, "step1")

	sm := payload.StepMetrics["step1"]
	assert.Equal(t, "step1", sm.StepID)
	assert.Equal(t, int64(100), sm.Count)
	assert.Equal(t, int64(95), sm.SuccessCount)
	assert.Equal(t, int64(5), sm.FailureCount)
	assert.NotNil(t, sm.Duration)
	assert.Equal(t, 10.0, sm.Duration.MinMs)
	assert.Equal(t, 500.0, sm.Duration.MaxMs)

	assert.NotNil(t, payload.SystemMetrics)
	assert.Equal(t, 45.5, payload.SystemMetrics.CPUUsage)
	assert.Equal(t, 60.2, payload.SystemMetrics.MemoryUsage)
	assert.Equal(t, 50, payload.SystemMetrics.GoroutineCount)
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()

	reporter, err := factory(nil)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)
	assert.Equal(t, "webhook", reporter.Name())

	config := map[string]any{
		"url":            "http://example.com/webhook",
		"method":         "PUT",
		"batch_size":     20,
		"retry_attempts": 5,
		"retry_delay":    "2s",
		"timeout":        "30s",
		"headers": map[string]any{
			"Authorization": "Bearer token",
		},
	}
	reporter, err = factory(config)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)

	webhookReporter := reporter.(*Reporter)
	assert.Equal(t, "http://example.com/webhook", webhookReporter.config.URL)
	assert.Equal(t, "PUT", webhookReporter.config.Method)
	assert.Equal(t, 20, webhookReporter.config.BatchSize)
	assert.Equal(t, 5, webhookReporter.config.RetryAttempts)
	assert.Equal(t, 2*time.Second, webhookReporter.config.RetryDelay)
	assert.Equal(t, 30*time.Second, webhookReporter.config.Timeout)
	assert.Equal(t, "Bearer token", webhookReporter.config.Headers["Authorization"])
}

func TestReporter_MetricsWithoutDuration(t *testing.T) {
	r := New(nil)
	metrics := &types.Metrics{
		Timestamp: time.Now(),
		StepMetrics: map[string]*types.StepMetrics{
			"step1": {
				StepID:       "step1",
				Count:        100,
				SuccessCount: 100,
				FailureCount: 0,
				Duration:     nil, // No duration metrics
			},
		},
	}

	payload := r.convertMetrics(metrics)

	assert.NotNil(t, payload.StepMetrics["step1"])
	assert.Nil(t, payload.StepMetrics["step1"].Duration)
}

func TestReporter_MetricsWithoutSystemMetrics(t *testing.T) {
	r := New(nil)
	metrics := &types.Metrics{
		Timestamp: time.Now(),
		StepMetrics: map[string]*types.StepMetrics{
			"step1": {
				StepID: "step1",
				Count:  100,
			},
		},
		SystemMetrics: nil, // No system metrics
	}

	payload := r.convertMetrics(metrics)

	assert.NotNil(t, payload.StepMetrics["step1"])
	assert.Nil(t, payload.SystemMetrics)
}

func TestReporter_GetBufferSize(t *testing.T) {
	config := &Config{
		URL:       "http://example.com/webhook",
		BatchSize: 100,
	}
	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	assert.Equal(t, 0, r.GetBufferSize())

	// Add to buffer manually
	r.mu.Lock()
	r.buffer = append(r.buffer, &WebhookPayload{})
	r.mu.Unlock()

	assert.Equal(t, 1, r.GetBufferSize())
}

func TestReporter_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Slow response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		URL:           server.URL,
		BatchSize:     1,
		RetryAttempts: 0,
		Timeout:       10 * time.Second,
	}

	r := New(config)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	err = r.Report(ctx, createTestMetrics())
	assert.Error(t, err)
}
