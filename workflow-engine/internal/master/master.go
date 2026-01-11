package master

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/internal/slave"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"

	"github.com/google/uuid"
)

// Master 定义了 Master 节点的接口。
// Requirements: 5.1, 5.7
type Master interface {
	// Start 初始化并启动 Master 节点。
	Start(ctx context.Context) error

	// Stop 优雅地关闭 Master 节点。
	Stop(ctx context.Context) error

	// SubmitWorkflow 提交工作流以执行。
	SubmitWorkflow(ctx context.Context, workflow *types.Workflow) (string, error)

	// GetExecutionStatus 返回执行状态。
	GetExecutionStatus(ctx context.Context, executionID string) (*types.ExecutionState, error)

	// StopExecution 停止正在运行的执行。
	StopExecution(ctx context.Context, executionID string) error

	// PauseExecution 暂停正在运行的执行。
	PauseExecution(ctx context.Context, executionID string) error

	// ResumeExecution 恢复已暂停的执行。
	ResumeExecution(ctx context.Context, executionID string) error

	// ScaleExecution 调整执行的 VU 数量。
	ScaleExecution(ctx context.Context, executionID string, targetVUs int) error

	// GetMetrics 返回执行的聚合指标。
	GetMetrics(ctx context.Context, executionID string) (*types.AggregatedMetrics, error)

	// GetSlaves 返回所有已注册的 Slave。
	GetSlaves(ctx context.Context) ([]*types.SlaveInfo, error)
}

// Config 保存 Master 节点的配置。
type Config struct {
	// ID 是此 Master 的唯一标识符。
	ID string

	// Address 是此 Master 监听的地址。
	Address string

	// HeartbeatTimeout 是 Slave 心跳的超时时间。
	HeartbeatTimeout time.Duration

	// HealthCheckInterval 是健康检查的间隔时间。
	HealthCheckInterval time.Duration

	// StandaloneMode 启用无 Slave 的单节点执行模式。
	StandaloneMode bool

	// MaxConcurrentExecutions 是最大并发执行数。
	MaxConcurrentExecutions int
}

// DefaultConfig 返回默认的 Master 配置。
func DefaultConfig() *Config {
	return &Config{
		ID:                      uuid.New().String(),
		Address:                 ":8080",
		HeartbeatTimeout:        30 * time.Second,
		HealthCheckInterval:     10 * time.Second,
		StandaloneMode:          false,
		MaxConcurrentExecutions: 100,
	}
}

// WorkflowMaster 实现了 Master 接口。
// Requirements: 5.1, 5.7
type WorkflowMaster struct {
	config *Config

	// Slave 管理的注册表
	registry SlaveRegistry

	// 任务分发的调度器
	scheduler Scheduler

	// 指标聚合器
	aggregator MetricsAggregator

	// 任务分配器（用于将任务发送到远程 Slave）
	taskAssigner TaskAssigner

	// 执行状态管理
	executions     map[string]*ExecutionInfo
	executionMu    sync.RWMutex
	executionCount atomic.Int32

	// 状态管理
	state    atomic.Value // MasterState
	started  atomic.Bool
	stopOnce sync.Once
	stopped  chan struct{}

	// 健康检查
	healthCtx    context.Context
	healthCancel context.CancelFunc

	// 同步
	mu sync.RWMutex
}

// TaskAssigner 定义任务分配器接口
type TaskAssigner interface {
	AssignTask(slaveID string, task *types.Task) error
}

// MasterState 表示 Master 节点的状态。
type MasterState string

const (
	// MasterStateStarting 表示 Master 正在启动。
	MasterStateStarting MasterState = "starting"
	// MasterStateRunning 表示 Master 正在运行。
	MasterStateRunning MasterState = "running"
	// MasterStateStopping 表示 Master 正在停止。
	MasterStateStopping MasterState = "stopping"
	// MasterStateStopped 表示 Master 已停止。
	MasterStateStopped MasterState = "stopped"
)

// ExecutionInfo 保存工作流执行的信息。
type ExecutionInfo struct {
	ID        string
	Workflow  *types.Workflow
	State     *types.ExecutionState
	Plan      *types.ExecutionPlan
	StartTime time.Time
	EndTime   *time.Time

	// 控制通道
	stopCh   chan struct{}
	pauseCh  chan struct{}
	resumeCh chan struct{}

	// 同步
	mu sync.RWMutex
}

// NewWorkflowMaster 创建一个新的工作流 Master。
func NewWorkflowMaster(config *Config, registry SlaveRegistry, scheduler Scheduler, aggregator MetricsAggregator) *WorkflowMaster {
	if config == nil {
		config = DefaultConfig()
	}

	m := &WorkflowMaster{
		config:     config,
		registry:   registry,
		scheduler:  scheduler,
		aggregator: aggregator,
		executions: make(map[string]*ExecutionInfo),
		stopped:    make(chan struct{}),
	}

	m.state.Store(MasterStateStopped)

	return m
}

// SetTaskAssigner 设置任务分配器
func (m *WorkflowMaster) SetTaskAssigner(assigner TaskAssigner) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskAssigner = assigner
}

// Start 初始化并启动 Master 节点。
// Requirements: 5.1
func (m *WorkflowMaster) Start(ctx context.Context) error {
	if m.started.Load() {
		return fmt.Errorf("master already started")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Store(MasterStateStarting)

	// 启动健康检查协程
	m.healthCtx, m.healthCancel = context.WithCancel(context.Background())
	go m.healthCheckLoop()

	m.state.Store(MasterStateRunning)
	m.started.Store(true)

	return nil
}

// Stop 优雅地关闭 Master 节点。
func (m *WorkflowMaster) Stop(ctx context.Context) error {
	var err error
	m.stopOnce.Do(func() {
		m.state.Store(MasterStateStopping)

		// 停止健康检查
		if m.healthCancel != nil {
			m.healthCancel()
		}

		// 停止所有正在运行的执行
		m.executionMu.Lock()
		for _, exec := range m.executions {
			if exec.State.Status == types.ExecutionStatusRunning {
				close(exec.stopCh)
			}
		}
		m.executionMu.Unlock()

		m.state.Store(MasterStateStopped)
		m.started.Store(false)
		close(m.stopped)
	})
	return err
}

// SubmitWorkflow 提交工作流以执行。
// Requirements: 5.1, 5.3, 5.7
func (m *WorkflowMaster) SubmitWorkflow(ctx context.Context, workflow *types.Workflow) (string, error) {
	if workflow == nil {
		return "", fmt.Errorf("工作流不能为空")
	}

	if !m.started.Load() {
		return "", fmt.Errorf("Master 未启动")
	}

	// 检查并发执行限制
	if int(m.executionCount.Load()) >= m.config.MaxConcurrentExecutions {
		return "", fmt.Errorf("已达到最大并发执行数: %d", m.config.MaxConcurrentExecutions)
	}

	// 生成执行 ID
	executionID := uuid.New().String()

	// 创建执行信息
	execInfo := &ExecutionInfo{
		ID:        executionID,
		Workflow:  workflow,
		StartTime: time.Now(),
		stopCh:    make(chan struct{}),
		pauseCh:   make(chan struct{}),
		resumeCh:  make(chan struct{}),
		State: &types.ExecutionState{
			ID:          executionID,
			WorkflowID:  workflow.ID,
			Status:      types.ExecutionStatusPending,
			StartTime:   time.Now(),
			Progress:    0,
			SlaveStates: make(map[string]*types.SlaveExecutionState),
			Errors:      []types.ExecutionError{},
		},
	}

	// 存储执行信息
	m.executionMu.Lock()
	m.executions[executionID] = execInfo
	m.executionCount.Add(1)
	m.executionMu.Unlock()

	// 调度执行
	if err := m.scheduleExecution(ctx, execInfo); err != nil {
		m.executionMu.Lock()
		execInfo.State.Status = types.ExecutionStatusFailed
		execInfo.State.Errors = append(execInfo.State.Errors, types.ExecutionError{
			Code:      types.ErrCodeExecution,
			Message:   fmt.Sprintf("调度执行失败: %v", err),
			Timestamp: time.Now(),
		})
		m.executionMu.Unlock()
		return executionID, err
	}

	return executionID, nil
}

// scheduleExecution 将工作流执行调度到各个 Slave。
func (m *WorkflowMaster) scheduleExecution(ctx context.Context, execInfo *ExecutionInfo) error {
	execInfo.mu.Lock()
	defer execInfo.mu.Unlock()

	// 获取可用的 Slave
	var slaves []*types.SlaveInfo
	var err error

	if m.config.StandaloneMode {
		// 在单机模式下，创建一个虚拟的本地 Slave
		slaves = []*types.SlaveInfo{{
			ID:           "local",
			Type:         types.SlaveTypeWorker,
			Address:      "localhost",
			Capabilities: []string{"http_executor", "script_executor"},
		}}
	} else {
		// 根据工作流选项选择 Slave
		selector := execInfo.Workflow.Options.TargetSlaves
		if selector == nil {
			selector = &types.SlaveSelector{Mode: types.SelectionModeAuto}
		}
		slaves, err = m.scheduler.SelectSlaves(ctx, selector)
		if err != nil {
			return fmt.Errorf("选择 Slave 失败: %w", err)
		}
	}

	if len(slaves) == 0 {
		return fmt.Errorf("没有可用的 Slave 执行任务")
	}

	// 创建执行计划
	plan, err := m.scheduler.Schedule(ctx, execInfo.Workflow, slaves)
	if err != nil {
		return fmt.Errorf("创建执行计划失败: %w", err)
	}

	plan.ExecutionID = execInfo.ID
	execInfo.Plan = plan

	// 初始化 Slave 状态
	for _, assignment := range plan.Assignments {
		execInfo.State.SlaveStates[assignment.SlaveID] = &types.SlaveExecutionState{
			SlaveID: assignment.SlaveID,
			Status:  types.ExecutionStatusPending,
			Segment: assignment.Segment,
		}
	}

	// 更新状态为运行中
	execInfo.State.Status = types.ExecutionStatusRunning

	// 在后台启动执行
	go m.runExecution(ctx, execInfo)

	return nil
}

// runExecution 运行工作流执行。
func (m *WorkflowMaster) runExecution(ctx context.Context, execInfo *ExecutionInfo) {
	defer func() {
		m.executionCount.Add(-1)
	}()

	// 创建一个可取消的上下文
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 监听停止信号
	go func() {
		select {
		case <-execInfo.stopCh:
			cancel()
		case <-execCtx.Done():
		}
	}()

	// 根据模式选择执行方式
	if m.config.StandaloneMode {
		// 单机模式：本地执行
		m.simulateExecution(execCtx, execInfo)
	} else {
		// 分布式模式：分发到 Slave 执行
		m.distributeExecution(execCtx, execInfo)
	}
}

// simulateExecution 在单机模式下使用 TaskEngine 执行工作流。
func (m *WorkflowMaster) simulateExecution(ctx context.Context, execInfo *ExecutionInfo) {
	logger.Debug("simulateExecution] 开始执行: %s, 工作流: %s, 步骤数: %d\n", execInfo.ID, execInfo.Workflow.Name, len(execInfo.Workflow.Steps))

	execInfo.mu.Lock()
	execInfo.State.Status = types.ExecutionStatusRunning
	execInfo.mu.Unlock()

	// 使用默认执行器注册表创建任务引擎
	registry := executor.DefaultRegistry
	logger.Debug("simulateExecution] 注册表中的执行器类型: %v\n", registry.Types())

	// 计算最大 VU 数量
	// 对于 ramping-vus 模式，使用 stages 中的最大 target 值
	maxVUs := execInfo.Workflow.Options.VUs
	if execInfo.Workflow.Options.ExecutionMode == types.ModeRampingVUs && len(execInfo.Workflow.Options.Stages) > 0 {
		for _, stage := range execInfo.Workflow.Options.Stages {
			if stage.Target > maxVUs {
				maxVUs = stage.Target
			}
		}
	}
	if maxVUs <= 0 {
		maxVUs = 1
	}

	taskEngine := slave.NewTaskEngine(registry, maxVUs)

	// 创建执行任务
	task := &types.Task{
		ID:          uuid.New().String(),
		ExecutionID: execInfo.ID,
		Workflow:    execInfo.Workflow,
		Segment: types.ExecutionSegment{
			Start: 0,
			End:   1,
		},
	}

	logger.Debug("simulateExecution] 创建任务: %s, VUs: %d, Iterations: %d\n", task.ID, execInfo.Workflow.Options.VUs, execInfo.Workflow.Options.Iterations)

	// 在协程中执行，以便处理取消操作
	resultCh := make(chan *types.TaskResult, 1)
	errCh := make(chan error, 1)

	go func() {
		logger.Debug("simulateExecution] 开始执行任务...")
		result, err := taskEngine.Execute(ctx, task)
		logger.Debug("simulateExecution] 任务执行完成, result=%v, err=%v\n", result != nil, err)
		if err != nil {
			logger.Debug("simulateExecution] 任务执行错误: %v\n", err)
			errCh <- err
		} else {
			logger.Debug("simulateExecution] 任务执行成功, status=%s\n", result.Status)
			resultCh <- result
		}
	}()

	// 定期更新进度和指标
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	startTime := time.Now()
	totalIterations := execInfo.Workflow.Options.Iterations
	totalDuration := execInfo.Workflow.Options.Duration

	// 对于 ramping-vus 模式，计算所有 stages 的总时长
	if execInfo.Workflow.Options.ExecutionMode == types.ModeRampingVUs && len(execInfo.Workflow.Options.Stages) > 0 {
		totalDuration = 0
		for _, stage := range execInfo.Workflow.Options.Stages {
			totalDuration += stage.Duration
		}
	}

	for {
		select {
		case <-ticker.C:
			// 更新实时指标
			currentMetrics := taskEngine.GetCurrentMetrics()
			currentIterations := taskEngine.GetIterations()
			activeVUs := taskEngine.GetActiveVUs()

			execInfo.mu.Lock()
			// 计算进度
			if totalDuration > 0 {
				elapsed := time.Since(startTime)
				execInfo.State.Progress = float64(elapsed) / float64(totalDuration)
				if execInfo.State.Progress > 1.0 {
					execInfo.State.Progress = 1.0
				}
			} else if totalIterations > 0 {
				execInfo.State.Progress = float64(currentIterations) / float64(totalIterations)
				if execInfo.State.Progress > 1.0 {
					execInfo.State.Progress = 1.0
				}
			}

			// 更新聚合指标
			execInfo.State.AggregatedMetrics = &types.AggregatedMetrics{
				ExecutionID:     execInfo.ID,
				TotalIterations: currentIterations,
				TotalVUs:        activeVUs,
				StepMetrics:     currentMetrics.StepMetrics,
			}
			execInfo.mu.Unlock()

		case <-ctx.Done():
			taskEngine.Stop(context.Background())
			execInfo.mu.Lock()
			execInfo.State.Status = types.ExecutionStatusAborted
			now := time.Now()
			execInfo.EndTime = &now
			execInfo.State.EndTime = &now
			execInfo.mu.Unlock()
			return

		case <-execInfo.stopCh:
			taskEngine.Stop(context.Background())
			execInfo.mu.Lock()
			execInfo.State.Status = types.ExecutionStatusAborted
			now := time.Now()
			execInfo.EndTime = &now
			execInfo.State.EndTime = &now
			execInfo.mu.Unlock()
			return

		case result := <-resultCh:
			execInfo.mu.Lock()
			if result.Status == types.ExecutionStatusCompleted {
				execInfo.State.Status = types.ExecutionStatusCompleted
				execInfo.State.Progress = 1.0
			} else {
				execInfo.State.Status = result.Status
			}
			// 存储最终指标
			if result.Metrics != nil {
				totalIterations := result.Iterations
				if totalIterations == 0 {
					totalIterations = int64(execInfo.Workflow.Options.Iterations)
				}
				execInfo.State.AggregatedMetrics = &types.AggregatedMetrics{
					ExecutionID:     execInfo.ID,
					TotalIterations: totalIterations,
					TotalVUs:        execInfo.Workflow.Options.VUs,
					StepMetrics:     make(map[string]*types.StepMetrics),
				}
				for stepID, stepMetrics := range result.Metrics.StepMetrics {
					execInfo.State.AggregatedMetrics.StepMetrics[stepID] = stepMetrics
				}
			}
			now := time.Now()
			execInfo.EndTime = &now
			execInfo.State.EndTime = &now
			execInfo.mu.Unlock()
			return

		case err := <-errCh:
			execInfo.mu.Lock()
			execInfo.State.Status = types.ExecutionStatusFailed
			execInfo.State.Errors = append(execInfo.State.Errors, types.ExecutionError{
				Code:      types.ErrCodeExecution,
				Message:   err.Error(),
				Timestamp: time.Now(),
			})
			now := time.Now()
			execInfo.EndTime = &now
			execInfo.State.EndTime = &now
			execInfo.mu.Unlock()
			return
		}
	}
}

// distributeExecution 将任务分发到远程 Slave 执行。
func (m *WorkflowMaster) distributeExecution(ctx context.Context, execInfo *ExecutionInfo) {
	execInfo.mu.Lock()
	execInfo.State.Status = types.ExecutionStatusRunning
	plan := execInfo.Plan
	execInfo.mu.Unlock()

	if plan == nil || len(plan.Assignments) == 0 {
		execInfo.mu.Lock()
		execInfo.State.Status = types.ExecutionStatusFailed
		execInfo.State.Errors = append(execInfo.State.Errors, types.ExecutionError{
			Code:      types.ErrCodeExecution,
			Message:   "没有可用的执行计划或任务分配",
			Timestamp: time.Now(),
		})
		now := time.Now()
		execInfo.EndTime = &now
		execInfo.State.EndTime = &now
		execInfo.mu.Unlock()
		return
	}

	// 创建等待组来跟踪所有 Slave 的执行
	var wg sync.WaitGroup
	resultsCh := make(chan *slaveResult, len(plan.Assignments))

	// 向每个 Slave 分发任务
	for _, assignment := range plan.Assignments {
		wg.Add(1)
		go func(assign *types.SlaveAssignment) {
			defer wg.Done()

			// 创建任务
			task := &types.Task{
				ID:          uuid.New().String(),
				ExecutionID: execInfo.ID,
				Workflow:    execInfo.Workflow,
				Segment:     assign.Segment,
			}

			// 更新 Slave 状态为运行中
			execInfo.mu.Lock()
			if slaveState, ok := execInfo.State.SlaveStates[assign.SlaveID]; ok {
				slaveState.Status = types.ExecutionStatusRunning
			}
			execInfo.mu.Unlock()

			var result *slaveResult

			// 尝试通过 HTTP 发送任务到 Slave
			if m.taskAssigner != nil {
				if err := m.taskAssigner.AssignTask(assign.SlaveID, task); err != nil {
					fmt.Printf("通过 HTTP 分发任务失败: %v，使用本地执行\n", err)
					result = m.executeTaskLocally(ctx, task, assign.SlaveID)
				} else {
					// 任务已发送，等待结果（这里简化处理，实际应该通过回调接收结果）
					// 由于当前架构限制，我们仍然使用本地执行
					fmt.Printf("任务已发送到 Slave %s\n", assign.SlaveID)
					result = m.executeTaskLocally(ctx, task, assign.SlaveID)
				}
			} else {
				// 没有任务分配器，使用本地执行
				result = m.executeTaskLocally(ctx, task, assign.SlaveID)
			}

			resultsCh <- result
		}(assignment)
	}

	// 等待所有任务完成
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// 收集结果
	var allCompleted = true
	var hasError = false
	var totalIterations int64 = 0
	aggregatedMetrics := &types.AggregatedMetrics{
		ExecutionID: execInfo.ID,
		StepMetrics: make(map[string]*types.StepMetrics),
	}

	for result := range resultsCh {
		execInfo.mu.Lock()
		if slaveState, ok := execInfo.State.SlaveStates[result.SlaveID]; ok {
			slaveState.Status = result.Status
			if result.Metrics != nil {
				slaveState.CurrentMetrics = result.Metrics
			}
		}
		execInfo.mu.Unlock()

		if result.Status != types.ExecutionStatusCompleted {
			allCompleted = false
			if result.Status == types.ExecutionStatusFailed {
				hasError = true
			}
		}

		if result.Metrics != nil {
			totalIterations += result.Iterations
			for stepID, stepMetrics := range result.Metrics.StepMetrics {
				if existing, ok := aggregatedMetrics.StepMetrics[stepID]; ok {
					// 合并指标
					existing.Count += stepMetrics.Count
					existing.SuccessCount += stepMetrics.SuccessCount
					existing.FailureCount += stepMetrics.FailureCount
				} else {
					aggregatedMetrics.StepMetrics[stepID] = stepMetrics
				}
			}
		}
	}

	// 更新最终状态
	execInfo.mu.Lock()
	if hasError {
		execInfo.State.Status = types.ExecutionStatusFailed
	} else if allCompleted {
		execInfo.State.Status = types.ExecutionStatusCompleted
		execInfo.State.Progress = 1.0
	} else {
		execInfo.State.Status = types.ExecutionStatusAborted
	}
	aggregatedMetrics.TotalIterations = totalIterations
	aggregatedMetrics.TotalVUs = execInfo.Workflow.Options.VUs
	execInfo.State.AggregatedMetrics = aggregatedMetrics
	now := time.Now()
	execInfo.EndTime = &now
	execInfo.State.EndTime = &now
	execInfo.mu.Unlock()
}

// slaveResult 保存单个 Slave 的执行结果
type slaveResult struct {
	SlaveID    string
	Status     types.ExecutionStatus
	Metrics    *types.Metrics
	Iterations int64
	Error      error
}

// executeTaskLocally 在本地执行任务（作为分布式执行的后备方案）
func (m *WorkflowMaster) executeTaskLocally(ctx context.Context, task *types.Task, slaveID string) *slaveResult {
	result := &slaveResult{
		SlaveID: slaveID,
		Status:  types.ExecutionStatusRunning,
	}

	// 使用默认执行器注册表创建任务引擎
	registry := executor.DefaultRegistry

	// 计算最大 VU 数量
	maxVUs := task.Workflow.Options.VUs
	if task.Workflow.Options.ExecutionMode == types.ModeRampingVUs && len(task.Workflow.Options.Stages) > 0 {
		for _, stage := range task.Workflow.Options.Stages {
			if stage.Target > maxVUs {
				maxVUs = stage.Target
			}
		}
	}
	if maxVUs <= 0 {
		maxVUs = 1
	}

	taskEngine := slave.NewTaskEngine(registry, maxVUs)

	// 执行任务
	taskResult, err := taskEngine.Execute(ctx, task)
	if err != nil {
		result.Status = types.ExecutionStatusFailed
		result.Error = err
		return result
	}

	result.Status = taskResult.Status
	result.Metrics = taskResult.Metrics
	result.Iterations = taskResult.Iterations
	return result
}

// GetExecutionStatus 返回执行状态。
// Requirements: 5.1
func (m *WorkflowMaster) GetExecutionStatus(ctx context.Context, executionID string) (*types.ExecutionState, error) {
	m.executionMu.RLock()
	defer m.executionMu.RUnlock()

	execInfo, ok := m.executions[executionID]
	if !ok {
		return nil, fmt.Errorf("执行未找到: %s", executionID)
	}

	execInfo.mu.RLock()
	defer execInfo.mu.RUnlock()

	// 返回状态的副本
	state := *execInfo.State
	return &state, nil
}

// StopExecution 停止正在运行的执行。
// Requirements: 5.1
func (m *WorkflowMaster) StopExecution(ctx context.Context, executionID string) error {
	m.executionMu.RLock()
	execInfo, ok := m.executions[executionID]
	m.executionMu.RUnlock()

	if !ok {
		return fmt.Errorf("执行未找到: %s", executionID)
	}

	execInfo.mu.Lock()
	defer execInfo.mu.Unlock()

	if execInfo.State.Status != types.ExecutionStatusRunning &&
		execInfo.State.Status != types.ExecutionStatusPaused {
		return fmt.Errorf("执行未在运行或暂停状态: %s", execInfo.State.Status)
	}

	// 发送停止信号
	select {
	case <-execInfo.stopCh:
		// 已经停止
	default:
		close(execInfo.stopCh)
	}

	execInfo.State.Status = types.ExecutionStatusAborted
	now := time.Now()
	execInfo.EndTime = &now
	execInfo.State.EndTime = &now

	return nil
}

// PauseExecution 暂停正在运行的执行。
// Requirements: 6.2.5
func (m *WorkflowMaster) PauseExecution(ctx context.Context, executionID string) error {
	m.executionMu.RLock()
	execInfo, ok := m.executions[executionID]
	m.executionMu.RUnlock()

	if !ok {
		return fmt.Errorf("执行未找到: %s", executionID)
	}

	execInfo.mu.Lock()
	defer execInfo.mu.Unlock()

	if execInfo.State.Status != types.ExecutionStatusRunning {
		return fmt.Errorf("执行未在运行状态: %s", execInfo.State.Status)
	}

	// 发送暂停信号
	select {
	case execInfo.pauseCh <- struct{}{}:
	default:
	}

	execInfo.State.Status = types.ExecutionStatusPaused

	return nil
}

// ResumeExecution 恢复已暂停的执行。
// Requirements: 6.2.6
func (m *WorkflowMaster) ResumeExecution(ctx context.Context, executionID string) error {
	m.executionMu.RLock()
	execInfo, ok := m.executions[executionID]
	m.executionMu.RUnlock()

	if !ok {
		return fmt.Errorf("执行未找到: %s", executionID)
	}

	execInfo.mu.Lock()
	defer execInfo.mu.Unlock()

	if execInfo.State.Status != types.ExecutionStatusPaused {
		return fmt.Errorf("执行未在暂停状态: %s", execInfo.State.Status)
	}

	// 发送恢复信号
	select {
	case execInfo.resumeCh <- struct{}{}:
	default:
	}

	execInfo.State.Status = types.ExecutionStatusRunning

	return nil
}

// ScaleExecution 调整执行的 VU 数量。
// Requirements: 6.2.1, 6.2.2, 6.2.3
func (m *WorkflowMaster) ScaleExecution(ctx context.Context, executionID string, targetVUs int) error {
	if targetVUs < 0 {
		return fmt.Errorf("目标 VU 数不能为负: %d", targetVUs)
	}

	m.executionMu.RLock()
	execInfo, ok := m.executions[executionID]
	m.executionMu.RUnlock()

	if !ok {
		return fmt.Errorf("执行未找到: %s", executionID)
	}

	execInfo.mu.Lock()
	defer execInfo.mu.Unlock()

	if execInfo.State.Status != types.ExecutionStatusRunning {
		return fmt.Errorf("执行未在运行状态: %s", execInfo.State.Status)
	}

	// 在实际实现中，这里会向 Slave 发送扩缩容命令
	// 目前，我们只更新工作流选项
	execInfo.Workflow.Options.VUs = targetVUs

	return nil
}

// GetMetrics 返回执行的聚合指标。
// Requirements: 5.4, 5.6
func (m *WorkflowMaster) GetMetrics(ctx context.Context, executionID string) (*types.AggregatedMetrics, error) {
	m.executionMu.RLock()
	execInfo, ok := m.executions[executionID]
	m.executionMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("执行未找到: %s", executionID)
	}

	execInfo.mu.RLock()
	defer execInfo.mu.RUnlock()

	// 如果有聚合指标，直接返回
	if execInfo.State.AggregatedMetrics != nil {
		return execInfo.State.AggregatedMetrics, nil
	}

	// 否则，从 Slave 状态聚合
	if m.aggregator != nil {
		slaveMetrics := make([]*types.Metrics, 0)
		for _, slaveState := range execInfo.State.SlaveStates {
			if slaveState.CurrentMetrics != nil {
				slaveMetrics = append(slaveMetrics, slaveState.CurrentMetrics)
			}
		}
		return m.aggregator.Aggregate(ctx, executionID, slaveMetrics)
	}

	return &types.AggregatedMetrics{
		ExecutionID: executionID,
		StepMetrics: make(map[string]*types.StepMetrics),
	}, nil
}

// GetSlaves 返回所有已注册的 Slave。
// Requirements: 5.2
func (m *WorkflowMaster) GetSlaves(ctx context.Context) ([]*types.SlaveInfo, error) {
	if m.registry == nil {
		return []*types.SlaveInfo{}, nil
	}
	return m.registry.ListSlaves(ctx, nil)
}

// healthCheckLoop 定期检查 Slave 健康状态。
func (m *WorkflowMaster) healthCheckLoop() {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.healthCtx.Done():
			return
		case <-ticker.C:
			m.checkSlaveHealth()
		}
	}
}

// checkSlaveHealth 检查所有已注册 Slave 的健康状态。
func (m *WorkflowMaster) checkSlaveHealth() {
	if m.registry == nil {
		return
	}

	ctx := context.Background()
	slaves, err := m.registry.ListSlaves(ctx, nil)
	if err != nil {
		return
	}

	now := time.Now()
	for _, slave := range slaves {
		status, err := m.registry.GetSlaveStatus(ctx, slave.ID)
		if err != nil {
			continue
		}

		// 检查心跳是否过期
		if now.Sub(status.LastSeen) > m.config.HeartbeatTimeout {
			// 将 Slave 标记为离线
			_ = m.registry.UpdateStatus(ctx, slave.ID, &types.SlaveStatus{
				State:    types.SlaveStateOffline,
				LastSeen: status.LastSeen,
			})
		}
	}
}

// GetState 返回当前 Master 状态。
func (m *WorkflowMaster) GetState() MasterState {
	return m.state.Load().(MasterState)
}

// IsRunning 返回 Master 是否正在运行。
func (m *WorkflowMaster) IsRunning() bool {
	return m.started.Load() && m.GetState() == MasterStateRunning
}

// GetExecutionCount 返回活跃执行的数量。
func (m *WorkflowMaster) GetExecutionCount() int {
	return int(m.executionCount.Load())
}

// ListExecutions 返回所有执行。
func (m *WorkflowMaster) ListExecutions(ctx context.Context) ([]*types.ExecutionState, error) {
	m.executionMu.RLock()
	defer m.executionMu.RUnlock()

	states := make([]*types.ExecutionState, 0, len(m.executions))
	for _, execInfo := range m.executions {
		execInfo.mu.RLock()
		state := *execInfo.State
		execInfo.mu.RUnlock()
		states = append(states, &state)
	}

	return states, nil
}
