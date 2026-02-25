package engine

import (
	"time"

	"yqhp/workflow-engine/pkg/controlsurface"
	"yqhp/workflow-engine/pkg/output"
)

const collectRate = 50 * time.Millisecond

// Compile-time check: OutputIngester implements output.Output.
var _ output.Output = &OutputIngester{}

// OutputIngester implements output.Output and feeds metric samples
// into the MetricsEngine. This is the key design from k6: the engine's
// ingester sits in the same output pipeline as InfluxDB, Kafka, etc.
type OutputIngester struct {
	output.SampleBuffer
	metricsEngine   *MetricsEngine
	periodicFlusher *output.PeriodicFlusher
}

func (oi *OutputIngester) Description() string {
	return "Internal Metrics Engine Ingester"
}

func (oi *OutputIngester) Start() error {
	pf, err := output.NewPeriodicFlusher(collectRate, oi.flushMetrics)
	if err != nil {
		return err
	}
	oi.periodicFlusher = pf
	return nil
}

func (oi *OutputIngester) Stop() error {
	if oi.periodicFlusher != nil {
		oi.periodicFlusher.Stop()
	}
	return nil
}

func (oi *OutputIngester) SetRunStatus(_ output.RunStatus) {}

// flushMetrics processes buffered samples and updates the MetricsEngine.
func (oi *OutputIngester) flushMetrics() {
	containers := oi.GetBufferedSamples()
	if len(containers) == 0 {
		return
	}

	oi.metricsEngine.MetricsLock.Lock()
	defer oi.metricsEngine.MetricsLock.Unlock()

	for _, container := range containers {
		for _, sample := range container.GetSamples() {
			m := sample.Metric
			oi.metricsEngine.MarkObserved(m)
			m.Sink.Add(sample)

			if sid := sample.Tags["step_id"]; sid != "" {
				if sn := sample.Tags["step_name"]; sn != "" {
					oi.metricsEngine.StepNames[sid] = sn
				}
			}
		}
	}
}

// Type aliases from controlsurface package.
type RealtimeMetrics = controlsurface.RealtimeMetrics
type StepMetricStats = controlsurface.StepMetricStats

// BuildRealtimeMetrics constructs a RealtimeMetrics snapshot from the engine state.
func (me *MetricsEngine) BuildRealtimeMetrics(
	status string,
	getVUs func() int64,
	getIterations func() int64,
	getErrors func() []string,
) *RealtimeMetrics {
	me.MetricsLock.Lock()
	defer me.MetricsLock.Unlock()

	elapsed := time.Since(me.startTime)
	rm := &RealtimeMetrics{
		Status:          status,
		ElapsedMs:       elapsed.Milliseconds(),
		TotalVUs:        getVUs(),
		TotalIterations: getIterations(),
		Errors:          getErrors(),
	}

	latest := me.GetLatestSnapshot()
	if latest != nil {
		rm.QPS = latest.QPS
		rm.ErrorRate = latest.ErrorRate
	}

	rm.StepMetrics = me.buildStepMetrics(elapsed.Seconds())

	return rm
}

// buildStepMetrics extracts per-step metrics from observed metrics.
// Metrics are expected to have tags like {"step_id": "xxx"}.
func (me *MetricsEngine) buildStepMetrics(durationSec float64) map[string]*StepMetricStats {
	result := make(map[string]*StepMetricStats)

	for name, m := range me.ObservedMetrics {
		if m.Sink == nil || m.Sink.IsEmpty() {
			continue
		}

		stats := m.Sink.Format(durationSec)

		switch {
		case len(name) > 14 && name[:14] == "step_duration_":
			stepID := name[14:]
			s := me.getOrCreateStepStats(result, stepID)
			s.AvgMs = stats["avg"]
			s.MinMs = stats["min"]
			s.MaxMs = stats["max"]
			s.P50Ms = stats["med"]
			s.P90Ms = stats["p(90)"]
			s.P95Ms = stats["p(95)"]
			s.P99Ms = stats["p(99)"]
			s.Count = int64(stats["count"])
		case len(name) > 10 && name[:10] == "step_reqs_":
			stepID := name[10:]
			s := me.getOrCreateStepStats(result, stepID)
			s.Count = int64(stats["count"])
		case len(name) > 12 && name[:12] == "step_failed_":
			stepID := name[12:]
			s := me.getOrCreateStepStats(result, stepID)
			s.FailureCount = int64(stats["passes"])
			s.SuccessCount = int64(stats["fails"])
		}
	}

	return result
}

func (me *MetricsEngine) getOrCreateStepStats(m map[string]*StepMetricStats, stepID string) *StepMetricStats {
	if s, ok := m[stepID]; ok {
		return s
	}
	s := &StepMetricStats{StepID: stepID, StepName: me.StepNames[stepID]}
	m[stepID] = s
	return s
}
