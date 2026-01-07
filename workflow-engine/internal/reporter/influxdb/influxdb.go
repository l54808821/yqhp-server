// Package influxdb provides an InfluxDB reporter for the workflow execution engine.
// Requirements: 9.1.5
package influxdb

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// Config holds configuration for the InfluxDB reporter.
type Config struct {
	// URL is the InfluxDB server URL.
	URL string `yaml:"url"`
	// Token is the authentication token.
	Token string `yaml:"token"`
	// Organization is the InfluxDB organization.
	Organization string `yaml:"organization"`
	// Bucket is the InfluxDB bucket.
	Bucket string `yaml:"bucket"`
	// BatchSize is the number of points to buffer before writing.
	BatchSize int `yaml:"batch_size"`
	// FlushInterval is the interval between flushes.
	FlushInterval time.Duration `yaml:"flush_interval"`
	// Timeout is the HTTP request timeout.
	Timeout time.Duration `yaml:"timeout"`
	// Tags are additional tags to add to all points.
	Tags map[string]string `yaml:"tags,omitempty"`
}

// DefaultConfig returns the default InfluxDB reporter configuration.
func DefaultConfig() *Config {
	return &Config{
		URL:           "http://localhost:8086",
		Organization:  "default",
		Bucket:        "workflow_metrics",
		BatchSize:     100,
		FlushInterval: 10 * time.Second,
		Timeout:       5 * time.Second,
		Tags:          make(map[string]string),
	}
}

// Reporter implements the InfluxDB reporter.
// Requirements: 9.1.5
type Reporter struct {
	config     *Config
	httpClient *http.Client

	// Buffer for batch writes
	buffer []string
	mu     sync.Mutex

	// State
	initialized bool
	writeURL    string
}

// New creates a new InfluxDB reporter.
func New(config *Config) *Reporter {
	if config == nil {
		config = DefaultConfig()
	}
	return &Reporter{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		buffer: make([]string, 0, config.BatchSize),
	}
}

// NewFactory returns a factory function for creating InfluxDB reporters.
func NewFactory() func(config map[string]any) (interface{ Name() string }, error) {
	return func(config map[string]any) (interface{ Name() string }, error) {
		cfg := DefaultConfig()
		if config != nil {
			if v, ok := config["url"].(string); ok {
				cfg.URL = v
			}
			if v, ok := config["token"].(string); ok {
				cfg.Token = v
			}
			if v, ok := config["organization"].(string); ok {
				cfg.Organization = v
			}
			if v, ok := config["bucket"].(string); ok {
				cfg.Bucket = v
			}
			if v, ok := config["batch_size"].(int); ok {
				cfg.BatchSize = v
			}
			if v, ok := config["flush_interval"].(string); ok {
				if d, err := time.ParseDuration(v); err == nil {
					cfg.FlushInterval = d
				}
			}
			if v, ok := config["timeout"].(string); ok {
				if d, err := time.ParseDuration(v); err == nil {
					cfg.Timeout = d
				}
			}
			if v, ok := config["tags"].(map[string]any); ok {
				for k, val := range v {
					if s, ok := val.(string); ok {
						cfg.Tags[k] = s
					}
				}
			}
		}
		return New(cfg), nil
	}
}

// Name returns the reporter name.
func (r *Reporter) Name() string {
	return "influxdb"
}

// Init initializes the reporter.
func (r *Reporter) Init(ctx context.Context, config map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return fmt.Errorf("报告器已初始化")
	}

	// Build write URL
	r.writeURL = fmt.Sprintf("%s/api/v2/write?org=%s&bucket=%s&precision=ns",
		strings.TrimSuffix(r.config.URL, "/"),
		r.config.Organization,
		r.config.Bucket,
	)

	r.initialized = true
	return nil
}

// Report sends metrics to InfluxDB.
func (r *Reporter) Report(ctx context.Context, metrics *types.Metrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return fmt.Errorf("报告器未初始化")
	}

	// Convert metrics to line protocol
	lines := r.convertMetrics(metrics)
	r.buffer = append(r.buffer, lines...)

	// Flush if buffer is full
	if len(r.buffer) >= r.config.BatchSize {
		return r.flushBuffer(ctx)
	}

	return nil
}

// Flush flushes any buffered data.
func (r *Reporter) Flush(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return nil
	}

	return r.flushBuffer(ctx)
}

// Close closes the reporter.
func (r *Reporter) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return nil
	}

	// Flush remaining buffer
	if err := r.flushBuffer(ctx); err != nil {
		return err
	}

	r.initialized = false
	return nil
}

// flushBuffer writes buffered points to InfluxDB.
func (r *Reporter) flushBuffer(ctx context.Context) error {
	if len(r.buffer) == 0 {
		return nil
	}

	// Join all lines
	body := strings.Join(r.buffer, "\n")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.writeURL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if r.config.Token != "" {
		req.Header.Set("Authorization", "Token "+r.config.Token)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("写入 InfluxDB 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("InfluxDB 返回状态码 %d: %s", resp.StatusCode, string(respBody))
	}

	// Clear buffer
	r.buffer = r.buffer[:0]
	return nil
}

// convertMetrics converts types.Metrics to InfluxDB line protocol.
func (r *Reporter) convertMetrics(metrics *types.Metrics) []string {
	var lines []string
	timestamp := metrics.Timestamp.UnixNano()

	// Build base tags
	baseTags := r.buildBaseTags()

	// Sort step IDs for consistent output
	stepIDs := make([]string, 0, len(metrics.StepMetrics))
	for stepID := range metrics.StepMetrics {
		stepIDs = append(stepIDs, stepID)
	}
	sort.Strings(stepIDs)

	for _, stepID := range stepIDs {
		sm := metrics.StepMetrics[stepID]
		tags := baseTags + fmt.Sprintf(",step_id=%s", escapeTag(stepID))

		// Request metrics
		fields := fmt.Sprintf("count=%di,success_count=%di,failure_count=%di",
			sm.Count, sm.SuccessCount, sm.FailureCount)

		// Duration metrics
		if sm.Duration != nil {
			fields += fmt.Sprintf(",min_ns=%di,max_ns=%di,avg_ns=%di,p50_ns=%di,p90_ns=%di,p95_ns=%di,p99_ns=%di",
				sm.Duration.Min.Nanoseconds(),
				sm.Duration.Max.Nanoseconds(),
				sm.Duration.Avg.Nanoseconds(),
				sm.Duration.P50.Nanoseconds(),
				sm.Duration.P90.Nanoseconds(),
				sm.Duration.P95.Nanoseconds(),
				sm.Duration.P99.Nanoseconds(),
			)
		}

		line := fmt.Sprintf("workflow_step%s %s %d", tags, fields, timestamp)
		lines = append(lines, line)
	}

	// System metrics
	if metrics.SystemMetrics != nil {
		fields := fmt.Sprintf("cpu_usage=%f,memory_usage=%f,goroutine_count=%di",
			metrics.SystemMetrics.CPUUsage,
			metrics.SystemMetrics.MemoryUsage,
			metrics.SystemMetrics.GoroutineCount,
		)
		line := fmt.Sprintf("workflow_system%s %s %d", baseTags, fields, timestamp)
		lines = append(lines, line)
	}

	return lines
}

// buildBaseTags builds the base tags string from config.
func (r *Reporter) buildBaseTags() string {
	if len(r.config.Tags) == 0 {
		return ""
	}

	// Sort tag keys for consistent output
	keys := make([]string, 0, len(r.config.Tags))
	for k := range r.config.Tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", escapeTag(k), escapeTag(r.config.Tags[k])))
	}

	return "," + strings.Join(parts, ",")
}

// escapeTag escapes a tag key or value for InfluxDB line protocol.
func escapeTag(s string) string {
	s = strings.ReplaceAll(s, " ", "\\ ")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "=", "\\=")
	return s
}

// escapeField escapes a field value for InfluxDB line protocol.
func escapeField(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// GetWriteURL returns the InfluxDB write URL.
func (r *Reporter) GetWriteURL() string {
	return r.writeURL
}

// GetConfig returns the reporter configuration.
func (r *Reporter) GetConfig() *Config {
	return r.config
}

// GetBufferSize returns the current buffer size.
func (r *Reporter) GetBufferSize() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.buffer)
}
