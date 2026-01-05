package client

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

func TestNewClient(t *testing.T) {
	client := NewClient(nil)

	assert.NotNil(t, client)
	assert.NotNil(t, client.config)
	assert.Equal(t, "localhost:9090", client.config.MasterAddress)
	assert.Equal(t, types.SlaveTypeWorker, client.config.SlaveType)
}

func TestClientConfig(t *testing.T) {
	config := &Config{
		MasterAddress:        "master:8080",
		SlaveID:              "slave-1",
		SlaveType:            types.SlaveTypeAggregator,
		Capabilities:         []string{"http_executor"},
		Labels:               map[string]string{"region": "us-east"},
		Address:              "localhost:9091",
		HeartbeatInterval:    10 * time.Second,
		ReconnectInterval:    10 * time.Second,
		MaxReconnectAttempts: 5,
		ResultBufferSize:     500,
		MetricsBufferSize:    500,
		ConnectionTimeout:    60 * time.Second,
	}

	client := NewClient(config)

	assert.Equal(t, "master:8080", client.config.MasterAddress)
	assert.Equal(t, "slave-1", client.config.SlaveID)
	assert.Equal(t, types.SlaveTypeAggregator, client.config.SlaveType)
	assert.Equal(t, 5, client.config.MaxReconnectAttempts)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "localhost:9090", config.MasterAddress)
	assert.Equal(t, types.SlaveTypeWorker, config.SlaveType)
	assert.Equal(t, 5*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 5*time.Second, config.ReconnectInterval)
	assert.Equal(t, 0, config.MaxReconnectAttempts)
	assert.Equal(t, 1000, config.ResultBufferSize)
	assert.Equal(t, 1000, config.MetricsBufferSize)
	assert.Equal(t, 30*time.Second, config.ConnectionTimeout)
}

func TestClientNotConnected(t *testing.T) {
	client := NewClient(nil)

	// Should fail when not connected
	err := client.Register(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestClientNotRegistered(t *testing.T) {
	client := NewClient(nil)
	client.connected.Store(true)

	// Should fail when not registered
	err := client.StartHeartbeat(context.Background(), func() *types.SlaveStatus {
		return &types.SlaveStatus{}
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")

	err = client.StartTaskStream(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")

	err = client.StartMetricsStream(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestResultBuffering(t *testing.T) {
	config := &Config{
		ResultBufferSize: 10,
	}
	client := NewClient(config)

	// Buffer results when not connected
	for i := 0; i < 5; i++ {
		result := &types.TaskResult{
			TaskID:      "task-" + string(rune('0'+i)),
			ExecutionID: "exec-1",
			Status:      types.ExecutionStatusCompleted,
		}
		err := client.SendTaskResult(result)
		// Should not error, just buffer
		assert.NoError(t, err)
	}

	// Check buffered count
	assert.Equal(t, int64(5), client.GetBufferedCount())
}

func TestMetricsBuffering(t *testing.T) {
	config := &Config{
		MetricsBufferSize: 10,
	}
	client := NewClient(config)

	// Buffer metrics when not connected
	for i := 0; i < 5; i++ {
		metrics := &types.Metrics{
			Timestamp: time.Now(),
		}
		err := client.SendMetrics("exec-1", metrics)
		// Should not error, just buffer
		assert.NoError(t, err)
	}
}

func TestSendTaskResultNil(t *testing.T) {
	client := NewClient(nil)

	err := client.SendTaskResult(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "result cannot be nil")
}

func TestSendMetricsNil(t *testing.T) {
	client := NewClient(nil)

	err := client.SendMetrics("exec-1", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metrics cannot be nil")
}

func TestSetHandlers(t *testing.T) {
	client := NewClient(nil)

	taskHandlerCalled := false
	commandHandlerCalled := false
	disconnectHandlerCalled := false
	reconnectHandlerCalled := false

	client.SetTaskHandler(func(ctx context.Context, task *types.Task) error {
		taskHandlerCalled = true
		return nil
	})

	client.SetCommandHandler(func(ctx context.Context, cmdType string, executionID string, params map[string]string) error {
		commandHandlerCalled = true
		return nil
	})

	client.SetDisconnectHandler(func(err error) {
		disconnectHandlerCalled = true
	})

	client.SetReconnectHandler(func() {
		reconnectHandlerCalled = true
	})

	// Verify handlers are set
	assert.NotNil(t, client.onTask)
	assert.NotNil(t, client.onCommand)
	assert.NotNil(t, client.onDisconnect)
	assert.NotNil(t, client.onReconnect)

	// Call handlers
	require.NoError(t, client.onTask(context.Background(), &types.Task{}))
	require.NoError(t, client.onCommand(context.Background(), "stop", "exec-1", nil))
	client.onDisconnect(nil)
	client.onReconnect()

	assert.True(t, taskHandlerCalled)
	assert.True(t, commandHandlerCalled)
	assert.True(t, disconnectHandlerCalled)
	assert.True(t, reconnectHandlerCalled)
}

func TestIsConnected(t *testing.T) {
	client := NewClient(nil)

	assert.False(t, client.IsConnected())

	client.connected.Store(true)
	assert.True(t, client.IsConnected())
}

func TestIsRegistered(t *testing.T) {
	client := NewClient(nil)

	assert.False(t, client.IsRegistered())

	client.registered.Store(true)
	assert.True(t, client.IsRegistered())
}

func TestGetSlaveID(t *testing.T) {
	config := &Config{
		SlaveID: "test-slave",
	}
	client := NewClient(config)

	assert.Equal(t, "test-slave", client.GetSlaveID())
}

func TestIsRetryableError(t *testing.T) {
	// Nil error is not retryable
	assert.False(t, IsRetryableError(nil))

	// Non-gRPC errors are assumed retryable
	assert.True(t, IsRetryableError(assert.AnError))
}

func TestDisconnect(t *testing.T) {
	client := NewClient(nil)
	client.connected.Store(true)

	err := client.Disconnect(context.Background())
	assert.NoError(t, err)
	assert.False(t, client.IsConnected())
}

func TestConnectAlreadyConnected(t *testing.T) {
	client := NewClient(nil)
	client.connected.Store(true)

	err := client.Connect(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already connected")
}
