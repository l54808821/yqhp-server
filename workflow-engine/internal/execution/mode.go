package execution

import (
	"context"
	"sync"
	"time"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// Mode defines the interface for execution modes.
// Each mode controls how VUs are managed and iterations are executed.
type Mode interface {
	// Name returns the name of the execution mode.
	Name() types.ExecutionMode

	// Run starts the execution mode with the given configuration.
	// It blocks until execution completes or context is cancelled.
	Run(ctx context.Context, config *ModeConfig) error

	// Stop gracefully stops the execution mode.
	Stop(ctx context.Context) error

	// GetState returns the current execution state.
	GetState() *ModeState
}

// ModeConfig contains configuration for execution modes.
type ModeConfig struct {
	// VUs is the number of virtual users (for VU-based modes).
	VUs int

	// Duration is the total execution duration.
	Duration time.Duration

	// Iterations is the total number of iterations.
	Iterations int

	// Stages defines the execution stages (for ramping modes).
	Stages []types.Stage

	// Rate is the request rate (for arrival rate modes).
	Rate int

	// TimeUnit is the time unit for rate calculation.
	TimeUnit time.Duration

	// PreAllocatedVUs is the number of pre-allocated VUs (for arrival rate modes).
	PreAllocatedVUs int

	// MaxVUs is the maximum number of VUs (for arrival rate modes).
	MaxVUs int

	// GracefulStop is the duration to wait for graceful shutdown.
	GracefulStop time.Duration

	// IterationFunc is the function to execute for each iteration.
	IterationFunc IterationFunc

	// OnVUStart is called when a VU starts.
	OnVUStart func(vuID int)

	// OnVUStop is called when a VU stops.
	OnVUStop func(vuID int)

	// OnIterationComplete is called when an iteration completes.
	OnIterationComplete func(vuID int, iteration int, duration time.Duration, err error)
}

// IterationFunc is the function signature for executing a single iteration.
type IterationFunc func(ctx context.Context, vuID int, iteration int) error

// ModeState represents the current state of an execution mode.
type ModeState struct {
	// ActiveVUs is the current number of active VUs.
	ActiveVUs int

	// TargetVUs is the target number of VUs.
	TargetVUs int

	// CompletedIterations is the number of completed iterations.
	CompletedIterations int64

	// CurrentRate is the current request rate (for arrival rate modes).
	CurrentRate float64

	// Running indicates if the mode is currently running.
	Running bool

	// Paused indicates if the mode is paused.
	Paused bool

	// StartTime is when the execution started.
	StartTime time.Time

	// ElapsedTime is the elapsed execution time.
	ElapsedTime time.Duration
}

// BaseMode provides common functionality for execution modes.
type BaseMode struct {
	name    types.ExecutionMode
	state   ModeState
	stateMu sync.RWMutex
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewBaseMode creates a new base mode.
func NewBaseMode(name types.ExecutionMode) *BaseMode {
	return &BaseMode{
		name:   name,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Name returns the mode name.
func (b *BaseMode) Name() types.ExecutionMode {
	return b.name
}

// GetState returns the current state.
func (b *BaseMode) GetState() *ModeState {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	state := b.state
	return &state
}

// SetState updates the state.
func (b *BaseMode) SetState(fn func(*ModeState)) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	fn(&b.state)
}

// IsStopped returns true if stop has been requested.
func (b *BaseMode) IsStopped() bool {
	select {
	case <-b.stopCh:
		return true
	default:
		return false
	}
}

// RequestStop signals the mode to stop.
func (b *BaseMode) RequestStop() {
	select {
	case <-b.stopCh:
		// Already stopped
	default:
		close(b.stopCh)
	}
}

// SignalDone signals that the mode has completed.
func (b *BaseMode) SignalDone() {
	select {
	case <-b.doneCh:
		// Already done
	default:
		close(b.doneCh)
	}
}

// WaitDone waits for the mode to complete.
func (b *BaseMode) WaitDone(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-b.doneCh:
		return nil
	}
}
