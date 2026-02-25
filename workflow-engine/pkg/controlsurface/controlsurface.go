// Package controlsurface provides the ControlSurface type and registry
// for accessing running execution state from any layer (REST API, engine, etc.)
// without creating import cycles.
package controlsurface

import (
	"context"
	"sync"

	"yqhp/workflow-engine/pkg/metrics"
	"yqhp/workflow-engine/pkg/types"
)

// MetricsEngineInterface abstracts the MetricsEngine to avoid importing internal packages.
type MetricsEngineInterface interface {
	GetTimeSeriesData() []*TimeSeriesPoint
	GetLatestSnapshot() *TimeSeriesPoint
	BuildRealtimeMetrics(status string, getVUs func() int64, getIterations func() int64, getErrors func() []string) *RealtimeMetrics
	StopTimeSeriesCollection()
}

// SummaryOutputInterface abstracts the SummaryOutput to avoid importing internal packages.
type SummaryOutputInterface interface {
	RecordVUChange(vus int, source, reason string)
}

// TimeSeriesPoint captures a snapshot of key metrics at a point in time.
type TimeSeriesPoint struct {
	Timestamp  string  `json:"timestamp"`
	ElapsedMs  int64   `json:"elapsed_ms"`
	Iterations int64   `json:"iterations"`
	ActiveVUs  int64   `json:"active_vus"`
	QPS        float64 `json:"qps"`
	ErrorRate  float64 `json:"error_rate"`
	AvgRT      float64 `json:"avg_rt_ms"`
	P50RT      float64 `json:"p50_rt_ms"`
	P90RT      float64 `json:"p90_rt_ms"`
	P95RT      float64 `json:"p95_rt_ms"`
	P99RT      float64 `json:"p99_rt_ms"`

	DataSentPerSec     float64 `json:"data_sent_per_sec"`
	DataReceivedPerSec float64 `json:"data_received_per_sec"`
}

// RealtimeMetrics is the data structure pushed to clients.
type RealtimeMetrics struct {
	Status          string                      `json:"status"`
	ElapsedMs       int64                       `json:"elapsed_ms"`
	TotalVUs        int64                       `json:"total_vus"`
	TotalIterations int64                       `json:"total_iterations"`
	QPS             float64                     `json:"qps"`
	ErrorRate       float64                     `json:"error_rate"`
	StepMetrics     map[string]*StepMetricStats `json:"step_metrics,omitempty"`
	Errors          []string                    `json:"errors,omitempty"`
}

// StepMetricStats contains per-step aggregated statistics.
type StepMetricStats struct {
	StepID       string  `json:"step_id"`
	StepName     string  `json:"step_name,omitempty"`
	Count        int64   `json:"count"`
	SuccessCount int64   `json:"success_count"`
	FailureCount int64   `json:"failure_count"`
	AvgMs        float64 `json:"avg_ms"`
	MinMs        float64 `json:"min_ms"`
	MaxMs        float64 `json:"max_ms"`
	P50Ms        float64 `json:"p50_ms"`
	P90Ms        float64 `json:"p90_ms"`
	P95Ms        float64 `json:"p95_ms"`
	P99Ms        float64 `json:"p99_ms"`
}

// ExecutionStatus represents the current test execution status.
type ExecutionStatus struct {
	Status           string `json:"status"`
	Running          bool   `json:"running"`
	Paused           bool   `json:"paused"`
	VUs              int64  `json:"vus"`
	MaxVUs           int64  `json:"max_vus"`
	Iterations       int64  `json:"iterations"`
	DurationMs       int64  `json:"duration_ms"`
	ThresholdsFailed int    `json:"thresholds_failed"`
}

// ControlSurface provides access to a running test's internal state.
// Inspired by k6's api/v1/ControlSurface.
type ControlSurface struct {
	mu sync.RWMutex

	RunCtx        context.Context
	MetricsEngine MetricsEngineInterface
	SummaryOutput SummaryOutputInterface
	SamplesChan   chan metrics.SampleContainer

	GetStatus       func() *ExecutionStatus
	ScaleVUs        func(vus int) error
	StopExecution   func() error
	PauseExecution  func() error
	ResumeExecution func() error

	GetVUs        func() int64
	GetIterations func() int64
	GetErrors     func() []string

	FinalReport *types.PerformanceTestReport
}

// --- Global registry ---

var registry = struct {
	mu       sync.RWMutex
	surfaces map[string]*ControlSurface
}{
	surfaces: make(map[string]*ControlSurface),
}

// Register registers a ControlSurface for an execution.
func Register(executionID string, cs *ControlSurface) {
	registry.mu.Lock()
	registry.surfaces[executionID] = cs
	registry.mu.Unlock()
}

// Unregister removes a ControlSurface for an execution.
func Unregister(executionID string) {
	registry.mu.Lock()
	delete(registry.surfaces, executionID)
	registry.mu.Unlock()
}

// Get retrieves the ControlSurface for an execution.
func Get(executionID string) *ControlSurface {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.surfaces[executionID]
}
