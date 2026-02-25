// Package summary provides a Summary Output that collects all metric samples
// during a test run and generates a PerformanceTestReport at the end.
// Inspired by k6's internal/output/summary.
package summary

import (
	"sort"
	"sync"
	"time"

	"yqhp/workflow-engine/internal/metrics/engine"
	"yqhp/workflow-engine/pkg/metrics"
	"yqhp/workflow-engine/pkg/output"
	"yqhp/workflow-engine/pkg/types"
)

// Compile-time check.
var _ output.Output = &Output{}

// Output implements output.Output and collects all metric samples
// for generating the final performance test report.
type Output struct {
	output.SampleBuffer

	mu              sync.Mutex
	stepDurations   map[string]*metrics.TrendSink  // step_id -> duration sink
	stepCounts      map[string]*metrics.CounterSink // step_id -> request count
	stepFailures    map[string]*metrics.RateSink    // step_id -> failure rate
	stepNames       map[string]string               // step_id -> step_name
	errorTracker    *errorTracker
	vuTimeline      []*types.VUTimelineEvent
	timeSeriesData  []*types.ReportTimeSeriesPoint

	periodicFlusher *output.PeriodicFlusher
	startTime       time.Time
}

// New creates a new Summary Output.
func New() *Output {
	return &Output{
		stepDurations: make(map[string]*metrics.TrendSink),
		stepCounts:    make(map[string]*metrics.CounterSink),
		stepFailures:  make(map[string]*metrics.RateSink),
		stepNames:     make(map[string]string),
		errorTracker:  newErrorTracker(),
	}
}

func (o *Output) Description() string {
	return "Performance Test Summary"
}

func (o *Output) Start() error {
	o.startTime = time.Now()
	pf, err := output.NewPeriodicFlusher(100*time.Millisecond, o.flushSamples)
	if err != nil {
		return err
	}
	o.periodicFlusher = pf
	return nil
}

func (o *Output) Stop() error {
	if o.periodicFlusher != nil {
		o.periodicFlusher.Stop()
	}
	return nil
}

func (o *Output) SetRunStatus(_ output.RunStatus) {}

// flushSamples processes buffered samples into internal aggregation structures.
func (o *Output) flushSamples() {
	containers := o.GetBufferedSamples()
	if len(containers) == 0 {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	for _, container := range containers {
		for _, sample := range container.GetSamples() {
			o.processSample(sample)
		}
	}
}

func (o *Output) processSample(sample metrics.Sample) {
	if sample.Metric == nil {
		return
	}

	name := sample.Metric.Name
	stepID := sample.Tags["step_id"]

	if stepID != "" {
		if sn := sample.Tags["step_name"]; sn != "" {
			o.stepNames[stepID] = sn
		}
	}

	switch sample.Metric.Type {
	case metrics.Trend:
		if stepID != "" {
			sink := o.getOrCreateTrendSink(stepID)
			sink.Add(sample)
		}
	case metrics.Counter:
		if stepID != "" && (name == "step_reqs" || len(name) > 10 && name[:10] == "step_reqs_") {
			sink := o.getOrCreateCounterSink(stepID)
			sink.Add(sample)
		}
	case metrics.Rate:
		if stepID != "" && (name == "step_failed" || len(name) > 12 && name[:12] == "step_failed_") {
			sink := o.getOrCreateRateSink(stepID)
			sink.Add(sample)
			if sample.Value == 0 { // Value=0 → success=false → 步骤失败
				errMsg := sample.Tags["error"]
				if errMsg == "" {
					errMsg = sample.Metadata["error"]
				}
				if errMsg != "" {
					o.errorTracker.Record(errMsg, stepID, sample.Time)
				}
			}
		}
	}
}

func (o *Output) getOrCreateTrendSink(stepID string) *metrics.TrendSink {
	if s, ok := o.stepDurations[stepID]; ok {
		return s
	}
	s := &metrics.TrendSink{}
	o.stepDurations[stepID] = s
	return s
}

func (o *Output) getOrCreateCounterSink(stepID string) *metrics.CounterSink {
	if s, ok := o.stepCounts[stepID]; ok {
		return s
	}
	s := &metrics.CounterSink{}
	o.stepCounts[stepID] = s
	return s
}

func (o *Output) getOrCreateRateSink(stepID string) *metrics.RateSink {
	if s, ok := o.stepFailures[stepID]; ok {
		return s
	}
	s := &metrics.RateSink{}
	o.stepFailures[stepID] = s
	return s
}

// RecordVUChange records a VU change event.
func (o *Output) RecordVUChange(vus int, source, reason string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.vuTimeline = append(o.vuTimeline, &types.VUTimelineEvent{
		Timestamp: time.Now(),
		ElapsedMs: time.Since(o.startTime).Milliseconds(),
		VUs:       vus,
		Source:    source,
		Reason:   reason,
	})
}

// GenerateReport builds the final PerformanceTestReport from all collected data.
func (o *Output) GenerateReport(
	metricsEngine *engine.MetricsEngine,
	executionID, workflowID, workflowName, status string,
	totalIterations int64,
	maxVUs int,
) *types.PerformanceTestReport {
	o.mu.Lock()
	defer o.mu.Unlock()

	endTime := time.Now()
	duration := endTime.Sub(o.startTime)
	durationMs := duration.Milliseconds()

	report := &types.PerformanceTestReport{
		ExecutionID:  executionID,
		WorkflowID:   workflowID,
		WorkflowName: workflowName,
		StartTime:    o.startTime,
		EndTime:      endTime,
		Status:       status,
		VUTimeline:   o.vuTimeline,
	}

	// Build step details
	stepDetails := o.buildStepDetails(duration.Seconds())
	report.StepDetails = stepDetails

	// Build summary
	report.Summary = o.buildSummary(stepDetails, durationMs, totalIterations, maxVUs)

	// Time-series data from MetricsEngine
	if metricsEngine != nil {
		tsData := metricsEngine.GetTimeSeriesData()
		report.TimeSeries = make([]*types.ReportTimeSeriesPoint, len(tsData))
		peakQPS := 0.0
		var totalDataSent, totalDataReceived float64
		for i, p := range tsData {
			ts, _ := time.Parse(time.RFC3339, p.Timestamp)
			report.TimeSeries[i] = &types.ReportTimeSeriesPoint{
				Timestamp:          ts,
				ElapsedMs:          p.ElapsedMs,
				QPS:                p.QPS,
				AvgRTMs:            p.AvgRT,
				P50RTMs:            p.P50RT,
				P90RTMs:            p.P90RT,
				P95RTMs:            p.P95RT,
				P99RTMs:            p.P99RT,
				ActiveVUs:          p.ActiveVUs,
				ErrorRate:          p.ErrorRate,
				Iterations:         p.Iterations,
				DataSentPerSec:     p.DataSentPerSec,
				DataReceivedPerSec: p.DataReceivedPerSec,
			}
			if p.QPS > peakQPS {
				peakQPS = p.QPS
			}
			totalDataSent += p.DataSentPerSec
			totalDataReceived += p.DataReceivedPerSec
		}
		report.Summary.PeakQPS = peakQPS
		report.Summary.TotalDataSent = int64(totalDataSent)
		report.Summary.TotalDataReceived = int64(totalDataReceived)
		if durationMs > 0 {
			report.Summary.ThroughputBytesPerSec = (totalDataSent + totalDataReceived) / (float64(durationMs) / 1000)
		}
	}

	// Error analysis
	report.ErrorAnalysis = o.errorTracker.BuildAnalysis()

	// Threshold results
	if metricsEngine != nil {
		report.Thresholds = o.buildThresholdResults(metricsEngine, duration.Seconds())
	}

	return report
}

func (o *Output) buildStepDetails(durationSec float64) []*types.StepDetailReport {
	stepIDs := make([]string, 0)
	seen := make(map[string]bool)
	for id := range o.stepDurations {
		if !seen[id] {
			stepIDs = append(stepIDs, id)
			seen[id] = true
		}
	}
	for id := range o.stepCounts {
		if !seen[id] {
			stepIDs = append(stepIDs, id)
			seen[id] = true
		}
	}
	sort.Strings(stepIDs)

	details := make([]*types.StepDetailReport, 0, len(stepIDs))
	for _, stepID := range stepIDs {
		d := &types.StepDetailReport{StepID: stepID, StepName: o.stepNames[stepID]}

		if sink, ok := o.stepDurations[stepID]; ok {
			stats := sink.Format(durationSec)
			d.AvgMs = stats["avg"]
			d.MinMs = stats["min"]
			d.MaxMs = stats["max"]
			d.P50Ms = stats["med"]
			d.P90Ms = stats["p(90)"]
			d.P95Ms = stats["p(95)"]
			d.P99Ms = stats["p(99)"]
			d.Count = int64(stats["count"])
		}

		if sink, ok := o.stepCounts[stepID]; ok {
			stats := sink.Format(durationSec)
			if c := int64(stats["count"]); c > d.Count {
				d.Count = c
			}
		}

		if sink, ok := o.stepFailures[stepID]; ok {
			stats := sink.Format(0)
			d.SuccessCount = int64(stats["passes"])
			d.FailureCount = int64(stats["fails"])
			// 用 passes + fails 统一 Count，避免不同 Sink 间的 flush 时序差异
			if rateTotal := d.SuccessCount + d.FailureCount; rateTotal > 0 {
				d.Count = rateTotal
			}
			if d.Count > 0 {
				d.ErrorRate = float64(d.FailureCount) / float64(d.Count) * 100
			}
		} else {
			d.SuccessCount = d.Count
		}

		details = append(details, d)
	}
	return details
}

func (o *Output) buildSummary(
	steps []*types.StepDetailReport,
	durationMs, totalIterations int64,
	maxVUs int,
) types.ReportSummary {
	s := types.ReportSummary{
		TotalDurationMs: durationMs,
		TotalIterations: totalIterations,
		MaxVUs:          maxVUs,
	}

	var totalAvgRT, totalMinRT, totalMaxRT float64
	var totalP50, totalP90, totalP95, totalP99 float64
	minSet := false

	for _, step := range steps {
		s.TotalRequests += step.Count
		s.SuccessRequests += step.SuccessCount
		s.FailedRequests += step.FailureCount

		totalAvgRT += step.AvgMs * float64(step.Count)
		if !minSet || step.MinMs < totalMinRT {
			totalMinRT = step.MinMs
			minSet = true
		}
		if step.MaxMs > totalMaxRT {
			totalMaxRT = step.MaxMs
		}
		totalP50 += step.P50Ms * float64(step.Count)
		totalP90 += step.P90Ms * float64(step.Count)
		totalP95 += step.P95Ms * float64(step.Count)
		totalP99 += step.P99Ms * float64(step.Count)
	}

	if s.TotalRequests > 0 {
		s.ErrorRate = float64(s.FailedRequests) / float64(s.TotalRequests) * 100
		s.AvgResponseTimeMs = totalAvgRT / float64(s.TotalRequests)
		s.P50ResponseTimeMs = totalP50 / float64(s.TotalRequests)
		s.P90ResponseTimeMs = totalP90 / float64(s.TotalRequests)
		s.P95ResponseTimeMs = totalP95 / float64(s.TotalRequests)
		s.P99ResponseTimeMs = totalP99 / float64(s.TotalRequests)
	}
	s.MinResponseTimeMs = totalMinRT
	s.MaxResponseTimeMs = totalMaxRT

	if durationMs > 0 {
		s.AvgQPS = float64(s.TotalRequests) / (float64(durationMs) / 1000)
	}

	return s
}

func (o *Output) buildThresholdResults(me *engine.MetricsEngine, durationSec float64) []*types.ReportThresholdResult {
	// Threshold results will be populated by the engine during finalization.
	// For now, return nil if no thresholds are configured.
	return nil
}

// --- Error Tracker ---

type errorTracker struct {
	mu     sync.Mutex
	errors map[string]*errorEntry // message -> entry
}

type errorEntry struct {
	Message   string
	StepID    string
	Count     int64
	FirstSeen time.Time
	LastSeen  time.Time
}

func newErrorTracker() *errorTracker {
	return &errorTracker{errors: make(map[string]*errorEntry)}
}

func (et *errorTracker) Record(message, stepID string, t time.Time) {
	et.mu.Lock()
	defer et.mu.Unlock()

	key := stepID + ":" + message
	if e, ok := et.errors[key]; ok {
		e.Count++
		e.LastSeen = t
	} else {
		et.errors[key] = &errorEntry{
			Message:   message,
			StepID:    stepID,
			Count:     1,
			FirstSeen: t,
			LastSeen:  t,
		}
	}
}

func (et *errorTracker) BuildAnalysis() *types.ReportErrorAnalysis {
	et.mu.Lock()
	defer et.mu.Unlock()

	if len(et.errors) == 0 {
		return &types.ReportErrorAnalysis{}
	}

	var totalErrors int64
	typeCounts := make(map[string]int64)
	entries := make([]*errorEntry, 0, len(et.errors))

	for _, e := range et.errors {
		totalErrors += e.Count
		typeCounts[e.Message] += e.Count
		entries = append(entries, e)
	}

	// Sort by count descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Count > entries[j].Count
	})

	// Top errors (max 20)
	topN := len(entries)
	if topN > 20 {
		topN = 20
	}
	topErrors := make([]*types.ErrorDetail, topN)
	for i := 0; i < topN; i++ {
		topErrors[i] = &types.ErrorDetail{
			Message:   entries[i].Message,
			StepID:    entries[i].StepID,
			Count:     entries[i].Count,
			FirstSeen: entries[i].FirstSeen,
			LastSeen:  entries[i].LastSeen,
		}
	}

	// Error type distribution
	errorTypes := make([]*types.ErrorTypeStats, 0, len(typeCounts))
	for typ, count := range typeCounts {
		errorTypes = append(errorTypes, &types.ErrorTypeStats{
			Type:       typ,
			Count:      count,
			Percentage: float64(count) / float64(totalErrors) * 100,
		})
	}
	sort.Slice(errorTypes, func(i, j int) bool {
		return errorTypes[i].Count > errorTypes[j].Count
	})

	return &types.ReportErrorAnalysis{
		TotalErrors: totalErrors,
		ErrorTypes:  errorTypes,
		TopErrors:   topErrors,
	}
}
