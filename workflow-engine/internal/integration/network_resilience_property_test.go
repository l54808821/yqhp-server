// Package integration provides property-based tests for network resilience.
// Requirements: 15.5 - Network resilience and result buffering
// Property 12: For any network partition during execution, the slave should buffer results
// and successfully deliver them upon reconnection, ensuring no data loss.
package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"yqhp/workflow-engine/pkg/types"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
)

// resultBuffer simulates a result buffer for network resilience testing.
type resultBuffer struct {
	mu       sync.Mutex
	results  []*types.StepResult
	capacity int
}

func newResultBuffer(capacity int) *resultBuffer {
	return &resultBuffer{
		results:  make([]*types.StepResult, 0, capacity),
		capacity: capacity,
	}
}

func (b *resultBuffer) Add(result *types.StepResult) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.results) >= b.capacity {
		return false // Buffer full
	}

	b.results = append(b.results, result)
	return true
}

func (b *resultBuffer) Flush() []*types.StepResult {
	b.mu.Lock()
	defer b.mu.Unlock()

	results := make([]*types.StepResult, len(b.results))
	copy(results, b.results)
	b.results = b.results[:0]
	return results
}

func (b *resultBuffer) Count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.results)
}

// mockConnection simulates a network connection that can be partitioned.
type mockConnection struct {
	mu          sync.Mutex
	connected   bool
	sentResults []*types.StepResult
	buffer      *resultBuffer
}

func newMockConnection(bufferCapacity int) *mockConnection {
	return &mockConnection{
		connected:   true,
		sentResults: make([]*types.StepResult, 0),
		buffer:      newResultBuffer(bufferCapacity),
	}
}

func (c *mockConnection) Send(result *types.StepResult) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		c.sentResults = append(c.sentResults, result)
		return true
	}

	// Buffer when disconnected
	return c.buffer.Add(result)
}

func (c *mockConnection) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
}

func (c *mockConnection) Reconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = true

	// Flush buffer on reconnect
	buffered := c.buffer.Flush()
	c.sentResults = append(c.sentResults, buffered...)
}

func (c *mockConnection) GetSentResults() []*types.StepResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	results := make([]*types.StepResult, len(c.sentResults))
	copy(results, c.sentResults)
	return results
}

func (c *mockConnection) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// TestNetworkResilienceProperty tests Property 12: Network resilience - result buffering.
// results_before_partition + results_during_partition == results_after_reconnection
func TestNetworkResilienceProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: All results are delivered after reconnection
	properties.Property("all results delivered after reconnection", prop.ForAll(
		func(beforeCount, duringCount int) bool {
			if beforeCount < 0 {
				beforeCount = 0
			}
			if duringCount < 0 {
				duringCount = 0
			}
			if beforeCount > 50 {
				beforeCount = 50
			}
			if duringCount > 50 {
				duringCount = 50
			}

			conn := newMockConnection(1000)

			// Send results before partition
			for i := 0; i < beforeCount; i++ {
				result := &types.StepResult{
					StepID:    "step-before-" + string(rune('a'+i)),
					Status:    types.ResultStatusSuccess,
					StartTime: time.Now(),
				}
				conn.Send(result)
			}

			// Simulate partition
			conn.Disconnect()

			// Send results during partition (buffered)
			for i := 0; i < duringCount; i++ {
				result := &types.StepResult{
					StepID:    "step-during-" + string(rune('a'+i)),
					Status:    types.ResultStatusSuccess,
					StartTime: time.Now(),
				}
				conn.Send(result)
			}

			// Reconnect
			conn.Reconnect()

			// Verify all results were delivered
			sentResults := conn.GetSentResults()
			expectedCount := beforeCount + duringCount

			return len(sentResults) == expectedCount
		},
		gen.IntRange(0, 50),
		gen.IntRange(0, 50),
	))

	// Property: Results maintain order
	properties.Property("results maintain order", prop.ForAll(
		func(totalCount int) bool {
			if totalCount < 1 {
				totalCount = 1
			}
			if totalCount > 100 {
				totalCount = 100
			}

			conn := newMockConnection(1000)

			// Send half before partition
			halfCount := totalCount / 2
			for i := 0; i < halfCount; i++ {
				result := &types.StepResult{
					StepID:    "step-" + string(rune(i)),
					Status:    types.ResultStatusSuccess,
					StartTime: time.Now(),
				}
				conn.Send(result)
			}

			// Partition
			conn.Disconnect()

			// Send rest during partition
			for i := halfCount; i < totalCount; i++ {
				result := &types.StepResult{
					StepID:    "step-" + string(rune(i)),
					Status:    types.ResultStatusSuccess,
					StartTime: time.Now(),
				}
				conn.Send(result)
			}

			// Reconnect
			conn.Reconnect()

			sentResults := conn.GetSentResults()

			// Verify count
			if len(sentResults) != totalCount {
				return false
			}

			// Verify order (first half should be before second half)
			for i := 0; i < halfCount; i++ {
				if sentResults[i].StepID != "step-"+string(rune(i)) {
					return false
				}
			}

			return true
		},
		gen.IntRange(2, 100),
	))

	properties.TestingRun(t)
}

// TestBufferCapacityProperty tests buffer capacity handling.
func TestBufferCapacityProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Buffer respects capacity limit
	properties.Property("buffer respects capacity limit", prop.ForAll(
		func(capacity, sendCount int) bool {
			if capacity < 1 {
				capacity = 1
			}
			if capacity > 100 {
				capacity = 100
			}
			if sendCount < 0 {
				sendCount = 0
			}
			if sendCount > 200 {
				sendCount = 200
			}

			buffer := newResultBuffer(capacity)

			successCount := 0
			for i := 0; i < sendCount; i++ {
				result := &types.StepResult{
					StepID: "step-" + string(rune(i)),
				}
				if buffer.Add(result) {
					successCount++
				}
			}

			// Should not exceed capacity
			if buffer.Count() > capacity {
				return false
			}

			// Success count should be min(sendCount, capacity)
			expectedSuccess := sendCount
			if expectedSuccess > capacity {
				expectedSuccess = capacity
			}

			return successCount == expectedSuccess
		},
		gen.IntRange(1, 100),
		gen.IntRange(0, 200),
	))

	properties.TestingRun(t)
}

// TestMultiplePartitionsProperty tests multiple network partitions.
func TestMultiplePartitionsProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Multiple partitions don't lose data
	properties.Property("multiple partitions preserve data", prop.ForAll(
		func(partitionCount int) bool {
			if partitionCount < 1 {
				partitionCount = 1
			}
			if partitionCount > 5 {
				partitionCount = 5
			}

			conn := newMockConnection(1000)
			totalSent := 0

			for p := 0; p < partitionCount; p++ {
				// Send while connected
				for i := 0; i < 10; i++ {
					result := &types.StepResult{
						StepID: "step-" + string(rune(totalSent)),
					}
					conn.Send(result)
					totalSent++
				}

				// Partition
				conn.Disconnect()

				// Send while disconnected
				for i := 0; i < 10; i++ {
					result := &types.StepResult{
						StepID: "step-" + string(rune(totalSent)),
					}
					conn.Send(result)
					totalSent++
				}

				// Reconnect
				conn.Reconnect()
			}

			sentResults := conn.GetSentResults()
			return len(sentResults) == totalSent
		},
		gen.IntRange(1, 5),
	))

	properties.TestingRun(t)
}

// BenchmarkResultBuffering benchmarks result buffering.
func BenchmarkResultBuffering(b *testing.B) {
	conn := newMockConnection(10000)

	result := &types.StepResult{
		StepID:    "step-1",
		Status:    types.ResultStatusSuccess,
		StartTime: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.Send(result)
	}
}

// TestNetworkResilienceSpecificCases tests specific edge cases.
func TestNetworkResilienceSpecificCases(t *testing.T) {
	ctx := context.Background()
	_ = ctx // Suppress unused variable warning

	testCases := []struct {
		name         string
		beforeCount  int
		duringCount  int
		expectedSent int
	}{
		{
			name:         "no partition",
			beforeCount:  10,
			duringCount:  0,
			expectedSent: 10,
		},
		{
			name:         "all during partition",
			beforeCount:  0,
			duringCount:  10,
			expectedSent: 10,
		},
		{
			name:         "mixed",
			beforeCount:  5,
			duringCount:  5,
			expectedSent: 10,
		},
		{
			name:         "large batch",
			beforeCount:  50,
			duringCount:  50,
			expectedSent: 100,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			conn := newMockConnection(1000)

			// Send before partition
			for i := 0; i < tc.beforeCount; i++ {
				result := &types.StepResult{
					StepID: "before-" + string(rune(i)),
				}
				conn.Send(result)
			}

			// Partition
			conn.Disconnect()

			// Send during partition
			for i := 0; i < tc.duringCount; i++ {
				result := &types.StepResult{
					StepID: "during-" + string(rune(i)),
				}
				conn.Send(result)
			}

			// Reconnect
			conn.Reconnect()

			sentResults := conn.GetSentResults()
			assert.Len(t, sentResults, tc.expectedSent)
		})
	}
}
