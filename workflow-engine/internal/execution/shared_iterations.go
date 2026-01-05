package execution

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// SharedIterationsMode implements the shared-iterations execution mode.
// Total iterations are distributed across all VUs.
// Requirements: 6.1.6
type SharedIterationsMode struct {
	*BaseMode

	// VU management
	activeVUs  atomic.Int32
	iterations atomic.Int64

	// Synchronization
	wg     sync.WaitGroup
	vuCtxs map[int]context.CancelFunc
	vuMu   sync.Mutex

	// Work queue
	iterCh chan int
}

// NewSharedIterationsMode creates a new shared iterations mode.
func NewSharedIterationsMode() *SharedIterationsMode {
	return &SharedIterationsMode{
		BaseMode: NewBaseMode(types.ModeSharedIterations),
		vuCtxs:   make(map[int]context.CancelFunc),
	}
}

// Run starts the shared iterations execution.
// Requirements: 6.1.6
func (m *SharedIterationsMode) Run(ctx context.Context, config *ModeConfig) error {
	if config == nil {
		return ErrNilConfig
	}

	if config.IterationFunc == nil {
		return ErrNilIterationFunc
	}

	vus := config.VUs
	if vus <= 0 {
		vus = 1
	}

	totalIterations := config.Iterations
	if totalIterations <= 0 {
		totalIterations = 1
	}

	// Initialize state
	m.SetState(func(s *ModeState) {
		s.Running = true
		s.TargetVUs = vus
		s.StartTime = time.Now()
	})

	defer func() {
		m.SetState(func(s *ModeState) {
			s.Running = false
			s.ElapsedTime = time.Since(s.StartTime)
		})
		m.SignalDone()
	}()

	// Create iteration channel
	m.iterCh = make(chan int, totalIterations)
	for i := 0; i < totalIterations; i++ {
		m.iterCh <- i
	}
	close(m.iterCh)

	// Create execution context with timeout if duration is specified
	execCtx := ctx
	var cancel context.CancelFunc
	if config.Duration > 0 {
		execCtx, cancel = context.WithTimeout(ctx, config.Duration)
		defer cancel()
	}

	// Start all VUs
	for i := 0; i < vus; i++ {
		m.startVU(execCtx, i, config)
	}

	// Wait for completion or cancellation
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-execCtx.Done():
		// Context cancelled or timed out
		m.stopAllVUs()
		m.wg.Wait()
	case <-m.stopCh:
		// Stop requested
		m.stopAllVUs()
		m.wg.Wait()
	case <-done:
		// All iterations completed
	}

	return nil
}

// Stop gracefully stops the execution.
func (m *SharedIterationsMode) Stop(ctx context.Context) error {
	m.RequestStop()
	return m.WaitDone(ctx)
}

// startVU starts a single VU.
func (m *SharedIterationsMode) startVU(ctx context.Context, vuID int, config *ModeConfig) {
	vuCtx, vuCancel := context.WithCancel(ctx)

	m.vuMu.Lock()
	m.vuCtxs[vuID] = vuCancel
	m.vuMu.Unlock()

	m.wg.Add(1)
	m.activeVUs.Add(1)

	m.SetState(func(s *ModeState) {
		s.ActiveVUs = int(m.activeVUs.Load())
	})

	if config.OnVUStart != nil {
		config.OnVUStart(vuID)
	}

	go m.runVU(vuCtx, vuID, config)
}

// runVU runs a single VU, consuming iterations from the shared queue.
func (m *SharedIterationsMode) runVU(ctx context.Context, vuID int, config *ModeConfig) {
	defer func() {
		m.wg.Done()
		m.activeVUs.Add(-1)

		m.SetState(func(s *ModeState) {
			s.ActiveVUs = int(m.activeVUs.Load())
		})

		m.vuMu.Lock()
		delete(m.vuCtxs, vuID)
		m.vuMu.Unlock()

		if config.OnVUStop != nil {
			config.OnVUStop(vuID)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case iteration, ok := <-m.iterCh:
			if !ok {
				// Channel closed, no more iterations
				return
			}

			// Execute iteration
			start := time.Now()
			err := config.IterationFunc(ctx, vuID, iteration)
			duration := time.Since(start)

			// Update state
			m.iterations.Add(1)
			m.SetState(func(s *ModeState) {
				s.CompletedIterations = m.iterations.Load()
			})

			// Callback
			if config.OnIterationComplete != nil {
				config.OnIterationComplete(vuID, iteration, duration, err)
			}
		}
	}
}

// stopAllVUs stops all running VUs.
func (m *SharedIterationsMode) stopAllVUs() {
	m.vuMu.Lock()
	defer m.vuMu.Unlock()

	for _, cancel := range m.vuCtxs {
		cancel()
	}
}

// GetActiveVUs returns the current number of active VUs.
func (m *SharedIterationsMode) GetActiveVUs() int {
	return int(m.activeVUs.Load())
}

// GetCompletedIterations returns the number of completed iterations.
func (m *SharedIterationsMode) GetCompletedIterations() int64 {
	return m.iterations.Load()
}
