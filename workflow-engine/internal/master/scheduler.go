package master

import (
	"context"
	"fmt"
	"sort"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// WorkflowScheduler implements the Scheduler interface.
// Requirements: 5.3, 13.1, 13.2, 13.3
type WorkflowScheduler struct {
	registry SlaveRegistry
}

// NewWorkflowScheduler creates a new workflow scheduler.
func NewWorkflowScheduler(registry SlaveRegistry) *WorkflowScheduler {
	return &WorkflowScheduler{
		registry: registry,
	}
}

// Schedule distributes workflow execution to slaves.
// Requirements: 5.3
func (s *WorkflowScheduler) Schedule(ctx context.Context, workflow *types.Workflow, slaves []*types.SlaveInfo) (*types.ExecutionPlan, error) {
	if workflow == nil {
		return nil, fmt.Errorf("workflow cannot be nil")
	}
	if len(slaves) == 0 {
		return nil, fmt.Errorf("no slaves available for scheduling")
	}

	// Calculate execution segments for each slave
	assignments := s.calculateSegments(workflow, slaves)

	return &types.ExecutionPlan{
		Assignments: assignments,
	}, nil
}

// calculateSegments calculates execution segments for each slave.
// The segments are distributed evenly across all slaves.
func (s *WorkflowScheduler) calculateSegments(workflow *types.Workflow, slaves []*types.SlaveInfo) []*types.SlaveAssignment {
	numSlaves := len(slaves)
	segmentSize := 1.0 / float64(numSlaves)

	assignments := make([]*types.SlaveAssignment, numSlaves)
	for i, slave := range slaves {
		start := float64(i) * segmentSize
		end := float64(i+1) * segmentSize

		// Ensure the last segment ends exactly at 1.0
		if i == numSlaves-1 {
			end = 1.0
		}

		assignments[i] = &types.SlaveAssignment{
			SlaveID:  slave.ID,
			Workflow: workflow,
			Segment: types.ExecutionSegment{
				Start: start,
				End:   end,
			},
		}
	}

	return assignments
}

// Reschedule handles task redistribution on slave failure.
// Requirements: 5.5
func (s *WorkflowScheduler) Reschedule(ctx context.Context, failedSlaveID string, plan *types.ExecutionPlan) (*types.ExecutionPlan, error) {
	if plan == nil {
		return nil, fmt.Errorf("execution plan cannot be nil")
	}

	// Find the failed assignment
	var failedAssignment *types.SlaveAssignment
	remainingAssignments := make([]*types.SlaveAssignment, 0)

	for _, assignment := range plan.Assignments {
		if assignment.SlaveID == failedSlaveID {
			failedAssignment = assignment
		} else {
			remainingAssignments = append(remainingAssignments, assignment)
		}
	}

	if failedAssignment == nil {
		return plan, nil // No change needed
	}

	if len(remainingAssignments) == 0 {
		return nil, fmt.Errorf("no remaining slaves to reschedule work")
	}

	// Redistribute the failed segment among remaining slaves
	failedSegmentSize := failedAssignment.Segment.End - failedAssignment.Segment.Start
	redistributeSize := failedSegmentSize / float64(len(remainingAssignments))

	for i, assignment := range remainingAssignments {
		// Extend each remaining assignment's segment
		if i == len(remainingAssignments)-1 {
			// Last slave takes any remaining work
			assignment.Segment.End = 1.0
		} else {
			assignment.Segment.End += redistributeSize
		}
	}

	return &types.ExecutionPlan{
		ExecutionID: plan.ExecutionID,
		Assignments: remainingAssignments,
	}, nil
}

// SelectSlaves selects slaves based on the selection strategy.
// Requirements: 13.1, 13.2, 13.3
func (s *WorkflowScheduler) SelectSlaves(ctx context.Context, selector *types.SlaveSelector) ([]*types.SlaveInfo, error) {
	if selector == nil {
		selector = &types.SlaveSelector{Mode: types.SelectionModeAuto}
	}

	switch selector.Mode {
	case types.SelectionModeManual:
		return s.selectManual(ctx, selector)
	case types.SelectionModeLabel:
		return s.selectByLabel(ctx, selector)
	case types.SelectionModeCapability:
		return s.selectByCapability(ctx, selector)
	case types.SelectionModeAuto:
		return s.selectAuto(ctx, selector)
	default:
		return nil, fmt.Errorf("unknown selection mode: %s", selector.Mode)
	}
}

// selectManual selects slaves by their IDs.
// Requirements: 13.1
func (s *WorkflowScheduler) selectManual(ctx context.Context, selector *types.SlaveSelector) ([]*types.SlaveInfo, error) {
	if len(selector.SlaveIDs) == 0 {
		return nil, fmt.Errorf("no slave IDs specified for manual selection")
	}

	slaves := make([]*types.SlaveInfo, 0, len(selector.SlaveIDs))
	for _, id := range selector.SlaveIDs {
		slave, err := s.registry.GetSlave(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("slave not found: %s", id)
		}

		// Check if slave is online
		status, err := s.registry.GetSlaveStatus(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get slave status: %s", id)
		}
		if status.State != types.SlaveStateOnline {
			return nil, fmt.Errorf("slave is not online: %s (state: %s)", id, status.State)
		}

		slaves = append(slaves, slave)
	}

	return slaves, nil
}

// selectByLabel selects slaves by their labels.
// Requirements: 13.2
func (s *WorkflowScheduler) selectByLabel(ctx context.Context, selector *types.SlaveSelector) ([]*types.SlaveInfo, error) {
	if len(selector.Labels) == 0 {
		return nil, fmt.Errorf("no labels specified for label selection")
	}

	filter := &SlaveFilter{
		Labels: selector.Labels,
		States: []types.SlaveState{types.SlaveStateOnline},
	}

	slaves, err := s.registry.ListSlaves(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list slaves: %w", err)
	}

	if len(slaves) == 0 {
		return nil, fmt.Errorf("no slaves found matching labels: %v", selector.Labels)
	}

	return slaves, nil
}

// selectByCapability selects slaves by their capabilities.
// Requirements: 13.3
func (s *WorkflowScheduler) selectByCapability(ctx context.Context, selector *types.SlaveSelector) ([]*types.SlaveInfo, error) {
	if len(selector.Capabilities) == 0 {
		return nil, fmt.Errorf("no capabilities specified for capability selection")
	}

	filter := &SlaveFilter{
		Capabilities: selector.Capabilities,
		States:       []types.SlaveState{types.SlaveStateOnline},
	}

	slaves, err := s.registry.ListSlaves(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list slaves: %w", err)
	}

	if len(slaves) == 0 {
		return nil, fmt.Errorf("no slaves found with capabilities: %v", selector.Capabilities)
	}

	return slaves, nil
}

// selectAuto automatically selects slaves based on load balancing.
// Requirements: 13.4
func (s *WorkflowScheduler) selectAuto(ctx context.Context, selector *types.SlaveSelector) ([]*types.SlaveInfo, error) {
	// Get all online slaves
	slaves, err := s.registry.GetOnlineSlaves(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get online slaves: %w", err)
	}

	if len(slaves) == 0 {
		return nil, fmt.Errorf("no online slaves available")
	}

	// Sort by load (lowest first)
	slavesWithLoad := make([]slaveWithLoad, len(slaves))
	for i, slave := range slaves {
		status, _ := s.registry.GetSlaveStatus(ctx, slave.ID)
		load := float64(0)
		if status != nil {
			load = status.Load
		}
		slavesWithLoad[i] = slaveWithLoad{slave: slave, load: load}
	}

	sort.Slice(slavesWithLoad, func(i, j int) bool {
		return slavesWithLoad[i].load < slavesWithLoad[j].load
	})

	// Apply min/max constraints
	minSlaves := selector.MinSlaves
	maxSlaves := selector.MaxSlaves

	if minSlaves <= 0 {
		minSlaves = 1
	}
	if maxSlaves <= 0 || maxSlaves > len(slavesWithLoad) {
		maxSlaves = len(slavesWithLoad)
	}

	if len(slavesWithLoad) < minSlaves {
		return nil, fmt.Errorf("not enough slaves available: need %d, have %d", minSlaves, len(slavesWithLoad))
	}

	// Select slaves up to maxSlaves
	selectedCount := maxSlaves
	if selectedCount > len(slavesWithLoad) {
		selectedCount = len(slavesWithLoad)
	}

	result := make([]*types.SlaveInfo, selectedCount)
	for i := 0; i < selectedCount; i++ {
		result[i] = slavesWithLoad[i].slave
	}

	return result, nil
}

// slaveWithLoad pairs a slave with its current load.
type slaveWithLoad struct {
	slave *types.SlaveInfo
	load  float64
}

// CalculateVUsPerSlave calculates how many VUs each slave should handle.
func (s *WorkflowScheduler) CalculateVUsPerSlave(totalVUs int, numSlaves int) []int {
	if numSlaves <= 0 {
		return []int{}
	}

	baseVUs := totalVUs / numSlaves
	remainder := totalVUs % numSlaves

	vusPerSlave := make([]int, numSlaves)
	for i := 0; i < numSlaves; i++ {
		vusPerSlave[i] = baseVUs
		if i < remainder {
			vusPerSlave[i]++
		}
	}

	return vusPerSlave
}

// CalculateIterationsPerSlave calculates how many iterations each slave should handle.
func (s *WorkflowScheduler) CalculateIterationsPerSlave(totalIterations int64, numSlaves int) []int64 {
	if numSlaves <= 0 {
		return []int64{}
	}

	baseIters := totalIterations / int64(numSlaves)
	remainder := totalIterations % int64(numSlaves)

	itersPerSlave := make([]int64, numSlaves)
	for i := 0; i < numSlaves; i++ {
		itersPerSlave[i] = baseIters
		if int64(i) < remainder {
			itersPerSlave[i]++
		}
	}

	return itersPerSlave
}
