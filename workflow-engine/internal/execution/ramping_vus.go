package execution

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// RampingVUsMode implements the ramping-vus execution mode.
// It adjusts VU count according to defined stages.
// Requirements: 6.1.2
type RampingVUsMode struct {
	*BaseMode

	// VU management
	activeVUs  atomic.Int32
	iterations atomic.Int64

	// Synchronization
	wg     sync.WaitGroup
	vuCtxs map[int]context.CancelFunc
	vuMu   sync.Mutex

	// Stage tracking
	currentStage int
}

// NewRampingVUsMode creates a new ramping VUs mode.
func NewRampingVUsMode() *RampingVUsMode {
	return &RampingVUsMode{
		BaseMode: NewBaseMode(types.ModeRampingVUs),
		vuCtxs:   make(map[int]context.CancelFunc),
	}
}

// Run starts the ramping VUs execution.
// Requirements: 6.1.2
func (m *RampingVUsMode) Run(ctx context.Context, config *ModeConfig) error {
	if config == nil {
		return ErrNilConfig
	}

	if config.IterationFunc == nil {
		return ErrNilIterationFunc
	}

	if len(config.Stages) == 0 {
		return ErrNoStages
	}

	// Initialize state
	m.SetState(func(s *ModeState) {
		s.Running = true
		s.StartTime = time.Now()
	})

	defer func() {
		m.SetState(func(s *ModeState) {
			s.Running = false
			s.ElapsedTime = time.Since(s.StartTime)
		})
		m.SignalDone()
	}()

	// Execute stages
	currentVUs := 0
	for stageIdx, stage := range config.Stages {
		select {
		case <-ctx.Done():
			m.stopAllVUs()
			m.wg.Wait()
			return nil
		case <-m.stopCh:
			m.stopAllVUs()
			m.wg.Wait()
			return nil
		default:
		}

		m.currentStage = stageIdx
		targetVUs := stage.Target
		stageDuration := stage.Duration

		if stageDuration <= 0 {
			stageDuration = time.Second
		}

		// Execute the stage with ramping
		err := m.executeStage(ctx, config, currentVUs, targetVUs, stageDuration)
		if err != nil {
			return err
		}

		currentVUs = targetVUs
	}

	// Wait for all VUs to complete
	m.stopAllVUs()
	m.wg.Wait()

	return nil
}

// executeStage executes a single stage with ramping from startVUs to targetVUs.
func (m *RampingVUsMode) executeStage(ctx context.Context, config *ModeConfig, startVUs, targetVUs int, duration time.Duration) error {
	stageStart := time.Now()
	ticker := time.NewTicker(50 * time.Millisecond) // Adjust VUs every 50ms
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-m.stopCh:
			return nil
		case <-ticker.C:
			elapsed := time.Since(stageStart)
			if elapsed >= duration {
				// Ensure we reach the target at the end of the stage
				m.adjustVUs(ctx, config, targetVUs)
				return nil
			}

			// Calculate target VUs at this point in time
			progress := float64(elapsed) / float64(duration)
			targetAtPoint := startVUs + int(float64(targetVUs-startVUs)*progress)

			m.adjustVUs(ctx, config, targetAtPoint)
		}
	}
}

// adjustVUs adjusts the number of VUs to the target.
func (m *RampingVUsMode) adjustVUs(ctx context.Context, config *ModeConfig, targetVUs int) {
	m.vuMu.Lock()
	defer m.vuMu.Unlock()

	currentVUs := len(m.vuCtxs)

	// Update state
	m.SetState(func(s *ModeState) {
		s.TargetVUs = targetVUs
		s.ActiveVUs = currentVUs
	})

	// Scale up
	for i := currentVUs; i < targetVUs; i++ {
		m.startVULocked(ctx, i, config)
	}

	// Scale down
	for i := currentVUs - 1; i >= targetVUs; i-- {
		if cancel, ok := m.vuCtxs[i]; ok {
			cancel()
			delete(m.vuCtxs, i)
		}
	}
}

// startVULocked starts a VU (must be called with vuMu held).
func (m *RampingVUsMode) startVULocked(ctx context.Context, vuID int, config *ModeConfig) {
	vuCtx, vuCancel := context.WithCancel(ctx)
	m.vuCtxs[vuID] = vuCancel

	m.wg.Add(1)
	m.activeVUs.Add(1)

	if config.OnVUStart != nil {
		config.OnVUStart(vuID)
	}

	go m.runVU(vuCtx, vuID, config)
}

// runVU runs a single VU until context is cancelled.
func (m *RampingVUsMode) runVU(ctx context.Context, vuID int, config *ModeConfig) {
	defer func() {
		m.wg.Done()
		m.activeVUs.Add(-1)

		m.SetState(func(s *ModeState) {
			s.ActiveVUs = int(m.activeVUs.Load())
		})

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

			iteration++
		}
	}
}

// stopAllVUs stops all running VUs.
func (m *RampingVUsMode) stopAllVUs() {
	m.vuMu.Lock()
	defer m.vuMu.Unlock()

	for _, cancel := range m.vuCtxs {
		cancel()
	}
	m.vuCtxs = make(map[int]context.CancelFunc)
}

// Stop gracefully stops the execution.
func (m *RampingVUsMode) Stop(ctx context.Context) error {
	m.RequestStop()
	return m.WaitDone(ctx)
}

// GetActiveVUs returns the current number of active VUs.
func (m *RampingVUsMode) GetActiveVUs() int {
	return int(m.activeVUs.Load())
}

// GetCompletedIterations returns the number of completed iterations.
func (m *RampingVUsMode) GetCompletedIterations() int64 {
	return m.iterations.Load()
}

// GetCurrentStage returns the current stage index.
func (m *RampingVUsMode) GetCurrentStage() int {
	return m.currentStage
}
