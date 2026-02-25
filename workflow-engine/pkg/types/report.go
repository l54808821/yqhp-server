package types

import "time"

// PerformanceTestReport is the comprehensive final report generated
// after a performance test completes. Inspired by k6's summary system.
type PerformanceTestReport struct {
	// Meta information
	ExecutionID  string    `json:"execution_id"`
	WorkflowID   string    `json:"workflow_id"`
	WorkflowName string    `json:"workflow_name"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	Status       string    `json:"status"`

	// Summary dashboard
	Summary ReportSummary `json:"summary"`

	// Full time-series data for charts
	TimeSeries []*ReportTimeSeriesPoint `json:"time_series"`

	// Per-step detailed metrics
	StepDetails []*StepDetailReport `json:"step_details"`

	// Threshold evaluation results
	Thresholds []*ReportThresholdResult `json:"thresholds"`

	// Error analysis
	ErrorAnalysis *ReportErrorAnalysis `json:"error_analysis"`

	// VU schedule timeline (including manual adjustments)
	VUTimeline []*VUTimelineEvent `json:"vu_timeline"`

	// Execution configuration
	Config *ReportConfig `json:"config"`
}

// ReportSummary contains the high-level summary statistics.
type ReportSummary struct {
	TotalDurationMs int64   `json:"total_duration_ms"`
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	FailedRequests  int64   `json:"failed_requests"`
	ErrorRate       float64 `json:"error_rate"`

	AvgQPS  float64 `json:"avg_qps"`
	PeakQPS float64 `json:"peak_qps"`

	AvgResponseTimeMs float64 `json:"avg_response_time_ms"`
	MinResponseTimeMs float64 `json:"min_response_time_ms"`
	MaxResponseTimeMs float64 `json:"max_response_time_ms"`
	P50ResponseTimeMs float64 `json:"p50_response_time_ms"`
	P90ResponseTimeMs float64 `json:"p90_response_time_ms"`
	P95ResponseTimeMs float64 `json:"p95_response_time_ms"`
	P99ResponseTimeMs float64 `json:"p99_response_time_ms"`

	MaxVUs          int   `json:"max_vus"`
	TotalIterations int64 `json:"total_iterations"`

	ThroughputBytesPerSec float64 `json:"throughput_bytes_per_sec"`
	TotalDataSent         int64   `json:"total_data_sent"`
	TotalDataReceived     int64   `json:"total_data_received"`
	ThresholdsPassRate    float64 `json:"thresholds_pass_rate"`
}

// ReportTimeSeriesPoint is a single point in the time-series data.
type ReportTimeSeriesPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	ElapsedMs  int64     `json:"elapsed_ms"`
	QPS        float64   `json:"qps"`
	AvgRTMs    float64   `json:"avg_rt_ms"`
	P50RTMs    float64   `json:"p50_rt_ms"`
	P90RTMs    float64   `json:"p90_rt_ms"`
	P95RTMs    float64   `json:"p95_rt_ms"`
	P99RTMs    float64   `json:"p99_rt_ms"`
	ActiveVUs  int64     `json:"active_vus"`
	ErrorRate  float64   `json:"error_rate"`
	Iterations int64     `json:"iterations"`

	DataSentPerSec     float64 `json:"data_sent_per_sec"`
	DataReceivedPerSec float64 `json:"data_received_per_sec"`
}

// StepDetailReport contains detailed metrics for a single step.
type StepDetailReport struct {
	StepID   string `json:"step_id"`
	StepName string `json:"step_name,omitempty"`
	StepType string `json:"step_type,omitempty"`

	Count        int64   `json:"count"`
	SuccessCount int64   `json:"success_count"`
	FailureCount int64   `json:"failure_count"`
	ErrorRate    float64 `json:"error_rate"`

	AvgMs float64 `json:"avg_ms"`
	MinMs float64 `json:"min_ms"`
	MaxMs float64 `json:"max_ms"`
	P50Ms float64 `json:"p50_ms"`
	P90Ms float64 `json:"p90_ms"`
	P95Ms float64 `json:"p95_ms"`
	P99Ms float64 `json:"p99_ms"`
}

// ReportThresholdResult contains the outcome of a threshold evaluation.
type ReportThresholdResult struct {
	Metric      string  `json:"metric"`
	Expression  string  `json:"expression"`
	Passed      bool    `json:"passed"`
	ActualValue float64 `json:"actual_value"`
}

// ReportErrorAnalysis contains error distribution and details.
type ReportErrorAnalysis struct {
	TotalErrors int64                 `json:"total_errors"`
	ErrorTypes  []*ErrorTypeStats     `json:"error_types"`
	TopErrors   []*ErrorDetail        `json:"top_errors"`
}

// ErrorTypeStats shows the distribution of a specific error type.
type ErrorTypeStats struct {
	Type       string  `json:"type"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// ErrorDetail contains information about a specific error occurrence.
type ErrorDetail struct {
	Message   string    `json:"message"`
	StepID    string    `json:"step_id,omitempty"`
	Count     int64     `json:"count"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// VUTimelineEvent records a VU count change event.
type VUTimelineEvent struct {
	Timestamp time.Time `json:"timestamp"`
	ElapsedMs int64     `json:"elapsed_ms"`
	VUs       int       `json:"vus"`
	Source    string    `json:"source"` // "auto" (from stages) or "manual" (user API call)
	Reason   string    `json:"reason,omitempty"`
}

// ReportConfig records the execution configuration for the report.
type ReportConfig struct {
	Mode       string  `json:"mode"`
	VUs        int     `json:"vus"`
	Duration   string  `json:"duration"`
	Iterations int     `json:"iterations,omitempty"`
	Stages     []Stage `json:"stages,omitempty"`
}
