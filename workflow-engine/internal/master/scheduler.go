package master

import (
	"context"
	"fmt"
	"sort"

	"yqhp/workflow-engine/pkg/types"
)

// WorkflowScheduler 实现了 Scheduler 接口。
// Requirements: 5.3, 13.1, 13.2, 13.3
type WorkflowScheduler struct {
	registry SlaveRegistry
}

// NewWorkflowScheduler 创建一个新的工作流调度器。
func NewWorkflowScheduler(registry SlaveRegistry) *WorkflowScheduler {
	return &WorkflowScheduler{
		registry: registry,
	}
}

// Schedule 将工作流执行分发到 Slave。
// Requirements: 5.3
func (s *WorkflowScheduler) Schedule(ctx context.Context, workflow *types.Workflow, slaves []*types.SlaveInfo) (*types.ExecutionPlan, error) {
	if workflow == nil {
		return nil, fmt.Errorf("workflow cannot be nil")
	}
	if len(slaves) == 0 {
		return nil, fmt.Errorf("no slaves available for scheduling")
	}

	// 为每个 Slave 计算执行分段
	assignments := s.calculateSegments(workflow, slaves)

	return &types.ExecutionPlan{
		Assignments: assignments,
	}, nil
}

// calculateSegments 为每个 Slave 计算执行分段。
// 分段在所有 Slave 之间均匀分配。
func (s *WorkflowScheduler) calculateSegments(workflow *types.Workflow, slaves []*types.SlaveInfo) []*types.SlaveAssignment {
	numSlaves := len(slaves)
	segmentSize := 1.0 / float64(numSlaves)

	assignments := make([]*types.SlaveAssignment, numSlaves)
	for i, slave := range slaves {
		start := float64(i) * segmentSize
		end := float64(i+1) * segmentSize

		// 确保最后一个分段正好结束于 1.0
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

// Reschedule 处理 Slave 故障时的任务重新分配。
// Requirements: 5.5
func (s *WorkflowScheduler) Reschedule(ctx context.Context, failedSlaveID string, plan *types.ExecutionPlan) (*types.ExecutionPlan, error) {
	if plan == nil {
		return nil, fmt.Errorf("execution plan cannot be nil")
	}

	// 查找失败的分配
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
		return plan, nil // 无需更改
	}

	if len(remainingAssignments) == 0 {
		return nil, fmt.Errorf("no remaining slaves to reschedule work")
	}

	// 将失败的分段重新分配给剩余的 Slave
	failedSegmentSize := failedAssignment.Segment.End - failedAssignment.Segment.Start
	redistributeSize := failedSegmentSize / float64(len(remainingAssignments))

	for i, assignment := range remainingAssignments {
		// 扩展每个剩余分配的分段
		if i == len(remainingAssignments)-1 {
			// 最后一个 Slave 接管所有剩余工作
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

// SelectSlaves 根据选择策略选择 Slave。
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

// selectManual 通过 ID 选择 Slave。
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

		// 检查 Slave 是否在线
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

// selectByLabel 通过标签选择 Slave。
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

// selectByCapability 通过能力选择 Slave。
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

// selectAuto 基于负载均衡自动选择 Slave。
// Requirements: 13.4
func (s *WorkflowScheduler) selectAuto(ctx context.Context, selector *types.SlaveSelector) ([]*types.SlaveInfo, error) {
	// 获取所有在线的 Slave
	slaves, err := s.registry.GetOnlineSlaves(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get online slaves: %w", err)
	}

	if len(slaves) == 0 {
		return nil, fmt.Errorf("no online slaves available")
	}

	// 按负载排序（最低优先）
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

	// 应用最小/最大约束
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

	// 选择最多 maxSlaves 个 Slave
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

// slaveWithLoad 将 Slave 与其当前负载配对。
type slaveWithLoad struct {
	slave *types.SlaveInfo
	load  float64
}

// CalculateVUsPerSlave 计算每个 Slave 应处理的 VU 数量。
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

// CalculateIterationsPerSlave 计算每个 Slave 应处理的迭代次数。
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
