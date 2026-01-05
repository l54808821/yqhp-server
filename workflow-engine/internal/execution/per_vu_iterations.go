package execution

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// PerVUIterationsMode implements the per-vu-iterations execution mode.
// Each VU executes a fixed number of iterations.
// Requirements: 6.1.5
type PerVUIterationsMode struct {
	*BaseMode

	// VU management
	activeVUs  atomic.Int32
	iterations atomic.Int64

	// Synchronization
	wg     sync.WaitGroup
	vuCtxs map[int]context.CancelFunc
	vuMu   sync.Mutex
}

// NewPerVUIterationsMode creates a new per-VU iterations mode.
func NewPerVUIterationsMode() *PerVUIterationsMode {
	return &PerVUIterationsMode{
		BaseMode: NewBaseMode(types.ModePerVUIterations),
		vuCtxs:   make(map[int]context.CancelFunc),
	}
}

// Run starts the per-VU iterations execution.
// Requirements: 6.1.5
func (m *PerVUIterationsMode) Run(ctx context.Context, config *ModeConfig) error {
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

	iterationsPerVU := config.Iterations
	if iterationsPerVU <= 0 {
		iterationsPerVU = 1
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

	// Create execution context with timeout if duration is specified
	execCtx := ctx
	var cancel context.CancelFunc
	if config.Duration > 0 {
		execCtx, cancel = context.WithTimeout(ctx, config.Duration)
		defer cancel()
	}

	// Start all VUs
	for i := 0; i < vus; i++ {
		m.startVU(execCtx, i, config, iterationsPerVU)
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
		// All VUs completed their iterations
	}

	return nil
}

// Stop gracefully stops the execution.
func (m *PerVUIterationsMode) Stop(ctx context.Context) error {
	m.RequestStop()
	return m.WaitDone(ctx)
}

// startVU starts a single VU.
func (m *PerVUIterationsMode) startVU(ctx context.Context, vuID int, config *ModeConfig, maxIterations int) {
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

	go m.runVU(vuCtx, vuID, config, maxIterations)
}

// runVU runs a single VU for the specified number of iterations.
func (m *PerVUIterationsMode) runVU(ctx context.Context, vuID int, config *ModeConfig, maxIterations int) {
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

	for iteration := 0; iteration < maxIterations; iteration++ {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		default:
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
func (m *PerVUIterationsMode) stopAllVUs() {
	m.vuMu.Lock()
	defer m.vuMu.Unlock()

	for _, cancel := range m.vuCtxs {
		cancel()
	}
}

// GetActiveVUs returns the current number of active VUs.
func (m *PerVUIterationsMode) GetActiveVUs() int {
	return int(m.activeVUs.Load())
}

// GetCompletedIterations returns the number of completed iterations.
func (m *PerVUIterationsMode) GetCompletedIterations() int64 {
	return m.iterations.Load()
}
