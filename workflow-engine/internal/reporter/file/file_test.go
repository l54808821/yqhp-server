package file

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
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

// JSON Reporter Tests

func TestJSONReporter_New(t *testing.T) {
	// Test with nil config
	r := NewJSONReporter(nil)
	assert.NotNil(t, r)
	assert.Equal(t, "json", r.Name())
	assert.Equal(t, "metrics.json", r.GetFilePath())

	// Test with custom config
	config := &JSONConfig{
		FilePath: "custom.json",
		Pretty:   false,
	}
	r = NewJSONReporter(config)
	assert.Equal(t, "custom.json", r.GetFilePath())
}

func TestJSONReporter_Init(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")

	config := &JSONConfig{
		FilePath:   filePath,
		Pretty:     true,
		BufferSize: 10,
	}

	r := NewJSONReporter(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	assert.NoError(t, err)
	assert.True(t, r.initialized)

	// File should exist
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	// Double init should fail
	err = r.Init(ctx, nil)
	assert.Error(t, err)

	r.Close(ctx)
}

func TestJSONReporter_Report(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")

	config := &JSONConfig{
		FilePath:   filePath,
		Pretty:     true,
		BufferSize: 2, // Small buffer to test flushing
	}

	r := NewJSONReporter(config)
	ctx := context.Background()

	// Report without init should fail
	err := r.Report(ctx, createTestMetrics())
	assert.Error(t, err)

	// Init and report
	err = r.Init(ctx, nil)
	require.NoError(t, err)

	// Report multiple times
	for i := 0; i < 3; i++ {
		err = r.Report(ctx, createTestMetrics())
		assert.NoError(t, err)
	}

	err = r.Close(ctx)
	assert.NoError(t, err)

	// Verify file content
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var records []JSONRecord
	err = json.Unmarshal(data, &records)
	assert.NoError(t, err)
	assert.Len(t, records, 3)
}

func TestJSONReporter_Flush(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")

	config := &JSONConfig{
		FilePath:   filePath,
		BufferSize: 100, // Large buffer
	}

	r := NewJSONReporter(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	// Report without triggering auto-flush
	err = r.Report(ctx, createTestMetrics())
	assert.NoError(t, err)

	// Manual flush
	err = r.Flush(ctx)
	assert.NoError(t, err)

	r.Close(ctx)
}

func TestJSONReporter_Close(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")

	config := &JSONConfig{
		FilePath:   filePath,
		BufferSize: 100,
	}

	r := NewJSONReporter(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	err = r.Report(ctx, createTestMetrics())
	require.NoError(t, err)

	err = r.Close(ctx)
	assert.NoError(t, err)
	assert.False(t, r.initialized)

	// Verify valid JSON
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var records []JSONRecord
	err = json.Unmarshal(data, &records)
	assert.NoError(t, err)
}

func TestNewJSONFactory(t *testing.T) {
	factory := NewJSONFactory()

	reporter, err := factory(nil)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)
	assert.Equal(t, "json", reporter.Name())

	config := map[string]any{
		"file_path": "custom.json",
		"pretty":    false,
	}
	reporter, err = factory(config)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)
}

// CSV Reporter Tests

func TestCSVReporter_New(t *testing.T) {
	// Test with nil config
	r := NewCSVReporter(nil)
	assert.NotNil(t, r)
	assert.Equal(t, "csv", r.Name())
	assert.Equal(t, "metrics.csv", r.GetFilePath())

	// Test with custom config
	config := &CSVConfig{
		FilePath:  "custom.csv",
		Delimiter: ';',
	}
	r = NewCSVReporter(config)
	assert.Equal(t, "custom.csv", r.GetFilePath())
}

func TestCSVReporter_Init(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.csv")

	config := &CSVConfig{
		FilePath:      filePath,
		IncludeHeader: true,
		BufferSize:    10,
	}

	r := NewCSVReporter(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	assert.NoError(t, err)
	assert.True(t, r.initialized)
	assert.True(t, r.headerWritten)

	// File should exist
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	// Double init should fail
	err = r.Init(ctx, nil)
	assert.Error(t, err)

	r.Close(ctx)
}

func TestCSVReporter_Report(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.csv")

	config := &CSVConfig{
		FilePath:      filePath,
		IncludeHeader: true,
		BufferSize:    2, // Small buffer to test flushing
	}

	r := NewCSVReporter(config)
	ctx := context.Background()

	// Report without init should fail
	err := r.Report(ctx, createTestMetrics())
	assert.Error(t, err)

	// Init and report
	err = r.Init(ctx, nil)
	require.NoError(t, err)

	// Report multiple times
	for i := 0; i < 3; i++ {
		err = r.Report(ctx, createTestMetrics())
		assert.NoError(t, err)
	}

	err = r.Close(ctx)
	assert.NoError(t, err)

	// Verify file content
	file, err := os.Open(filePath)
	require.NoError(t, err)
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// 1 header + 3 data rows
	assert.Len(t, records, 4)
	assert.Equal(t, "timestamp", records[0][0])
}

func TestCSVReporter_NoHeader(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.csv")

	config := &CSVConfig{
		FilePath:      filePath,
		IncludeHeader: false,
		BufferSize:    10,
	}

	r := NewCSVReporter(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)
	assert.False(t, r.headerWritten)

	err = r.Report(ctx, createTestMetrics())
	require.NoError(t, err)

	err = r.Close(ctx)
	assert.NoError(t, err)

	// Verify file content - no header
	file, err := os.Open(filePath)
	require.NoError(t, err)
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// Only 1 data row, no header
	assert.Len(t, records, 1)
	assert.NotEqual(t, "timestamp", records[0][0])
}

func TestCSVReporter_Flush(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.csv")

	config := &CSVConfig{
		FilePath:   filePath,
		BufferSize: 100, // Large buffer
	}

	r := NewCSVReporter(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	// Report without triggering auto-flush
	err = r.Report(ctx, createTestMetrics())
	assert.NoError(t, err)

	// Manual flush
	err = r.Flush(ctx)
	assert.NoError(t, err)

	r.Close(ctx)
}

func TestCSVReporter_CustomDelimiter(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.csv")

	config := &CSVConfig{
		FilePath:      filePath,
		Delimiter:     ';',
		IncludeHeader: true,
		BufferSize:    10,
	}

	r := NewCSVReporter(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	err = r.Report(ctx, createTestMetrics())
	require.NoError(t, err)

	err = r.Close(ctx)
	assert.NoError(t, err)

	// Verify file uses semicolon delimiter
	file, err := os.Open(filePath)
	require.NoError(t, err)
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	records, err := reader.ReadAll()
	require.NoError(t, err)

	assert.Len(t, records, 2) // header + 1 data row
}

func TestNewCSVFactory(t *testing.T) {
	factory := NewCSVFactory()

	reporter, err := factory(nil)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)
	assert.Equal(t, "csv", reporter.Name())

	config := map[string]any{
		"file_path":      "custom.csv",
		"delimiter":      ";",
		"include_header": false,
	}
	reporter, err = factory(config)
	assert.NoError(t, err)
	assert.NotNil(t, reporter)
}

func TestCSVReporter_EmptyMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.csv")

	config := &CSVConfig{
		FilePath:      filePath,
		IncludeHeader: true,
		BufferSize:    10,
	}

	r := NewCSVReporter(config)
	ctx := context.Background()

	err := r.Init(ctx, nil)
	require.NoError(t, err)

	// Report with empty step metrics
	metrics := &types.Metrics{
		Timestamp:   time.Now(),
		StepMetrics: map[string]*types.StepMetrics{},
		SystemMetrics: &types.SystemMetrics{
			CPUUsage:       10.0,
			MemoryUsage:    20.0,
			GoroutineCount: 5,
		},
	}

	err = r.Report(ctx, metrics)
	require.NoError(t, err)

	err = r.Close(ctx)
	assert.NoError(t, err)

	// Verify file content
	file, err := os.Open(filePath)
	require.NoError(t, err)
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// 1 header + 1 data row (with system metrics only)
	assert.Len(t, records, 2)
}
