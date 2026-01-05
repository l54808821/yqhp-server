package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "yqhp/workflow-engine/api/grpc/proto"
	"yqhp/workflow-engine/internal/master"
	"yqhp/workflow-engine/pkg/types"
)

// mockRegistry implements master.SlaveRegistry for testing.
type mockRegistry struct {
	slaves   map[string]*types.SlaveInfo
	statuses map[string]*types.SlaveStatus
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		slaves:   make(map[string]*types.SlaveInfo),
		statuses: make(map[string]*types.SlaveStatus),
	}
}

func (m *mockRegistry) Register(ctx context.Context, slave *types.SlaveInfo) error {
	m.slaves[slave.ID] = slave
	m.statuses[slave.ID] = &types.SlaveStatus{
		State:    types.SlaveStateOnline,
		LastSeen: time.Now(),
	}
	return nil
}

func (m *mockRegistry) Unregister(ctx context.Context, slaveID string) error {
	delete(m.slaves, slaveID)
	delete(m.statuses, slaveID)
	return nil
}

func (m *mockRegistry) UpdateStatus(ctx context.Context, slaveID string, status *types.SlaveStatus) error {
	m.statuses[slaveID] = status
	return nil
}

func (m *mockRegistry) GetSlave(ctx context.Context, slaveID string) (*types.SlaveInfo, error) {
	return m.slaves[slaveID], nil
}

func (m *mockRegistry) GetSlaveStatus(ctx context.Context, slaveID string) (*types.SlaveStatus, error) {
	return m.statuses[slaveID], nil
}

func (m *mockRegistry) ListSlaves(ctx context.Context, filter *master.SlaveFilter) ([]*types.SlaveInfo, error) {
	result := make([]*types.SlaveInfo, 0, len(m.slaves))
	for _, slave := range m.slaves {
		result = append(result, slave)
	}
	return result, nil
}

func (m *mockRegistry) GetOnlineSlaves(ctx context.Context) ([]*types.SlaveInfo, error) {
	return m.ListSlaves(ctx, nil)
}

func (m *mockRegistry) WatchSlaves(ctx context.Context) (<-chan *types.SlaveEvent, error) {
	return make(chan *types.SlaveEvent), nil
}

func TestNewServer(t *testing.T) {
	registry := newMockRegistry()
	server := NewServer(nil, registry, nil, nil, nil)

	assert.NotNil(t, server)
	assert.NotNil(t, server.config)
	assert.Equal(t, ":9090", server.config.Address)
}

func TestServerConfig(t *testing.T) {
	config := &Config{
		Address:           ":8080",
		MaxRecvMsgSize:    8 * 1024 * 1024,
		MaxSendMsgSize:    8 * 1024 * 1024,
		HeartbeatInterval: 10 * time.Second,
		ConnectionTimeout: 60 * time.Second,
		MasterID:          "test-master",
		Version:           "2.0.0",
	}

	registry := newMockRegistry()
	server := NewServer(config, registry, nil, nil, nil)

	assert.Equal(t, ":8080", server.config.Address)
	assert.Equal(t, "test-master", server.config.MasterID)
	assert.Equal(t, "2.0.0", server.config.Version)
}

func TestRegister(t *testing.T) {
	registry := newMockRegistry()
	server := NewServer(nil, registry, nil, nil, nil)

	ctx := context.Background()
	req := &pb.RegisterRequest{
		SlaveId:      "slave-1",
		SlaveType:    "worker",
		Capabilities: []string{"http_executor", "script_executor"},
		Labels:       map[string]string{"region": "us-east"},
		Address:      "localhost:9091",
		Resources: &pb.ResourceInfo{
			CpuCores:    4,
			MemoryMb:    8192,
			MaxVus:      100,
			CurrentLoad: 0.0,
		},
	}

	resp, err := server.Register(ctx, req)
	require.NoError(t, err)
	assert.True(t, resp.Accepted)
	assert.Equal(t, "slave-1", resp.AssignedId)
	assert.NotNil(t, resp.MasterInfo)

	// Verify slave was registered
	slave, err := registry.GetSlave(ctx, "slave-1")
	require.NoError(t, err)
	assert.NotNil(t, slave)
	assert.Equal(t, "slave-1", slave.ID)
	assert.Equal(t, types.SlaveTypeWorker, slave.Type)
}

func TestRegisterNilRequest(t *testing.T) {
	registry := newMockRegistry()
	server := NewServer(nil, registry, nil, nil, nil)

	ctx := context.Background()
	_, err := server.Register(ctx, nil)
	assert.Error(t, err)
}

func TestAssignTask(t *testing.T) {
	registry := newMockRegistry()
	server := NewServer(nil, registry, nil, nil, nil)

	ctx := context.Background()

	// Register a slave first
	req := &pb.RegisterRequest{
		SlaveId:   "slave-1",
		SlaveType: "worker",
	}
	_, err := server.Register(ctx, req)
	require.NoError(t, err)

	// Assign a task
	task := &types.Task{
		ID:          "task-1",
		ExecutionID: "exec-1",
		Workflow: &types.Workflow{
			ID:   "workflow-1",
			Name: "Test Workflow",
		},
		Segment: types.ExecutionSegment{
			Start: 0.0,
			End:   1.0,
		},
	}

	err = server.AssignTask("slave-1", task)
	assert.NoError(t, err)
}

func TestAssignTaskUnknownSlave(t *testing.T) {
	registry := newMockRegistry()
	server := NewServer(nil, registry, nil, nil, nil)

	task := &types.Task{
		ID:          "task-1",
		ExecutionID: "exec-1",
	}

	err := server.AssignTask("unknown-slave", task)
	assert.Error(t, err)
}

func TestSendCommand(t *testing.T) {
	registry := newMockRegistry()
	server := NewServer(nil, registry, nil, nil, nil)

	ctx := context.Background()

	// Register a slave first
	req := &pb.RegisterRequest{
		SlaveId:   "slave-1",
		SlaveType: "worker",
	}
	_, err := server.Register(ctx, req)
	require.NoError(t, err)

	// Send a command
	err = server.SendCommand("slave-1", "stop", "exec-1", nil)
	assert.NoError(t, err)
}

func TestSendCommandUnknownSlave(t *testing.T) {
	registry := newMockRegistry()
	server := NewServer(nil, registry, nil, nil, nil)

	err := server.SendCommand("unknown-slave", "stop", "exec-1", nil)
	assert.Error(t, err)
}

func TestMetricsHandler(t *testing.T) {
	registry := newMockRegistry()
	server := NewServer(nil, registry, nil, nil, nil)

	// Create a mock handler
	handler := &mockMetricsHandler{}

	// Register handler
	server.RegisterMetricsHandler("exec-1", handler)

	// Verify handler is registered
	server.metricsHandlersMu.RLock()
	h, ok := server.metricsHandlers["exec-1"]
	server.metricsHandlersMu.RUnlock()
	assert.True(t, ok)
	assert.Equal(t, handler, h)

	// Unregister handler
	server.UnregisterMetricsHandler("exec-1")

	// Verify handler is unregistered
	server.metricsHandlersMu.RLock()
	_, ok = server.metricsHandlers["exec-1"]
	server.metricsHandlersMu.RUnlock()
	assert.False(t, ok)
}

type mockMetricsHandler struct {
	metrics []*types.Metrics
}

func (m *mockMetricsHandler) HandleMetrics(ctx context.Context, slaveID string, metrics *types.Metrics) error {
	m.metrics = append(m.metrics, metrics)
	return nil
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, ":9090", config.Address)
	assert.Equal(t, 16*1024*1024, config.MaxRecvMsgSize)
	assert.Equal(t, 16*1024*1024, config.MaxSendMsgSize)
	assert.Equal(t, 5*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 30*time.Second, config.ConnectionTimeout)
}
