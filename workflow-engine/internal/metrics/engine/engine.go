// Package engine contains the internal metrics engine responsible for
// aggregating metrics during the test and evaluating thresholds against them.
// Design inspired by k6's internal/metrics/engine.
package engine

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"yqhp/workflow-engine/pkg/controlsurface"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/metrics"
)

const thresholdsRate = 2 * time.Second

// MetricsEngine aggregates metric samples and evaluates thresholds.
type MetricsEngine struct {
	registry *metrics.Registry

	metricsWithThresholds   []*thresholdMetric
	breachedThresholdsCount uint32

	MetricsLock     sync.Mutex
	ObservedMetrics map[string]*metrics.Metric

	// Step name mapping (step_id -> step_name)
	StepNames map[string]string

	// Time-series snapshots for report generation
	timeSeriesMu   sync.Mutex
	timeSeriesData []*TimeSeriesPoint
	snapshotTicker *time.Ticker
	snapshotDone   chan struct{}

	startTime time.Time
}

// TimeSeriesPoint is an alias for the controlsurface type.
type TimeSeriesPoint = controlsurface.TimeSeriesPoint

type thresholdMetric struct {
	metric     *metrics.Metric
	expression string
	abort      bool
}

// NewMetricsEngine creates a new MetricsEngine with the given registry.
func NewMetricsEngine(registry *metrics.Registry) *MetricsEngine {
	return &MetricsEngine{
		registry:        registry,
		ObservedMetrics: make(map[string]*metrics.Metric),
		StepNames:       make(map[string]string),
	}
}

// CreateIngester returns an OutputIngester (implements output.Output)
// that feeds metric samples into this engine.
func (me *MetricsEngine) CreateIngester() *OutputIngester {
	return &OutputIngester{
		metricsEngine: me,
	}
}

// MarkObserved marks a metric as observed so it shows in the final report.
func (me *MetricsEngine) MarkObserved(m *metrics.Metric) {
	if _, exists := me.ObservedMetrics[m.Name]; !exists {
		me.ObservedMetrics[m.Name] = m
	}
}

// InitThresholds parses and initializes threshold definitions.
// Format: map[metricName][]ThresholdConfig
func (me *MetricsEngine) InitThresholds(thresholds map[string][]ThresholdConfig) error {
	for metricName, configs := range thresholds {
		m := me.registry.Get(metricName)
		if m == nil {
			logger.Warn("Threshold references unknown metric", "metric", metricName)
			continue
		}

		for _, cfg := range configs {
			me.metricsWithThresholds = append(me.metricsWithThresholds, &thresholdMetric{
				metric:     m,
				expression: cfg.Expression,
				abort:      cfg.AbortOnFail,
			})
		}
		me.MarkObserved(m)
	}
	return nil
}

// ThresholdConfig defines a single threshold.
type ThresholdConfig struct {
	Expression  string `json:"expression"`
	AbortOnFail bool   `json:"abort_on_fail"`
}

// StartThresholdCalculations starts a goroutine that evaluates thresholds
// periodically and returns a finalize callback.
func (me *MetricsEngine) StartThresholdCalculations(
	ingester *OutputIngester,
	abortRun func(error),
	getCurrentDuration func() time.Duration,
) (finalize func() []string) {
	if len(me.metricsWithThresholds) == 0 {
		return nil
	}

	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(thresholdsRate)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				breached, shouldAbort := me.evaluateThresholds(getCurrentDuration)
				if shouldAbort && abortRun != nil {
					abortRun(fmt.Errorf(
						"thresholds on metrics '%s' were crossed; abortOnFail enabled",
						strings.Join(breached, ", "),
					))
				}
			case <-stop:
				return
			}
		}
	}()

	return func() []string {
		if ingester != nil {
			ingester.Stop()
		}
		close(stop)
		<-done

		breached, _ := me.evaluateThresholds(getCurrentDuration)
		return breached
	}
}

// evaluateThresholds checks all thresholds against current metric sinks.
func (me *MetricsEngine) evaluateThresholds(
	getCurrentDuration func() time.Duration,
) (breachedThresholds []string, shouldAbort bool) {
	me.MetricsLock.Lock()
	defer me.MetricsLock.Unlock()

	duration := getCurrentDuration().Seconds()

	for _, tm := range me.metricsWithThresholds {
		if tm.metric.Sink == nil || tm.metric.Sink.IsEmpty() {
			continue
		}

		stats := tm.metric.Sink.Format(duration)
		if !evaluateExpression(tm.expression, stats) {
			breachedThresholds = append(breachedThresholds, tm.metric.Name)
			if tm.abort {
				shouldAbort = true
			}
		}
	}

	if len(breachedThresholds) > 0 {
		sort.Strings(breachedThresholds)
	}
	atomic.StoreUint32(&me.breachedThresholdsCount, uint32(len(breachedThresholds)))
	return breachedThresholds, shouldAbort
}

// evaluateExpression evaluates a simple threshold expression like "p(95) < 500", "rate > 0.99".
func evaluateExpression(expr string, stats map[string]float64) bool {
	expr = strings.TrimSpace(expr)

	operators := []string{"<=", ">=", "!=", "<", ">", "=="}
	for _, op := range operators {
		parts := strings.SplitN(expr, op, 2)
		if len(parts) != 2 {
			continue
		}

		metricKey := strings.TrimSpace(parts[0])
		var threshold float64
		if _, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &threshold); err != nil {
			continue
		}

		value, ok := stats[metricKey]
		if !ok {
			return true // metric not found, assume passing
		}

		switch op {
		case "<":
			return value < threshold
		case "<=":
			return value <= threshold
		case ">":
			return value > threshold
		case ">=":
			return value >= threshold
		case "==":
			return value == threshold
		case "!=":
			return value != threshold
		}
	}

	return true
}

// GetBreachedThresholdsCount returns the number of breached thresholds.
func (me *MetricsEngine) GetBreachedThresholdsCount() uint32 {
	return atomic.LoadUint32(&me.breachedThresholdsCount)
}

// StartTimeSeriesCollection starts periodic snapshots of aggregated metrics.
func (me *MetricsEngine) StartTimeSeriesCollection(
	getVUs func() int64,
	getIterations func() int64,
) {
	me.startTime = time.Now()
	me.snapshotDone = make(chan struct{})
	me.snapshotTicker = time.NewTicker(1 * time.Second)

	var lastIterations int64
	var lastDataSent, lastDataReceived float64

	go func() {
		defer close(me.snapshotDone)
		for {
			select {
			case <-me.snapshotTicker.C:
				now := time.Now()
				currentIter := getIterations()

				me.MetricsLock.Lock()

				point := &TimeSeriesPoint{
					Timestamp:  now.Format(time.RFC3339),
					ElapsedMs:  now.Sub(me.startTime).Milliseconds(),
					Iterations: currentIter,
					ActiveVUs:  getVUs(),
					QPS:        float64(currentIter - lastIterations),
				}

				if m := me.ObservedMetrics["step_duration"]; m != nil && m.Sink != nil {
					stats := m.Sink.Format(now.Sub(me.startTime).Seconds())
					point.AvgRT = stats["avg"]
					point.P50RT = stats["med"]
					point.P90RT = stats["p(90)"]
					point.P95RT = stats["p(95)"]
					point.P99RT = stats["p(99)"]
				}

				if m := me.ObservedMetrics["step_failed"]; m != nil && m.Sink != nil {
					stats := m.Sink.Format(0)
					if total := stats["passes"] + stats["fails"]; total > 0 {
						point.ErrorRate = stats["fails"] / total * 100
					}
				}

				if m := me.ObservedMetrics["data_sent"]; m != nil && m.Sink != nil {
					stats := m.Sink.Format(0)
					currentDataSent := stats["count"]
					point.DataSentPerSec = currentDataSent - lastDataSent
					lastDataSent = currentDataSent
				}

				if m := me.ObservedMetrics["data_received"]; m != nil && m.Sink != nil {
					stats := m.Sink.Format(0)
					currentDataReceived := stats["count"]
					point.DataReceivedPerSec = currentDataReceived - lastDataReceived
					lastDataReceived = currentDataReceived
				}

				me.MetricsLock.Unlock()

				lastIterations = currentIter

				me.timeSeriesMu.Lock()
				me.timeSeriesData = append(me.timeSeriesData, point)
				me.timeSeriesMu.Unlock()

			case <-me.snapshotDone:
				return
			}
		}
	}()
}

// StopTimeSeriesCollection stops the periodic snapshots.
func (me *MetricsEngine) StopTimeSeriesCollection() {
	if me.snapshotTicker != nil {
		me.snapshotTicker.Stop()
	}
	if me.snapshotDone != nil {
		select {
		case me.snapshotDone <- struct{}{}:
		default:
		}
	}
}

// GetTimeSeriesData returns a copy of all time-series snapshots.
func (me *MetricsEngine) GetTimeSeriesData() []*TimeSeriesPoint {
	me.timeSeriesMu.Lock()
	defer me.timeSeriesMu.Unlock()
	result := make([]*TimeSeriesPoint, len(me.timeSeriesData))
	copy(result, me.timeSeriesData)
	return result
}

// GetLatestSnapshot returns the most recent time-series snapshot.
func (me *MetricsEngine) GetLatestSnapshot() *TimeSeriesPoint {
	me.timeSeriesMu.Lock()
	defer me.timeSeriesMu.Unlock()
	if len(me.timeSeriesData) == 0 {
		return nil
	}
	return me.timeSeriesData[len(me.timeSeriesData)-1]
}

// GetAggregatedStats returns aggregated statistics for all observed metrics.
func (me *MetricsEngine) GetAggregatedStats(duration time.Duration) map[string]map[string]float64 {
	me.MetricsLock.Lock()
	defer me.MetricsLock.Unlock()

	result := make(map[string]map[string]float64)
	for name, m := range me.ObservedMetrics {
		if m.Sink != nil && !m.Sink.IsEmpty() {
			result[name] = m.Sink.Format(duration.Seconds())
		}
	}
	return result
}
