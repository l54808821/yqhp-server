package output

import (
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/metrics"
)

// SampleBuffer is a simple thread-safe buffer for metric samples.
// Outputs can embed this to simplify buffering and periodic flushing.
type SampleBuffer struct {
	mu      sync.Mutex
	samples []metrics.SampleContainer
}

// AddMetricSamples buffers the given sample containers.
func (sb *SampleBuffer) AddMetricSamples(samples []metrics.SampleContainer) {
	sb.mu.Lock()
	sb.samples = append(sb.samples, samples...)
	sb.mu.Unlock()
}

// GetBufferedSamples returns all buffered samples and resets the buffer.
func (sb *SampleBuffer) GetBufferedSamples() []metrics.SampleContainer {
	sb.mu.Lock()
	samples := sb.samples
	sb.samples = nil
	sb.mu.Unlock()
	return samples
}

// PeriodicFlusher calls a flush function at a regular interval.
type PeriodicFlusher struct {
	stop chan struct{}
	done chan struct{}
}

// NewPeriodicFlusher creates and starts a PeriodicFlusher that calls flushFunc
// at the given interval. A final flush is performed on Stop().
func NewPeriodicFlusher(interval time.Duration, flushFunc func()) (*PeriodicFlusher, error) {
	pf := &PeriodicFlusher{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}

	go func() {
		defer close(pf.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				flushFunc()
			case <-pf.stop:
				flushFunc()
				return
			}
		}
	}()

	return pf, nil
}

// Stop signals the flusher to stop and waits for the final flush.
func (pf *PeriodicFlusher) Stop() {
	close(pf.stop)
	<-pf.done
}
