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

func TestNewConstantVUsMode(t *testing.T) {
	mode := NewConstantVUsMode()
	assert.NotNil(t, mode)
	assert.Equal(t, types.ModeConstantVUs, mode.Name())
}

func TestConstantVUsMode_Run_NilConfig(t *testing.T) {
	mode := NewConstantVUsMode()
	err := mode.Run(context.Background(), nil)
	assert.ErrorIs(t, err, ErrNilConfig)
}

func TestConstantVUsMode_Run_NilIterationFunc(t *testing.T) {
	mode := NewConstantVUsMode()
	config := &ModeConfig{
		VUs:           1,
		IterationFunc: nil,
	}
	err := mode.Run(context.Background(), config)
	assert.ErrorIs(t, err, ErrNilIterationFunc)
}

func TestConstantVUsMode_Run_WithDuration(t *testing.T) {
	mode := NewConstantVUsMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		VUs:      2,
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

func TestConstantVUsMode_Run_WithIterations(t *testing.T) {
	mode := NewConstantVUsMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		VUs:        2,
		Iterations: 5, // 5 iterations per VU
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			iterationCount.Add(1)
			return nil
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// 2 VUs * 5 iterations = 10 total
	assert.Equal(t, int32(10), iterationCount.Load())
}

func TestConstantVUsMode_Run_MaintainsConstantVUs(t *testing.T) {
	mode := NewConstantVUsMode()

	maxActiveVUs := atomic.Int32{}
	var currentVUs atomic.Int32

	config := &ModeConfig{
		VUs:      5,
		Duration: 100 * time.Millisecond,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
		OnVUStart: func(vuID int) {
			current := currentVUs.Add(1)
			for {
				max := maxActiveVUs.Load()
				if current <= max || maxActiveVUs.CompareAndSwap(max, current) {
					break
				}
			}
		},
		OnVUStop: func(vuID int) {
			currentVUs.Add(-1)
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// Should have maintained 5 VUs
	assert.Equal(t, int32(5), maxActiveVUs.Load())
}

func TestConstantVUsMode_Run_ContextCancellation(t *testing.T) {
	mode := NewConstantVUsMode()

	ctx, cancel := context.WithCancel(context.Background())

	config := &ModeConfig{
		VUs: 2,
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
	time.Sleep(30 * time.Millisecond)
	cancel()

	err := <-done
	assert.NoError(t, err)
}

func TestConstantVUsMode_Stop(t *testing.T) {
	mode := NewConstantVUsMode()

	config := &ModeConfig{
		VUs: 2,
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
	time.Sleep(20 * time.Millisecond)

	// Stop the mode
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := mode.Stop(ctx)
	assert.NoError(t, err)

	// Run should complete
	runErr := <-done
	assert.NoError(t, runErr)
}

func TestConstantVUsMode_GetState(t *testing.T) {
	mode := NewConstantVUsMode()

	state := mode.GetState()
	assert.NotNil(t, state)
	assert.False(t, state.Running)
	assert.Equal(t, 0, state.ActiveVUs)
}

func TestConstantVUsMode_DefaultVUs(t *testing.T) {
	mode := NewConstantVUsMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		VUs:        0, // Should default to 1
		Iterations: 3,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			iterationCount.Add(1)
			return nil
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// 1 VU * 3 iterations = 3 total
	assert.Equal(t, int32(3), iterationCount.Load())
}

func TestConstantVUsMode_OnIterationComplete(t *testing.T) {
	mode := NewConstantVUsMode()

	var completedIterations atomic.Int32

	config := &ModeConfig{
		VUs:        1,
		Iterations: 3,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			return nil
		},
		OnIterationComplete: func(vuID int, iteration int, duration time.Duration, err error) {
			completedIterations.Add(1)
			assert.NoError(t, err)
			assert.True(t, duration >= 0)
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	assert.Equal(t, int32(3), completedIterations.Load())
}
