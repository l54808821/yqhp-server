package execution

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

func TestNewExternallyControlledMode(t *testing.T) {
	mode := NewExternallyControlledMode()
	assert.NotNil(t, mode)
	assert.Equal(t, types.ModeExternally, mode.Name())
}

func TestExternallyControlledMode_Run_NilConfig(t *testing.T) {
	mode := NewExternallyControlledMode()
	err := mode.Run(context.Background(), nil)
	assert.ErrorIs(t, err, ErrNilConfig)
}

func TestExternallyControlledMode_Run_NilIterationFunc(t *testing.T) {
	mode := NewExternallyControlledMode()
	config := &ModeConfig{
		VUs:           1,
		IterationFunc: nil,
	}
	err := mode.Run(context.Background(), config)
	assert.ErrorIs(t, err, ErrNilIterationFunc)
}

func TestExternallyControlledMode_Run_WithDuration(t *testing.T) {
	mode := NewExternallyControlledMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		VUs:      2,
		MaxVUs:   10,
		Duration: 100 * time.Millisecond,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			iterationCount.Add(1)
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}

	start := time.Now()
	err := mode.Run(context.Background(), config)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.True(t, elapsed >= 100*time.Millisecond)
	assert.True(t, elapsed < 200*time.Millisecond)
	assert.True(t, iterationCount.Load() > 0)
}

func TestExternallyControlledMode_Scale_Up(t *testing.T) {
	mode := NewExternallyControlledMode()

	var maxVUs atomic.Int32
	var currentVUs atomic.Int32

	config := &ModeConfig{
		VUs:      2,
		MaxVUs:   10,
		Duration: 200 * time.Millisecond,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
		OnVUStart: func(vuID int) {
			current := currentVUs.Add(1)
			for {
				max := maxVUs.Load()
				if current <= max || maxVUs.CompareAndSwap(max, current) {
					break
				}
			}
		},
		OnVUStop: func(vuID int) {
			currentVUs.Add(-1)
		},
	}

	done := make(chan error)
	go func() {
		done <- mode.Run(context.Background(), config)
	}()

	// Wait for initial VUs to start
	time.Sleep(50 * time.Millisecond)

	// Scale up to 5 VUs
	err := mode.Scale(5)
	require.NoError(t, err)

	// Wait for scaling
	time.Sleep(100 * time.Millisecond)

	// Check that we scaled up
	assert.True(t, maxVUs.Load() >= 5, "Expected at least 5 VUs, got %d", maxVUs.Load())

	// Wait for completion
	<-done
}

func TestExternallyControlledMode_Scale_Down(t *testing.T) {
	mode := NewExternallyControlledMode()

	var currentVUs atomic.Int32

	config := &ModeConfig{
		VUs:      5,
		MaxVUs:   10,
		Duration: 200 * time.Millisecond,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
		OnVUStart: func(vuID int) {
			currentVUs.Add(1)
		},
		OnVUStop: func(vuID int) {
			currentVUs.Add(-1)
		},
	}

	done := make(chan error)
	go func() {
		done <- mode.Run(context.Background(), config)
	}()

	// Wait for initial VUs to start
	time.Sleep(50 * time.Millisecond)

	// Scale down to 2 VUs
	err := mode.Scale(2)
	require.NoError(t, err)

	// Wait for scaling
	time.Sleep(100 * time.Millisecond)

	// Check that we scaled down
	assert.True(t, mode.GetTargetVUs() == 2, "Expected target 2 VUs, got %d", mode.GetTargetVUs())

	// Wait for completion
	<-done
}

func TestExternallyControlledMode_Pause_Resume(t *testing.T) {
	mode := NewExternallyControlledMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		VUs:      2,
		MaxVUs:   10,
		Duration: 500 * time.Millisecond,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			iterationCount.Add(1)
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}

	done := make(chan error)
	go func() {
		done <- mode.Run(context.Background(), config)
	}()

	// Wait for some iterations
	time.Sleep(100 * time.Millisecond)
	countBeforePause := iterationCount.Load()

	// Pause
	err := mode.Pause()
	require.NoError(t, err)

	// Wait for pause to take effect
	time.Sleep(50 * time.Millisecond)
	assert.True(t, mode.IsPaused(), "Mode should be paused")

	// Wait while paused
	time.Sleep(100 * time.Millisecond)
	countWhilePaused := iterationCount.Load()

	// Should have minimal new iterations while paused (allow some due to in-flight iterations)
	pausedIterations := countWhilePaused - countBeforePause
	assert.True(t, pausedIterations <= 10, "Expected minimal iterations while paused, got %d", pausedIterations)

	// Resume
	err = mode.Resume()
	require.NoError(t, err)

	// Wait for resume to take effect
	time.Sleep(50 * time.Millisecond)
	assert.False(t, mode.IsPaused(), "Mode should not be paused")

	// Wait for more iterations
	time.Sleep(100 * time.Millisecond)
	countAfterResume := iterationCount.Load()

	// Should have more iterations after resume
	assert.True(t, countAfterResume > countWhilePaused, "Expected more iterations after resume")

	// Stop the mode
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	mode.Stop(ctx)

	// Wait for completion
	<-done
}

func TestExternallyControlledMode_Scale_NotRunning(t *testing.T) {
	mode := NewExternallyControlledMode()

	err := mode.Scale(5)
	assert.ErrorIs(t, err, ErrModeNotRunning)
}

func TestExternallyControlledMode_Pause_NotRunning(t *testing.T) {
	mode := NewExternallyControlledMode()

	err := mode.Pause()
	assert.ErrorIs(t, err, ErrModeNotRunning)
}

func TestExternallyControlledMode_Resume_NotRunning(t *testing.T) {
	mode := NewExternallyControlledMode()

	err := mode.Resume()
	assert.ErrorIs(t, err, ErrModeNotRunning)
}

func TestExternallyControlledMode_Stop(t *testing.T) {
	mode := NewExternallyControlledMode()

	config := &ModeConfig{
		VUs:    2,
		MaxVUs: 10,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		},
	}

	done := make(chan error)
	go func() {
		done <- mode.Run(context.Background(), config)
	}()

	// Wait for VUs to start
	time.Sleep(50 * time.Millisecond)

	// Stop the mode
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := mode.Stop(ctx)
	assert.NoError(t, err)

	// Run should complete
	runErr := <-done
	assert.NoError(t, runErr)
}

func TestExternallyControlledMode_ContextCancellation(t *testing.T) {
	mode := NewExternallyControlledMode()

	ctx, cancel := context.WithCancel(context.Background())

	config := &ModeConfig{
		VUs:    2,
		MaxVUs: 10,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		},
	}

	done := make(chan error)
	go func() {
		done <- mode.Run(ctx, config)
	}()

	// Cancel after a short time
	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-done
	assert.NoError(t, err)
}

func TestExternallyControlledMode_GetState(t *testing.T) {
	mode := NewExternallyControlledMode()

	state := mode.GetState()
	assert.NotNil(t, state)
	assert.False(t, state.Running)
	assert.Equal(t, 0, state.ActiveVUs)
}

func TestExternallyControlledMode_Scale_MaxVUs(t *testing.T) {
	mode := NewExternallyControlledMode()

	config := &ModeConfig{
		VUs:      2,
		MaxVUs:   5,
		Duration: 100 * time.Millisecond,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}

	done := make(chan error)
	go func() {
		done <- mode.Run(context.Background(), config)
	}()

	// Wait for initial VUs to start
	time.Sleep(30 * time.Millisecond)

	// Try to scale beyond max
	err := mode.Scale(10)
	require.NoError(t, err)

	// Wait for scaling
	time.Sleep(50 * time.Millisecond)

	// Should be capped at maxVUs
	assert.True(t, mode.GetTargetVUs() <= 5, "Expected target <= 5 VUs, got %d", mode.GetTargetVUs())

	// Wait for completion
	<-done
}

func TestExternallyControlledMode_DefaultValues(t *testing.T) {
	mode := NewExternallyControlledMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		VUs:      0, // Should default to 1
		MaxVUs:   0, // Should default to 100
		Duration: 50 * time.Millisecond,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			iterationCount.Add(1)
			return nil
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// Should have run with at least 1 VU
	assert.True(t, iterationCount.Load() > 0)
}
