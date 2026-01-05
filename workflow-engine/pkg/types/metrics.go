package types

import "time"

// Metrics contains collected performance metrics.
type Metrics struct {
	Timestamp     time.Time
	StepMetrics   map[string]*StepMetrics
	SystemMetrics *SystemMetrics
}

// StepMetrics contains metrics for a specific step.
type StepMetrics struct {
	StepID        string
	Count         int64
	SuccessCount  int64
	FailureCount  int64
	Duration      *DurationMetrics
	CustomMetrics map[string]float64
}

// DurationMetrics contains timing statistics.
type DurationMetrics struct {
	Min time.Duration
	Max time.Duration
	Avg time.Duration
	P50 time.Duration
	P90 time.Duration
	P95 time.Duration
	P99 time.Duration
}

// SystemMetrics contains system-level metrics.
type SystemMetrics struct {
	CPUUsage       float64
	MemoryUsage    float64
	GoroutineCount int
}

// AggregatedMetrics contains metrics aggregated from all slaves.
type AggregatedMetrics struct {
	ExecutionID     string
	TotalVUs        int
	TotalIterations int64
	Duration        time.Duration
	StepMetrics     map[string]*StepMetrics
	Thresholds      []ThresholdResult
}
