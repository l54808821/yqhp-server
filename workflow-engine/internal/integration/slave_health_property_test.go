// Package integration provides property-based tests for slave health monitoring.
// Requirements: 14.3 - Slave health monitoring
// Property 11: For any slave whose heartbeat exceeds the configured timeout,
// the Master should mark it as unhealthy.
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"

	"yqhp/workflow-engine/internal/master"
	"yqhp/workflow-engine/pkg/types"
)

// TestSlaveHealthMonitoringProperty tests Property 11: Slave health monitoring.
// missed_heartbeats > timeout => slave.status == unhealthy
func TestSlaveHealthMonitoringProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Slave with recent heartbeat is healthy
	properties.Property("slave with recent heartbeat is healthy", prop.ForAll(
		func(secondsAgo int) bool {
			if secondsAgo < 0 {
				secondsAgo = 0
			}
			if secondsAgo > 10 {
				secondsAgo = 10
			}

			registry := master.NewInMemorySlaveRegistry()
			ctx := context.Background()

			// Register slave
			slave := &types.SlaveInfo{
				ID:   "slave-1",
				Type: types.SlaveTypeWorker,
			}
			registry.Register(ctx, slave)

			// Update status with recent heartbeat
			status := &types.SlaveStatus{
				State:    types.SlaveStateOnline,
				LastSeen: time.Now().Add(-time.Duration(secondsAgo) * time.Second),
			}
			registry.UpdateStatus(ctx, "slave-1", status)

			// Check health with 30 second timeout
			isHealthy := registry.IsSlaveHealthy(ctx, "slave-1", 30*time.Second)

			// Should be healthy if heartbeat was within timeout
			return isHealthy
		},
		gen.IntRange(0, 10),
	))

	// Property: Slave with old heartbeat is unhealthy
	properties.Property("slave with old heartbeat is unhealthy", prop.ForAll(
		func(secondsAgo int) bool {
			if secondsAgo < 35 {
				secondsAgo = 35
			}
			if secondsAgo > 120 {
				secondsAgo = 120
			}

			registry := master.NewInMemorySlaveRegistry()
			ctx := context.Background()

			// Register slave
			slave := &types.SlaveInfo{
				ID:   "slave-1",
				Type: types.SlaveTypeWorker,
			}
			registry.Register(ctx, slave)

			// Update status with old heartbeat
			status := &types.SlaveStatus{
				State:    types.SlaveStateOnline,
				LastSeen: time.Now().Add(-time.Duration(secondsAgo) * time.Second),
			}
			registry.UpdateStatus(ctx, "slave-1", status)

			// Check health with 30 second timeout
			isHealthy := registry.IsSlaveHealthy(ctx, "slave-1", 30*time.Second)

			// Should be unhealthy if heartbeat was beyond timeout
			return !isHealthy
		},
		gen.IntRange(35, 120),
	))

	// Property: Health status changes based on heartbeat timing
	properties.Property("health status reflects heartbeat timing", prop.ForAll(
		func(secondsAgo, timeoutSeconds int) bool {
			if secondsAgo < 0 {
				secondsAgo = 0
			}
			if timeoutSeconds < 1 {
				timeoutSeconds = 1
			}

			registry := master.NewInMemorySlaveRegistry()
			ctx := context.Background()

			slave := &types.SlaveInfo{
				ID:   "slave-1",
				Type: types.SlaveTypeWorker,
			}
			registry.Register(ctx, slave)

			status := &types.SlaveStatus{
				State:    types.SlaveStateOnline,
				LastSeen: time.Now().Add(-time.Duration(secondsAgo) * time.Second),
			}
			registry.UpdateStatus(ctx, "slave-1", status)

			timeout := time.Duration(timeoutSeconds) * time.Second
			isHealthy := registry.IsSlaveHealthy(ctx, "slave-1", timeout)

			// Health should match whether heartbeat is within timeout
			expectedHealthy := secondsAgo <= timeoutSeconds
			return isHealthy == expectedHealthy
		},
		gen.IntRange(0, 60),
		gen.IntRange(10, 60),
	))

	properties.TestingRun(t)
}

// TestSlaveStateTransitionsProperty tests slave state transitions.
func TestSlaveStateTransitionsProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: New slave starts as online
	properties.Property("new slave starts as online", prop.ForAll(
		func(_ bool) bool {
			registry := master.NewInMemorySlaveRegistry()
			ctx := context.Background()

			slave := &types.SlaveInfo{
				ID:   "slave-1",
				Type: types.SlaveTypeWorker,
			}
			registry.Register(ctx, slave)

			status, err := registry.GetSlaveStatus(ctx, "slave-1")
			if err != nil {
				return false
			}

			return status.State == types.SlaveStateOnline
		},
		gen.Bool(),
	))

	// Property: Offline slave is not healthy
	properties.Property("offline slave is not healthy", prop.ForAll(
		func(_ bool) bool {
			registry := master.NewInMemorySlaveRegistry()
			ctx := context.Background()

			slave := &types.SlaveInfo{
				ID:   "slave-1",
				Type: types.SlaveTypeWorker,
			}
			registry.Register(ctx, slave)

			// Mark as offline
			registry.MarkOffline(ctx, "slave-1")

			isHealthy := registry.IsSlaveHealthy(ctx, "slave-1", 30*time.Second)
			return !isHealthy
		},
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// TestMultipleSlaveHealthProperty tests health monitoring with multiple slaves.
func TestMultipleSlaveHealthProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Each slave's health is independent
	properties.Property("slave health is independent", prop.ForAll(
		func(numSlaves int) bool {
			if numSlaves < 2 {
				numSlaves = 2
			}
			if numSlaves > 10 {
				numSlaves = 10
			}

			registry := master.NewInMemorySlaveRegistry()
			ctx := context.Background()

			timeout := 30 * time.Second

			// Register slaves with alternating health
			for i := 0; i < numSlaves; i++ {
				slave := &types.SlaveInfo{
					ID:   string(rune('a' + i)),
					Type: types.SlaveTypeWorker,
				}
				registry.Register(ctx, slave)

				var lastSeen time.Time
				if i%2 == 0 {
					// Healthy - recent heartbeat
					lastSeen = time.Now()
				} else {
					// Unhealthy - old heartbeat
					lastSeen = time.Now().Add(-60 * time.Second)
				}

				status := &types.SlaveStatus{
					State:    types.SlaveStateOnline,
					LastSeen: lastSeen,
				}
				registry.UpdateStatus(ctx, string(rune('a'+i)), status)
			}

			// Verify each slave's health independently
			for i := 0; i < numSlaves; i++ {
				isHealthy := registry.IsSlaveHealthy(ctx, string(rune('a'+i)), timeout)
				expectedHealthy := i%2 == 0

				if isHealthy != expectedHealthy {
					return false
				}
			}

			return true
		},
		gen.IntRange(2, 10),
	))

	properties.TestingRun(t)
}

// BenchmarkHealthCheck benchmarks health checking.
func BenchmarkHealthCheck(b *testing.B) {
	registry := master.NewInMemorySlaveRegistry()
	ctx := context.Background()

	// Register slaves
	for i := 0; i < 100; i++ {
		slave := &types.SlaveInfo{
			ID:   string(rune(i)),
			Type: types.SlaveTypeWorker,
		}
		registry.Register(ctx, slave)
		registry.UpdateStatus(ctx, string(rune(i)), &types.SlaveStatus{
			State:    types.SlaveStateOnline,
			LastSeen: time.Now(),
		})
	}

	timeout := 30 * time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.IsSlaveHealthy(ctx, string(rune(i%100)), timeout)
	}
}

// TestSlaveHealthMonitoringSpecificCases tests specific edge cases.
func TestSlaveHealthMonitoringSpecificCases(t *testing.T) {
	testCases := []struct {
		name           string
		lastSeenAgo    time.Duration
		timeout        time.Duration
		expectedHealth bool
	}{
		{
			name:           "just within timeout",
			lastSeenAgo:    29 * time.Second,
			timeout:        30 * time.Second,
			expectedHealth: true,
		},
		{
			name:           "exactly at timeout",
			lastSeenAgo:    30 * time.Second,
			timeout:        30 * time.Second,
			expectedHealth: false,
		},
		{
			name:           "just beyond timeout",
			lastSeenAgo:    31 * time.Second,
			timeout:        30 * time.Second,
			expectedHealth: false,
		},
		{
			name:           "very recent heartbeat",
			lastSeenAgo:    1 * time.Second,
			timeout:        30 * time.Second,
			expectedHealth: true,
		},
		{
			name:           "very old heartbeat",
			lastSeenAgo:    5 * time.Minute,
			timeout:        30 * time.Second,
			expectedHealth: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			registry := master.NewInMemorySlaveRegistry()
			ctx := context.Background()

			slave := &types.SlaveInfo{
				ID:   "slave-1",
				Type: types.SlaveTypeWorker,
			}
			registry.Register(ctx, slave)

			status := &types.SlaveStatus{
				State:    types.SlaveStateOnline,
				LastSeen: time.Now().Add(-tc.lastSeenAgo),
			}
			registry.UpdateStatus(ctx, "slave-1", status)

			isHealthy := registry.IsSlaveHealthy(ctx, "slave-1", tc.timeout)
			assert.Equal(t, tc.expectedHealth, isHealthy)
		})
	}
}
