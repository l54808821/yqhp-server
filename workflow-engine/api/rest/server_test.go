package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yqhp/workflow-engine/internal/master"
	"yqhp/workflow-engine/pkg/types"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMaster implements master.Master for testing.
type mockMaster struct {
	executions map[string]*types.ExecutionState
	workflows  map[string]*types.Workflow
}

func newMockMaster() *mockMaster {
	return &mockMaster{
		executions: make(map[string]*types.ExecutionState),
		workflows:  make(map[string]*types.Workflow),
	}
}

func (m *mockMaster) Start(ctx context.Context) error {
	return nil
}

func (m *mockMaster) Stop(ctx context.Context) error {
	return nil
}

func (m *mockMaster) SubmitWorkflow(ctx context.Context, workflow *types.Workflow) (string, error) {
	executionID := "exec-" + workflow.ID
	m.workflows[workflow.ID] = workflow
	m.executions[executionID] = &types.ExecutionState{
		ID:          executionID,
		WorkflowID:  workflow.ID,
		Status:      types.ExecutionStatusRunning,
		StartTime:   time.Now(),
		Progress:    0,
		SlaveStates: make(map[string]*types.SlaveExecutionState),
		Errors:      []types.ExecutionError{},
	}
	return executionID, nil
}

func (m *mockMaster) GetExecutionStatus(ctx context.Context, executionID string) (*types.ExecutionState, error) {
	if state, ok := m.executions[executionID]; ok {
		return state, nil
	}
	return nil, fiber.NewError(fiber.StatusNotFound, "execution not found")
}

func (m *mockMaster) StopExecution(ctx context.Context, executionID string) error {
	if state, ok := m.executions[executionID]; ok {
		state.Status = types.ExecutionStatusAborted
		return nil
	}
	return fiber.NewError(fiber.StatusNotFound, "execution not found")
}

func (m *mockMaster) PauseExecution(ctx context.Context, executionID string) error {
	if state, ok := m.executions[executionID]; ok {
		state.Status = types.ExecutionStatusPaused
		return nil
	}
	return fiber.NewError(fiber.StatusNotFound, "execution not found")
}

func (m *mockMaster) ResumeExecution(ctx context.Context, executionID string) error {
	if state, ok := m.executions[executionID]; ok {
		state.Status = types.ExecutionStatusRunning
		return nil
	}
	return fiber.NewError(fiber.StatusNotFound, "execution not found")
}

func (m *mockMaster) ScaleExecution(ctx context.Context, executionID string, targetVUs int) error {
	if _, ok := m.executions[executionID]; ok {
		return nil
	}
	return fiber.NewError(fiber.StatusNotFound, "execution not found")
}

func (m *mockMaster) GetMetrics(ctx context.Context, executionID string) (*types.AggregatedMetrics, error) {
	if _, ok := m.executions[executionID]; ok {
		return &types.AggregatedMetrics{
			ExecutionID:     executionID,
			TotalVUs:        10,
			TotalIterations: 100,
			Duration:        time.Minute,
			StepMetrics:     make(map[string]*types.StepMetrics),
		}, nil
	}
	return nil, fiber.NewError(fiber.StatusNotFound, "execution not found")
}

func (m *mockMaster) GetSlaves(ctx context.Context) ([]*types.SlaveInfo, error) {
	return []*types.SlaveInfo{}, nil
}

// mockRegistry implements master.SlaveRegistry for testing.
type mockRegistry struct {
	slaves map[string]*types.SlaveInfo
	status map[string]*types.SlaveStatus
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		slaves: make(map[string]*types.SlaveInfo),
		status: make(map[string]*types.SlaveStatus),
	}
}

func (r *mockRegistry) Register(ctx context.Context, slave *types.SlaveInfo) error {
	r.slaves[slave.ID] = slave
	r.status[slave.ID] = &types.SlaveStatus{
		State:    types.SlaveStateOnline,
		LastSeen: time.Now(),
	}
	return nil
}

func (r *mockRegistry) Unregister(ctx context.Context, slaveID string) error {
	delete(r.slaves, slaveID)
	delete(r.status, slaveID)
	return nil
}

func (r *mockRegistry) UpdateStatus(ctx context.Context, slaveID string, status *types.SlaveStatus) error {
	r.status[slaveID] = status
	return nil
}

func (r *mockRegistry) GetSlave(ctx context.Context, slaveID string) (*types.SlaveInfo, error) {
	if slave, ok := r.slaves[slaveID]; ok {
		return slave, nil
	}
	return nil, fiber.NewError(fiber.StatusNotFound, "slave not found")
}

func (r *mockRegistry) GetSlaveStatus(ctx context.Context, slaveID string) (*types.SlaveStatus, error) {
	if status, ok := r.status[slaveID]; ok {
		return status, nil
	}
	return nil, fiber.NewError(fiber.StatusNotFound, "slave not found")
}

func (r *mockRegistry) ListSlaves(ctx context.Context, filter *master.SlaveFilter) ([]*types.SlaveInfo, error) {
	result := make([]*types.SlaveInfo, 0, len(r.slaves))
	for _, slave := range r.slaves {
		result = append(result, slave)
	}
	return result, nil
}

func (r *mockRegistry) GetOnlineSlaves(ctx context.Context) ([]*types.SlaveInfo, error) {
	return r.ListSlaves(ctx, nil)
}

func (r *mockRegistry) WatchSlaves(ctx context.Context) (<-chan *types.SlaveEvent, error) {
	ch := make(chan *types.SlaveEvent)
	return ch, nil
}

func (r *mockRegistry) DrainSlave(ctx context.Context, slaveID string) error {
	if status, ok := r.status[slaveID]; ok {
		status.State = types.SlaveStateDraining
		return nil
	}
	return fiber.NewError(fiber.StatusNotFound, "slave not found")
}

func TestHealthCheck(t *testing.T) {
	mockM := newMockMaster()
	server := NewServer(mockM, nil, nil)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result HealthResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, "healthy", result.Status)
}

func TestReadyCheck(t *testing.T) {
	mockM := newMockMaster()
	server := NewServer(mockM, nil, nil)

	req := httptest.NewRequest("GET", "/ready", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result ReadyResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.True(t, result.Ready)
}

func TestSubmitWorkflow(t *testing.T) {
	mockM := newMockMaster()
	server := NewServer(mockM, nil, nil)

	workflowJSON := `{
		"workflow": {
			"id": "test-workflow",
			"name": "Test Workflow",
			"steps": []
		}
	}`

	req := httptest.NewRequest("POST", "/api/v1/workflows", strings.NewReader(workflowJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result WorkflowSubmitResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, "test-workflow", result.WorkflowID)
	assert.NotEmpty(t, result.ExecutionID)
}

func TestGetExecution(t *testing.T) {
	mockM := newMockMaster()
	server := NewServer(mockM, nil, nil)

	// First submit a workflow
	workflowJSON := `{
		"workflow": {
			"id": "test-workflow",
			"name": "Test Workflow",
			"steps": []
		}
	}`
	req := httptest.NewRequest("POST", "/api/v1/workflows", strings.NewReader(workflowJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := server.App().Test(req)
	body, _ := io.ReadAll(resp.Body)
	var submitResult WorkflowSubmitResponse
	json.Unmarshal(body, &submitResult)

	// Now get the execution
	req = httptest.NewRequest("GET", "/api/v1/executions/"+submitResult.ExecutionID, nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ = io.ReadAll(resp.Body)
	var result ExecutionResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, submitResult.ExecutionID, result.ID)
	assert.Equal(t, "running", result.Status)
}

func TestPauseExecution(t *testing.T) {
	mockM := newMockMaster()
	server := NewServer(mockM, nil, nil)

	// First submit a workflow
	workflowJSON := `{
		"workflow": {
			"id": "test-workflow",
			"name": "Test Workflow",
			"steps": []
		}
	}`
	req := httptest.NewRequest("POST", "/api/v1/workflows", strings.NewReader(workflowJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := server.App().Test(req)
	body, _ := io.ReadAll(resp.Body)
	var submitResult WorkflowSubmitResponse
	json.Unmarshal(body, &submitResult)

	// Pause the execution
	req = httptest.NewRequest("POST", "/api/v1/executions/"+submitResult.ExecutionID+"/pause", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ = io.ReadAll(resp.Body)
	var result SuccessResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestResumeExecution(t *testing.T) {
	mockM := newMockMaster()
	server := NewServer(mockM, nil, nil)

	// First submit a workflow
	workflowJSON := `{
		"workflow": {
			"id": "test-workflow",
			"name": "Test Workflow",
			"steps": []
		}
	}`
	req := httptest.NewRequest("POST", "/api/v1/workflows", strings.NewReader(workflowJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := server.App().Test(req)
	body, _ := io.ReadAll(resp.Body)
	var submitResult WorkflowSubmitResponse
	json.Unmarshal(body, &submitResult)

	// Pause first
	req = httptest.NewRequest("POST", "/api/v1/executions/"+submitResult.ExecutionID+"/pause", nil)
	server.App().Test(req)

	// Resume the execution
	req = httptest.NewRequest("POST", "/api/v1/executions/"+submitResult.ExecutionID+"/resume", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ = io.ReadAll(resp.Body)
	var result SuccessResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestScaleExecution(t *testing.T) {
	mockM := newMockMaster()
	server := NewServer(mockM, nil, nil)

	// First submit a workflow
	workflowJSON := `{
		"workflow": {
			"id": "test-workflow",
			"name": "Test Workflow",
			"steps": []
		}
	}`
	req := httptest.NewRequest("POST", "/api/v1/workflows", strings.NewReader(workflowJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := server.App().Test(req)
	body, _ := io.ReadAll(resp.Body)
	var submitResult WorkflowSubmitResponse
	json.Unmarshal(body, &submitResult)

	// Scale the execution
	scaleJSON := `{"target_vus": 50}`
	req = httptest.NewRequest("POST", "/api/v1/executions/"+submitResult.ExecutionID+"/scale", strings.NewReader(scaleJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ = io.ReadAll(resp.Body)
	var result SuccessResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestStopExecution(t *testing.T) {
	mockM := newMockMaster()
	server := NewServer(mockM, nil, nil)

	// First submit a workflow
	workflowJSON := `{
		"workflow": {
			"id": "test-workflow",
			"name": "Test Workflow",
			"steps": []
		}
	}`
	req := httptest.NewRequest("POST", "/api/v1/workflows", strings.NewReader(workflowJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := server.App().Test(req)
	body, _ := io.ReadAll(resp.Body)
	var submitResult WorkflowSubmitResponse
	json.Unmarshal(body, &submitResult)

	// Stop the execution
	req = httptest.NewRequest("DELETE", "/api/v1/executions/"+submitResult.ExecutionID, nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ = io.ReadAll(resp.Body)
	var result SuccessResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestGetMetrics(t *testing.T) {
	mockM := newMockMaster()
	server := NewServer(mockM, nil, nil)

	// First submit a workflow
	workflowJSON := `{
		"workflow": {
			"id": "test-workflow",
			"name": "Test Workflow",
			"steps": []
		}
	}`
	req := httptest.NewRequest("POST", "/api/v1/workflows", strings.NewReader(workflowJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := server.App().Test(req)
	body, _ := io.ReadAll(resp.Body)
	var submitResult WorkflowSubmitResponse
	json.Unmarshal(body, &submitResult)

	// Get metrics
	req = httptest.NewRequest("GET", "/api/v1/executions/"+submitResult.ExecutionID+"/metrics", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ = io.ReadAll(resp.Body)
	var result MetricsResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, submitResult.ExecutionID, result.ExecutionID)
	assert.Equal(t, 10, result.TotalVUs)
}

func TestListSlaves(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()

	// Register a slave
	mockR.Register(context.Background(), &types.SlaveInfo{
		ID:           "slave-1",
		Type:         types.SlaveTypeWorker,
		Address:      "localhost:9000",
		Capabilities: []string{"http_executor"},
	})

	server := NewServer(mockM, mockR, nil)

	req := httptest.NewRequest("GET", "/api/v1/slaves", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result SlaveListResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, "slave-1", result.Slaves[0].ID)
}

func TestGetSlave(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()

	// Register a slave
	mockR.Register(context.Background(), &types.SlaveInfo{
		ID:           "slave-1",
		Type:         types.SlaveTypeWorker,
		Address:      "localhost:9000",
		Capabilities: []string{"http_executor"},
	})

	server := NewServer(mockM, mockR, nil)

	req := httptest.NewRequest("GET", "/api/v1/slaves/slave-1", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result SlaveResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, "slave-1", result.ID)
	assert.Equal(t, "worker", result.Type)
}

func TestDrainSlave(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()

	// Register a slave
	mockR.Register(context.Background(), &types.SlaveInfo{
		ID:           "slave-1",
		Type:         types.SlaveTypeWorker,
		Address:      "localhost:9000",
		Capabilities: []string{"http_executor"},
	})

	server := NewServer(mockM, mockR, nil)

	req := httptest.NewRequest("POST", "/api/v1/slaves/slave-1/drain", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result SuccessResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestAPIKeyAuth(t *testing.T) {
	mockM := newMockMaster()
	config := &Config{
		Address:      ":8080",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		EnableCORS:   true,
		Auth: &AuthConfig{
			Enabled: true,
			Type:    "api_key",
			APIKey:  "test-api-key",
		},
	}
	server := NewServer(mockM, nil, config)

	// Request without API key should fail
	req := httptest.NewRequest("GET", "/api/v1/executions/test", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	// Request with wrong API key should fail
	req = httptest.NewRequest("GET", "/api/v1/executions/test", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	resp, err = server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	// Request with correct API key should succeed (or return 404 for non-existent execution)
	req = httptest.NewRequest("GET", "/api/v1/executions/test", nil)
	req.Header.Set("X-API-Key", "test-api-key")
	resp, err = server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode) // 404 because execution doesn't exist

	// Health check should work without auth
	req = httptest.NewRequest("GET", "/health", nil)
	resp, err = server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestExtractExecutionID(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "valid path",
			path:     "/api/v1/executions/exec-123/metrics/stream",
			expected: "exec-123",
		},
		{
			name:     "valid path with uuid",
			path:     "/api/v1/executions/550e8400-e29b-41d4-a716-446655440000/metrics/stream",
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "path too short",
			path:     "/api/v1/executions//metrics/stream",
			expected: "",
		},
		{
			name:     "invalid path format",
			path:     "/api/v1/workflows/123",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractExecutionID(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMetricsStreamConfig(t *testing.T) {
	config := DefaultMetricsStreamConfig()
	assert.Equal(t, time.Second, config.Interval)
	assert.Equal(t, 100, config.BufferSize)
}

func TestMetricsStreamer(t *testing.T) {
	mockM := newMockMaster()
	server := NewServer(mockM, nil, nil)
	streamer := NewMetricsStreamer(server, nil)

	assert.NotNil(t, streamer)
	assert.NotNil(t, streamer.connections)
	assert.Equal(t, time.Second, streamer.config.Interval)
}

// ============================================================================
// Slave 通信端点测试
// ============================================================================

func TestRegisterSlave(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()
	server := NewServer(mockM, mockR, nil)

	registerJSON := `{
		"slave_id": "slave-test-1",
		"slave_type": "worker",
		"capabilities": ["http_executor", "script_executor"],
		"address": "localhost:9001",
		"resources": {
			"cpu_cores": 4,
			"memory_mb": 8192,
			"max_vus": 100
		}
	}`

	req := httptest.NewRequest("POST", "/api/v1/slaves/register", strings.NewReader(registerJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result SlaveRegisterResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, "slave-test-1", result.AssignedID)
	assert.Greater(t, result.HeartbeatInterval, int64(0))
}

func TestRegisterSlaveWithoutID(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()
	server := NewServer(mockM, mockR, nil)

	registerJSON := `{
		"slave_type": "worker",
		"address": "localhost:9001"
	}`

	req := httptest.NewRequest("POST", "/api/v1/slaves/register", strings.NewReader(registerJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestRegisterSlaveWithoutRegistry(t *testing.T) {
	mockM := newMockMaster()
	server := NewServer(mockM, nil, nil)

	registerJSON := `{
		"slave_id": "slave-test-1",
		"slave_type": "worker",
		"address": "localhost:9001"
	}`

	req := httptest.NewRequest("POST", "/api/v1/slaves/register", strings.NewReader(registerJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
}

func TestSlaveHeartbeat(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()
	server := NewServer(mockM, mockR, nil)

	// 先注册 Slave
	mockR.Register(context.Background(), &types.SlaveInfo{
		ID:      "slave-test-1",
		Type:    types.SlaveTypeWorker,
		Address: "localhost:9001",
	})

	heartbeatJSON := `{
		"slave_id": "slave-test-1",
		"status": {
			"state": "online",
			"load": 25.5,
			"active_tasks": 2,
			"metrics": {
				"cpu_usage": 30.0,
				"memory_usage": 45.0,
				"active_vus": 10,
				"throughput": 100.5
			}
		},
		"timestamp": 1234567890
	}`

	req := httptest.NewRequest("POST", "/api/v1/slaves/slave-test-1/heartbeat", strings.NewReader(heartbeatJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result SlaveHeartbeatResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Greater(t, result.Timestamp, int64(0))
}

func TestSlaveHeartbeatNotFound(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()
	server := NewServer(mockM, mockR, nil)

	heartbeatJSON := `{
		"slave_id": "non-existent-slave",
		"status": {
			"state": "online"
		}
	}`

	req := httptest.NewRequest("POST", "/api/v1/slaves/non-existent-slave/heartbeat", strings.NewReader(heartbeatJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestGetSlaveTasks(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()
	server := NewServer(mockM, mockR, nil)

	// 先注册 Slave
	mockR.Register(context.Background(), &types.SlaveInfo{
		ID:      "slave-test-1",
		Type:    types.SlaveTypeWorker,
		Address: "localhost:9001",
	})

	req := httptest.NewRequest("GET", "/api/v1/slaves/slave-test-1/tasks", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result PendingTasksResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	// 初始应该没有任务
	assert.Empty(t, result.Tasks)
}

func TestGetSlaveTasksNotFound(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()
	server := NewServer(mockM, mockR, nil)

	req := httptest.NewRequest("GET", "/api/v1/slaves/non-existent-slave/tasks", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestReceiveTaskResult(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()
	server := NewServer(mockM, mockR, nil)

	resultJSON := `{
		"task_id": "task-1",
		"execution_id": "exec-1",
		"slave_id": "slave-test-1",
		"status": "completed",
		"result": {
			"iterations": 100,
			"success_count": 98,
			"failure_count": 2
		}
	}`

	req := httptest.NewRequest("POST", "/api/v1/tasks/task-1/result", strings.NewReader(resultJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result TaskResultResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestReceiveTaskResultMissingFields(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()
	server := NewServer(mockM, mockR, nil)

	// 缺少 execution_id
	resultJSON := `{
		"task_id": "task-1",
		"slave_id": "slave-test-1",
		"status": "completed"
	}`

	req := httptest.NewRequest("POST", "/api/v1/tasks/task-1/result", strings.NewReader(resultJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestReceiveMetricsReport(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()
	server := NewServer(mockM, mockR, nil)

	metricsJSON := `{
		"slave_id": "slave-test-1",
		"execution_id": "exec-1",
		"timestamp": 1234567890,
		"metrics": {
			"total_vus": 10,
			"total_iterations": 1000,
			"duration_ms": 60000
		},
		"step_metrics": {
			"step-1": {
				"step_id": "step-1",
				"count": 500,
				"success_count": 490,
				"failure_count": 10
			}
		}
	}`

	req := httptest.NewRequest("POST", "/api/v1/executions/exec-1/metrics/report", strings.NewReader(metricsJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result MetricsReportResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestReceiveMetricsReportMissingSlaveID(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()
	server := NewServer(mockM, mockR, nil)

	metricsJSON := `{
		"execution_id": "exec-1",
		"timestamp": 1234567890,
		"metrics": {
			"total_vus": 10
		}
	}`

	req := httptest.NewRequest("POST", "/api/v1/executions/exec-1/metrics/report", strings.NewReader(metricsJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestUnregisterSlave(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()
	server := NewServer(mockM, mockR, nil)

	// 先注册 Slave
	mockR.Register(context.Background(), &types.SlaveInfo{
		ID:      "slave-test-1",
		Type:    types.SlaveTypeWorker,
		Address: "localhost:9001",
	})

	req := httptest.NewRequest("POST", "/api/v1/slaves/slave-test-1/unregister", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result SlaveUnregisterResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// 验证 Slave 已被注销
	_, err = mockR.GetSlave(context.Background(), "slave-test-1")
	assert.Error(t, err)
}

func TestAssignTaskAndGetTasks(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()
	server := NewServer(mockM, mockR, nil)

	// 先注册 Slave
	mockR.Register(context.Background(), &types.SlaveInfo{
		ID:      "slave-test-1",
		Type:    types.SlaveTypeWorker,
		Address: "localhost:9001",
	})

	// 分配任务
	task := &TaskAssignment{
		TaskID:      "task-1",
		ExecutionID: "exec-1",
		Segment: &ExecutionSegment{
			Start: 0.0,
			End:   0.5,
		},
		Options: &ExecutionOptions{
			VUs:        10,
			DurationMs: 60000,
		},
	}
	err := server.AssignTask("slave-test-1", task)
	require.NoError(t, err)

	// 获取任务
	req := httptest.NewRequest("GET", "/api/v1/slaves/slave-test-1/tasks", nil)
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result PendingTasksResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Len(t, result.Tasks, 1)
	assert.Equal(t, "task-1", result.Tasks[0].TaskID)
}

func TestSendCommandAndHeartbeat(t *testing.T) {
	mockM := newMockMaster()
	mockR := newMockRegistry()
	server := NewServer(mockM, mockR, nil)

	// 先注册 Slave
	mockR.Register(context.Background(), &types.SlaveInfo{
		ID:      "slave-test-1",
		Type:    types.SlaveTypeWorker,
		Address: "localhost:9001",
	})

	// 发送命令
	cmd := &ControlCommand{
		Type:        "stop",
		ExecutionID: "exec-1",
		Params:      map[string]string{"reason": "user_request"},
	}
	err := server.SendCommand("slave-test-1", cmd)
	require.NoError(t, err)

	// 通过心跳获取命令
	heartbeatJSON := `{
		"slave_id": "slave-test-1",
		"status": {
			"state": "online"
		}
	}`

	req := httptest.NewRequest("POST", "/api/v1/slaves/slave-test-1/heartbeat", strings.NewReader(heartbeatJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.App().Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result SlaveHeartbeatResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Len(t, result.Commands, 1)
	assert.Equal(t, "stop", result.Commands[0].Type)
	assert.Equal(t, "exec-1", result.Commands[0].ExecutionID)
}
