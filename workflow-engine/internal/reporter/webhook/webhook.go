// Package webhook provides a Webhook reporter for the workflow execution engine.
// Requirements: 9.1.6
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// Config holds configuration for the Webhook reporter.
type Config struct {
	// URL is the webhook endpoint URL.
	URL string `yaml:"url"`
	// Method is the HTTP method (default: POST).
	Method string `yaml:"method"`
	// Headers are additional HTTP headers.
	Headers map[string]string `yaml:"headers,omitempty"`
	// BatchSize is the number of records to batch before sending.
	BatchSize int `yaml:"batch_size"`
	// RetryAttempts is the number of retry attempts on failure.
	RetryAttempts int `yaml:"retry_attempts"`
	// RetryDelay is the delay between retry attempts.
	RetryDelay time.Duration `yaml:"retry_delay"`
	// Timeout is the HTTP request timeout.
	Timeout time.Duration `yaml:"timeout"`
}

// DefaultConfig returns the default Webhook reporter configuration.
func DefaultConfig() *Config {
	return &Config{
		URL:           "",
		Method:        http.MethodPost,
		Headers:       make(map[string]string),
		BatchSize:     10,
		RetryAttempts: 3,
		RetryDelay:    time.Second,
		Timeout:       10 * time.Second,
	}
}

// Reporter implements the Webhook reporter.
// Requirements: 9.1.6
type Reporter struct {
	config     *Config
	httpClient *http.Client

	// Buffer for batch sends
	buffer []*WebhookPayload
	mu     sync.Mutex

	// State
	initialized bool
}

// WebhookPayload represents the payload sent to the webhook.
type WebhookPayload struct {
	Timestamp     time.Time                      `json:"timestamp"`
	StepMetrics   map[string]*WebhookStepMetrics `json:"step_metrics,omitempty"`
	SystemMetrics *WebhookSystemMetrics          `json:"system_metrics,omitempty"`
}

// WebhookStepMetrics represents step metrics in the webhook payload.
type WebhookStepMetrics struct {
	StepID        string             `json:"step_id"`
	Count         int64              `json:"count"`
	SuccessCount  int64              `json:"success_count"`
	FailureCount  int64              `json:"failure_count"`
	Duration      *WebhookDuration   `json:"duration,omitempty"`
	CustomMetrics map[string]float64 `json:"custom_metrics,omitempty"`
}

// WebhookDuration represents duration metrics in the webhook payload.
type WebhookDuration struct {
	MinMs float64 `json:"min_ms"`
	MaxMs float64 `json:"max_ms"`
	AvgMs float64 `json:"avg_ms"`
	P50Ms float64 `json:"p50_ms"`
	P90Ms float64 `json:"p90_ms"`
	P95Ms float64 `json:"p95_ms"`
	P99Ms float64 `json:"p99_ms"`
}

// WebhookSystemMetrics represents system metrics in the webhook payload.
type WebhookSystemMetrics struct {
	CPUUsage       float64 `json:"cpu_usage"`
	MemoryUsage    float64 `json:"memory_usage"`
	GoroutineCount int     `json:"goroutine_count"`
}

// WebhookBatchPayload represents a batch of payloads.
type WebhookBatchPayload struct {
	Records []*WebhookPayload `json:"records"`
	Count   int               `json:"count"`
}

// New creates a new Webhook reporter.
func New(config *Config) *Reporter {
	if config == nil {
		config = DefaultConfig()
	}
	if config.Method == "" {
		config.Method = http.MethodPost
	}
	return &Reporter{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		buffer: make([]*WebhookPayload, 0, config.BatchSize),
	}
}

// NewFactory returns a factory function for creating Webhook reporters.
func NewFactory() func(config map[string]any) (interface{ Name() string }, error) {
	return func(config map[string]any) (interface{ Name() string }, error) {
		cfg := DefaultConfig()
		if config != nil {
			if v, ok := config["url"].(string); ok {
				cfg.URL = v
			}
			if v, ok := config["method"].(string); ok {
				cfg.Method = v
			}
			if v, ok := config["headers"].(map[string]any); ok {
				for k, val := range v {
					if s, ok := val.(string); ok {
						cfg.Headers[k] = s
					}
				}
			}
			if v, ok := config["batch_size"].(int); ok {
				cfg.BatchSize = v
			}
			if v, ok := config["retry_attempts"].(int); ok {
				cfg.RetryAttempts = v
			}
			if v, ok := config["retry_delay"].(string); ok {
				if d, err := time.ParseDuration(v); err == nil {
					cfg.RetryDelay = d
				}
			}
			if v, ok := config["timeout"].(string); ok {
				if d, err := time.ParseDuration(v); err == nil {
					cfg.Timeout = d
				}
			}
		}
		return New(cfg), nil
	}
}

// Name returns the reporter name.
func (r *Reporter) Name() string {
	return "webhook"
}

// Init initializes the reporter.
func (r *Reporter) Init(ctx context.Context, config map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return fmt.Errorf("reporter already initialized")
	}

	if r.config.URL == "" {
		return fmt.Errorf("webhook URL is required")
	}

	r.initialized = true
	return nil
}

// Report sends metrics to the webhook.
func (r *Reporter) Report(ctx context.Context, metrics *types.Metrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return fmt.Errorf("reporter not initialized")
	}

	// Convert metrics to webhook payload
	payload := r.convertMetrics(metrics)
	r.buffer = append(r.buffer, payload)

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

// flushBuffer sends buffered payloads to the webhook.
func (r *Reporter) flushBuffer(ctx context.Context) error {
	if len(r.buffer) == 0 {
		return nil
	}

	// Create batch payload
	batch := &WebhookBatchPayload{
		Records: r.buffer,
		Count:   len(r.buffer),
	}

	// Send with retry
	err := r.sendWithRetry(ctx, batch)
	if err != nil {
		return err
	}

	// Clear buffer
	r.buffer = r.buffer[:0]
	return nil
}

// sendWithRetry sends the payload with retry logic.
func (r *Reporter) sendWithRetry(ctx context.Context, payload *WebhookBatchPayload) error {
	var lastErr error

	for attempt := 0; attempt <= r.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(r.config.RetryDelay * time.Duration(attempt)):
			}
		}

		err := r.send(ctx, payload)
		if err == nil {
			return nil
		}

		lastErr = err
	}

	return fmt.Errorf("failed after %d attempts: %w", r.config.RetryAttempts+1, lastErr)
}

// send sends the payload to the webhook.
func (r *Reporter) send(ctx context.Context, payload *WebhookBatchPayload) error {
	// Marshal payload
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, r.config.Method, r.config.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range r.config.Headers {
		req.Header.Set(k, v)
	}

	// Send request
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// convertMetrics converts types.Metrics to WebhookPayload.
func (r *Reporter) convertMetrics(metrics *types.Metrics) *WebhookPayload {
	payload := &WebhookPayload{
		Timestamp:   metrics.Timestamp,
		StepMetrics: make(map[string]*WebhookStepMetrics),
	}

	for stepID, sm := range metrics.StepMetrics {
		wsm := &WebhookStepMetrics{
			StepID:        sm.StepID,
			Count:         sm.Count,
			SuccessCount:  sm.SuccessCount,
			FailureCount:  sm.FailureCount,
			CustomMetrics: sm.CustomMetrics,
		}

		if sm.Duration != nil {
			wsm.Duration = &WebhookDuration{
				MinMs: float64(sm.Duration.Min.Microseconds()) / 1000,
				MaxMs: float64(sm.Duration.Max.Microseconds()) / 1000,
				AvgMs: float64(sm.Duration.Avg.Microseconds()) / 1000,
				P50Ms: float64(sm.Duration.P50.Microseconds()) / 1000,
				P90Ms: float64(sm.Duration.P90.Microseconds()) / 1000,
				P95Ms: float64(sm.Duration.P95.Microseconds()) / 1000,
				P99Ms: float64(sm.Duration.P99.Microseconds()) / 1000,
			}
		}

		payload.StepMetrics[stepID] = wsm
	}

	if metrics.SystemMetrics != nil {
		payload.SystemMetrics = &WebhookSystemMetrics{
			CPUUsage:       metrics.SystemMetrics.CPUUsage,
			MemoryUsage:    metrics.SystemMetrics.MemoryUsage,
			GoroutineCount: metrics.SystemMetrics.GoroutineCount,
		}
	}

	return payload
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
