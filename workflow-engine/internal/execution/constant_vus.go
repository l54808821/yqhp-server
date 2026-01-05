package execution

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// ConstantVUsMode implements the constant-vus execution mode.
// It maintains a fixed number of VUs throughout the test duration.
// Requirements: 6.1.1
type ConstantVUsMode struct {
	*BaseMode

	// VU management
	activeVUs  atomic.Int32
	iterations atomic.Int64

	// Synchronization
	wg     sync.WaitGroup
	vuCtxs map[int]context.CancelFunc
	vuMu   sync.Mutex
}

// NewConstantVUsMode creates a new constant VUs mode.
func NewConstantVUsMode() *ConstantVUsMode {
	return &ConstantVUsMode{
		BaseMode: NewBaseMode(types.ModeConstantVUs),
		vuCtxs:   make(map[int]context.CancelFunc),
	}
}

// Run starts the constant VUs execution.
// Requirements: 6.1.1
func (m *ConstantVUsMode) Run(ctx context.Context, config *ModeConfig) error {
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
		// All VUs completed (iteration-based)
	}

	return nil
}

// Stop gracefully stops the execution.
func (m *ConstantVUsMode) Stop(ctx context.Context) error {
	m.RequestStop()
	return m.WaitDone(ctx)
}

// startVU starts a single VU.
func (m *ConstantVUsMode) startVU(ctx context.Context, vuID int, config *ModeConfig) {
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

// runVU runs a single VU until context is cancelled or iterations complete.
func (m *ConstantVUsMode) runVU(ctx context.Context, vuID int, config *ModeConfig) {
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

	iteration := 0
	maxIterations := config.Iterations

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		default:
			// Check if we've reached max iterations (if specified)
			if maxIterations > 0 && iteration >= maxIterations {
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

			iteration++
		}
	}
}

// stopAllVUs stops all running VUs.
func (m *ConstantVUsMode) stopAllVUs() {
	m.vuMu.Lock()
	defer m.vuMu.Unlock()

	for _, cancel := range m.vuCtxs {
		cancel()
	}
}

// GetActiveVUs returns the current number of active VUs.
func (m *ConstantVUsMode) GetActiveVUs() int {
	return int(m.activeVUs.Load())
}

// GetCompletedIterations returns the number of completed iterations.
func (m *ConstantVUsMode) GetCompletedIterations() int64 {
	return m.iterations.Load()
}
