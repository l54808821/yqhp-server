package prometheus

import (
	"context"
	"fmt"
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
	assert.Equal(t, "prometheus", r.Name())

	// Test with custom config
	config := &Config{
		PushGatewayURL: "http://custom:9091",
		JobName:        "custom_job",
	}
	r = New(config)
	assert.Equal(t, "http://custom:9091", r.config.PushGatewayURL)
	assert.Equal(t, "custom_job", r.config.JobName)
}

func TestReporter_Init(t *testing.T) {
	config := &Config{
		PushGatewayURL: "http://localhost:9091",
		JobName:        "test_job",
		Labels: map[string]string{
			"env": "test",
		},
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	assert.NoError(t, err)
	assert.True(t, r.initialized)

	// Check push URL was built correctly
	assert.Contains(t, r.GetPushURL(), "http://localhost:9091/metrics/job/test_job")
	assert.Contains(t, r.GetPushURL(), "env/test")

	// Double init should fail
	err = r.Init(ctx, nil)
	assert.Error(t, err)
}

func TestReporter_Report(t *testing.T) {
	// Create a test server
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		PushGatewayURL: server.URL,
		JobName:        "test_job",
		Timeout:        5 * time.Second,
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

	// Verify metrics were sent
	assert.Contains(t, receivedBody, "workflow_requests_total")
	assert.Contains(t, receivedBody, "workflow_requests_success_total")
	assert.Contains(t, receivedBody, "workflow_requests_failed_total")
	assert.Contains(t, receivedBody, "workflow_request_duration_seconds")
	assert.Contains(t, receivedBody, "workflow_cpu_usage_percent")
	assert.Contains(t, receivedBody, "workflow_memory_usage_percent")
	assert.Contains(t, receivedBody, "workflow_goroutines")
}

func TestReporter_ReportError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	config := &Config{
		PushGatewayURL: server.URL,
		JobName:        "test_job",
		Timeout:        5 * time.Second,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	err = r.Report(ctx, createTestMetrics())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestReporter_Flush(t *testing.T) {
	var pushCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pushCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		PushGatewayURL: server.URL,
		JobName:        "test_job",
		Timeout:        5 * time.Second,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	// Flush without any metrics should not push
	err = r.Flush(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, pushCount)

	// Report metrics
	err = r.Report(ctx, createTestMetrics())
	require.NoError(t, err)
	assert.Equal(t, 1, pushCount)

	// Flush should push again
	err = r.Flush(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, pushCount)
}

func TestReporter_Close(t *testing.T) {
	var deleteReceived bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteReceived = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		PushGatewayURL: server.URL,
		JobName:        "test_job",
		Timeout:        5 * time.Second,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	err = r.Close(ctx)
	assert.NoError(t, err)
	assert.False(t, r.initialized)
	assert.True(t, deleteReceived)
}

func TestReporter_ConvertMetrics(t *testing.T) {
	r := New(nil)
	metrics := createTestMetrics()

	result := r.convertMetrics(metrics)

	// Check metric names
	assert.Contains(t, result, "workflow_requests_total")
	assert.Contains(t, result, "workflow_requests_success_total")
	assert.Contains(t, result, "workflow_requests_failed_total")
	assert.Contains(t, result, "workflow_request_duration_seconds")
	assert.Contains(t, result, "workflow_cpu_usage_percent")
	assert.Contains(t, result, "workflow_memory_usage_percent")
	assert.Contains(t, result, "workflow_goroutines")

	// Check labels
	assert.Contains(t, result, `step_id="step1"`)

	// Check quantiles
	assert.Contains(t, result, `quantile="0.5"`)
	assert.Contains(t, result, `quantile="0.9"`)
	assert.Contains(t, result, `quantile="0.95"`)
	assert.Contains(t, result, `quantile="0.99"`)

	// Check HELP and TYPE comments
	assert.Contains(t, result, "# HELP workflow_requests_total")
	assert.Contains(t, result, "# TYPE workflow_requests_total counter")
}

func TestEscapeLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with space", "with space"},
		{`with"quote`, `with\"quote`},
		{"with\\backslash", "with\\\\backslash"},
		{"with\nnewline", "with\\nnewline"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeLabel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()

	reporter, err := factory(nil)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)
	assert.Equal(t, "prometheus", reporter.Name())

	config := map[string]any{
		"push_gateway_url": "http://custom:9091",
		"job_name":         "custom_job",
		"labels": map[string]any{
			"env": "prod",
		},
		"push_interval": "30s",
		"timeout":       "10s",
	}
	reporter, err = factory(config)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)

	promReporter := reporter.(*Reporter)
	assert.Equal(t, "http://custom:9091", promReporter.config.PushGatewayURL)
	assert.Equal(t, "custom_job", promReporter.config.JobName)
	assert.Equal(t, "prod", promReporter.config.Labels["env"])
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

	result := r.convertMetrics(metrics)

	// Should have count metrics but no duration
	assert.Contains(t, result, "workflow_requests_total")
	assert.NotContains(t, result, "workflow_request_duration_seconds")
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

	result := r.convertMetrics(metrics)

	// Should have step metrics but no system metrics
	assert.Contains(t, result, "workflow_requests_total")
	assert.NotContains(t, result, "workflow_cpu_usage_percent")
}

func TestReporter_URLBuilding(t *testing.T) {
	tests := []struct {
		name           string
		pushGatewayURL string
		jobName        string
		labels         map[string]string
		expectedURL    string
	}{
		{
			name:           "basic URL",
			pushGatewayURL: "http://localhost:9091",
			jobName:        "test",
			labels:         nil,
			expectedURL:    "http://localhost:9091/metrics/job/test",
		},
		{
			name:           "URL with trailing slash",
			pushGatewayURL: "http://localhost:9091/",
			jobName:        "test",
			labels:         nil,
			expectedURL:    "http://localhost:9091/metrics/job/test",
		},
		{
			name:           "URL with labels",
			pushGatewayURL: "http://localhost:9091",
			jobName:        "test",
			labels:         map[string]string{"env": "prod"},
			expectedURL:    "http://localhost:9091/metrics/job/test/env/prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				PushGatewayURL: tt.pushGatewayURL,
				JobName:        tt.jobName,
				Labels:         tt.labels,
			}
			r := New(config)
			ctx := context.Background()

			err := r.Init(ctx, nil)
			require.NoError(t, err)

			// For labels, we need to check if the URL contains the expected parts
			// since map iteration order is not guaranteed
			if tt.labels == nil {
				assert.Equal(t, tt.expectedURL, r.GetPushURL())
			} else {
				assert.True(t, strings.HasPrefix(r.GetPushURL(), "http://localhost:9091/metrics/job/test"))
				for k, v := range tt.labels {
					assert.Contains(t, r.GetPushURL(), fmt.Sprintf("%s/%s", k, v))
				}
			}
		})
	}
}
