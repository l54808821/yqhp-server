package execution

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// ExternallyControlledMode implements the externally-controlled execution mode.
// It allows runtime control via API for VU scaling, pausing, and resuming.
// Requirements: 6.1.7, 6.2.1, 6.2.2, 6.2.3
type ExternallyControlledMode struct {
	*BaseMode

	// VU management
	activeVUs  atomic.Int32
	targetVUs  atomic.Int32
	iterations atomic.Int64

	// Control state
	paused atomic.Bool

	// Synchronization
	wg     sync.WaitGroup
	vuCtxs map[int]context.CancelFunc
	vuMu   sync.Mutex

	// Control channels
	scaleCh  chan int
	pauseCh  chan struct{}
	resumeCh chan struct{}

	// Configuration
	config *ModeConfig
	maxVUs int
}

// NewExternallyControlledMode creates a new externally controlled mode.
func NewExternallyControlledMode() *ExternallyControlledMode {
	return &ExternallyControlledMode{
		BaseMode: NewBaseMode(types.ModeExternally),
		vuCtxs:   make(map[int]context.CancelFunc),
		scaleCh:  make(chan int, 10),
		pauseCh:  make(chan struct{}, 1),
		resumeCh: make(chan struct{}, 1),
	}
}

// Run starts the externally controlled execution.
// Requirements: 6.1.7
func (m *ExternallyControlledMode) Run(ctx context.Context, config *ModeConfig) error {
	if config == nil {
		return ErrNilConfig
	}

	if config.IterationFunc == nil {
		return ErrNilIterationFunc
	}

	m.config = config

	initialVUs := config.VUs
	if initialVUs <= 0 {
		initialVUs = 1
	}

	m.maxVUs = config.MaxVUs
	if m.maxVUs <= 0 {
		m.maxVUs = 100 // Default max
	}

	m.targetVUs.Store(int32(initialVUs))

	// Initialize state
	m.SetState(func(s *ModeState) {
		s.Running = true
		s.TargetVUs = initialVUs
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

	// Start initial VUs
	for i := 0; i < initialVUs; i++ {
		m.startVU(execCtx, i)
	}

	// Run control loop
	m.runControlLoop(execCtx)

	// Wait for all VUs to complete
	m.stopAllVUs()
	m.wg.Wait()

	return nil
}

// runControlLoop handles external control commands.
func (m *ExternallyControlledMode) runControlLoop(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case target := <-m.scaleCh:
			m.scaleToTarget(ctx, target)
		case <-m.pauseCh:
			m.paused.Store(true)
			m.SetState(func(s *ModeState) {
				s.Paused = true
			})
		case <-m.resumeCh:
			m.paused.Store(false)
			m.SetState(func(s *ModeState) {
				s.Paused = false
			})
		case <-ticker.C:
			// Periodic check to ensure VU count matches target
			target := int(m.targetVUs.Load())
			m.vuMu.Lock()
			current := len(m.vuCtxs)
			m.vuMu.Unlock()

			if current != target && !m.paused.Load() {
				m.scaleToTarget(ctx, target)
			}
		}
	}
}

// scaleToTarget scales VUs to the target count.
func (m *ExternallyControlledMode) scaleToTarget(ctx context.Context, target int) {
	if target < 0 {
		target = 0
	}
	if target > m.maxVUs {
		target = m.maxVUs
	}

	m.targetVUs.Store(int32(target))
	m.SetState(func(s *ModeState) {
		s.TargetVUs = target
	})

	m.vuMu.Lock()
	defer m.vuMu.Unlock()

	current := len(m.vuCtxs)

	// Scale up
	for i := current; i < target; i++ {
		m.startVULocked(ctx, i)
	}

	// Scale down (gracefully)
	for i := current - 1; i >= target; i-- {
		if cancel, ok := m.vuCtxs[i]; ok {
			cancel()
			delete(m.vuCtxs, i)
		}
	}
}

// startVU starts a single VU.
func (m *ExternallyControlledMode) startVU(ctx context.Context, vuID int) {
	m.vuMu.Lock()
	defer m.vuMu.Unlock()
	m.startVULocked(ctx, vuID)
}

// startVULocked starts a VU (must be called with vuMu held).
func (m *ExternallyControlledMode) startVULocked(ctx context.Context, vuID int) {
	if vuID >= m.maxVUs {
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

	if m.config.OnVUStart != nil {
		m.config.OnVUStart(vuID)
	}

	go m.runVU(vuCtx, vuID)
}

// runVU runs a single VU until context is cancelled.
func (m *ExternallyControlledMode) runVU(ctx context.Context, vuID int) {
	defer func() {
		m.wg.Done()
		m.activeVUs.Add(-1)

		m.SetState(func(s *ModeState) {
			s.ActiveVUs = int(m.activeVUs.Load())
		})

		m.vuMu.Lock()
		delete(m.vuCtxs, vuID)
		m.vuMu.Unlock()

		if m.config.OnVUStop != nil {
			m.config.OnVUStop(vuID)
		}
	}()

	iteration := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		default:
			// Check if paused
			if m.paused.Load() {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			// Execute iteration
			start := time.Now()
			err := m.config.IterationFunc(ctx, vuID, iteration)
			duration := time.Since(start)

			// Update state
			m.iterations.Add(1)
			m.SetState(func(s *ModeState) {
				s.CompletedIterations = m.iterations.Load()
			})

			// Callback
			if m.config.OnIterationComplete != nil {
				m.config.OnIterationComplete(vuID, iteration, duration, err)
			}

			iteration++
		}
	}
}

// stopAllVUs stops all running VUs.
func (m *ExternallyControlledMode) stopAllVUs() {
	m.vuMu.Lock()
	defer m.vuMu.Unlock()

	for _, cancel := range m.vuCtxs {
		cancel()
	}
}

// Stop gracefully stops the execution.
func (m *ExternallyControlledMode) Stop(ctx context.Context) error {
	m.RequestStop()
	return m.WaitDone(ctx)
}

// Scale adjusts the VU count to the specified target.
// Requirements: 6.2.1, 6.2.2, 6.2.3
func (m *ExternallyControlledMode) Scale(target int) error {
	state := m.GetState()
	if !state.Running {
		return ErrModeNotRunning
	}

	select {
	case m.scaleCh <- target:
		return nil
	default:
		// Channel full, try to replace
		select {
		case <-m.scaleCh:
		default:
		}
		m.scaleCh <- target
		return nil
	}
}

// Pause pauses the execution (stops starting new iterations).
// Requirements: 6.2.5
func (m *ExternallyControlledMode) Pause() error {
	state := m.GetState()
	if !state.Running {
		return ErrModeNotRunning
	}

	select {
	case m.pauseCh <- struct{}{}:
	default:
		// Already paused
	}
	return nil
}

// Resume resumes the execution.
// Requirements: 6.2.6
func (m *ExternallyControlledMode) Resume() error {
	state := m.GetState()
	if !state.Running {
		return ErrModeNotRunning
	}

	select {
	case m.resumeCh <- struct{}{}:
	default:
		// Already running
	}
	return nil
}

// IsPaused returns whether the execution is paused.
func (m *ExternallyControlledMode) IsPaused() bool {
	return m.paused.Load()
}

// GetActiveVUs returns the current number of active VUs.
func (m *ExternallyControlledMode) GetActiveVUs() int {
	return int(m.activeVUs.Load())
}

// GetTargetVUs returns the target number of VUs.
func (m *ExternallyControlledMode) GetTargetVUs() int {
	return int(m.targetVUs.Load())
}

// GetCompletedIterations returns the number of completed iterations.
func (m *ExternallyControlledMode) GetCompletedIterations() int64 {
	return m.iterations.Load()
}
