package slave

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVUPool_Acquire(t *testing.T) {
	pool := NewVUPool(10)

	vu := pool.Acquire(0)
	assert.NotNil(t, vu)
	assert.Equal(t, 0, vu.ID)
	assert.Equal(t, 0, vu.Iteration)
}

func TestVUPool_AcquireRelease(t *testing.T) {
	pool := NewVUPool(10)

	// Acquire VU
	vu := pool.Acquire(0)
	assert.NotNil(t, vu)
	assert.True(t, pool.IsInUse(0))

	// Release VU
	pool.Release(vu)
	assert.False(t, pool.IsInUse(0))

	// Acquire again
	vu2 := pool.Acquire(0)
	assert.NotNil(t, vu2)
	assert.Equal(t, vu.ID, vu2.ID)
}

func TestVUPool_AcquireSameID(t *testing.T) {
	pool := NewVUPool(10)

	vu1 := pool.Acquire(0)
	assert.NotNil(t, vu1)

	// Try to acquire same ID again
	vu2 := pool.Acquire(0)
	assert.Nil(t, vu2)
}

func TestVUPool_AcquireExceedsMax(t *testing.T) {
	pool := NewVUPool(5)

	// Try to acquire beyond max
	vu := pool.Acquire(10)
	assert.Nil(t, vu)
}

func TestVUPool_ActiveCount(t *testing.T) {
	pool := NewVUPool(10)

	assert.Equal(t, 0, pool.ActiveCount())

	vu1 := pool.Acquire(0)
	assert.Equal(t, 1, pool.ActiveCount())

	vu2 := pool.Acquire(1)
	assert.Equal(t, 2, pool.ActiveCount())

	pool.Release(vu1)
	assert.Equal(t, 1, pool.ActiveCount())

	pool.Release(vu2)
	assert.Equal(t, 0, pool.ActiveCount())
}

func TestVUPool_StopAll(t *testing.T) {
	pool := NewVUPool(10)

	pool.Acquire(0)
	pool.Acquire(1)
	pool.Acquire(2)

	assert.Equal(t, 3, pool.ActiveCount())

	pool.StopAll()

	assert.Equal(t, 0, pool.ActiveCount())
}

func TestVUPool_Reset(t *testing.T) {
	pool := NewVUPool(10)

	pool.Acquire(0)
	pool.Acquire(1)

	pool.Reset()

	assert.Equal(t, 0, pool.ActiveCount())
	assert.Nil(t, pool.GetVU(0))
	assert.Nil(t, pool.GetVU(1))
}

func TestVUPool_Concurrent(t *testing.T) {
	pool := NewVUPool(100)
	var wg sync.WaitGroup

	// Concurrently acquire and release VUs
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			vu := pool.Acquire(id)
			if vu != nil {
				pool.Release(vu)
			}
		}(i)
	}

	wg.Wait()

	// All VUs should be released
	assert.Equal(t, 0, pool.ActiveCount())
}

func TestVUPool_ReleaseNil(t *testing.T) {
	pool := NewVUPool(10)

	// Should not panic
	pool.Release(nil)
}
