package slave

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"yqhp/workflow-engine/api/rest"
	httpclient "yqhp/workflow-engine/api/rest/client"
	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

// Slave 定义了 Slave 节点的接口。
// Requirements: 12.1, 12.4
type Slave interface {
	// Start 初始化并启动 Slave 节点。
	Start(ctx context.Context) error

	// Stop 优雅地关闭 Slave 节点。
	Stop(ctx context.Context) error

	// Connect 连接到 Master 节点。
	Connect(ctx context.Context, masterAddr string) error

	// Disconnect 断开与 Master 节点的连接。
	Disconnect(ctx context.Context) error

	// ExecuteTask 执行分配的任务。
	ExecuteTask(ctx context.Context, task *types.Task) (*types.TaskResult, error)

	// GetStatus 返回当前 Slave 状态。
	GetStatus() *types.SlaveStatus

	// GetInfo 返回 Slave 注册信息。
	GetInfo() *types.SlaveInfo
}

// Config 保存 Slave 节点的配置信息。
type Config struct {
	// ID 是此 Slave 的唯一标识符。
	ID string

	// Type 是 Slave 的类型（worker、gateway、aggregator）。
	Type types.SlaveType

	// Address 是此 Slave 监听的地址。
	Address string

	// MasterAddress 是 Master 节点的地址。
	MasterAddress string

	// Capabilities 是此 Slave 支持的能力列表。
	Capabilities []string

	// Labels 是此 Slave 的键值标签。
	Labels map[string]string

	// HeartbeatInterval 是心跳发送间隔。
	HeartbeatInterval time.Duration

	// HeartbeatTimeout 是心跳响应超时时间。
	HeartbeatTimeout time.Duration

	// MaxVUs 是此 Slave 可处理的最大虚拟用户数。
	MaxVUs int

	// CPUCores 是可用的 CPU 核心数。
	CPUCores int

	// MemoryMB 是可用内存大小（单位：MB）。
	MemoryMB int64
}

// DefaultConfig 返回默认的 Slave 配置。
func DefaultConfig() *Config {
	return &Config{
		Type:              types.SlaveTypeWorker,
		HeartbeatInterval: 5 * time.Second,
		HeartbeatTimeout:  10 * time.Second,
		MaxVUs:            100,
		CPUCores:          4,
		MemoryMB:          4096,
	}
}

// WorkerSlave 实现了用于执行工作流的 Slave 接口。
// Requirements: 12.1, 12.4
type WorkerSlave struct {
	config   *Config
	registry *executor.Registry

	// HTTP 客户端
	httpClient *httpclient.Client

	// 状态管理
	state       atomic.Value // types.SlaveState
	connected   atomic.Bool
	masterAddr  string
	activeTasks atomic.Int32
	currentLoad atomic.Value // float64

	// 心跳管理
	heartbeatCtx    context.Context
	heartbeatCancel context.CancelFunc
	lastHeartbeat   atomic.Value // time.Time

	// 指标数据
	metrics *types.SlaveMetrics

	// 任务执行
	taskEngine *TaskEngine

	// 同步控制
	mu       sync.RWMutex
	stopOnce sync.Once
	stopped  chan struct{}
}

// NewWorkerSlave 创建一个新的 Worker Slave。
func NewWorkerSlave(config *Config, registry *executor.Registry) *WorkerSlave {
	if config == nil {
		config = DefaultConfig()
	}

	s := &WorkerSlave{
		config:   config,
		registry: registry,
		metrics:  &types.SlaveMetrics{},
		stopped:  make(chan struct{}),
	}

	s.state.Store(types.SlaveStateOffline)
	s.currentLoad.Store(float64(0))
	s.lastHeartbeat.Store(time.Time{})

	return s
}

// Start 初始化并启动 Slave 节点。
// Requirements: 12.1
func (s *WorkerSlave) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 初始化任务引擎
	s.taskEngine = NewTaskEngine(s.registry, s.config.MaxVUs)

	// 设置状态为在线
	s.state.Store(types.SlaveStateOnline)

	return nil
}

// Stop 优雅地关闭 Slave 节点。
func (s *WorkerSlave) Stop(ctx context.Context) error {
	var err error
	s.stopOnce.Do(func() {
		// 停止心跳
		if s.heartbeatCancel != nil {
			s.heartbeatCancel()
		}

		// 停止任务引擎
		if s.taskEngine != nil {
			err = s.taskEngine.Stop(ctx)
		}

		// 设置状态为离线
		s.state.Store(types.SlaveStateOffline)
		s.connected.Store(false)

		close(s.stopped)
	})
	return err
}

// Connect 连接到 Master 节点。
// Requirements: 12.1
func (s *WorkerSlave) Connect(ctx context.Context, masterAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.connected.Load() {
		return fmt.Errorf("已连接到 Master")
	}

	s.masterAddr = masterAddr

	// 创建 HTTP 客户端配置
	clientCfg := &httpclient.Config{
		MasterURL:         masterAddr,
		SlaveID:           s.config.ID,
		SlaveType:         s.config.Type,
		Address:           s.config.Address,
		Capabilities:      s.config.Capabilities,
		Labels:            s.config.Labels,
		HeartbeatInterval: s.config.HeartbeatInterval,
		RequestTimeout:    s.config.HeartbeatTimeout,
		ReconnectInterval: 5 * time.Second,
		ResultBufferSize:  1000,
		MetricsBufferSize: 1000,
		TaskPollInterval:  1 * time.Second,
		Resources: &rest.ResourceInfo{
			CPUCores: s.config.CPUCores,
			MemoryMB: s.config.MemoryMB,
			MaxVUs:   s.config.MaxVUs,
		},
	}

	// 创建 HTTP 客户端
	s.httpClient = httpclient.NewClient(clientCfg)

	// 设置任务处理回调
	s.httpClient.SetTaskHandler(func(ctx context.Context, task *rest.TaskAssignment) error {
		fmt.Printf("收到任务: %s, 执行ID: %s\n", task.TaskID, task.ExecutionID)

		// 转换任务格式
		internalTask := &types.Task{
			ID:          task.TaskID,
			ExecutionID: task.ExecutionID,
			Workflow:    task.Workflow,
		}
		if task.Segment != nil {
			internalTask.Segment = &types.ExecutionSegment{
				Start: task.Segment.Start,
				End:   task.Segment.End,
			}
		}

		result, err := s.ExecuteTask(ctx, internalTask)
		if err != nil {
			fmt.Printf("任务执行失败: %v\n", err)
			return err
		}

		// 发送结果回 Master
		bufferedResult := &httpclient.BufferedResult{
			TaskID:      result.TaskID,
			ExecutionID: result.ExecutionID,
			Status:      string(result.Status),
			Result:      result.Result,
		}
		if len(result.Errors) > 0 {
			bufferedResult.Errors = make([]*rest.ExecutionErrorRequest, len(result.Errors))
			for i, e := range result.Errors {
				bufferedResult.Errors[i] = &rest.ExecutionErrorRequest{
					Code:      string(e.Code),
					Message:   e.Message,
					StepID:    e.StepID,
					Timestamp: e.Timestamp.UnixMilli(),
				}
			}
		}

		if err := s.httpClient.SendTaskResult(bufferedResult); err != nil {
			fmt.Printf("发送任务结果失败: %v\n", err)
			return err
		}
		fmt.Printf("任务完成: %s, 状态: %s\n", result.TaskID, result.Status)
		return nil
	})

	// 设置命令处理回调
	s.httpClient.SetCommandHandler(func(ctx context.Context, cmd *rest.ControlCommand) error {
		fmt.Printf("收到命令: %s, 执行ID: %s\n", cmd.Type, cmd.ExecutionID)
		// TODO: 处理控制命令（停止、暂停、恢复等）
		return nil
	})

	// 设置断开连接回调
	s.httpClient.SetDisconnectHandler(func(err error) {
		fmt.Printf("与 Master 断开连接: %v\n", err)
		s.connected.Store(false)
	})

	// 设置重连回调
	s.httpClient.SetReconnectHandler(func() {
		fmt.Println("已重新连接到 Master")
		s.connected.Store(true)
	})

	// 连接到 Master
	if err := s.httpClient.Connect(ctx); err != nil {
		return fmt.Errorf("连接 Master 失败: %w", err)
	}

	// 注册到 Master
	if err := s.httpClient.Register(ctx); err != nil {
		s.httpClient.Disconnect(ctx)
		return fmt.Errorf("注册到 Master 失败: %w", err)
	}

	// 启动心跳
	s.heartbeatCtx, s.heartbeatCancel = context.WithCancel(context.Background())
	if err := s.httpClient.StartHeartbeat(s.heartbeatCtx, func() *rest.SlaveStatusInfo {
		status := s.GetStatus()
		return &rest.SlaveStatusInfo{
			State:       string(status.State),
			Load:        status.Load,
			ActiveTasks: status.ActiveTasks,
			LastSeen:    status.LastSeen.UnixMilli(),
			Metrics: &rest.SlaveMetrics{
				CPUUsage:    status.Metrics.CPUUsage,
				MemoryUsage: status.Metrics.MemoryUsage,
				ActiveVUs:   status.Metrics.ActiveVUs,
				Throughput:  status.Metrics.Throughput,
			},
		}
	}); err != nil {
		s.httpClient.Disconnect(ctx)
		return fmt.Errorf("启动心跳失败: %w", err)
	}

	// 启动任务轮询
	if err := s.httpClient.StartTaskPolling(s.heartbeatCtx); err != nil {
		fmt.Printf("启动任务轮询失败: %v\n", err)
		// 任务轮询启动失败不是致命错误
	}

	s.connected.Store(true)
	s.state.Store(types.SlaveStateOnline)

	return nil
}

// Disconnect 断开与 Master 节点的连接。
func (s *WorkerSlave) Disconnect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.connected.Load() {
		return nil
	}

	// 停止心跳
	if s.heartbeatCancel != nil {
		s.heartbeatCancel()
	}

	// 断开 HTTP 连接
	if s.httpClient != nil {
		if err := s.httpClient.Disconnect(ctx); err != nil {
			return fmt.Errorf("断开 HTTP 连接失败: %w", err)
		}
	}

	s.connected.Store(false)
	s.masterAddr = ""
	s.state.Store(types.SlaveStateOffline)

	return nil
}

// ExecuteTask 执行分配的任务。
// Requirements: 12.1
func (s *WorkerSlave) ExecuteTask(ctx context.Context, task *types.Task) (*types.TaskResult, error) {
	if task == nil {
		return nil, fmt.Errorf("任务不能为空")
	}

	s.activeTasks.Add(1)
	defer s.activeTasks.Add(-1)

	// 如果有活跃任务，更新状态为忙碌
	if s.activeTasks.Load() > 0 {
		s.state.Store(types.SlaveStateBusy)
	}

	// 使用任务引擎执行任务
	result, err := s.taskEngine.Execute(ctx, task)

	// 如果没有更多活跃任务，将状态恢复为在线
	if s.activeTasks.Load() == 0 {
		s.state.Store(types.SlaveStateOnline)
	}

	return result, err
}

// GetStatus 返回当前 Slave 状态。
// Requirements: 12.4
func (s *WorkerSlave) GetStatus() *types.SlaveStatus {
	load, _ := s.currentLoad.Load().(float64)
	lastSeen, _ := s.lastHeartbeat.Load().(time.Time)
	state, _ := s.state.Load().(types.SlaveState)

	return &types.SlaveStatus{
		State:       state,
		Load:        load,
		ActiveTasks: int(s.activeTasks.Load()),
		LastSeen:    lastSeen,
		Metrics:     s.getMetrics(),
	}
}

// GetInfo 返回 Slave 注册信息。
// Requirements: 12.1
func (s *WorkerSlave) GetInfo() *types.SlaveInfo {
	return &types.SlaveInfo{
		ID:           s.config.ID,
		Type:         s.config.Type,
		Address:      s.config.Address,
		Capabilities: s.getCapabilities(),
		Labels:       s.config.Labels,
		Resources: &types.ResourceInfo{
			CPUCores:    s.config.CPUCores,
			MemoryMB:    s.config.MemoryMB,
			MaxVUs:      s.config.MaxVUs,
			CurrentLoad: s.currentLoad.Load().(float64),
		},
	}
}

// getCapabilities 返回此 Slave 的能力列表。
// Requirements: 12.1
func (s *WorkerSlave) getCapabilities() []string {
	// 从配置的能力开始
	caps := make([]string, len(s.config.Capabilities))
	copy(caps, s.config.Capabilities)

	// 根据已注册的执行器添加能力
	if s.registry != nil {
		for _, execType := range s.registry.Types() {
			caps = append(caps, execType+"_executor")
		}
	}

	return caps
}

// getMetrics 返回当前 Slave 指标数据。
func (s *WorkerSlave) getMetrics() *types.SlaveMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.taskEngine != nil {
		return s.taskEngine.GetMetrics()
	}

	return &types.SlaveMetrics{}
}

// heartbeatLoop 定期向 Master 发送心跳。
// Requirements: 12.4
func (s *WorkerSlave) heartbeatLoop() {
	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.heartbeatCtx.Done():
			return
		case <-ticker.C:
			s.sendHeartbeat()
		}
	}
}

// sendHeartbeat 向 Master 发送心跳。
// Requirements: 12.4
func (s *WorkerSlave) sendHeartbeat() {
	s.lastHeartbeat.Store(time.Now())

	// 更新负载指标
	s.updateLoad()

	// 心跳通过 HTTP 客户端发送
	// 目前只更新本地状态
}

// updateLoad 更新当前负载指标。
func (s *WorkerSlave) updateLoad() {
	// 根据活跃任务数和最大 VU 数计算负载
	if s.config.MaxVUs > 0 {
		load := float64(s.activeTasks.Load()) / float64(s.config.MaxVUs) * 100
		s.currentLoad.Store(load)
	}
}

// IsConnected 返回 Slave 是否已连接到 Master。
func (s *WorkerSlave) IsConnected() bool {
	return s.connected.Load()
}

// GetMasterAddress 返回已连接的 Master 地址。
func (s *WorkerSlave) GetMasterAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.masterAddr
}
