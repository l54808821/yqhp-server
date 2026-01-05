// Package integration provides property-based tests for capability-based routing.
// Requirements: 11.3, 12.3 - Capability-based routing correctness
// Property 9: For any step with capability requirements and a set of slaves with declared
// capabilities, the step should only be routed to slaves that have all required capabilities.
package integration

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"

	"yqhp/workflow-engine/internal/master"
	"yqhp/workflow-engine/pkg/types"
)

// TestCapabilityBasedRoutingProperty tests Property 9: Capability-based routing correctness.
// routed_slave.capabilities ⊇ step.required_capabilities
func TestCapabilityBasedRoutingProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Selected slaves have all required capabilities
	properties.Property("selected slaves have required capabilities", prop.ForAll(
		func(requiredCaps []string, slaveCaps [][]string) bool {
			if len(requiredCaps) == 0 || len(slaveCaps) == 0 {
				return true
			}

			registry := master.NewInMemorySlaveRegistry()
			scheduler := master.NewWorkflowScheduler(registry)
			ctx := context.Background()

			// Register slaves with different capabilities
			for i, caps := range slaveCaps {
				slave := &types.SlaveInfo{
					ID:           string(rune('a' + i)),
					Type:         types.SlaveTypeWorker,
					Capabilities: caps,
				}
				registry.Register(ctx, slave)
			}

			selector := &types.SlaveSelector{
				Mode:         types.SelectionModeCapability,
				Capabilities: requiredCaps,
			}

			selected, err := scheduler.SelectSlaves(ctx, selector)
			if err != nil {
				// No slaves with required capabilities - this is valid
				return true
			}

			// Verify all selected slaves have all required capabilities
			for _, slave := range selected {
				if !hasAllCapabilities(slave.Capabilities, requiredCaps) {
					return false
				}
			}

			return true
		},
		genCapabilities(),
		gen.SliceOfN(5, genCapabilities()),
	))

	// Property: Slaves without required capabilities are not selected
	properties.Property("slaves without capabilities are not selected", prop.ForAll(
		func(requiredCap string) bool {
			registry := master.NewInMemorySlaveRegistry()
			scheduler := master.NewWorkflowScheduler(registry)
			ctx := context.Background()

			// Register slave without the required capability
			slave := &types.SlaveInfo{
				ID:           "slave-1",
				Type:         types.SlaveTypeWorker,
				Capabilities: []string{"other_capability"},
			}
			registry.Register(ctx, slave)

			selector := &types.SlaveSelector{
				Mode:         types.SelectionModeCapability,
				Capabilities: []string{requiredCap},
			}

			selected, err := scheduler.SelectSlaves(ctx, selector)

			// Should either error or return empty
			return err != nil || len(selected) == 0
		},
		gen.OneConstOf("http_executor", "script_executor", "grpc_executor"),
	))

	// Property: All matching slaves are included in selection
	properties.Property("all matching slaves are included", prop.ForAll(
		func(numSlaves int) bool {
			if numSlaves < 1 {
				numSlaves = 1
			}
			if numSlaves > 10 {
				numSlaves = 10
			}

			registry := master.NewInMemorySlaveRegistry()
			scheduler := master.NewWorkflowScheduler(registry)
			ctx := context.Background()

			requiredCap := "http_executor"

			// Register slaves all with the required capability
			for i := 0; i < numSlaves; i++ {
				slave := &types.SlaveInfo{
					ID:           string(rune('a' + i)),
					Type:         types.SlaveTypeWorker,
					Capabilities: []string{requiredCap},
				}
				registry.Register(ctx, slave)
			}

			selector := &types.SlaveSelector{
				Mode:         types.SelectionModeCapability,
				Capabilities: []string{requiredCap},
			}

			selected, err := scheduler.SelectSlaves(ctx, selector)
			if err != nil {
				return false
			}

			// All slaves should be selected
			return len(selected) == numSlaves
		},
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

// TestMultipleCapabilitiesProperty tests selection with multiple required capabilities.
func TestMultipleCapabilitiesProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Slave must have ALL required capabilities
	properties.Property("slave must have all required capabilities", prop.ForAll(
		func(numRequired int) bool {
			if numRequired < 1 {
				numRequired = 1
			}
			if numRequired > 5 {
				numRequired = 5
			}

			registry := master.NewInMemorySlaveRegistry()
			scheduler := master.NewWorkflowScheduler(registry)
			ctx := context.Background()

			allCaps := []string{"cap1", "cap2", "cap3", "cap4", "cap5"}
			requiredCaps := allCaps[:numRequired]

			// Register slave with only some capabilities
			partialSlave := &types.SlaveInfo{
				ID:           "partial",
				Type:         types.SlaveTypeWorker,
				Capabilities: allCaps[:numRequired-1], // Missing one
			}
			registry.Register(ctx, partialSlave)

			// Register slave with all capabilities
			fullSlave := &types.SlaveInfo{
				ID:           "full",
				Type:         types.SlaveTypeWorker,
				Capabilities: allCaps,
			}
			registry.Register(ctx, fullSlave)

			selector := &types.SlaveSelector{
				Mode:         types.SelectionModeCapability,
				Capabilities: requiredCaps,
			}

			selected, err := scheduler.SelectSlaves(ctx, selector)
			if err != nil {
				return false
			}

			// Only full slave should be selected
			if len(selected) != 1 {
				return false
			}
			return selected[0].ID == "full"
		},
		gen.IntRange(2, 5),
	))

	properties.TestingRun(t)
}

// Helper functions

// hasAllCapabilities checks if slave has all required capabilities.
func hasAllCapabilities(slaveCaps, requiredCaps []string) bool {
	capSet := make(map[string]bool)
	for _, cap := range slaveCaps {
		capSet[cap] = true
	}

	for _, required := range requiredCaps {
		if !capSet[required] {
			return false
		}
	}
	return true
}

// genCapabilities generates a list of capabilities.
func genCapabilities() gopter.Gen {
	return gen.SliceOfN(3, gen.OneConstOf(
		"http_executor",
		"script_executor",
		"grpc_executor",
		"websocket_executor",
		"database_executor",
	)).Map(func(caps []string) []string {
		// Remove duplicates
		seen := make(map[string]bool)
		result := make([]string, 0)
		for _, cap := range caps {
			if !seen[cap] {
				seen[cap] = true
				result = append(result, cap)
			}
		}
		return result
	})
}

// BenchmarkCapabilityRouting benchmarks capability-based routing.
func BenchmarkCapabilityRouting(b *testing.B) {
	registry := master.NewInMemorySlaveRegistry()
	scheduler := master.NewWorkflowScheduler(registry)
	ctx := context.Background()

	// Register slaves
	for i := 0; i < 100; i++ {
		caps := []string{"http_executor"}
		if i%2 == 0 {
			caps = append(caps, "script_executor")
		}
		if i%3 == 0 {
			caps = append(caps, "grpc_executor")
		}
		slave := &types.SlaveInfo{
			ID:           string(rune(i)),
			Type:         types.SlaveTypeWorker,
			Capabilities: caps,
		}
		registry.Register(ctx, slave)
	}

	selector := &types.SlaveSelector{
		Mode:         types.SelectionModeCapability,
		Capabilities: []string{"http_executor", "script_executor"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.SelectSlaves(ctx, selector)
	}
}

// TestCapabilityRoutingSpecificCases tests specific edge cases.
func TestCapabilityRoutingSpecificCases(t *testing.T) {
	testCases := []struct {
		name         string
		slaveCaps    [][]string
		requiredCaps []string
		expectedNum  int
	}{
		{
			name: "single capability match",
			slaveCaps: [][]string{
				{"http_executor"},
				{"script_executor"},
			},
			requiredCaps: []string{"http_executor"},
			expectedNum:  1,
		},
		{
			name: "multiple capability match",
			slaveCaps: [][]string{
				{"http_executor", "script_executor"},
				{"http_executor"},
			},
			requiredCaps: []string{"http_executor", "script_executor"},
			expectedNum:  1,
		},
		{
			name: "all slaves match",
			slaveCaps: [][]string{
				{"http_executor"},
				{"http_executor"},
				{"http_executor"},
			},
			requiredCaps: []string{"http_executor"},
			expectedNum:  3,
		},
		{
			name: "superset capabilities match",
			slaveCaps: [][]string{
				{"http_executor", "script_executor", "grpc_executor"},
			},
			requiredCaps: []string{"http_executor"},
			expectedNum:  1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			registry := master.NewInMemorySlaveRegistry()
			scheduler := master.NewWorkflowScheduler(registry)
			ctx := context.Background()

			for i, caps := range tc.slaveCaps {
				slave := &types.SlaveInfo{
					ID:           string(rune('a' + i)),
					Type:         types.SlaveTypeWorker,
					Capabilities: caps,
				}
				registry.Register(ctx, slave)
			}

			selector := &types.SlaveSelector{
				Mode:         types.SelectionModeCapability,
				Capabilities: tc.requiredCaps,
			}

			selected, err := scheduler.SelectSlaves(ctx, selector)
			assert.NoError(t, err)
			assert.Len(t, selected, tc.expectedNum)
		})
	}
}
