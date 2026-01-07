package file

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// CSVConfig holds configuration for the CSV reporter.
type CSVConfig struct {
	// FilePath is the output file path.
	FilePath string `yaml:"file_path"`
	// Delimiter is the field delimiter (default: comma).
	Delimiter rune `yaml:"delimiter"`
	// IncludeHeader writes header row.
	IncludeHeader bool `yaml:"include_header"`
	// BufferSize is the number of records to buffer before writing.
	BufferSize int `yaml:"buffer_size"`
}

// DefaultCSVConfig returns the default CSV reporter configuration.
func DefaultCSVConfig() *CSVConfig {
	return &CSVConfig{
		FilePath:      "metrics.csv",
		Delimiter:     ',',
		IncludeHeader: true,
		BufferSize:    100,
	}
}

// CSVReporter implements the CSV file reporter.
// Requirements: 9.1.3
type CSVReporter struct {
	config *CSVConfig
	file   *os.File
	writer *csv.Writer
	buffer [][]string
	mu     sync.Mutex

	initialized   bool
	headerWritten bool
}

// NewCSVReporter creates a new CSV reporter.
func NewCSVReporter(config *CSVConfig) *CSVReporter {
	if config == nil {
		config = DefaultCSVConfig()
	}
	// Ensure delimiter is set to a valid value
	if config.Delimiter == 0 {
		config.Delimiter = ','
	}
	// Ensure buffer size is positive
	if config.BufferSize <= 0 {
		config.BufferSize = 100
	}
	return &CSVReporter{
		config: config,
		buffer: make([][]string, 0, config.BufferSize),
	}
}

// NewCSVFactory returns a factory function for creating CSV reporters.
func NewCSVFactory() func(config map[string]any) (interface{ Name() string }, error) {
	return func(config map[string]any) (interface{ Name() string }, error) {
		cfg := DefaultCSVConfig()
		if config != nil {
			if v, ok := config["file_path"].(string); ok {
				cfg.FilePath = v
			}
			if v, ok := config["delimiter"].(string); ok && len(v) > 0 {
				cfg.Delimiter = rune(v[0])
			}
			if v, ok := config["include_header"].(bool); ok {
				cfg.IncludeHeader = v
			}
			if v, ok := config["buffer_size"].(int); ok {
				cfg.BufferSize = v
			}
		}
		return NewCSVReporter(cfg), nil
	}
}

// Name returns the reporter name.
func (r *CSVReporter) Name() string {
	return "csv"
}

// Init initializes the reporter.
func (r *CSVReporter) Init(ctx context.Context, config map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return fmt.Errorf("报告器已初始化")
	}

	// Ensure directory exists
	dir := filepath.Dir(r.config.FilePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
	}

	// Open file
	file, err := os.Create(r.config.FilePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}

	r.file = file
	r.writer = csv.NewWriter(file)
	r.writer.Comma = r.config.Delimiter

	// Write header if configured
	if r.config.IncludeHeader {
		header := r.getHeader()
		if err := r.writer.Write(header); err != nil {
			r.file.Close()
			return fmt.Errorf("写入头部失败: %w", err)
		}
		r.writer.Flush()
		r.headerWritten = true
	}

	r.initialized = true
	return nil
}

// Report sends metrics to the CSV file.
func (r *CSVReporter) Report(ctx context.Context, metrics *types.Metrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return fmt.Errorf("报告器未初始化")
	}

	records := r.convertMetrics(metrics)
	r.buffer = append(r.buffer, records...)

	// Flush if buffer is full
	if len(r.buffer) >= r.config.BufferSize {
		return r.flushBuffer()
	}

	return nil
}

// Flush flushes any buffered data.
func (r *CSVReporter) Flush(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return nil
	}

	return r.flushBuffer()
}

// Close closes the reporter.
func (r *CSVReporter) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return nil
	}

	// Flush remaining buffer
	if err := r.flushBuffer(); err != nil {
		return err
	}

	r.writer.Flush()
	if err := r.writer.Error(); err != nil {
		return fmt.Errorf("CSV 写入错误: %w", err)
	}

	if err := r.file.Close(); err != nil {
		return fmt.Errorf("关闭文件失败: %w", err)
	}

	r.initialized = false
	r.file = nil
	r.writer = nil
	return nil
}

// flushBuffer writes buffered records to file.
func (r *CSVReporter) flushBuffer() error {
	if len(r.buffer) == 0 {
		return nil
	}

	if err := r.writer.WriteAll(r.buffer); err != nil {
		return fmt.Errorf("写入记录失败: %w", err)
	}

	r.buffer = r.buffer[:0]
	return nil
}

// getHeader returns the CSV header row.
func (r *CSVReporter) getHeader() []string {
	return []string{
		"timestamp",
		"step_id",
		"count",
		"success_count",
		"failure_count",
		"min_ms",
		"max_ms",
		"avg_ms",
		"p50_ms",
		"p90_ms",
		"p95_ms",
		"p99_ms",
		"cpu_usage",
		"memory_usage",
		"goroutine_count",
	}
}

// convertMetrics converts types.Metrics to CSV records.
func (r *CSVReporter) convertMetrics(metrics *types.Metrics) [][]string {
	var records [][]string

	timestamp := metrics.Timestamp.Format(time.RFC3339)

	// System metrics (shared across all step records)
	cpuUsage := ""
	memoryUsage := ""
	goroutineCount := ""
	if metrics.SystemMetrics != nil {
		cpuUsage = formatFloat(metrics.SystemMetrics.CPUUsage)
		memoryUsage = formatFloat(metrics.SystemMetrics.MemoryUsage)
		goroutineCount = strconv.Itoa(metrics.SystemMetrics.GoroutineCount)
	}

	// Create a record for each step
	for stepID, sm := range metrics.StepMetrics {
		record := []string{
			timestamp,
			stepID,
			strconv.FormatInt(sm.Count, 10),
			strconv.FormatInt(sm.SuccessCount, 10),
			strconv.FormatInt(sm.FailureCount, 10),
		}

		// Duration metrics
		if sm.Duration != nil {
			record = append(record,
				formatFloat(float64(sm.Duration.Min.Microseconds())/1000),
				formatFloat(float64(sm.Duration.Max.Microseconds())/1000),
				formatFloat(float64(sm.Duration.Avg.Microseconds())/1000),
				formatFloat(float64(sm.Duration.P50.Microseconds())/1000),
				formatFloat(float64(sm.Duration.P90.Microseconds())/1000),
				formatFloat(float64(sm.Duration.P95.Microseconds())/1000),
				formatFloat(float64(sm.Duration.P99.Microseconds())/1000),
			)
		} else {
			record = append(record, "", "", "", "", "", "", "")
		}

		// System metrics
		record = append(record, cpuUsage, memoryUsage, goroutineCount)

		records = append(records, record)
	}

	// If no step metrics, create a single record with just system metrics
	if len(metrics.StepMetrics) == 0 {
		record := []string{
			timestamp,
			"",         // step_id
			"", "", "", // counts
			"", "", "", "", "", "", "", // durations
			cpuUsage, memoryUsage, goroutineCount,
		}
		records = append(records, record)
	}

	return records
}

// GetFilePath returns the output file path.
func (r *CSVReporter) GetFilePath() string {
	return r.config.FilePath
}

// formatFloat formats a float64 for CSV output.
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}
