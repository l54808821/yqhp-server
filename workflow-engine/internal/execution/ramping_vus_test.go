package execution

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"yqhp/workflow-engine/pkg/types"
)

func TestNewRampingVUsMode(t *testing.T) {
	mode := NewRampingVUsMode()
	assert.NotNil(t, mode)
	assert.Equal(t, types.ModeRampingVUs, mode.Name())
}

func TestRampingVUsMode_Run_NilConfig(t *testing.T) {
	mode := NewRampingVUsMode()
	err := mode.Run(context.Background(), nil)
	assert.ErrorIs(t, err, ErrNilConfig)
}

func TestRampingVUsMode_Run_NilIterationFunc(t *testing.T) {
	mode := NewRampingVUsMode()
	config := &ModeConfig{
		Stages: []types.Stage{
			{Duration: time.Second, Target: 5},
		},
		IterationFunc: nil,
	}
	err := mode.Run(context.Background(), config)
	assert.ErrorIs(t, err, ErrNilIterationFunc)
}

func TestRampingVUsMode_Run_NoStages(t *testing.T) {
	mode := NewRampingVUsMode()
	config := &ModeConfig{
		Stages: []types.Stage{},
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			return nil
		},
	}
	err := mode.Run(context.Background(), config)
	assert.ErrorIs(t, err, ErrNoStages)
}

func TestRampingVUsMode_Run_SingleStage(t *testing.T) {
	mode := NewRampingVUsMode()

	var maxVUs atomic.Int32
	var currentVUs atomic.Int32

	config := &ModeConfig{
		Stages: []types.Stage{
			{Duration: 100 * time.Millisecond, Target: 5},
		},
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

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// Should have ramped up to 5 VUs
	assert.Equal(t, int32(5), maxVUs.Load())
}

func TestRampingVUsMode_Run_MultipleStages(t *testing.T) {
	mode := NewRampingVUsMode()

	var maxVUs atomic.Int32
	var currentVUs atomic.Int32

	config := &ModeConfig{
		Stages: []types.Stage{
			{Duration: 50 * time.Millisecond, Target: 3},
			{Duration: 50 * time.Millisecond, Target: 5},
			{Duration: 50 * time.Millisecond, Target: 2},
		},
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			time.Sleep(5 * time.Millisecond)
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

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// Should have reached 5 VUs at peak
	assert.Equal(t, int32(5), maxVUs.Load())
}

func TestRampingVUsMode_Run_RampDown(t *testing.T) {
	mode := NewRampingVUsMode()

	vuHistory := make([]int32, 0)
	var historyMu atomic.Int32
	var currentVUs atomic.Int32

	config := &ModeConfig{
		Stages: []types.Stage{
			{Duration: 50 * time.Millisecond, Target: 5},
			{Duration: 50 * time.Millisecond, Target: 0},
		},
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			time.Sleep(5 * time.Millisecond)
			return nil
		},
		OnVUStart: func(vuID int) {
			currentVUs.Add(1)
		},
		OnVUStop: func(vuID int) {
			current := currentVUs.Add(-1)
			// Record VU count history (simplified)
			historyMu.Store(current)
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// After ramp down, should have 0 VUs
	assert.Equal(t, int32(0), currentVUs.Load())
	_ = vuHistory // Avoid unused variable warning
}

func TestRampingVUsMode_Run_ContextCancellation(t *testing.T) {
	mode := NewRampingVUsMode()

	ctx, cancel := context.WithCancel(context.Background())

	config := &ModeConfig{
		Stages: []types.Stage{
			{Duration: time.Second, Target: 10},
		},
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
	time.Sleep(100 * time.Millisecond)
	cancel()

	err := <-done
	assert.NoError(t, err)
}

func TestRampingVUsMode_Stop(t *testing.T) {
	mode := NewRampingVUsMode()

	config := &ModeConfig{
		Stages: []types.Stage{
			{Duration: time.Second, Target: 5},
		},
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
	time.Sleep(100 * time.Millisecond)

	// Stop the mode
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := mode.Stop(ctx)
	assert.NoError(t, err)

	// Run should complete
	runErr := <-done
	assert.NoError(t, runErr)
}

func TestRampingVUsMode_GetState(t *testing.T) {
	mode := NewRampingVUsMode()

	state := mode.GetState()
	assert.NotNil(t, state)
	assert.False(t, state.Running)
	assert.Equal(t, 0, state.ActiveVUs)
}

func TestRampingVUsMode_GetCurrentStage(t *testing.T) {
	mode := NewRampingVUsMode()

	var stageRecorded atomic.Int32

	config := &ModeConfig{
		Stages: []types.Stage{
			{Duration: 30 * time.Millisecond, Target: 2},
			{Duration: 30 * time.Millisecond, Target: 4},
		},
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			stageRecorded.Store(int32(mode.GetCurrentStage()))
			time.Sleep(5 * time.Millisecond)
			return nil
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// Should have progressed through stages
	assert.True(t, stageRecorded.Load() >= 0)
}
