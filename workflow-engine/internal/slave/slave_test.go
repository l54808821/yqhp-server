package slave

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/pkg/types"
)

func TestNewWorkerSlave(t *testing.T) {
	registry := executor.NewRegistry()
	config := &Config{
		ID:                "test-slave-1",
		Type:              types.SlaveTypeWorker,
		Address:           "localhost:8080",
		HeartbeatInterval: 5 * time.Second,
		MaxVUs:            100,
	}

	slave := NewWorkerSlave(config, registry)

	assert.NotNil(t, slave)
	assert.Equal(t, types.SlaveStateOffline, slave.state.Load())
	assert.False(t, slave.connected.Load())
}

func TestWorkerSlave_Start(t *testing.T) {
	registry := executor.NewRegistry()
	config := DefaultConfig()
	config.ID = "test-slave"

	slave := NewWorkerSlave(config, registry)

	err := slave.Start(context.Background())
	require.NoError(t, err)

	assert.Equal(t, types.SlaveStateOnline, slave.state.Load())
	assert.NotNil(t, slave.taskEngine)

	err = slave.Stop(context.Background())
	require.NoError(t, err)
}

func TestWorkerSlave_Connect(t *testing.T) {
	registry := executor.NewRegistry()
	config := DefaultConfig()
	config.ID = "test-slave"

	slave := NewWorkerSlave(config, registry)
	err := slave.Start(context.Background())
	require.NoError(t, err)

	// Connect to master
	err = slave.Connect(context.Background(), "localhost:9090")
	require.NoError(t, err)

	assert.True(t, slave.IsConnected())
	assert.Equal(t, "localhost:9090", slave.GetMasterAddress())

	// Disconnect
	err = slave.Disconnect(context.Background())
	require.NoError(t, err)

	assert.False(t, slave.IsConnected())

	err = slave.Stop(context.Background())
	require.NoError(t, err)
}

func TestWorkerSlave_GetStatus(t *testing.T) {
	registry := executor.NewRegistry()
	config := &Config{
		ID:     "test-slave",
		Type:   types.SlaveTypeWorker,
		MaxVUs: 100,
	}

	slave := NewWorkerSlave(config, registry)
	err := slave.Start(context.Background())
	require.NoError(t, err)

	status := slave.GetStatus()

	assert.Equal(t, types.SlaveStateOnline, status.State)
	assert.Equal(t, 0, status.ActiveTasks)
	assert.NotNil(t, status.Metrics)

	err = slave.Stop(context.Background())
	require.NoError(t, err)
}

func TestWorkerSlave_GetInfo(t *testing.T) {
	registry := executor.NewRegistry()
	config := &Config{
		ID:           "test-slave",
		Type:         types.SlaveTypeWorker,
		Address:      "localhost:8080",
		Capabilities: []string{"http", "script"},
		Labels:       map[string]string{"region": "us-east"},
		MaxVUs:       100,
		CPUCores:     4,
		MemoryMB:     4096,
	}

	slave := NewWorkerSlave(config, registry)

	info := slave.GetInfo()

	assert.Equal(t, "test-slave", info.ID)
	assert.Equal(t, types.SlaveTypeWorker, info.Type)
	assert.Equal(t, "localhost:8080", info.Address)
	assert.Contains(t, info.Capabilities, "http")
	assert.Contains(t, info.Capabilities, "script")
	assert.Equal(t, "us-east", info.Labels["region"])
	assert.Equal(t, 4, info.Resources.CPUCores)
	assert.Equal(t, int64(4096), info.Resources.MemoryMB)
	assert.Equal(t, 100, info.Resources.MaxVUs)
}

func TestWorkerSlave_DoubleConnect(t *testing.T) {
	registry := executor.NewRegistry()
	config := DefaultConfig()
	config.ID = "test-slave"

	slave := NewWorkerSlave(config, registry)
	err := slave.Start(context.Background())
	require.NoError(t, err)

	// First connect
	err = slave.Connect(context.Background(), "localhost:9090")
	require.NoError(t, err)

	// Second connect should fail
	err = slave.Connect(context.Background(), "localhost:9091")
	assert.Error(t, err)

	err = slave.Stop(context.Background())
	require.NoError(t, err)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, types.SlaveTypeWorker, config.Type)
	assert.Equal(t, 5*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 10*time.Second, config.HeartbeatTimeout)
	assert.Equal(t, 100, config.MaxVUs)
	assert.Equal(t, 4, config.CPUCores)
	assert.Equal(t, int64(4096), config.MemoryMB)
}
