package execution

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// ConstantArrivalRateMode implements the constant-arrival-rate execution mode.
// It maintains a fixed request rate regardless of response time.
// Requirements: 6.1.3
type ConstantArrivalRateMode struct {
	*BaseMode

	// VU management
	activeVUs  atomic.Int32
	iterations atomic.Int64

	// Rate tracking
	currentRate atomic.Int64

	// Synchronization
	wg     sync.WaitGroup
	vuCtxs map[int]context.CancelFunc
	vuMu   sync.Mutex

	// Work queue
	workCh chan struct{}
}

// NewConstantArrivalRateMode creates a new constant arrival rate mode.
func NewConstantArrivalRateMode() *ConstantArrivalRateMode {
	return &ConstantArrivalRateMode{
		BaseMode: NewBaseMode(types.ModeConstantArrivalRate),
		vuCtxs:   make(map[int]context.CancelFunc),
	}
}

// Run starts the constant arrival rate execution.
// Requirements: 6.1.3
func (m *ConstantArrivalRateMode) Run(ctx context.Context, config *ModeConfig) error {
	if config == nil {
		return ErrNilConfig
	}

	if config.IterationFunc == nil {
		return ErrNilIterationFunc
	}

	if config.Rate <= 0 {
		return ErrInvalidRate
	}

	timeUnit := config.TimeUnit
	if timeUnit <= 0 {
		timeUnit = time.Second
	}

	preAllocatedVUs := config.PreAllocatedVUs
	if preAllocatedVUs <= 0 {
		preAllocatedVUs = 1
	}

	maxVUs := config.MaxVUs
	if maxVUs <= 0 {
		maxVUs = preAllocatedVUs * 2
	}

	// Initialize state
	m.SetState(func(s *ModeState) {
		s.Running = true
		s.StartTime = time.Now()
		s.CurrentRate = float64(config.Rate)
	})

	defer func() {
		m.SetState(func(s *ModeState) {
			s.Running = false
			s.ElapsedTime = time.Since(s.StartTime)
		})
		m.SignalDone()
	}()

	// Create work channel with buffer
	m.workCh = make(chan struct{}, maxVUs*2)

	// Create execution context with timeout if duration is specified
	execCtx := ctx
	var cancel context.CancelFunc
	if config.Duration > 0 {
		execCtx, cancel = context.WithTimeout(ctx, config.Duration)
		defer cancel()
	}

	// Start pre-allocated VUs
	for i := 0; i < preAllocatedVUs; i++ {
		m.startVU(execCtx, i, config, maxVUs)
	}

	// Start rate generator
	m.runRateGenerator(execCtx, config.Rate, timeUnit, maxVUs, config)

	// Wait for all VUs to complete
	m.stopAllVUs()
	m.wg.Wait()

	return nil
}

// runRateGenerator generates work at the specified rate.
func (m *ConstantArrivalRateMode) runRateGenerator(ctx context.Context, rate int, timeUnit time.Duration, maxVUs int, config *ModeConfig) {
	// Calculate interval between iterations
	interval := timeUnit / time.Duration(rate)
	if interval < time.Millisecond {
		interval = time.Millisecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			// Try to send work
			select {
			case m.workCh <- struct{}{}:
				// Work queued
			default:
				// Queue full, try to spawn more VUs
				m.vuMu.Lock()
				currentVUs := len(m.vuCtxs)
				if currentVUs < maxVUs {
					m.startVULocked(ctx, currentVUs, config, maxVUs)
				}
				m.vuMu.Unlock()

				// Try again
				select {
				case m.workCh <- struct{}{}:
				default:
					// Still full, drop this iteration
				}
			}
		}
	}
}

// startVU starts a single VU.
func (m *ConstantArrivalRateMode) startVU(ctx context.Context, vuID int, config *ModeConfig, maxVUs int) {
	m.vuMu.Lock()
	defer m.vuMu.Unlock()
	m.startVULocked(ctx, vuID, config, maxVUs)
}

// startVULocked starts a VU (must be called with vuMu held).
func (m *ConstantArrivalRateMode) startVULocked(ctx context.Context, vuID int, config *ModeConfig, maxVUs int) {
	if vuID >= maxVUs {
		return
	}

	if _, exists := m.vuCtxs[vuID]; exists {
		return
	}

	vuCtx, vuCancel := context.WithCancel(ctx)
	m.vuCtxs[vuID] = vuCancel

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

// runVU runs a single VU, processing work from the queue.
func (m *ConstantArrivalRateMode) runVU(ctx context.Context, vuID int, config *ModeConfig) {
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
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-m.workCh:
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
func (m *ConstantArrivalRateMode) stopAllVUs() {
	m.vuMu.Lock()
	defer m.vuMu.Unlock()

	for _, cancel := range m.vuCtxs {
		cancel()
	}
}

// Stop gracefully stops the execution.
func (m *ConstantArrivalRateMode) Stop(ctx context.Context) error {
	m.RequestStop()
	return m.WaitDone(ctx)
}

// GetActiveVUs returns the current number of active VUs.
func (m *ConstantArrivalRateMode) GetActiveVUs() int {
	return int(m.activeVUs.Load())
}

// GetCompletedIterations returns the number of completed iterations.
func (m *ConstantArrivalRateMode) GetCompletedIterations() int64 {
	return m.iterations.Load()
}
