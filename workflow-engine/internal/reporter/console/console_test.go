package console

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"yqhp/workflow-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	// Test with nil config
	r := New(nil)
	assert.NotNil(t, r)
	assert.Equal(t, "console", r.Name())

	// Test with custom config
	config := &Config{
		ShowProgress: false,
		ShowMetrics:  true,
		ColorOutput:  false,
	}
	r = New(config)
	assert.NotNil(t, r)
	assert.False(t, r.config.ShowProgress)
	assert.True(t, r.config.ShowMetrics)
	assert.False(t, r.config.ColorOutput)
}

func TestReporter_Init(t *testing.T) {
	buf := &bytes.Buffer{}
	config := &Config{
		ShowProgress: true,
		ShowMetrics:  true,
		ColorOutput:  false,
		Writer:       buf,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	assert.NoError(t, err)
	assert.True(t, r.initialized)

	// Check header was printed
	output := buf.String()
	assert.Contains(t, output, "Workflow Execution Started")

	// Double init should fail
	err = r.Init(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already initialized")
}

func TestReporter_Report(t *testing.T) {
	buf := &bytes.Buffer{}
	config := &Config{
		ShowProgress: true,
		ShowMetrics:  true,
		ColorOutput:  false,
		Writer:       buf,
	}

	r := New(config)
	ctx := context.Background()

	// Report without init should fail
	err := r.Report(ctx, &types.Metrics{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Init and report
	err = r.Init(ctx, nil)
	require.NoError(t, err)

	buf.Reset()

	metrics := &types.Metrics{
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

	err = r.Report(ctx, metrics)
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "step1")
	assert.Contains(t, output, "Count: 100")
	assert.Contains(t, output, "95.0%")
	assert.Contains(t, output, "CPU: 45.5%")
}

func TestReporter_Close(t *testing.T) {
	buf := &bytes.Buffer{}
	config := &Config{
		ShowProgress: true,
		ShowMetrics:  true,
		ColorOutput:  false,
		Writer:       buf,
	}

	r := New(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	// Report some metrics
	metrics := &types.Metrics{
		Timestamp: time.Now(),
		StepMetrics: map[string]*types.StepMetrics{
			"step1": {
				StepID:       "step1",
				Count:        100,
				SuccessCount: 100,
			},
		},
	}
	r.Report(ctx, metrics)

	buf.Reset()

	err = r.Close(ctx)
	assert.NoError(t, err)
	assert.False(t, r.initialized)

	output := buf.String()
	assert.Contains(t, output, "Execution Summary")
	assert.Contains(t, output, "Total Iterations: 100")
	assert.Contains(t, output, "Success Rate: 100.00%")
}

func TestReporter_Flush(t *testing.T) {
	r := New(nil)
	ctx := context.Background()

	// Flush should always succeed (no-op for console)
	err := r.Flush(ctx)
	assert.NoError(t, err)
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		current  int
		width    int
		contains string
	}{
		{
			name:     "empty progress",
			total:    100,
			current:  0,
			width:    10,
			contains: "0.0%",
		},
		{
			name:     "half progress",
			total:    100,
			current:  50,
			width:    10,
			contains: "50.0%",
		},
		{
			name:     "full progress",
			total:    100,
			current:  100,
			width:    10,
			contains: "100.0%",
		},
		{
			name:     "zero total",
			total:    0,
			current:  0,
			width:    10,
			contains: "----------",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := NewProgressBar(tt.total, tt.width)
			pb.Update(tt.current)
			result := pb.String()
			assert.Contains(t, result, tt.contains)
		})
	}
}

func TestReporter_FormatDuration(t *testing.T) {
	buf := &bytes.Buffer{}
	config := &Config{Writer: buf}
	r := New(config)

	tests := []struct {
		duration time.Duration
		expected string
	}{
		{500 * time.Microsecond, "500µs"},
		{1500 * time.Microsecond, "1.50ms"},
		{100 * time.Millisecond, "100.00ms"},
		{1500 * time.Millisecond, "1.50s"},
		{5 * time.Second, "5.00s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := r.formatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReporter_Colorize(t *testing.T) {
	// With color enabled
	config := &Config{ColorOutput: true}
	r := New(config)
	result := r.colorize("test", colorGreen)
	assert.True(t, strings.HasPrefix(result, colorGreen))
	assert.True(t, strings.HasSuffix(result, colorReset))

	// With color disabled
	config = &Config{ColorOutput: false}
	r = New(config)
	result = r.colorize("test", colorGreen)
	assert.Equal(t, "test", result)
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()

	// Test with nil config
	reporter, err := factory(nil)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)
	assert.Equal(t, "console", reporter.Name())

	// Test with custom config
	config := map[string]any{
		"show_progress":    false,
		"show_metrics":     true,
		"color_output":     false,
		"refresh_interval": "2s",
	}
	reporter, err = factory(config)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)
}
