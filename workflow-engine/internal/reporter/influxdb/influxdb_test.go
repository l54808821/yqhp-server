package influxdb

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	assert.Equal(t, "influxdb", r.Name())

	// Test with custom config
	config := &Config{
		URL:          "http://custom:8086",
		Organization: "custom_org",
		Bucket:       "custom_bucket",
	}
	r = New(config)
	assert.Equal(t, "http://custom:8086", r.config.URL)
	assert.Equal(t, "custom_org", r.config.Organization)
	assert.Equal(t, "custom_bucket", r.config.Bucket)
}

func TestReporter_Init(t *testing.T) {
	config := &Config{
		URL:          "http://localhost:8086",
		Organization: "test_org",
		Bucket:       "test_bucket",
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	assert.NoError(t, err)
	assert.True(t, r.initialized)

	// Check write URL was built correctly
	assert.Contains(t, r.GetWriteURL(), "http://localhost:8086/api/v2/write")
	assert.Contains(t, r.GetWriteURL(), "org=test_org")
	assert.Contains(t, r.GetWriteURL(), "bucket=test_bucket")

	// Double init should fail
	err = r.Init(ctx, nil)
	assert.Error(t, err)
}

func TestReporter_Report(t *testing.T) {
	// Create a test server
	var receivedBody string
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	config := &Config{
		URL:          server.URL,
		Token:        "test_token",
		Organization: "test_org",
		Bucket:       "test_bucket",
		BatchSize:    1, // Flush immediately
		Timeout:      5 * time.Second,
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

	// Verify data was sent
	assert.Contains(t, receivedBody, "workflow_step")
	assert.Contains(t, receivedBody, "step_id=step1")
	assert.Contains(t, receivedBody, "count=100i")
	assert.Contains(t, receivedBody, "success_count=95i")
	assert.Contains(t, receivedBody, "failure_count=5i")
	assert.Contains(t, receivedBody, "workflow_system")
	assert.Contains(t, receivedBody, "cpu_usage=45.5")

	// Verify auth header
	assert.Equal(t, "Token test_token", receivedAuth)
}

func TestReporter_ReportError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid line protocol"))
	}))
	defer server.Close()

	config := &Config{
		URL:       server.URL,
		BatchSize: 1,
		Timeout:   5 * time.Second,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	err = r.Report(ctx, createTestMetrics())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestReporter_BatchWrite(t *testing.T) {
	var writeCount int
	var lastBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeCount++
		body, _ := io.ReadAll(r.Body)
		lastBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	config := &Config{
		URL:       server.URL,
		BatchSize: 3, // Buffer 3 points before writing
		Timeout:   5 * time.Second,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	// Report twice (2 points each = 4 points total)
	// First report: 2 points (step + system), buffer not full
	err = r.Report(ctx, createTestMetrics())
	assert.NoError(t, err)
	assert.Equal(t, 0, writeCount) // Not flushed yet

	// Second report: 2 more points, buffer now has 4 points >= 3
	err = r.Report(ctx, createTestMetrics())
	assert.NoError(t, err)
	assert.Equal(t, 1, writeCount) // Flushed

	// Verify multiple lines were sent
	lines := strings.Split(lastBody, "\n")
	assert.GreaterOrEqual(t, len(lines), 3)
}

func TestReporter_Flush(t *testing.T) {
	var writeCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeCount++
		w.WriteHeader(http.StatusNoContent)
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
	assert.Equal(t, 0, writeCount)

	// Manual flush
	err = r.Flush(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, writeCount)

	// Flush empty buffer should not write
	err = r.Flush(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, writeCount)
}

func TestReporter_Close(t *testing.T) {
	var writeCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeCount++
		w.WriteHeader(http.StatusNoContent)
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
	assert.Equal(t, 0, writeCount)

	// Close should flush
	err = r.Close(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, writeCount)
	assert.False(t, r.initialized)
}

func TestReporter_ConvertMetrics(t *testing.T) {
	config := &Config{
		Tags: map[string]string{
			"env": "test",
		},
	}
	r := New(config)
	metrics := createTestMetrics()

	lines := r.convertMetrics(metrics)

	// Should have 2 lines: step metrics and system metrics
	assert.Len(t, lines, 2)

	// Check step metrics line
	stepLine := lines[0]
	assert.Contains(t, stepLine, "workflow_step")
	assert.Contains(t, stepLine, "step_id=step1")
	assert.Contains(t, stepLine, "env=test")
	assert.Contains(t, stepLine, "count=100i")
	assert.Contains(t, stepLine, "success_count=95i")
	assert.Contains(t, stepLine, "failure_count=5i")
	assert.Contains(t, stepLine, "min_ns=")
	assert.Contains(t, stepLine, "max_ns=")
	assert.Contains(t, stepLine, "avg_ns=")
	assert.Contains(t, stepLine, "p50_ns=")
	assert.Contains(t, stepLine, "p90_ns=")
	assert.Contains(t, stepLine, "p95_ns=")
	assert.Contains(t, stepLine, "p99_ns=")

	// Check system metrics line
	systemLine := lines[1]
	assert.Contains(t, systemLine, "workflow_system")
	assert.Contains(t, systemLine, "env=test")
	assert.Contains(t, systemLine, "cpu_usage=45.5")
	assert.Contains(t, systemLine, "memory_usage=60.2")
	assert.Contains(t, systemLine, "goroutine_count=50i")
}

func TestEscapeTag(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with space", "with\\ space"},
		{"with,comma", "with\\,comma"},
		{"with=equals", "with\\=equals"},
		{"all special, = chars", "all\\ special\\,\\ \\=\\ chars"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeTag(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()

	reporter, err := factory(nil)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)
	assert.Equal(t, "influxdb", reporter.Name())

	config := map[string]any{
		"url":          "http://custom:8086",
		"token":        "custom_token",
		"organization": "custom_org",
		"bucket":       "custom_bucket",
		"batch_size":   50,
		"tags": map[string]any{
			"env": "prod",
		},
	}
	reporter, err = factory(config)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)

	influxReporter := reporter.(*Reporter)
	assert.Equal(t, "http://custom:8086", influxReporter.config.URL)
	assert.Equal(t, "custom_token", influxReporter.config.Token)
	assert.Equal(t, "custom_org", influxReporter.config.Organization)
	assert.Equal(t, "custom_bucket", influxReporter.config.Bucket)
	assert.Equal(t, 50, influxReporter.config.BatchSize)
	assert.Equal(t, "prod", influxReporter.config.Tags["env"])
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

	lines := r.convertMetrics(metrics)

	assert.Len(t, lines, 1)
	assert.Contains(t, lines[0], "count=100i")
	assert.NotContains(t, lines[0], "min_ns=")
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

	lines := r.convertMetrics(metrics)

	// Should only have step metrics line
	assert.Len(t, lines, 1)
	assert.Contains(t, lines[0], "workflow_step")
	assert.NotContains(t, lines[0], "workflow_system")
}

func TestReporter_GetBufferSize(t *testing.T) {
	config := &Config{
		BatchSize: 100,
	}
	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	assert.Equal(t, 0, r.GetBufferSize())

	// Add metrics to buffer (won't flush due to large batch size)
	metrics := &types.Metrics{
		Timestamp: time.Now(),
		StepMetrics: map[string]*types.StepMetrics{
			"step1": {StepID: "step1", Count: 1},
		},
	}

	// This will add 1 line to buffer
	r.mu.Lock()
	r.buffer = append(r.buffer, r.convertMetrics(metrics)...)
	r.mu.Unlock()

	assert.Equal(t, 1, r.GetBufferSize())
}

func TestReporter_NoToken(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	config := &Config{
		URL:       server.URL,
		Token:     "", // No token
		BatchSize: 1,
		Timeout:   5 * time.Second,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	err = r.Report(ctx, createTestMetrics())
	assert.NoError(t, err)

	// No auth header should be set
	assert.Empty(t, receivedAuth)
}
