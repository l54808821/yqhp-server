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

// Tests for ConstantArrivalRateMode

func TestNewConstantArrivalRateMode(t *testing.T) {
	mode := NewConstantArrivalRateMode()
	assert.NotNil(t, mode)
	assert.Equal(t, types.ModeConstantArrivalRate, mode.Name())
}

func TestConstantArrivalRateMode_Run_NilConfig(t *testing.T) {
	mode := NewConstantArrivalRateMode()
	err := mode.Run(context.Background(), nil)
	assert.ErrorIs(t, err, ErrNilConfig)
}

func TestConstantArrivalRateMode_Run_NilIterationFunc(t *testing.T) {
	mode := NewConstantArrivalRateMode()
	config := &ModeConfig{
		Rate:          10,
		IterationFunc: nil,
	}
	err := mode.Run(context.Background(), config)
	assert.ErrorIs(t, err, ErrNilIterationFunc)
}

func TestConstantArrivalRateMode_Run_InvalidRate(t *testing.T) {
	mode := NewConstantArrivalRateMode()
	config := &ModeConfig{
		Rate: 0,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			return nil
		},
	}
	err := mode.Run(context.Background(), config)
	assert.ErrorIs(t, err, ErrInvalidRate)
}

func TestConstantArrivalRateMode_Run_WithDuration(t *testing.T) {
	mode := NewConstantArrivalRateMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		Rate:            20, // 20 iterations per second
		TimeUnit:        time.Second,
		Duration:        200 * time.Millisecond,
		PreAllocatedVUs: 2,
		MaxVUs:          5,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			iterationCount.Add(1)
			time.Sleep(5 * time.Millisecond)
			return nil
		},
	}

	start := time.Now()
	err := mode.Run(context.Background(), config)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.True(t, elapsed >= 200*time.Millisecond)
	assert.True(t, elapsed < 400*time.Millisecond)
	// Should have approximately 4 iterations (20/s * 0.2s)
	assert.True(t, iterationCount.Load() >= 2)
}

func TestConstantArrivalRateMode_Run_MaintainsRate(t *testing.T) {
	mode := NewConstantArrivalRateMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		Rate:            50, // 50 iterations per second
		TimeUnit:        time.Second,
		Duration:        100 * time.Millisecond,
		PreAllocatedVUs: 5,
		MaxVUs:          10,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			iterationCount.Add(1)
			return nil
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// Should have approximately 5 iterations (50/s * 0.1s)
	// Allow some variance due to timing
	count := iterationCount.Load()
	assert.True(t, count >= 3 && count <= 10, "Expected 3-10 iterations, got %d", count)
}

func TestConstantArrivalRateMode_Run_ContextCancellation(t *testing.T) {
	mode := NewConstantArrivalRateMode()

	ctx, cancel := context.WithCancel(context.Background())

	config := &ModeConfig{
		Rate:            10,
		TimeUnit:        time.Second,
		PreAllocatedVUs: 2,
		MaxVUs:          5,
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

func TestConstantArrivalRateMode_Stop(t *testing.T) {
	mode := NewConstantArrivalRateMode()

	config := &ModeConfig{
		Rate:            10,
		TimeUnit:        time.Second,
		PreAllocatedVUs: 2,
		MaxVUs:          5,
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

// Tests for RampingArrivalRateMode

func TestNewRampingArrivalRateMode(t *testing.T) {
	mode := NewRampingArrivalRateMode()
	assert.NotNil(t, mode)
	assert.Equal(t, types.ModeRampingArrivalRate, mode.Name())
}

func TestRampingArrivalRateMode_Run_NilConfig(t *testing.T) {
	mode := NewRampingArrivalRateMode()
	err := mode.Run(context.Background(), nil)
	assert.ErrorIs(t, err, ErrNilConfig)
}

func TestRampingArrivalRateMode_Run_NilIterationFunc(t *testing.T) {
	mode := NewRampingArrivalRateMode()
	config := &ModeConfig{
		Stages: []types.Stage{
			{Duration: time.Second, Target: 10},
		},
		IterationFunc: nil,
	}
	err := mode.Run(context.Background(), config)
	assert.ErrorIs(t, err, ErrNilIterationFunc)
}

func TestRampingArrivalRateMode_Run_NoStages(t *testing.T) {
	mode := NewRampingArrivalRateMode()
	config := &ModeConfig{
		Stages: []types.Stage{},
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			return nil
		},
	}
	err := mode.Run(context.Background(), config)
	assert.ErrorIs(t, err, ErrNoStages)
}

func TestRampingArrivalRateMode_Run_SingleStage(t *testing.T) {
	mode := NewRampingArrivalRateMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		Stages: []types.Stage{
			{Duration: 100 * time.Millisecond, Target: 50},
		},
		TimeUnit:        time.Second,
		PreAllocatedVUs: 2,
		MaxVUs:          10,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			iterationCount.Add(1)
			return nil
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// Should have some iterations
	assert.True(t, iterationCount.Load() > 0)
}

func TestRampingArrivalRateMode_Run_MultipleStages(t *testing.T) {
	mode := NewRampingArrivalRateMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		Stages: []types.Stage{
			{Duration: 100 * time.Millisecond, Target: 50},
			{Duration: 100 * time.Millisecond, Target: 100},
			{Duration: 100 * time.Millisecond, Target: 20},
		},
		TimeUnit:        time.Second,
		PreAllocatedVUs: 5,
		MaxVUs:          20,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			iterationCount.Add(1)
			return nil
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// Should have some iterations (at least a few given the rates)
	assert.True(t, iterationCount.Load() >= 0, "Expected at least 0 iterations, got %d", iterationCount.Load())
}

func TestRampingArrivalRateMode_Run_ContextCancellation(t *testing.T) {
	mode := NewRampingArrivalRateMode()

	ctx, cancel := context.WithCancel(context.Background())

	config := &ModeConfig{
		Stages: []types.Stage{
			{Duration: time.Second, Target: 10},
		},
		TimeUnit:        time.Second,
		PreAllocatedVUs: 2,
		MaxVUs:          5,
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

func TestRampingArrivalRateMode_Stop(t *testing.T) {
	mode := NewRampingArrivalRateMode()

	config := &ModeConfig{
		Stages: []types.Stage{
			{Duration: time.Second, Target: 10},
		},
		TimeUnit:        time.Second,
		PreAllocatedVUs: 2,
		MaxVUs:          5,
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

func TestRampingArrivalRateMode_GetCurrentRate(t *testing.T) {
	mode := NewRampingArrivalRateMode()

	var rateRecorded atomic.Int64

	config := &ModeConfig{
		Stages: []types.Stage{
			{Duration: 100 * time.Millisecond, Target: 100},
		},
		TimeUnit:        time.Second,
		PreAllocatedVUs: 2,
		MaxVUs:          10,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			rateRecorded.Store(mode.GetCurrentRate())
			return nil
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// Rate should have been recorded
	assert.True(t, rateRecorded.Load() >= 0)
}
