// Package file provides file-based reporters for the workflow execution engine.
// Requirements: 9.1.3
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// JSONConfig holds configuration for the JSON reporter.
type JSONConfig struct {
	// FilePath is the output file path.
	FilePath string `yaml:"file_path"`
	// Pretty enables pretty-printed JSON output.
	Pretty bool `yaml:"pretty"`
	// IncludeTimestamp adds timestamp to each record.
	IncludeTimestamp bool `yaml:"include_timestamp"`
	// BufferSize is the number of records to buffer before writing.
	BufferSize int `yaml:"buffer_size"`
}

// DefaultJSONConfig returns the default JSON reporter configuration.
func DefaultJSONConfig() *JSONConfig {
	return &JSONConfig{
		FilePath:         "metrics.json",
		Pretty:           true,
		IncludeTimestamp: true,
		BufferSize:       100,
	}
}

// JSONReporter implements the JSON file reporter.
// Requirements: 9.1.3
type JSONReporter struct {
	config *JSONConfig
	file   *os.File
	buffer []*JSONRecord
	mu     sync.Mutex

	initialized   bool
	encoder       *json.Encoder
	recordWritten bool // tracks if any record has been written
}

// JSONRecord represents a single metrics record in JSON format.
type JSONRecord struct {
	Timestamp     time.Time                   `json:"timestamp"`
	StepMetrics   map[string]*JSONStepMetrics `json:"step_metrics,omitempty"`
	SystemMetrics *JSONSystemMetrics          `json:"system_metrics,omitempty"`
}

// JSONStepMetrics represents step metrics in JSON format.
type JSONStepMetrics struct {
	StepID        string             `json:"step_id"`
	Count         int64              `json:"count"`
	SuccessCount  int64              `json:"success_count"`
	FailureCount  int64              `json:"failure_count"`
	Duration      *JSONDuration      `json:"duration,omitempty"`
	CustomMetrics map[string]float64 `json:"custom_metrics,omitempty"`
}

// JSONDuration represents duration metrics in JSON format.
type JSONDuration struct {
	MinMs float64 `json:"min_ms"`
	MaxMs float64 `json:"max_ms"`
	AvgMs float64 `json:"avg_ms"`
	P50Ms float64 `json:"p50_ms"`
	P90Ms float64 `json:"p90_ms"`
	P95Ms float64 `json:"p95_ms"`
	P99Ms float64 `json:"p99_ms"`
}

// JSONSystemMetrics represents system metrics in JSON format.
type JSONSystemMetrics struct {
	CPUUsage       float64 `json:"cpu_usage"`
	MemoryUsage    float64 `json:"memory_usage"`
	GoroutineCount int     `json:"goroutine_count"`
}

// NewJSONReporter creates a new JSON reporter.
func NewJSONReporter(config *JSONConfig) *JSONReporter {
	if config == nil {
		config = DefaultJSONConfig()
	}
	return &JSONReporter{
		config: config,
		buffer: make([]*JSONRecord, 0, config.BufferSize),
	}
}

// NewJSONFactory returns a factory function for creating JSON reporters.
func NewJSONFactory() func(config map[string]any) (interface{ Name() string }, error) {
	return func(config map[string]any) (interface{ Name() string }, error) {
		cfg := DefaultJSONConfig()
		if config != nil {
			if v, ok := config["file_path"].(string); ok {
				cfg.FilePath = v
			}
			if v, ok := config["pretty"].(bool); ok {
				cfg.Pretty = v
			}
			if v, ok := config["include_timestamp"].(bool); ok {
				cfg.IncludeTimestamp = v
			}
			if v, ok := config["buffer_size"].(int); ok {
				cfg.BufferSize = v
			}
		}
		return NewJSONReporter(cfg), nil
	}
}

// Name returns the reporter name.
func (r *JSONReporter) Name() string {
	return "json"
}

// Init initializes the reporter.
func (r *JSONReporter) Init(ctx context.Context, config map[string]any) error {
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
	r.encoder = json.NewEncoder(file)
	if r.config.Pretty {
		r.encoder.SetIndent("", "  ")
	}

	// Write opening bracket for JSON array
	if _, err := r.file.WriteString("[\n"); err != nil {
		r.file.Close()
		return fmt.Errorf("写入头部失败: %w", err)
	}

	r.initialized = true
	return nil
}

// Report sends metrics to the JSON file.
func (r *JSONReporter) Report(ctx context.Context, metrics *types.Metrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return fmt.Errorf("报告器未初始化")
	}

	record := r.convertMetrics(metrics)
	r.buffer = append(r.buffer, record)

	// Flush if buffer is full
	if len(r.buffer) >= r.config.BufferSize {
		return r.flushBuffer()
	}

	return nil
}

// Flush flushes any buffered data.
func (r *JSONReporter) Flush(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return nil
	}

	return r.flushBuffer()
}

// Close closes the reporter.
func (r *JSONReporter) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return nil
	}

	// Flush remaining buffer
	if err := r.flushBuffer(); err != nil {
		return err
	}

	// Write closing bracket
	if _, err := r.file.WriteString("\n]"); err != nil {
		return fmt.Errorf("写入尾部失败: %w", err)
	}

	if err := r.file.Close(); err != nil {
		return fmt.Errorf("关闭文件失败: %w", err)
	}

	r.initialized = false
	r.file = nil
	return nil
}

// flushBuffer writes buffered records to file.
func (r *JSONReporter) flushBuffer() error {
	if len(r.buffer) == 0 {
		return nil
	}

	for _, record := range r.buffer {
		// Add comma separator between records
		if r.recordWritten {
			if _, err := r.file.WriteString(",\n"); err != nil {
				return fmt.Errorf("写入分隔符失败: %w", err)
			}
		}

		data, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化记录失败: %w", err)
		}

		if _, err := r.file.Write(data); err != nil {
			return fmt.Errorf("写入记录失败: %w", err)
		}

		r.recordWritten = true
	}

	r.buffer = r.buffer[:0]
	return nil
}

// convertMetrics converts types.Metrics to JSONRecord.
func (r *JSONReporter) convertMetrics(metrics *types.Metrics) *JSONRecord {
	record := &JSONRecord{
		Timestamp:   metrics.Timestamp,
		StepMetrics: make(map[string]*JSONStepMetrics),
	}

	for stepID, sm := range metrics.StepMetrics {
		jsm := &JSONStepMetrics{
			StepID:        sm.StepID,
			Count:         sm.Count,
			SuccessCount:  sm.SuccessCount,
			FailureCount:  sm.FailureCount,
			CustomMetrics: sm.CustomMetrics,
		}

		if sm.Duration != nil {
			jsm.Duration = &JSONDuration{
				MinMs: float64(sm.Duration.Min.Microseconds()) / 1000,
				MaxMs: float64(sm.Duration.Max.Microseconds()) / 1000,
				AvgMs: float64(sm.Duration.Avg.Microseconds()) / 1000,
				P50Ms: float64(sm.Duration.P50.Microseconds()) / 1000,
				P90Ms: float64(sm.Duration.P90.Microseconds()) / 1000,
				P95Ms: float64(sm.Duration.P95.Microseconds()) / 1000,
				P99Ms: float64(sm.Duration.P99.Microseconds()) / 1000,
			}
		}

		record.StepMetrics[stepID] = jsm
	}

	if metrics.SystemMetrics != nil {
		record.SystemMetrics = &JSONSystemMetrics{
			CPUUsage:       metrics.SystemMetrics.CPUUsage,
			MemoryUsage:    metrics.SystemMetrics.MemoryUsage,
			GoroutineCount: metrics.SystemMetrics.GoroutineCount,
		}
	}

	return record
}

// GetFilePath returns the output file path.
func (r *JSONReporter) GetFilePath() string {
	return r.config.FilePath
}
