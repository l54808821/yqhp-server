// Package client implements the HTTP client for Slave nodes using Fiber.
package client

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"yqhp/workflow-engine/api/rest"
	"yqhp/workflow-engine/pkg/types"
)

// setupTestServer creates a test Fiber server for testing.
func setupTestServer(t *testing.T) (*fiber.App, string) {
	app := fiber.New()

	// Health endpoint
	app.Get("/api/v1/health", func(c *fiber.Ctx) error {
		return c.JSON(rest.HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now().Format(time.RFC3339),
		})
	})

	// Register endpoint
	app.Post("/api/v1/slaves/register", func(c *fiber.Ctx) error {
		var req rest.SlaveRegisterRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(rest.ErrorResponse{
				Error:   "bad_request",
				Message: err.Error(),
			})
		}

		return c.JSON(rest.SlaveRegisterResponse{
			Accepted:          true,
			AssignedID:        req.SlaveID,
			HeartbeatInterval: 5000,
			MasterID:          "master-1",
			Version:           "1.0.0",
		})
	})

	// Heartbeat endpoint
	app.Post("/api/v1/slaves/:id/heartbeat", func(c *fiber.Ctx) error {
		return c.JSON(rest.SlaveHeartbeatResponse{
			Commands:  []*rest.ControlCommand{},
			Timestamp: time.Now().UnixMilli(),
		})
	})

	// Tasks endpoint
	app.Get("/api/v1/slaves/:id/tasks", func(c *fiber.Ctx) error {
		return c.JSON(rest.PendingTasksResponse{
			Tasks: []*rest.TaskAssignment{},
		})
	})

	// Task result endpoint
	app.Post("/api/v1/tasks/:id/result", func(c *fiber.Ctx) error {
		return c.JSON(rest.TaskResultResponse{
			Success: true,
			Message: "result received",
		})
	})

	// Metrics report endpoint
	app.Post("/api/v1/executions/:id/metrics/report", func(c *fiber.Ctx) error {
		return c.JSON(rest.MetricsReportResponse{
			Success: true,
		})
	})

	// Unregister endpoint
	app.Post("/api/v1/slaves/:id/unregister", func(c *fiber.Ctx) error {
		return c.JSON(rest.SlaveUnregisterResponse{
			Success: true,
			Message: "unregistered",
		})
	})

	// Start test server
	server := httptest.NewServer(app.Handler())
	t.Cleanup(func() {
		server.Close()
	})

	return app, server.URL
}

func TestNewClient(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		client := NewClient(nil)
		assert.NotNil(t, client)
		assert.Equal(t, "http://localhost:8080", client.config.MasterURL)
		assert.Equal(t, 5*time.Second, client.config.HeartbeatInterval)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &Config{
			MasterURL:         "http://custom:9090",
			SlaveID:           "slave-1",
			HeartbeatInterval: 10 * time.Second,
			ResultBufferSize:  500,
			MetricsBufferSize: 500,
		}
		client := NewClient(config)
		assert.NotNil(t, client)
		assert.Equal(t, "http://custom:9090", client.config.MasterURL)
		assert.Equal(t, "slave-1", client.config.SlaveID)
		assert.Equal(t, 10*time.Second, client.config.HeartbeatInterval)
	})
}

func TestClient_Connect(t *testing.T) {
	_, serverURL := setupTestServer(t)

	t.Run("successful connection", func(t *testing.T) {
		config := &Config{
			MasterURL:      serverURL,
			SlaveID:        "slave-1",
			RequestTimeout: 5 * time.Second,
		}
		client := NewClient(config)

		err := client.Connect(context.Background())
		assert.NoError(t, err)
		assert.True(t, client.IsConnected())
	})

	t.Run("already connected", func(t *testing.T) {
		config := &Config{
			MasterURL:      serverURL,
			SlaveID:        "slave-1",
			RequestTimeout: 5 * time.Second,
		}
		client := NewClient(config)

		err := client.Connect(context.Background())
		require.NoError(t, err)

		err = client.Connect(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already connected")
	})

	t.Run("connection to invalid server", func(t *testing.T) {
		config := &Config{
			MasterURL:      "http://invalid-server:9999",
			SlaveID:        "slave-1",
			RequestTimeout: 1 * time.Second,
		}
		client := NewClient(config)

		err := client.Connect(context.Background())
		assert.Error(t, err)
		assert.False(t, client.IsConnected())
	})
}

func TestClient_Register(t *testing.T) {
	_, serverURL := setupTestServer(t)

	t.Run("successful registration", func(t *testing.T) {
		config := &Config{
			MasterURL:      serverURL,
			SlaveID:        "slave-1",
			SlaveType:      types.SlaveTypeWorker,
			Capabilities:   []string{"http", "grpc"},
			Address:        "localhost:8081",
			RequestTimeout: 5 * time.Second,
		}
		client := NewClient(config)

		err := client.Connect(context.Background())
		require.NoError(t, err)

		err = client.Register(context.Background())
		assert.NoError(t, err)
		assert.True(t, client.IsRegistered())
	})

	t.Run("register without connection", func(t *testing.T) {
		config := &Config{
			MasterURL: serverURL,
			SlaveID:   "slave-1",
		}
		client := NewClient(config)

		err := client.Register(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})
}

func TestClient_Heartbeat(t *testing.T) {
	_, serverURL := setupTestServer(t)

	t.Run("send heartbeat", func(t *testing.T) {
		config := &Config{
			MasterURL:         serverURL,
			SlaveID:           "slave-1",
			HeartbeatInterval: 100 * time.Millisecond,
			RequestTimeout:    5 * time.Second,
		}
		client := NewClient(config)

		err := client.Connect(context.Background())
		require.NoError(t, err)

		err = client.Register(context.Background())
		require.NoError(t, err)

		resp, err := client.sendHeartbeat(&rest.SlaveStatusInfo{
			State:       "online",
			Load:        0.5,
			ActiveTasks: 2,
			LastSeen:    time.Now().UnixMilli(),
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("start heartbeat loop", func(t *testing.T) {
		config := &Config{
			MasterURL:         serverURL,
			SlaveID:           "slave-1",
			HeartbeatInterval: 50 * time.Millisecond,
			RequestTimeout:    5 * time.Second,
		}
		client := NewClient(config)

		err := client.Connect(context.Background())
		require.NoError(t, err)

		err = client.Register(context.Background())
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err = client.StartHeartbeat(ctx, func() *rest.SlaveStatusInfo {
			return &rest.SlaveStatusInfo{
				State: "online",
			}
		})
		assert.NoError(t, err)

		// Wait for a few heartbeats
		time.Sleep(150 * time.Millisecond)
		cancel()
	})
}

func TestClient_PollTasks(t *testing.T) {
	_, serverURL := setupTestServer(t)

	t.Run("poll tasks successfully", func(t *testing.T) {
		config := &Config{
			MasterURL:      serverURL,
			SlaveID:        "slave-1",
			RequestTimeout: 5 * time.Second,
		}
		client := NewClient(config)

		err := client.Connect(context.Background())
		require.NoError(t, err)

		err = client.Register(context.Background())
		require.NoError(t, err)

		tasks, err := client.PollTasks(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, tasks)
	})

	t.Run("poll tasks without registration", func(t *testing.T) {
		config := &Config{
			MasterURL: serverURL,
			SlaveID:   "slave-1",
		}
		client := NewClient(config)

		tasks, err := client.PollTasks(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
		assert.Nil(t, tasks)
	})
}

func TestClient_SendTaskResult(t *testing.T) {
	_, serverURL := setupTestServer(t)

	t.Run("send task result successfully", func(t *testing.T) {
		config := &Config{
			MasterURL:        serverURL,
			SlaveID:          "slave-1",
			RequestTimeout:   5 * time.Second,
			ResultBufferSize: 100,
		}
		client := NewClient(config)

		err := client.Connect(context.Background())
		require.NoError(t, err)

		err = client.Register(context.Background())
		require.NoError(t, err)

		result := &BufferedResult{
			TaskID:      "task-1",
			ExecutionID: "exec-1",
			Status:      "completed",
			Result:      map[string]interface{}{"key": "value"},
		}

		err = client.SendTaskResult(result)
		assert.NoError(t, err)
	})

	t.Run("send nil result", func(t *testing.T) {
		config := &Config{
			MasterURL: serverURL,
			SlaveID:   "slave-1",
		}
		client := NewClient(config)

		err := client.SendTaskResult(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "result cannot be nil")
	})

	t.Run("buffer result when not connected", func(t *testing.T) {
		config := &Config{
			MasterURL:        serverURL,
			SlaveID:          "slave-1",
			ResultBufferSize: 100,
		}
		client := NewClient(config)

		result := &BufferedResult{
			TaskID:      "task-1",
			ExecutionID: "exec-1",
			Status:      "completed",
		}

		err := client.SendTaskResult(result)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), client.GetBufferedCount())
	})
}

func TestClient_SendMetrics(t *testing.T) {
	_, serverURL := setupTestServer(t)

	t.Run("send metrics successfully", func(t *testing.T) {
		config := &Config{
			MasterURL:         serverURL,
			SlaveID:           "slave-1",
			RequestTimeout:    5 * time.Second,
			MetricsBufferSize: 100,
		}
		client := NewClient(config)

		err := client.Connect(context.Background())
		require.NoError(t, err)

		err = client.Register(context.Background())
		require.NoError(t, err)

		metrics := &rest.MetricsData{
			TotalVUs:        10,
			TotalIterations: 100,
			DurationMs:      5000,
		}

		err = client.SendMetrics("exec-1", metrics, nil)
		assert.NoError(t, err)
	})

	t.Run("send nil metrics", func(t *testing.T) {
		config := &Config{
			MasterURL: serverURL,
			SlaveID:   "slave-1",
		}
		client := NewClient(config)

		err := client.SendMetrics("exec-1", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "metrics cannot be nil")
	})
}

func TestClient_Disconnect(t *testing.T) {
	_, serverURL := setupTestServer(t)

	t.Run("disconnect successfully", func(t *testing.T) {
		config := &Config{
			MasterURL:      serverURL,
			SlaveID:        "slave-1",
			RequestTimeout: 5 * time.Second,
		}
		client := NewClient(config)

		err := client.Connect(context.Background())
		require.NoError(t, err)

		err = client.Register(context.Background())
		require.NoError(t, err)

		err = client.Disconnect(context.Background())
		assert.NoError(t, err)
		assert.False(t, client.IsConnected())
		assert.False(t, client.IsRegistered())
	})

	t.Run("disconnect multiple times is safe", func(t *testing.T) {
		config := &Config{
			MasterURL:      serverURL,
			SlaveID:        "slave-1",
			RequestTimeout: 5 * time.Second,
		}
		client := NewClient(config)

		err := client.Connect(context.Background())
		require.NoError(t, err)

		err = client.Disconnect(context.Background())
		assert.NoError(t, err)

		err = client.Disconnect(context.Background())
		assert.NoError(t, err)
	})
}

func TestClient_Callbacks(t *testing.T) {
	_, serverURL := setupTestServer(t)

	t.Run("set task handler", func(t *testing.T) {
		config := &Config{
			MasterURL: serverURL,
			SlaveID:   "slave-1",
		}
		client := NewClient(config)

		var called atomic.Bool
		client.SetTaskHandler(func(ctx context.Context, task *rest.TaskAssignment) error {
			called.Store(true)
			return nil
		})

		assert.NotNil(t, client.onTask)
	})

	t.Run("set command handler", func(t *testing.T) {
		config := &Config{
			MasterURL: serverURL,
			SlaveID:   "slave-1",
		}
		client := NewClient(config)

		var called atomic.Bool
		client.SetCommandHandler(func(ctx context.Context, cmd *rest.ControlCommand) error {
			called.Store(true)
			return nil
		})

		assert.NotNil(t, client.onCommand)
	})

	t.Run("set disconnect handler", func(t *testing.T) {
		config := &Config{
			MasterURL: serverURL,
			SlaveID:   "slave-1",
		}
		client := NewClient(config)

		var called atomic.Bool
		client.SetDisconnectHandler(func(err error) {
			called.Store(true)
		})

		assert.NotNil(t, client.onDisconnect)
	})

	t.Run("set reconnect handler", func(t *testing.T) {
		config := &Config{
			MasterURL: serverURL,
			SlaveID:   "slave-1",
		}
		client := NewClient(config)

		var called atomic.Bool
		client.SetReconnectHandler(func() {
			called.Store(true)
		})

		assert.NotNil(t, client.onReconnect)
	})
}

func TestClient_GettersAndHelpers(t *testing.T) {
	t.Run("GetSlaveID", func(t *testing.T) {
		config := &Config{
			SlaveID: "slave-123",
		}
		client := NewClient(config)
		assert.Equal(t, "slave-123", client.GetSlaveID())
	})

	t.Run("GetConfig", func(t *testing.T) {
		config := &Config{
			MasterURL: "http://test:8080",
			SlaveID:   "slave-1",
		}
		client := NewClient(config)
		assert.Equal(t, config, client.GetConfig())
	})

	t.Run("IsRetryableError", func(t *testing.T) {
		assert.True(t, IsRetryableError(fiber.StatusServiceUnavailable))
		assert.True(t, IsRetryableError(fiber.StatusGatewayTimeout))
		assert.True(t, IsRetryableError(fiber.StatusBadGateway))
		assert.True(t, IsRetryableError(fiber.StatusTooManyRequests))
		assert.True(t, IsRetryableError(fiber.StatusRequestTimeout))
		assert.False(t, IsRetryableError(fiber.StatusOK))
		assert.False(t, IsRetryableError(fiber.StatusBadRequest))
		assert.False(t, IsRetryableError(fiber.StatusNotFound))
	})
}

func TestClient_CommandHandling(t *testing.T) {
	app := fiber.New()

	// Heartbeat endpoint that returns commands
	app.Post("/api/v1/slaves/:id/heartbeat", func(c *fiber.Ctx) error {
		return c.JSON(rest.SlaveHeartbeatResponse{
			Commands: []*rest.ControlCommand{
				{
					Type:        "stop",
					ExecutionID: "exec-1",
					Params:      map[string]string{"reason": "test"},
				},
			},
			Timestamp: time.Now().UnixMilli(),
		})
	})

	app.Get("/api/v1/health", func(c *fiber.Ctx) error {
		return c.JSON(rest.HealthResponse{Status: "healthy"})
	})

	app.Post("/api/v1/slaves/register", func(c *fiber.Ctx) error {
		return c.JSON(rest.SlaveRegisterResponse{
			Accepted:          true,
			HeartbeatInterval: 50,
		})
	})

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	t.Run("receive and handle commands", func(t *testing.T) {
		config := &Config{
			MasterURL:         server.URL,
			SlaveID:           "slave-1",
			HeartbeatInterval: 50 * time.Millisecond,
			RequestTimeout:    5 * time.Second,
		}
		client := NewClient(config)

		var commandReceived atomic.Bool
		var receivedCmd *rest.ControlCommand

		client.SetCommandHandler(func(ctx context.Context, cmd *rest.ControlCommand) error {
			commandReceived.Store(true)
			receivedCmd = cmd
			return nil
		})

		err := client.Connect(context.Background())
		require.NoError(t, err)

		err = client.Register(context.Background())
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err = client.StartHeartbeat(ctx, func() *rest.SlaveStatusInfo {
			return &rest.SlaveStatusInfo{State: "online"}
		})
		require.NoError(t, err)

		// Wait for heartbeat to process
		time.Sleep(100 * time.Millisecond)

		assert.True(t, commandReceived.Load())
		assert.NotNil(t, receivedCmd)
		assert.Equal(t, "stop", receivedCmd.Type)
		assert.Equal(t, "exec-1", receivedCmd.ExecutionID)
	})
}

func TestClient_BufferOverflow(t *testing.T) {
	t.Run("result buffer overflow drops oldest", func(t *testing.T) {
		config := &Config{
			MasterURL:        "http://localhost:9999",
			SlaveID:          "slave-1",
			ResultBufferSize: 2,
		}
		client := NewClient(config)

		// Fill buffer
		for i := 0; i < 5; i++ {
			result := &BufferedResult{
				TaskID: "task-" + string(rune('0'+i)),
			}
			client.bufferResult(result)
		}

		// Buffer should be at capacity
		assert.LessOrEqual(t, len(client.resultBuffer), 2)
	})

	t.Run("metrics buffer overflow drops oldest", func(t *testing.T) {
		config := &Config{
			MasterURL:         "http://localhost:9999",
			SlaveID:           "slave-1",
			MetricsBufferSize: 2,
		}
		client := NewClient(config)

		// Fill buffer
		for i := 0; i < 5; i++ {
			metrics := &BufferedMetrics{
				ExecutionID: "exec-" + string(rune('0'+i)),
			}
			client.bufferMetrics(metrics)
		}

		// Buffer should be at capacity
		assert.LessOrEqual(t, len(client.metricsBuffer), 2)
	})
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "http://localhost:8080", config.MasterURL)
	assert.Equal(t, types.SlaveTypeWorker, config.SlaveType)
	assert.Equal(t, 5*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 30*time.Second, config.RequestTimeout)
	assert.Equal(t, 5*time.Second, config.ReconnectInterval)
	assert.Equal(t, 0, config.MaxReconnectAttempts)
	assert.Equal(t, 1000, config.ResultBufferSize)
	assert.Equal(t, 1000, config.MetricsBufferSize)
	assert.Equal(t, 1*time.Second, config.TaskPollInterval)
}

// TestJSONSerialization tests JSON serialization round-trip for request/response types.
func TestJSONSerialization(t *testing.T) {
	t.Run("SlaveRegisterRequest round-trip", func(t *testing.T) {
		original := &rest.SlaveRegisterRequest{
			SlaveID:      "slave-1",
			SlaveType:    "worker",
			Capabilities: []string{"http", "grpc"},
			Labels:       map[string]string{"env": "test"},
			Address:      "localhost:8081",
			Resources: &rest.ResourceInfo{
				CPUCores:    4,
				MemoryMB:    8192,
				MaxVUs:      100,
				CurrentLoad: 0.5,
			},
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded rest.SlaveRegisterRequest
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.SlaveID, decoded.SlaveID)
		assert.Equal(t, original.SlaveType, decoded.SlaveType)
		assert.Equal(t, original.Capabilities, decoded.Capabilities)
		assert.Equal(t, original.Labels, decoded.Labels)
		assert.Equal(t, original.Address, decoded.Address)
		assert.Equal(t, original.Resources.CPUCores, decoded.Resources.CPUCores)
	})

	t.Run("SlaveHeartbeatRequest round-trip", func(t *testing.T) {
		original := &rest.SlaveHeartbeatRequest{
			SlaveID: "slave-1",
			Status: &rest.SlaveStatusInfo{
				State:       "online",
				Load:        0.75,
				ActiveTasks: 5,
				LastSeen:    time.Now().UnixMilli(),
				Metrics: &rest.SlaveMetrics{
					CPUUsage:    50.0,
					MemoryUsage: 60.0,
					ActiveVUs:   10,
					Throughput:  100.5,
				},
			},
			Timestamp: time.Now().UnixMilli(),
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded rest.SlaveHeartbeatRequest
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.SlaveID, decoded.SlaveID)
		assert.Equal(t, original.Status.State, decoded.Status.State)
		assert.Equal(t, original.Status.Load, decoded.Status.Load)
		assert.Equal(t, original.Status.Metrics.CPUUsage, decoded.Status.Metrics.CPUUsage)
	})

	t.Run("TaskResultRequest round-trip", func(t *testing.T) {
		original := &rest.TaskResultRequest{
			TaskID:      "task-1",
			ExecutionID: "exec-1",
			SlaveID:     "slave-1",
			Status:      "completed",
			Result:      map[string]interface{}{"key": "value", "count": float64(42)},
			Errors: []*rest.ExecutionErrorRequest{
				{
					Code:      "ERR001",
					Message:   "test error",
					StepID:    "step-1",
					Timestamp: time.Now().UnixMilli(),
				},
			},
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded rest.TaskResultRequest
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.TaskID, decoded.TaskID)
		assert.Equal(t, original.ExecutionID, decoded.ExecutionID)
		assert.Equal(t, original.Status, decoded.Status)
		assert.Equal(t, original.Result["key"], decoded.Result["key"])
		assert.Equal(t, len(original.Errors), len(decoded.Errors))
	})

	t.Run("MetricsReportRequest round-trip", func(t *testing.T) {
		original := &rest.MetricsReportRequest{
			SlaveID:     "slave-1",
			ExecutionID: "exec-1",
			Timestamp:   time.Now().UnixMilli(),
			Metrics: &rest.MetricsData{
				TotalVUs:        10,
				TotalIterations: 1000,
				DurationMs:      60000,
			},
			StepMetrics: map[string]*rest.StepMetricsData{
				"step-1": {
					StepID:       "step-1",
					Count:        100,
					SuccessCount: 95,
					FailureCount: 5,
					Duration: &rest.DurationMetricsData{
						MinNs: 1000000,
						MaxNs: 5000000,
						AvgNs: 2500000,
						P50Ns: 2000000,
						P90Ns: 4000000,
						P95Ns: 4500000,
						P99Ns: 4900000,
					},
				},
			},
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded rest.MetricsReportRequest
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.SlaveID, decoded.SlaveID)
		assert.Equal(t, original.ExecutionID, decoded.ExecutionID)
		assert.Equal(t, original.Metrics.TotalVUs, decoded.Metrics.TotalVUs)
		assert.Equal(t, original.StepMetrics["step-1"].Count, decoded.StepMetrics["step-1"].Count)
	})
}
