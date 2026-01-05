package execution

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"yqhp/workflow-engine/pkg/types"
)

// Tests for PerVUIterationsMode

func TestNewPerVUIterationsMode(t *testing.T) {
	mode := NewPerVUIterationsMode()
	assert.NotNil(t, mode)
	assert.Equal(t, types.ModePerVUIterations, mode.Name())
}

func TestPerVUIterationsMode_Run_NilConfig(t *testing.T) {
	mode := NewPerVUIterationsMode()
	err := mode.Run(context.Background(), nil)
	assert.ErrorIs(t, err, ErrNilConfig)
}

func TestPerVUIterationsMode_Run_NilIterationFunc(t *testing.T) {
	mode := NewPerVUIterationsMode()
	config := &ModeConfig{
		VUs:           1,
		Iterations:    5,
		IterationFunc: nil,
	}
	err := mode.Run(context.Background(), config)
	assert.ErrorIs(t, err, ErrNilIterationFunc)
}

func TestPerVUIterationsMode_Run_FixedIterationsPerVU(t *testing.T) {
	mode := NewPerVUIterationsMode()

	var iterationCount atomic.Int32
	vuIterations := make(map[int]int)
	var mu sync.Mutex

	config := &ModeConfig{
		VUs:        3,
		Iterations: 5, // 5 iterations per VU
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			iterationCount.Add(1)
			mu.Lock()
			vuIterations[vuID]++
			mu.Unlock()
			return nil
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// 3 VUs * 5 iterations = 15 total
	assert.Equal(t, int32(15), iterationCount.Load())

	// Each VU should have exactly 5 iterations
	mu.Lock()
	for vuID, count := range vuIterations {
		assert.Equal(t, 5, count, "VU %d should have 5 iterations", vuID)
	}
	mu.Unlock()
}

func TestPerVUIterationsMode_Run_DefaultValues(t *testing.T) {
	mode := NewPerVUIterationsMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		VUs:        0, // Should default to 1
		Iterations: 0, // Should default to 1
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			iterationCount.Add(1)
			return nil
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// 1 VU * 1 iteration = 1 total
	assert.Equal(t, int32(1), iterationCount.Load())
}

func TestPerVUIterationsMode_Run_WithDuration(t *testing.T) {
	mode := NewPerVUIterationsMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		VUs:        2,
		Iterations: 100, // Many iterations
		Duration:   50 * time.Millisecond,
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
	// Should stop due to duration, not complete all iterations
	assert.True(t, elapsed >= 50*time.Millisecond)
	assert.True(t, elapsed < 200*time.Millisecond)
	assert.True(t, iterationCount.Load() < 200) // Less than 2 VUs * 100 iterations
}

func TestPerVUIterationsMode_Run_ContextCancellation(t *testing.T) {
	mode := NewPerVUIterationsMode()

	ctx, cancel := context.WithCancel(context.Background())

	config := &ModeConfig{
		VUs:        2,
		Iterations: 100,
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

func TestPerVUIterationsMode_Stop(t *testing.T) {
	mode := NewPerVUIterationsMode()

	config := &ModeConfig{
		VUs:        2,
		Iterations: 100,
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

func TestPerVUIterationsMode_OnIterationComplete(t *testing.T) {
	mode := NewPerVUIterationsMode()

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

// Tests for SharedIterationsMode

func TestNewSharedIterationsMode(t *testing.T) {
	mode := NewSharedIterationsMode()
	assert.NotNil(t, mode)
	assert.Equal(t, types.ModeSharedIterations, mode.Name())
}

func TestSharedIterationsMode_Run_NilConfig(t *testing.T) {
	mode := NewSharedIterationsMode()
	err := mode.Run(context.Background(), nil)
	assert.ErrorIs(t, err, ErrNilConfig)
}

func TestSharedIterationsMode_Run_NilIterationFunc(t *testing.T) {
	mode := NewSharedIterationsMode()
	config := &ModeConfig{
		VUs:           1,
		Iterations:    5,
		IterationFunc: nil,
	}
	err := mode.Run(context.Background(), config)
	assert.ErrorIs(t, err, ErrNilIterationFunc)
}

func TestSharedIterationsMode_Run_SharedIterations(t *testing.T) {
	mode := NewSharedIterationsMode()

	var iterationCount atomic.Int32
	executedIterations := make(map[int]bool)
	var mu sync.Mutex

	config := &ModeConfig{
		VUs:        3,
		Iterations: 10, // 10 total iterations shared
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			iterationCount.Add(1)
			mu.Lock()
			executedIterations[iteration] = true
			mu.Unlock()
			return nil
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// Should have exactly 10 iterations
	assert.Equal(t, int32(10), iterationCount.Load())

	// All iterations 0-9 should have been executed
	mu.Lock()
	for i := 0; i < 10; i++ {
		assert.True(t, executedIterations[i], "Iteration %d should have been executed", i)
	}
	mu.Unlock()
}

func TestSharedIterationsMode_Run_DefaultValues(t *testing.T) {
	mode := NewSharedIterationsMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		VUs:        0, // Should default to 1
		Iterations: 0, // Should default to 1
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			iterationCount.Add(1)
			return nil
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// 1 total iteration
	assert.Equal(t, int32(1), iterationCount.Load())
}

func TestSharedIterationsMode_Run_DistributesWork(t *testing.T) {
	mode := NewSharedIterationsMode()

	vuWorkCount := make(map[int]int)
	var mu sync.Mutex

	config := &ModeConfig{
		VUs:        5,
		Iterations: 100,
		IterationFunc: func(ctx context.Context, vuID int, iteration int) error {
			mu.Lock()
			vuWorkCount[vuID]++
			mu.Unlock()
			time.Sleep(time.Millisecond) // Small delay to allow distribution
			return nil
		},
	}

	err := mode.Run(context.Background(), config)
	require.NoError(t, err)

	// All VUs should have done some work
	mu.Lock()
	totalWork := 0
	for _, count := range vuWorkCount {
		totalWork += count
		assert.True(t, count > 0, "Each VU should have done some work")
	}
	assert.Equal(t, 100, totalWork)
	mu.Unlock()
}

func TestSharedIterationsMode_Run_WithDuration(t *testing.T) {
	mode := NewSharedIterationsMode()

	var iterationCount atomic.Int32

	config := &ModeConfig{
		VUs:        2,
		Iterations: 1000, // Many iterations
		Duration:   50 * time.Millisecond,
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
	// Should stop due to duration, not complete all iterations
	assert.True(t, elapsed >= 50*time.Millisecond)
	assert.True(t, elapsed < 200*time.Millisecond)
	assert.True(t, iterationCount.Load() < 1000)
}

func TestSharedIterationsMode_Run_ContextCancellation(t *testing.T) {
	mode := NewSharedIterationsMode()

	ctx, cancel := context.WithCancel(context.Background())

	config := &ModeConfig{
		VUs:        2,
		Iterations: 100,
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

func TestSharedIterationsMode_Stop(t *testing.T) {
	mode := NewSharedIterationsMode()

	config := &ModeConfig{
		VUs:        2,
		Iterations: 100,
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

func TestSharedIterationsMode_OnIterationComplete(t *testing.T) {
	mode := NewSharedIterationsMode()

	var completedIterations atomic.Int32

	config := &ModeConfig{
		VUs:        2,
		Iterations: 5,
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

	assert.Equal(t, int32(5), completedIterations.Load())
}
