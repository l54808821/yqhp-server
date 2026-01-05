package execution

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// RampingArrivalRateMode implements the ramping-arrival-rate execution mode.
// It adjusts request rate according to defined stages.
// Requirements: 6.1.4
type RampingArrivalRateMode struct {
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

	// Stage tracking
	currentStage int
}

// NewRampingArrivalRateMode creates a new ramping arrival rate mode.
func NewRampingArrivalRateMode() *RampingArrivalRateMode {
	return &RampingArrivalRateMode{
		BaseMode: NewBaseMode(types.ModeRampingArrivalRate),
		vuCtxs:   make(map[int]context.CancelFunc),
	}
}

// Run starts the ramping arrival rate execution.
// Requirements: 6.1.4
func (m *RampingArrivalRateMode) Run(ctx context.Context, config *ModeConfig) error {
	if config == nil {
		return ErrNilConfig
	}

	if config.IterationFunc == nil {
		return ErrNilIterationFunc
	}

	if len(config.Stages) == 0 {
		return ErrNoStages
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

	// Start pre-allocated VUs
	for i := 0; i < preAllocatedVUs; i++ {
		m.startVU(ctx, i, config, maxVUs)
	}

	// Execute stages
	currentRate := 0
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
		targetRate := stage.Target
		stageDuration := stage.Duration

		if stageDuration <= 0 {
			stageDuration = time.Second
		}

		// Execute the stage with ramping rate
		m.executeStage(ctx, config, currentRate, targetRate, stageDuration, timeUnit, maxVUs)

		currentRate = targetRate
	}

	// Wait for all VUs to complete
	m.stopAllVUs()
	m.wg.Wait()

	return nil
}

// executeStage executes a single stage with ramping rate.
func (m *RampingArrivalRateMode) executeStage(ctx context.Context, config *ModeConfig, startRate, targetRate int, duration, timeUnit time.Duration, maxVUs int) {
	stageStart := time.Now()

	// Use a ticker for rate adjustment
	adjustTicker := time.NewTicker(50 * time.Millisecond)
	defer adjustTicker.Stop()

	// Rate generator goroutine
	rateDone := make(chan struct{})
	var rateWg sync.WaitGroup
	rateWg.Add(1)

	go func() {
		defer rateWg.Done()
		defer close(rateDone)

		lastTick := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stopCh:
				return
			case <-adjustTicker.C:
				elapsed := time.Since(stageStart)
				if elapsed >= duration {
					return
				}

				// Calculate current rate based on progress
				progress := float64(elapsed) / float64(duration)
				currentRate := startRate + int(float64(targetRate-startRate)*progress)

				if currentRate <= 0 {
					continue
				}

				m.currentRate.Store(int64(currentRate))
				m.SetState(func(s *ModeState) {
					s.CurrentRate = float64(currentRate)
				})

				// Calculate how many iterations to generate since last tick
				tickDuration := time.Since(lastTick)
				lastTick = time.Now()

				iterationsToGenerate := float64(currentRate) * float64(tickDuration) / float64(timeUnit)

				for i := 0; i < int(iterationsToGenerate); i++ {
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
	}()

	// Wait for stage to complete
	<-rateDone
	rateWg.Wait()
}

// startVU starts a single VU.
func (m *RampingArrivalRateMode) startVU(ctx context.Context, vuID int, config *ModeConfig, maxVUs int) {
	m.vuMu.Lock()
	defer m.vuMu.Unlock()
	m.startVULocked(ctx, vuID, config, maxVUs)
}

// startVULocked starts a VU (must be called with vuMu held).
func (m *RampingArrivalRateMode) startVULocked(ctx context.Context, vuID int, config *ModeConfig, maxVUs int) {
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
func (m *RampingArrivalRateMode) runVU(ctx context.Context, vuID int, config *ModeConfig) {
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
func (m *RampingArrivalRateMode) stopAllVUs() {
	m.vuMu.Lock()
	defer m.vuMu.Unlock()

	for _, cancel := range m.vuCtxs {
		cancel()
	}
}

// Stop gracefully stops the execution.
func (m *RampingArrivalRateMode) Stop(ctx context.Context) error {
	m.RequestStop()
	return m.WaitDone(ctx)
}

// GetActiveVUs returns the current number of active VUs.
func (m *RampingArrivalRateMode) GetActiveVUs() int {
	return int(m.activeVUs.Load())
}

// GetCompletedIterations returns the number of completed iterations.
func (m *RampingArrivalRateMode) GetCompletedIterations() int64 {
	return m.iterations.Load()
}

// GetCurrentRate returns the current request rate.
func (m *RampingArrivalRateMode) GetCurrentRate() int64 {
	return m.currentRate.Load()
}

// GetCurrentStage returns the current stage index.
func (m *RampingArrivalRateMode) GetCurrentStage() int {
	return m.currentStage
}
