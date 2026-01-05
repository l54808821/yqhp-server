// Package prometheus provides a Prometheus Push Gateway reporter for the workflow execution engine.
// Requirements: 9.1.4
package prometheus

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// Config holds configuration for the Prometheus reporter.
type Config struct {
	// PushGatewayURL is the URL of the Prometheus Push Gateway.
	PushGatewayURL string `yaml:"push_gateway_url"`
	// JobName is the job name for metrics.
	JobName string `yaml:"job_name"`
	// Labels are additional labels to add to all metrics.
	Labels map[string]string `yaml:"labels,omitempty"`
	// PushInterval is the interval between pushes.
	PushInterval time.Duration `yaml:"push_interval"`
	// Timeout is the HTTP request timeout.
	Timeout time.Duration `yaml:"timeout"`
}

// DefaultConfig returns the default Prometheus reporter configuration.
func DefaultConfig() *Config {
	return &Config{
		PushGatewayURL: "http://localhost:9091",
		JobName:        "workflow_engine",
		Labels:         make(map[string]string),
		PushInterval:   10 * time.Second,
		Timeout:        5 * time.Second,
	}
}

// Reporter implements the Prometheus Push Gateway reporter.
// Requirements: 9.1.4
type Reporter struct {
	config     *Config
	httpClient *http.Client

	// Metrics buffer
	lastMetrics *types.Metrics
	mu          sync.RWMutex

	// State
	initialized bool
	pushURL     string
}

// New creates a new Prometheus reporter.
func New(config *Config) *Reporter {
	if config == nil {
		config = DefaultConfig()
	}
	return &Reporter{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// NewFactory returns a factory function for creating Prometheus reporters.
func NewFactory() func(config map[string]any) (interface{ Name() string }, error) {
	return func(config map[string]any) (interface{ Name() string }, error) {
		cfg := DefaultConfig()
		if config != nil {
			if v, ok := config["push_gateway_url"].(string); ok {
				cfg.PushGatewayURL = v
			}
			if v, ok := config["job_name"].(string); ok {
				cfg.JobName = v
			}
			if v, ok := config["labels"].(map[string]any); ok {
				for k, val := range v {
					if s, ok := val.(string); ok {
						cfg.Labels[k] = s
					}
				}
			}
			if v, ok := config["push_interval"].(string); ok {
				if d, err := time.ParseDuration(v); err == nil {
					cfg.PushInterval = d
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
	return "prometheus"
}

// Init initializes the reporter.
func (r *Reporter) Init(ctx context.Context, config map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return fmt.Errorf("reporter already initialized")
	}

	// Build push URL
	r.pushURL = fmt.Sprintf("%s/metrics/job/%s",
		strings.TrimSuffix(r.config.PushGatewayURL, "/"),
		r.config.JobName,
	)

	// Add labels to URL
	for k, v := range r.config.Labels {
		r.pushURL = fmt.Sprintf("%s/%s/%s", r.pushURL, k, v)
	}

	r.initialized = true
	return nil
}

// Report sends metrics to Prometheus Push Gateway.
func (r *Reporter) Report(ctx context.Context, metrics *types.Metrics) error {
	r.mu.Lock()
	r.lastMetrics = metrics
	r.mu.Unlock()

	if !r.initialized {
		return fmt.Errorf("reporter not initialized")
	}

	// Convert metrics to Prometheus format
	promMetrics := r.convertMetrics(metrics)

	// Push to gateway
	return r.push(ctx, promMetrics)
}

// Flush flushes any buffered data.
func (r *Reporter) Flush(ctx context.Context) error {
	r.mu.RLock()
	metrics := r.lastMetrics
	r.mu.RUnlock()

	if metrics == nil {
		return nil
	}

	promMetrics := r.convertMetrics(metrics)
	return r.push(ctx, promMetrics)
}

// Close closes the reporter.
func (r *Reporter) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return nil
	}

	// Delete metrics from Push Gateway
	if err := r.delete(ctx); err != nil {
		// Log but don't fail on delete error
		_ = err
	}

	r.initialized = false
	return nil
}

// push sends metrics to the Push Gateway.
func (r *Reporter) push(ctx context.Context, metrics string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.pushURL, strings.NewReader(metrics))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain; version=0.0.4")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to push metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("push gateway returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// delete removes metrics from the Push Gateway.
func (r *Reporter) delete(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, r.pushURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete metrics: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// convertMetrics converts types.Metrics to Prometheus exposition format.
func (r *Reporter) convertMetrics(metrics *types.Metrics) string {
	var buf bytes.Buffer

	// Sort step IDs for consistent output
	stepIDs := make([]string, 0, len(metrics.StepMetrics))
	for stepID := range metrics.StepMetrics {
		stepIDs = append(stepIDs, stepID)
	}
	sort.Strings(stepIDs)

	for _, stepID := range stepIDs {
		sm := metrics.StepMetrics[stepID]
		labels := fmt.Sprintf(`step_id="%s"`, escapeLabel(stepID))

		// Request count
		buf.WriteString(fmt.Sprintf("# HELP workflow_requests_total Total number of requests\n"))
		buf.WriteString(fmt.Sprintf("# TYPE workflow_requests_total counter\n"))
		buf.WriteString(fmt.Sprintf("workflow_requests_total{%s} %d\n", labels, sm.Count))

		// Success count
		buf.WriteString(fmt.Sprintf("# HELP workflow_requests_success_total Total number of successful requests\n"))
		buf.WriteString(fmt.Sprintf("# TYPE workflow_requests_success_total counter\n"))
		buf.WriteString(fmt.Sprintf("workflow_requests_success_total{%s} %d\n", labels, sm.SuccessCount))

		// Failure count
		buf.WriteString(fmt.Sprintf("# HELP workflow_requests_failed_total Total number of failed requests\n"))
		buf.WriteString(fmt.Sprintf("# TYPE workflow_requests_failed_total counter\n"))
		buf.WriteString(fmt.Sprintf("workflow_requests_failed_total{%s} %d\n", labels, sm.FailureCount))

		// Duration metrics
		if sm.Duration != nil {
			buf.WriteString(fmt.Sprintf("# HELP workflow_request_duration_seconds Request duration in seconds\n"))
			buf.WriteString(fmt.Sprintf("# TYPE workflow_request_duration_seconds summary\n"))
			buf.WriteString(fmt.Sprintf("workflow_request_duration_seconds{%s,quantile=\"0.5\"} %f\n", labels, sm.Duration.P50.Seconds()))
			buf.WriteString(fmt.Sprintf("workflow_request_duration_seconds{%s,quantile=\"0.9\"} %f\n", labels, sm.Duration.P90.Seconds()))
			buf.WriteString(fmt.Sprintf("workflow_request_duration_seconds{%s,quantile=\"0.95\"} %f\n", labels, sm.Duration.P95.Seconds()))
			buf.WriteString(fmt.Sprintf("workflow_request_duration_seconds{%s,quantile=\"0.99\"} %f\n", labels, sm.Duration.P99.Seconds()))

			// Min/Max/Avg as gauges
			buf.WriteString(fmt.Sprintf("# HELP workflow_request_duration_min_seconds Minimum request duration\n"))
			buf.WriteString(fmt.Sprintf("# TYPE workflow_request_duration_min_seconds gauge\n"))
			buf.WriteString(fmt.Sprintf("workflow_request_duration_min_seconds{%s} %f\n", labels, sm.Duration.Min.Seconds()))

			buf.WriteString(fmt.Sprintf("# HELP workflow_request_duration_max_seconds Maximum request duration\n"))
			buf.WriteString(fmt.Sprintf("# TYPE workflow_request_duration_max_seconds gauge\n"))
			buf.WriteString(fmt.Sprintf("workflow_request_duration_max_seconds{%s} %f\n", labels, sm.Duration.Max.Seconds()))

			buf.WriteString(fmt.Sprintf("# HELP workflow_request_duration_avg_seconds Average request duration\n"))
			buf.WriteString(fmt.Sprintf("# TYPE workflow_request_duration_avg_seconds gauge\n"))
			buf.WriteString(fmt.Sprintf("workflow_request_duration_avg_seconds{%s} %f\n", labels, sm.Duration.Avg.Seconds()))
		}

		buf.WriteString("\n")
	}

	// System metrics
	if metrics.SystemMetrics != nil {
		buf.WriteString("# HELP workflow_cpu_usage_percent CPU usage percentage\n")
		buf.WriteString("# TYPE workflow_cpu_usage_percent gauge\n")
		buf.WriteString(fmt.Sprintf("workflow_cpu_usage_percent %f\n", metrics.SystemMetrics.CPUUsage))

		buf.WriteString("# HELP workflow_memory_usage_percent Memory usage percentage\n")
		buf.WriteString("# TYPE workflow_memory_usage_percent gauge\n")
		buf.WriteString(fmt.Sprintf("workflow_memory_usage_percent %f\n", metrics.SystemMetrics.MemoryUsage))

		buf.WriteString("# HELP workflow_goroutines Number of goroutines\n")
		buf.WriteString("# TYPE workflow_goroutines gauge\n")
		buf.WriteString(fmt.Sprintf("workflow_goroutines %d\n", metrics.SystemMetrics.GoroutineCount))
	}

	return buf.String()
}

// escapeLabel escapes a label value for Prometheus format.
func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// GetPushURL returns the Push Gateway URL.
func (r *Reporter) GetPushURL() string {
	return r.pushURL
}

// GetConfig returns the reporter configuration.
func (r *Reporter) GetConfig() *Config {
	return r.config
}
