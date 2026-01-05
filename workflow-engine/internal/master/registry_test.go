package master

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

func TestNewInMemorySlaveRegistry(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	assert.NotNil(t, registry)
	assert.Equal(t, 0, registry.Count())
}

func TestRegisterSlave(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	slave := &types.SlaveInfo{
		ID:           "slave-1",
		Type:         types.SlaveTypeWorker,
		Address:      "localhost:8081",
		Capabilities: []string{"http_executor"},
		Labels:       map[string]string{"region": "us-east"},
	}

	err := registry.Register(ctx, slave)
	require.NoError(t, err)
	assert.Equal(t, 1, registry.Count())

	// Get slave
	retrieved, err := registry.GetSlave(ctx, "slave-1")
	require.NoError(t, err)
	assert.Equal(t, slave.ID, retrieved.ID)
	assert.Equal(t, slave.Type, retrieved.Type)

	// Get status
	status, err := registry.GetSlaveStatus(ctx, "slave-1")
	require.NoError(t, err)
	assert.Equal(t, types.SlaveStateOnline, status.State)
}

func TestRegisterSlaveNil(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	err := registry.Register(ctx, nil)
	assert.Error(t, err)
}

func TestRegisterSlaveEmptyID(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	err := registry.Register(ctx, &types.SlaveInfo{ID: ""})
	assert.Error(t, err)
}

func TestRegisterSlaveDuplicate(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	slave := &types.SlaveInfo{ID: "slave-1", Type: types.SlaveTypeWorker}

	err := registry.Register(ctx, slave)
	require.NoError(t, err)

	err = registry.Register(ctx, slave)
	assert.Error(t, err)
}

func TestUnregisterSlave(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	slave := &types.SlaveInfo{ID: "slave-1", Type: types.SlaveTypeWorker}
	err := registry.Register(ctx, slave)
	require.NoError(t, err)

	err = registry.Unregister(ctx, "slave-1")
	require.NoError(t, err)
	assert.Equal(t, 0, registry.Count())

	// Should not find slave
	_, err = registry.GetSlave(ctx, "slave-1")
	assert.Error(t, err)
}

func TestUnregisterSlaveNotFound(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	err := registry.Unregister(ctx, "non-existent")
	assert.Error(t, err)
}

func TestUpdateStatus(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	slave := &types.SlaveInfo{ID: "slave-1", Type: types.SlaveTypeWorker}
	err := registry.Register(ctx, slave)
	require.NoError(t, err)

	newStatus := &types.SlaveStatus{
		State:       types.SlaveStateBusy,
		Load:        50.0,
		ActiveTasks: 5,
		LastSeen:    time.Now(),
	}

	err = registry.UpdateStatus(ctx, "slave-1", newStatus)
	require.NoError(t, err)

	status, err := registry.GetSlaveStatus(ctx, "slave-1")
	require.NoError(t, err)
	assert.Equal(t, types.SlaveStateBusy, status.State)
	assert.Equal(t, 50.0, status.Load)
	assert.Equal(t, 5, status.ActiveTasks)
}

func TestUpdateStatusNotFound(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	err := registry.UpdateStatus(ctx, "non-existent", &types.SlaveStatus{})
	assert.Error(t, err)
}

func TestListSlaves(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	// Register multiple slaves
	slaves := []*types.SlaveInfo{
		{ID: "slave-1", Type: types.SlaveTypeWorker, Capabilities: []string{"http_executor"}},
		{ID: "slave-2", Type: types.SlaveTypeWorker, Capabilities: []string{"http_executor", "script_executor"}},
		{ID: "slave-3", Type: types.SlaveTypeAggregator, Capabilities: []string{"aggregator"}},
	}

	for _, slave := range slaves {
		err := registry.Register(ctx, slave)
		require.NoError(t, err)
	}

	// List all
	result, err := registry.ListSlaves(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestListSlavesFilterByType(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	slaves := []*types.SlaveInfo{
		{ID: "slave-1", Type: types.SlaveTypeWorker},
		{ID: "slave-2", Type: types.SlaveTypeWorker},
		{ID: "slave-3", Type: types.SlaveTypeAggregator},
	}

	for _, slave := range slaves {
		err := registry.Register(ctx, slave)
		require.NoError(t, err)
	}

	// Filter by worker type
	result, err := registry.ListSlaves(ctx, &SlaveFilter{
		Types: []types.SlaveType{types.SlaveTypeWorker},
	})
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestListSlavesFilterByState(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	slaves := []*types.SlaveInfo{
		{ID: "slave-1", Type: types.SlaveTypeWorker},
		{ID: "slave-2", Type: types.SlaveTypeWorker},
		{ID: "slave-3", Type: types.SlaveTypeWorker},
	}

	for _, slave := range slaves {
		err := registry.Register(ctx, slave)
		require.NoError(t, err)
	}

	// Mark one as offline
	err := registry.UpdateStatus(ctx, "slave-2", &types.SlaveStatus{
		State:    types.SlaveStateOffline,
		LastSeen: time.Now(),
	})
	require.NoError(t, err)

	// Filter by online state
	result, err := registry.ListSlaves(ctx, &SlaveFilter{
		States: []types.SlaveState{types.SlaveStateOnline},
	})
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestListSlavesFilterByLabels(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	slaves := []*types.SlaveInfo{
		{ID: "slave-1", Type: types.SlaveTypeWorker, Labels: map[string]string{"region": "us-east", "env": "prod"}},
		{ID: "slave-2", Type: types.SlaveTypeWorker, Labels: map[string]string{"region": "us-west", "env": "prod"}},
		{ID: "slave-3", Type: types.SlaveTypeWorker, Labels: map[string]string{"region": "us-east", "env": "dev"}},
	}

	for _, slave := range slaves {
		err := registry.Register(ctx, slave)
		require.NoError(t, err)
	}

	// Filter by region
	result, err := registry.ListSlaves(ctx, &SlaveFilter{
		Labels: map[string]string{"region": "us-east"},
	})
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// Filter by region and env
	result, err = registry.ListSlaves(ctx, &SlaveFilter{
		Labels: map[string]string{"region": "us-east", "env": "prod"},
	})
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestListSlavesFilterByCapabilities(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	slaves := []*types.SlaveInfo{
		{ID: "slave-1", Type: types.SlaveTypeWorker, Capabilities: []string{"http_executor"}},
		{ID: "slave-2", Type: types.SlaveTypeWorker, Capabilities: []string{"http_executor", "script_executor"}},
		{ID: "slave-3", Type: types.SlaveTypeWorker, Capabilities: []string{"grpc_executor"}},
	}

	for _, slave := range slaves {
		err := registry.Register(ctx, slave)
		require.NoError(t, err)
	}

	// Filter by http_executor
	result, err := registry.ListSlaves(ctx, &SlaveFilter{
		Capabilities: []string{"http_executor"},
	})
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// Filter by multiple capabilities
	result, err = registry.ListSlaves(ctx, &SlaveFilter{
		Capabilities: []string{"http_executor", "script_executor"},
	})
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestGetOnlineSlaves(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	slaves := []*types.SlaveInfo{
		{ID: "slave-1", Type: types.SlaveTypeWorker},
		{ID: "slave-2", Type: types.SlaveTypeWorker},
		{ID: "slave-3", Type: types.SlaveTypeWorker},
	}

	for _, slave := range slaves {
		err := registry.Register(ctx, slave)
		require.NoError(t, err)
	}

	// Mark one as offline
	err := registry.MarkOffline(ctx, "slave-2")
	require.NoError(t, err)

	// Get online slaves
	result, err := registry.GetOnlineSlaves(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 2, registry.CountOnline())
}

func TestWatchSlaves(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watching
	eventCh, err := registry.WatchSlaves(ctx)
	require.NoError(t, err)

	// Register a slave
	slave := &types.SlaveInfo{ID: "slave-1", Type: types.SlaveTypeWorker}
	err = registry.Register(ctx, slave)
	require.NoError(t, err)

	// Should receive registration event
	select {
	case event := <-eventCh:
		assert.Equal(t, types.SlaveEventRegistered, event.Type)
		assert.Equal(t, "slave-1", event.SlaveID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestUpdateHeartbeat(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	slave := &types.SlaveInfo{ID: "slave-1", Type: types.SlaveTypeWorker}
	err := registry.Register(ctx, slave)
	require.NoError(t, err)

	// Update heartbeat
	metrics := &types.SlaveMetrics{
		CPUUsage:    25.0,
		MemoryUsage: 50.0,
		ActiveVUs:   10,
	}
	err = registry.UpdateHeartbeat(ctx, "slave-1", metrics)
	require.NoError(t, err)

	status, err := registry.GetSlaveStatus(ctx, "slave-1")
	require.NoError(t, err)
	assert.Equal(t, 25.0, status.Load)
	assert.NotNil(t, status.Metrics)
}

func TestUpdateHeartbeatRecoversOffline(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	slave := &types.SlaveInfo{ID: "slave-1", Type: types.SlaveTypeWorker}
	err := registry.Register(ctx, slave)
	require.NoError(t, err)

	// Mark as offline
	err = registry.MarkOffline(ctx, "slave-1")
	require.NoError(t, err)

	status, err := registry.GetSlaveStatus(ctx, "slave-1")
	require.NoError(t, err)
	assert.Equal(t, types.SlaveStateOffline, status.State)

	// Update heartbeat should recover
	err = registry.UpdateHeartbeat(ctx, "slave-1", nil)
	require.NoError(t, err)

	status, err = registry.GetSlaveStatus(ctx, "slave-1")
	require.NoError(t, err)
	assert.Equal(t, types.SlaveStateOnline, status.State)
}

func TestDrainSlave(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	slave := &types.SlaveInfo{ID: "slave-1", Type: types.SlaveTypeWorker}
	err := registry.Register(ctx, slave)
	require.NoError(t, err)

	err = registry.DrainSlave(ctx, "slave-1")
	require.NoError(t, err)

	status, err := registry.GetSlaveStatus(ctx, "slave-1")
	require.NoError(t, err)
	assert.Equal(t, types.SlaveStateDraining, status.State)
}

func TestDrainSlaveNotFound(t *testing.T) {
	registry := NewInMemorySlaveRegistry()
	ctx := context.Background()

	err := registry.DrainSlave(ctx, "non-existent")
	assert.Error(t, err)
}
