package slave

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	grpcclient "yqhp/workflow-engine/api/grpc/client"
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

	// gRPC 客户端
	grpcClient *grpcclient.Client

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

	// 创建 gRPC 客户端配置
	clientCfg := &grpcclient.Config{
		MasterAddress:     masterAddr,
		SlaveID:           s.config.ID,
		SlaveType:         s.config.Type,
		Address:           s.config.Address,
		Capabilities:      s.config.Capabilities,
		Labels:            s.config.Labels,
		HeartbeatInterval: s.config.HeartbeatInterval,
		ConnectionTimeout: s.config.HeartbeatTimeout,
		Resources: &types.ResourceInfo{
			CPUCores: s.config.CPUCores,
			MemoryMB: s.config.MemoryMB,
			MaxVUs:   s.config.MaxVUs,
		},
	}

	// 创建 gRPC 客户端
	s.grpcClient = grpcclient.NewClient(clientCfg)

	// 连接到 Master
	if err := s.grpcClient.Connect(ctx); err != nil {
		return fmt.Errorf("连接 Master 失败: %w", err)
	}

	// 注册到 Master
	if err := s.grpcClient.Register(ctx); err != nil {
		s.grpcClient.Disconnect(ctx)
		return fmt.Errorf("注册到 Master 失败: %w", err)
	}

	// 启动心跳
	s.heartbeatCtx, s.heartbeatCancel = context.WithCancel(context.Background())
	if err := s.grpcClient.StartHeartbeat(s.heartbeatCtx, func() *types.SlaveStatus {
		return s.GetStatus()
	}); err != nil {
		s.grpcClient.Disconnect(ctx)
		return fmt.Errorf("启动心跳失败: %w", err)
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

	// 断开 gRPC 连接
	if s.grpcClient != nil {
		if err := s.grpcClient.Disconnect(ctx); err != nil {
			return fmt.Errorf("断开 gRPC 连接失败: %w", err)
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

	// 在实际实现中，这里会通过 gRPC 发送心跳
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
