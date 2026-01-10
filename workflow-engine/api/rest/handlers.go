// Package rest provides the REST API server for the workflow execution engine.
package rest

import (
	"context"
	"fmt"
	"time"

	"yqhp/workflow-engine/internal/master"
	"yqhp/workflow-engine/internal/parser"
	"yqhp/workflow-engine/pkg/types"

	"github.com/gofiber/fiber/v2"
)

// healthCheck handles GET /health
func (s *Server) healthCheck(c *fiber.Ctx) error {
	return c.JSON(HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// readyCheck handles GET /ready
func (s *Server) readyCheck(c *fiber.Ctx) error {
	ready := s.master != nil
	status := "ready"
	if !ready {
		status = "not_ready"
	}

	return c.JSON(ReadyResponse{
		Ready:     ready,
		Status:    status,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// submitWorkflow handles POST /api/v1/workflows
// Requirements: 7.1
func (s *Server) submitWorkflow(c *fiber.Ctx) error {
	ctx := context.Background()

	var req WorkflowSubmitRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body: " + err.Error(),
		})
	}

	var workflow *types.Workflow

	// If YAML is provided, parse it
	if req.YAML != "" {
		p := parser.NewYAMLParser()
		var err error
		workflow, err = p.Parse([]byte(req.YAML))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
				Error:   "invalid_workflow",
				Message: "Failed to parse workflow YAML: " + err.Error(),
			})
		}
	} else if req.Workflow != nil {
		workflow = req.Workflow
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Either 'workflow' or 'yaml' must be provided",
		})
	}

	// Validate workflow
	if workflow.ID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_workflow",
			Message: "Workflow ID is required",
		})
	}

	// Submit workflow
	executionID, err := s.master.SubmitWorkflow(ctx, workflow)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:   "submission_failed",
			Message: "Failed to submit workflow: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(WorkflowSubmitResponse{
		ExecutionID: executionID,
		WorkflowID:  workflow.ID,
		Status:      "submitted",
	})
}

// getWorkflow handles GET /api/v1/workflows/:id
// Requirements: 7.1
func (s *Server) getWorkflow(c *fiber.Ctx) error {
	ctx := context.Background()
	workflowID := c.Params("id")

	if workflowID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Workflow ID is required",
		})
	}

	// Get execution status by workflow ID
	state, err := s.master.GetExecutionStatus(ctx, workflowID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
			Error:   "not_found",
			Message: "Workflow not found: " + err.Error(),
		})
	}

	return c.JSON(WorkflowResponse{
		ID:     state.WorkflowID,
		Status: string(state.Status),
	})
}

// stopWorkflow handles DELETE /api/v1/workflows/:id
// Requirements: 7.3
func (s *Server) stopWorkflow(c *fiber.Ctx) error {
	ctx := context.Background()
	workflowID := c.Params("id")

	if workflowID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Workflow ID is required",
		})
	}

	if err := s.master.StopExecution(ctx, workflowID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:   "stop_failed",
			Message: "Failed to stop workflow: " + err.Error(),
		})
	}

	return c.JSON(SuccessResponse{
		Success: true,
		Message: "Workflow stopped successfully",
	})
}

// listExecutions handles GET /api/v1/executions
// Requirements: 7.2
func (s *Server) listExecutions(c *fiber.Ctx) error {
	ctx := context.Background()

	// Get the master implementation to access ListExecutions
	wm, ok := s.master.(*workflowMasterWrapper)
	if !ok {
		// Try to cast directly if it's a WorkflowMaster
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:   "internal_error",
			Message: "Master does not support listing executions",
		})
	}

	states, err := wm.ListExecutions(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:   "list_failed",
			Message: "Failed to list executions: " + err.Error(),
		})
	}

	executions := make([]*ExecutionResponse, len(states))
	for i, state := range states {
		executions[i] = toExecutionResponse(state)
	}

	return c.JSON(ExecutionListResponse{
		Executions: executions,
		Total:      len(executions),
	})
}

// getExecution handles GET /api/v1/executions/:id
// Requirements: 7.2
func (s *Server) getExecution(c *fiber.Ctx) error {
	ctx := context.Background()
	executionID := c.Params("id")

	if executionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Execution ID is required",
		})
	}

	state, err := s.master.GetExecutionStatus(ctx, executionID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
			Error:   "not_found",
			Message: "Execution not found: " + err.Error(),
		})
	}

	return c.JSON(toExecutionResponse(state))
}

// pauseExecution handles POST /api/v1/executions/:id/pause
// Requirements: 6.2.5
func (s *Server) pauseExecution(c *fiber.Ctx) error {
	ctx := context.Background()
	executionID := c.Params("id")

	if executionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Execution ID is required",
		})
	}

	if err := s.master.PauseExecution(ctx, executionID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:   "pause_failed",
			Message: "Failed to pause execution: " + err.Error(),
		})
	}

	return c.JSON(SuccessResponse{
		Success: true,
		Message: "Execution paused successfully",
	})
}

// resumeExecution handles POST /api/v1/executions/:id/resume
// Requirements: 6.2.6
func (s *Server) resumeExecution(c *fiber.Ctx) error {
	ctx := context.Background()
	executionID := c.Params("id")

	if executionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Execution ID is required",
		})
	}

	if err := s.master.ResumeExecution(ctx, executionID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:   "resume_failed",
			Message: "Failed to resume execution: " + err.Error(),
		})
	}

	return c.JSON(SuccessResponse{
		Success: true,
		Message: "Execution resumed successfully",
	})
}

// scaleExecution handles POST /api/v1/executions/:id/scale
// Requirements: 6.2.4
func (s *Server) scaleExecution(c *fiber.Ctx) error {
	ctx := context.Background()
	executionID := c.Params("id")

	if executionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Execution ID is required",
		})
	}

	var req ScaleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body: " + err.Error(),
		})
	}

	if req.TargetVUs < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Target VUs cannot be negative",
		})
	}

	if err := s.master.ScaleExecution(ctx, executionID, req.TargetVUs); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:   "scale_failed",
			Message: "Failed to scale execution: " + err.Error(),
		})
	}

	return c.JSON(SuccessResponse{
		Success: true,
		Message: "Execution scaled successfully",
	})
}

// stopExecution handles DELETE /api/v1/executions/:id
// Requirements: 7.3
func (s *Server) stopExecution(c *fiber.Ctx) error {
	ctx := context.Background()
	executionID := c.Params("id")

	if executionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Execution ID is required",
		})
	}

	if err := s.master.StopExecution(ctx, executionID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:   "stop_failed",
			Message: "Failed to stop execution: " + err.Error(),
		})
	}

	return c.JSON(SuccessResponse{
		Success: true,
		Message: "Execution stopped successfully",
	})
}

// getMetrics handles GET /api/v1/executions/:id/metrics
// Requirements: 7.4
func (s *Server) getMetrics(c *fiber.Ctx) error {
	ctx := context.Background()
	executionID := c.Params("id")

	if executionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Execution ID is required",
		})
	}

	metrics, err := s.master.GetMetrics(ctx, executionID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
			Error:   "not_found",
			Message: "Metrics not found: " + err.Error(),
		})
	}

	return c.JSON(toMetricsResponse(metrics))
}

// listSlaves handles GET /api/v1/slaves
// Requirements: 13.2
func (s *Server) listSlaves(c *fiber.Ctx) error {
	ctx := context.Background()

	if s.registry == nil {
		return c.JSON(SlaveListResponse{
			Slaves: []*SlaveResponse{},
			Total:  0,
		})
	}

	slaves, err := s.registry.ListSlaves(ctx, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:   "list_failed",
			Message: "Failed to list slaves: " + err.Error(),
		})
	}

	responses := make([]*SlaveResponse, len(slaves))
	for i, slave := range slaves {
		status, _ := s.registry.GetSlaveStatus(ctx, slave.ID)
		responses[i] = toSlaveResponse(slave, status)
	}

	return c.JSON(SlaveListResponse{
		Slaves: responses,
		Total:  len(responses),
	})
}

// getSlave handles GET /api/v1/slaves/:id
// Requirements: 13.2
func (s *Server) getSlave(c *fiber.Ctx) error {
	ctx := context.Background()
	slaveID := c.Params("id")

	if slaveID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Slave ID is required",
		})
	}

	if s.registry == nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
			Error:   "not_found",
			Message: "Slave not found",
		})
	}

	slave, err := s.registry.GetSlave(ctx, slaveID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
			Error:   "not_found",
			Message: "Slave not found: " + err.Error(),
		})
	}

	status, _ := s.registry.GetSlaveStatus(ctx, slaveID)
	return c.JSON(toSlaveResponse(slave, status))
}

// drainSlave handles POST /api/v1/slaves/:id/drain
// Requirements: 13.4
func (s *Server) drainSlave(c *fiber.Ctx) error {
	ctx := context.Background()
	slaveID := c.Params("id")

	if slaveID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Slave ID is required",
		})
	}

	if s.registry == nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
			Error:   "not_found",
			Message: "Slave not found",
		})
	}

	// Check if registry supports draining
	drainer, ok := s.registry.(interface {
		DrainSlave(ctx context.Context, slaveID string) error
	})
	if !ok {
		return c.Status(fiber.StatusNotImplemented).JSON(ErrorResponse{
			Error:   "not_implemented",
			Message: "Drain operation not supported",
		})
	}

	if err := drainer.DrainSlave(ctx, slaveID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:   "drain_failed",
			Message: "Failed to drain slave: " + err.Error(),
		})
	}

	return c.JSON(SuccessResponse{
		Success: true,
		Message: "Slave drain initiated successfully",
	})
}

// workflowMasterWrapper wraps a master.Master to provide ListExecutions.
type workflowMasterWrapper struct {
	master.Master
	listFn func(ctx context.Context) ([]*types.ExecutionState, error)
}

// ListExecutions returns all executions.
func (w *workflowMasterWrapper) ListExecutions(ctx context.Context) ([]*types.ExecutionState, error) {
	if w.listFn != nil {
		return w.listFn(ctx)
	}
	return nil, nil
}

// WrapMasterWithListExecutions wraps a master with ListExecutions support.
func WrapMasterWithListExecutions(m master.Master, listFn func(ctx context.Context) ([]*types.ExecutionState, error)) master.Master {
	return &workflowMasterWrapper{
		Master: m,
		listFn: listFn,
	}
}

// ============================================================================
// Slave 通信相关的处理函数
// ============================================================================

// registerSlave handles POST /api/v1/slaves/register
// Requirements: 2.1
func (s *Server) registerSlave(c *fiber.Ctx) error {
	ctx := context.Background()

	var req SlaveRegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body: " + err.Error(),
		})
	}

	// 验证必填字段
	if req.SlaveID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Slave ID is required",
		})
	}

	if req.Address == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Address is required",
		})
	}

	if s.registry == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(ErrorResponse{
			Error:   "service_unavailable",
			Message: "Slave registry is not available",
		})
	}

	// 转换为内部类型
	slaveInfo := &types.SlaveInfo{
		ID:           req.SlaveID,
		Type:         types.SlaveType(req.SlaveType),
		Address:      req.Address,
		Capabilities: req.Capabilities,
		Labels:       req.Labels,
	}

	if req.Resources != nil {
		slaveInfo.Resources = &types.ResourceInfo{
			CPUCores:    req.Resources.CPUCores,
			MemoryMB:    req.Resources.MemoryMB,
			MaxVUs:      req.Resources.MaxVUs,
			CurrentLoad: req.Resources.CurrentLoad,
		}
	}

	// 注册 Slave
	if err := s.registry.Register(ctx, slaveInfo); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SlaveRegisterResponse{
			Accepted: false,
			Error:    "Registration failed: " + err.Error(),
		})
	}

	// 返回成功响应
	return c.Status(fiber.StatusCreated).JSON(SlaveRegisterResponse{
		Accepted:          true,
		AssignedID:        req.SlaveID,
		HeartbeatInterval: 5000, // 默认 5 秒心跳间隔
		MasterID:          "master-1",
		Version:           "1.0.0",
	})
}

// slaveHeartbeat handles POST /api/v1/slaves/:id/heartbeat
// Requirements: 2.2
func (s *Server) slaveHeartbeat(c *fiber.Ctx) error {
	ctx := context.Background()
	slaveID := c.Params("id")

	if slaveID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Slave ID is required",
		})
	}

	var req SlaveHeartbeatRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body: " + err.Error(),
		})
	}

	if s.registry == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(ErrorResponse{
			Error:   "service_unavailable",
			Message: "Slave registry is not available",
		})
	}

	// 验证 Slave 是否存在
	_, err := s.registry.GetSlave(ctx, slaveID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
			Error:   "not_found",
			Message: "Slave not found: " + err.Error(),
		})
	}

	// 更新 Slave 状态
	if req.Status != nil {
		status := &types.SlaveStatus{
			State:       types.SlaveState(req.Status.State),
			Load:        req.Status.Load,
			ActiveTasks: req.Status.ActiveTasks,
			LastSeen:    time.Now(),
		}

		if req.Status.Metrics != nil {
			status.Metrics = &types.SlaveMetrics{
				CPUUsage:    req.Status.Metrics.CPUUsage,
				MemoryUsage: req.Status.Metrics.MemoryUsage,
				ActiveVUs:   req.Status.Metrics.ActiveVUs,
				Throughput:  req.Status.Metrics.Throughput,
			}
		}

		if err := s.registry.UpdateStatus(ctx, slaveID, status); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
				Error:   "update_failed",
				Message: "Failed to update slave status: " + err.Error(),
			})
		}
	}

	// 获取待执行的命令（从命令队列）
	commands := s.getPendingCommands(slaveID)

	return c.JSON(SlaveHeartbeatResponse{
		Commands:  commands,
		Timestamp: time.Now().UnixNano(),
	})
}

// getPendingCommands 获取 Slave 的待执行命令
func (s *Server) getPendingCommands(slaveID string) []*ControlCommand {
	s.commandQueuesMu.RLock()
	defer s.commandQueuesMu.RUnlock()

	queue, ok := s.commandQueues[slaveID]
	if !ok || queue == nil {
		return nil
	}

	var commands []*ControlCommand
	// 非阻塞地获取所有待执行命令
	for {
		select {
		case cmd := <-queue:
			commands = append(commands, cmd)
		default:
			return commands
		}
	}
}

// getSlaveTasks handles GET /api/v1/slaves/:id/tasks
// Requirements: 2.5
func (s *Server) getSlaveTasks(c *fiber.Ctx) error {
	ctx := context.Background()
	slaveID := c.Params("id")

	if slaveID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Slave ID is required",
		})
	}

	if s.registry == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(ErrorResponse{
			Error:   "service_unavailable",
			Message: "Slave registry is not available",
		})
	}

	// 验证 Slave 是否存在
	_, err := s.registry.GetSlave(ctx, slaveID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
			Error:   "not_found",
			Message: "Slave not found: " + err.Error(),
		})
	}

	// 获取待执行任务
	tasks := s.getPendingTasks(slaveID)

	return c.JSON(PendingTasksResponse{
		Tasks: tasks,
	})
}

// getPendingTasks 获取 Slave 的待执行任务
func (s *Server) getPendingTasks(slaveID string) []*TaskAssignment {
	s.taskQueuesMu.RLock()
	defer s.taskQueuesMu.RUnlock()

	queue, ok := s.taskQueues[slaveID]
	if !ok || queue == nil {
		return nil
	}

	var tasks []*TaskAssignment
	// 非阻塞地获取所有待执行任务
	for {
		select {
		case task := <-queue:
			tasks = append(tasks, task)
		default:
			return tasks
		}
	}
}

// receiveTaskResult handles POST /api/v1/tasks/:id/result
// Requirements: 2.3
func (s *Server) receiveTaskResult(c *fiber.Ctx) error {
	ctx := context.Background()
	taskID := c.Params("id")

	if taskID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Task ID is required",
		})
	}

	var req TaskResultRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body: " + err.Error(),
		})
	}

	// 验证必填字段
	if req.ExecutionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Execution ID is required",
		})
	}

	if req.SlaveID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Slave ID is required",
		})
	}

	// 处理任务结果
	// 这里可以调用 master 的方法来更新执行状态
	// 目前简单记录并返回成功
	_ = ctx // 预留给后续实现

	return c.JSON(TaskResultResponse{
		Success: true,
		Message: "Task result received successfully",
	})
}

// receiveMetricsReport handles POST /api/v1/executions/:id/metrics/report
// Requirements: 2.4
func (s *Server) receiveMetricsReport(c *fiber.Ctx) error {
	executionID := c.Params("id")

	if executionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Execution ID is required",
		})
	}

	var req MetricsReportRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body: " + err.Error(),
		})
	}

	// 验证必填字段
	if req.SlaveID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Slave ID is required",
		})
	}

	// 处理指标报告
	// 这里可以调用 MetricsAggregator 来聚合指标
	// 目前简单记录并返回成功

	return c.JSON(MetricsReportResponse{
		Success: true,
	})
}

// unregisterSlave handles POST /api/v1/slaves/:id/unregister
// Requirements: 2.1
func (s *Server) unregisterSlave(c *fiber.Ctx) error {
	ctx := context.Background()
	slaveID := c.Params("id")

	if slaveID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:   "invalid_request",
			Message: "Slave ID is required",
		})
	}

	if s.registry == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(ErrorResponse{
			Error:   "service_unavailable",
			Message: "Slave registry is not available",
		})
	}

	// 注销 Slave
	if err := s.registry.Unregister(ctx, slaveID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:   "unregister_failed",
			Message: "Failed to unregister slave: " + err.Error(),
		})
	}

	// 清理任务和命令队列
	s.cleanupSlaveQueues(slaveID)

	return c.JSON(SlaveUnregisterResponse{
		Success: true,
		Message: "Slave unregistered successfully",
	})
}

// cleanupSlaveQueues 清理 Slave 的任务和命令队列
func (s *Server) cleanupSlaveQueues(slaveID string) {
	s.taskQueuesMu.Lock()
	if queue, ok := s.taskQueues[slaveID]; ok {
		close(queue)
		delete(s.taskQueues, slaveID)
	}
	s.taskQueuesMu.Unlock()

	s.commandQueuesMu.Lock()
	if queue, ok := s.commandQueues[slaveID]; ok {
		close(queue)
		delete(s.commandQueues, slaveID)
	}
	s.commandQueuesMu.Unlock()
}

// AssignTask 分配任务给 Slave
func (s *Server) AssignTask(slaveID string, task *TaskAssignment) error {
	s.taskQueuesMu.Lock()
	queue, ok := s.taskQueues[slaveID]
	if !ok {
		queue = make(chan *TaskAssignment, 100)
		s.taskQueues[slaveID] = queue
	}
	s.taskQueuesMu.Unlock()

	select {
	case queue <- task:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending task to slave: %s", slaveID)
	}
}

// SendCommand 发送命令给 Slave
func (s *Server) SendCommand(slaveID string, cmd *ControlCommand) error {
	s.commandQueuesMu.Lock()
	queue, ok := s.commandQueues[slaveID]
	if !ok {
		queue = make(chan *ControlCommand, 100)
		s.commandQueues[slaveID] = queue
	}
	s.commandQueuesMu.Unlock()

	select {
	case queue <- cmd:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending command to slave: %s", slaveID)
	}
}
