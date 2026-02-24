// Package client implements the HTTP client for Slave nodes using Fiber.
// Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/websocket"

	"yqhp/workflow-engine/pkg/types"
)

// Config holds the configuration for the HTTP client.
type Config struct {
	// MasterURL is the base URL of the master node (e.g., "http://localhost:8080")
	MasterURL string

	// SlaveID is the unique identifier for this slave.
	SlaveID string

	// SlaveType is the type of this slave.
	SlaveType types.SlaveType

	// Capabilities are the capabilities this slave supports.
	Capabilities []string

	// Labels are key-value labels for this slave.
	Labels map[string]string

	// Address is the address this slave listens on.
	Address string

	// Resources contains resource information.
	Resources *types.APIResourceInfo

	// HeartbeatInterval is the interval between heartbeats.
	HeartbeatInterval time.Duration

	// RequestTimeout is the timeout for HTTP requests.
	RequestTimeout time.Duration

	// ReconnectInterval is the interval between reconnection attempts.
	ReconnectInterval time.Duration

	// MaxReconnectAttempts is the maximum number of reconnection attempts.
	// 0 means unlimited.
	MaxReconnectAttempts int

	// ResultBufferSize is the size of the result buffer.
	ResultBufferSize int

	// MetricsBufferSize is the size of the metrics buffer.
	MetricsBufferSize int

	// TaskPollInterval is the interval for polling tasks.
	TaskPollInterval time.Duration
}

// DefaultConfig returns a default client configuration.
func DefaultConfig() *Config {
	return &Config{
		MasterURL:            "http://localhost:8080",
		SlaveType:            types.SlaveTypeWorker,
		HeartbeatInterval:    5 * time.Second,
		RequestTimeout:       30 * time.Second,
		ReconnectInterval:    5 * time.Second,
		MaxReconnectAttempts: 0, // Unlimited
		ResultBufferSize:     1000,
		MetricsBufferSize:    1000,
		TaskPollInterval:     1 * time.Second,
	}
}

// Client implements the HTTP client for slave nodes.
type Client struct {
	config *Config
	agent  *fiber.Client

	// State management
	connected        atomic.Bool
	registered       atomic.Bool
	reconnecting     atomic.Bool
	reconnectAttempt atomic.Int32

	// Heartbeat management (HTTP mode)
	heartbeatCtx    context.Context
	heartbeatCancel context.CancelFunc

	// Task polling (HTTP mode)
	taskPollCtx    context.Context
	taskPollCancel context.CancelFunc

	// WebSocket connection (replaces heartbeat + task polling when active)
	wsConn      *websocket.Conn
	wsSend      chan []byte
	wsDone      chan struct{}
	wsCloseOnce sync.Once

	// Result buffering for network resilience
	resultBuffer  chan *BufferedResult
	metricsBuffer chan *BufferedMetrics
	bufferedCount atomic.Int64

	// Callbacks
	onTask       TaskHandler
	onCommand    CommandHandler
	onDisconnect DisconnectHandler
	onReconnect  ReconnectHandler

	// Synchronization
	mu       sync.RWMutex
	stopOnce sync.Once
	stopped  chan struct{}
}

// BufferedResult holds a task result for buffering.
type BufferedResult struct {
	TaskID      string
	ExecutionID string
	Status      string
	Result      map[string]interface{}
	Errors      []*types.ExecutionErrorRequest
}

// BufferedMetrics holds metrics for buffering.
type BufferedMetrics struct {
	ExecutionID string
	Metrics     *types.APIMetricsData
	StepMetrics map[string]*types.StepMetricsData
}

// TaskHandler handles incoming task assignments.
type TaskHandler func(ctx context.Context, task *types.TaskAssignment) error

// CommandHandler handles incoming control commands.
type CommandHandler func(ctx context.Context, cmd *types.ControlCommand) error

// DisconnectHandler is called when the client disconnects.
type DisconnectHandler func(err error)

// ReconnectHandler is called when the client reconnects.
type ReconnectHandler func()

// NewClient creates a new HTTP client.
func NewClient(config *Config) *Client {
	if config == nil {
		config = DefaultConfig()
	}

	agent := fiber.AcquireClient()

	return &Client{
		config:        config,
		agent:         agent,
		resultBuffer:  make(chan *BufferedResult, config.ResultBufferSize),
		metricsBuffer: make(chan *BufferedMetrics, config.MetricsBufferSize),
		stopped:       make(chan struct{}),
	}
}

// Connect establishes connection to the master node.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected.Load() {
		return fmt.Errorf("already connected")
	}

	// Test connection by calling health endpoint
	url := fmt.Sprintf("%s/api/v1/health", c.config.MasterURL)
	req := c.agent.Get(url)
	req.Timeout(c.config.RequestTimeout)

	statusCode, _, errs := req.Bytes()
	if len(errs) > 0 {
		return fmt.Errorf("failed to connect to master: %v", errs[0])
	}

	if statusCode != fiber.StatusOK {
		return fmt.Errorf("master health check failed with status: %d", statusCode)
	}

	c.connected.Store(true)
	return nil
}

// Disconnect disconnects from the master.
func (c *Client) Disconnect(ctx context.Context) error {
	c.stopOnce.Do(func() {
		close(c.stopped)

		// Cancel all contexts
		if c.heartbeatCancel != nil {
			c.heartbeatCancel()
		}
		if c.taskPollCancel != nil {
			c.taskPollCancel()
		}

		// Unregister from master
		if c.registered.Load() {
			c.unregister(ctx)
		}

		c.connected.Store(false)
		c.registered.Store(false)
	})
	return nil
}

// Register registers this slave with the master.
func (c *Client) Register(ctx context.Context) error {
	if !c.connected.Load() {
		return fmt.Errorf("not connected")
	}

	req := &types.SlaveRegisterRequest{
		SlaveID:      c.config.SlaveID,
		SlaveType:    string(c.config.SlaveType),
		Capabilities: c.config.Capabilities,
		Labels:       c.config.Labels,
		Address:      c.config.Address,
		Resources:    c.config.Resources,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal register request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/slaves/register", c.config.MasterURL)
	httpReq := c.agent.Post(url)
	httpReq.Timeout(c.config.RequestTimeout)
	httpReq.Body(body)
	httpReq.Set("Content-Type", "application/json")

	statusCode, respBody, errs := httpReq.Bytes()
	if len(errs) > 0 {
		return fmt.Errorf("failed to register: %v", errs[0])
	}

	if statusCode != fiber.StatusOK && statusCode != fiber.StatusCreated {
		var errResp types.ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return fmt.Errorf("registration failed: %s", errResp.Message)
		}
		return fmt.Errorf("registration failed with status: %d", statusCode)
	}

	var resp types.SlaveRegisterResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("failed to unmarshal register response: %w", err)
	}

	if !resp.Accepted {
		return fmt.Errorf("registration rejected: %s", resp.Error)
	}

	// Update slave ID if assigned
	if resp.AssignedID != "" {
		c.config.SlaveID = resp.AssignedID
	}

	// Update heartbeat interval if provided
	if resp.HeartbeatInterval > 0 {
		c.config.HeartbeatInterval = time.Duration(resp.HeartbeatInterval) * time.Millisecond
	}

	c.registered.Store(true)
	return nil
}

// StartHeartbeat starts the heartbeat loop.
func (c *Client) StartHeartbeat(ctx context.Context, statusProvider func() *types.APISlaveStatusInfo) error {
	if !c.registered.Load() {
		return fmt.Errorf("not registered")
	}

	c.heartbeatCtx, c.heartbeatCancel = context.WithCancel(ctx)

	// Start heartbeat goroutine
	go c.heartbeatLoop(statusProvider)

	return nil
}

// heartbeatLoop sends periodic heartbeats.
func (c *Client) heartbeatLoop(statusProvider func() *types.APISlaveStatusInfo) {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.heartbeatCtx.Done():
			return
		case <-c.stopped:
			return
		case <-ticker.C:
			resp, err := c.sendHeartbeat(statusProvider())
			if err != nil {
				c.handleDisconnect(err)
				return
			}

			// Process commands from heartbeat response
			if resp != nil && len(resp.Commands) > 0 {
				for _, cmd := range resp.Commands {
					if c.onCommand != nil {
						if err := c.onCommand(c.heartbeatCtx, cmd); err != nil {
							fmt.Printf("failed to handle command: %v\n", err)
						}
					}
				}
			}
		}
	}
}

// sendHeartbeat sends a single heartbeat.
func (c *Client) sendHeartbeat(status *types.APISlaveStatusInfo) (*types.SlaveHeartbeatResponse, error) {
	req := &types.SlaveHeartbeatRequest{
		SlaveID:   c.config.SlaveID,
		Status:    status,
		Timestamp: time.Now().UnixMilli(),
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal heartbeat request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/slaves/%s/heartbeat", c.config.MasterURL, c.config.SlaveID)
	httpReq := c.agent.Post(url)
	httpReq.Timeout(c.config.RequestTimeout)
	httpReq.Body(body)
	httpReq.Set("Content-Type", "application/json")

	statusCode, respBody, errs := httpReq.Bytes()
	if len(errs) > 0 {
		return nil, fmt.Errorf("heartbeat failed: %v", errs[0])
	}

	if statusCode != fiber.StatusOK {
		return nil, fmt.Errorf("heartbeat failed with status: %d", statusCode)
	}

	var resp types.SlaveHeartbeatResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal heartbeat response: %w", err)
	}

	return &resp, nil
}

// PollTasks polls for pending tasks from the master.
func (c *Client) PollTasks(ctx context.Context) ([]*types.TaskAssignment, error) {
	if !c.registered.Load() {
		return nil, fmt.Errorf("not registered")
	}

	url := fmt.Sprintf("%s/api/v1/slaves/%s/tasks", c.config.MasterURL, c.config.SlaveID)
	httpReq := c.agent.Get(url)
	httpReq.Timeout(c.config.RequestTimeout)

	statusCode, respBody, errs := httpReq.Bytes()
	if len(errs) > 0 {
		return nil, fmt.Errorf("poll tasks failed: %v", errs[0])
	}

	if statusCode != fiber.StatusOK {
		return nil, fmt.Errorf("poll tasks failed with status: %d", statusCode)
	}

	var resp types.PendingTasksResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tasks response: %w", err)
	}

	return resp.Tasks, nil
}

// StartTaskPolling starts the task polling loop.
func (c *Client) StartTaskPolling(ctx context.Context) error {
	if !c.registered.Load() {
		return fmt.Errorf("not registered")
	}

	c.taskPollCtx, c.taskPollCancel = context.WithCancel(ctx)

	// Start task polling goroutine
	go c.taskPollLoop()

	// Start sending buffered results
	go c.sendBufferedResults()

	// Start sending buffered metrics
	go c.sendBufferedMetrics()

	return nil
}

// taskPollLoop polls for tasks periodically.
func (c *Client) taskPollLoop() {
	ticker := time.NewTicker(c.config.TaskPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.taskPollCtx.Done():
			return
		case <-c.stopped:
			return
		case <-ticker.C:
			tasks, err := c.PollTasks(c.taskPollCtx)
			if err != nil {
				fmt.Printf("failed to poll tasks: %v\n", err)
				continue
			}

			// Handle each task
			for _, task := range tasks {
				if c.onTask != nil {
					if err := c.onTask(c.taskPollCtx, task); err != nil {
						fmt.Printf("failed to handle task: %v\n", err)
					}
				}
			}
		}
	}
}

// SendTaskResult sends a task result to the master.
// If not connected, the result is buffered for later delivery.
func (c *Client) SendTaskResult(result *BufferedResult) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}

	// If connected, send directly
	if c.connected.Load() && c.registered.Load() {
		if err := c.sendTaskResultDirect(result); err != nil {
			// Buffer the result for retry
			c.bufferResult(result)
			return err
		}
		return nil
	}

	// Buffer the result for later delivery
	c.bufferResult(result)
	return nil
}

// sendTaskResultDirect sends a task result directly to the master.
func (c *Client) sendTaskResultDirect(result *BufferedResult) error {
	req := &types.TaskResultRequest{
		TaskID:      result.TaskID,
		ExecutionID: result.ExecutionID,
		SlaveID:     c.config.SlaveID,
		Status:      result.Status,
		Result:      result.Result,
		Errors:      result.Errors,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal task result: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/tasks/%s/result", c.config.MasterURL, result.TaskID)
	httpReq := c.agent.Post(url)
	httpReq.Timeout(c.config.RequestTimeout)
	httpReq.Body(body)
	httpReq.Set("Content-Type", "application/json")

	statusCode, respBody, errs := httpReq.Bytes()
	if len(errs) > 0 {
		return fmt.Errorf("send task result failed: %v", errs[0])
	}

	if statusCode != fiber.StatusOK {
		var errResp types.ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return fmt.Errorf("send task result failed: %s", errResp.Message)
		}
		return fmt.Errorf("send task result failed with status: %d", statusCode)
	}

	return nil
}

// bufferResult adds a result to the buffer.
func (c *Client) bufferResult(result *BufferedResult) {
	select {
	case c.resultBuffer <- result:
		c.bufferedCount.Add(1)
	default:
		// Buffer full, drop oldest
		select {
		case <-c.resultBuffer:
			c.resultBuffer <- result
		default:
		}
	}
}

// sendBufferedResults sends buffered results when connected.
func (c *Client) sendBufferedResults() {
	for {
		select {
		case <-c.taskPollCtx.Done():
			return
		case <-c.stopped:
			return
		case result := <-c.resultBuffer:
			if c.connected.Load() && c.registered.Load() {
				if err := c.sendTaskResultDirect(result); err != nil {
					// Re-buffer on failure
					c.bufferResult(result)
					time.Sleep(100 * time.Millisecond)
				} else {
					c.bufferedCount.Add(-1)
				}
			} else {
				// Re-buffer if not connected
				c.bufferResult(result)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// SendMetrics sends metrics to the master.
// If not connected, the metrics are buffered for later delivery.
func (c *Client) SendMetrics(executionID string, metrics *types.APIMetricsData, stepMetrics map[string]*types.StepMetricsData) error {
	if metrics == nil {
		return fmt.Errorf("metrics cannot be nil")
	}

	buffered := &BufferedMetrics{
		ExecutionID: executionID,
		Metrics:     metrics,
		StepMetrics: stepMetrics,
	}

	// If connected, send directly
	if c.connected.Load() && c.registered.Load() {
		if err := c.sendMetricsDirect(buffered); err != nil {
			// Buffer the metrics for retry
			c.bufferMetrics(buffered)
			return err
		}
		return nil
	}

	// Buffer the metrics for later delivery
	c.bufferMetrics(buffered)
	return nil
}

// sendMetricsDirect sends metrics directly to the master.
func (c *Client) sendMetricsDirect(buffered *BufferedMetrics) error {
	req := &types.MetricsReportRequest{
		SlaveID:     c.config.SlaveID,
		ExecutionID: buffered.ExecutionID,
		Timestamp:   time.Now().UnixMilli(),
		Metrics:     buffered.Metrics,
		StepMetrics: buffered.StepMetrics,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/executions/%s/metrics/report", c.config.MasterURL, buffered.ExecutionID)
	httpReq := c.agent.Post(url)
	httpReq.Timeout(c.config.RequestTimeout)
	httpReq.Body(body)
	httpReq.Set("Content-Type", "application/json")

	statusCode, respBody, errs := httpReq.Bytes()
	if len(errs) > 0 {
		return fmt.Errorf("send metrics failed: %v", errs[0])
	}

	if statusCode != fiber.StatusOK {
		var errResp types.ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return fmt.Errorf("send metrics failed: %s", errResp.Message)
		}
		return fmt.Errorf("send metrics failed with status: %d", statusCode)
	}

	return nil
}

// bufferMetrics adds metrics to the buffer.
func (c *Client) bufferMetrics(metrics *BufferedMetrics) {
	select {
	case c.metricsBuffer <- metrics:
	default:
		// Buffer full, drop oldest
		select {
		case <-c.metricsBuffer:
			c.metricsBuffer <- metrics
		default:
		}
	}
}

// sendBufferedMetrics sends buffered metrics when connected.
func (c *Client) sendBufferedMetrics() {
	for {
		select {
		case <-c.taskPollCtx.Done():
			return
		case <-c.stopped:
			return
		case metrics := <-c.metricsBuffer:
			if c.connected.Load() && c.registered.Load() {
				if err := c.sendMetricsDirect(metrics); err != nil {
					// Re-buffer on failure
					c.bufferMetrics(metrics)
					time.Sleep(100 * time.Millisecond)
				}
			} else {
				// Re-buffer if not connected
				c.bufferMetrics(metrics)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// unregister unregisters this slave from the master.
func (c *Client) unregister(ctx context.Context) error {
	req := &types.SlaveUnregisterRequest{
		SlaveID: c.config.SlaveID,
		Reason:  "client disconnect",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal unregister request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/slaves/%s/unregister", c.config.MasterURL, c.config.SlaveID)
	httpReq := c.agent.Post(url)
	httpReq.Timeout(c.config.RequestTimeout)
	httpReq.Body(body)
	httpReq.Set("Content-Type", "application/json")

	statusCode, _, errs := httpReq.Bytes()
	if len(errs) > 0 {
		return fmt.Errorf("unregister failed: %v", errs[0])
	}

	if statusCode != fiber.StatusOK {
		return fmt.Errorf("unregister failed with status: %d", statusCode)
	}

	return nil
}

// handleDisconnect handles disconnection from the master.
func (c *Client) handleDisconnect(err error) {
	if !c.connected.Load() {
		return
	}

	c.connected.Store(false)
	c.registered.Store(false)

	// Notify callback
	if c.onDisconnect != nil {
		c.onDisconnect(err)
	}

	// Start reconnection if not already reconnecting
	if !c.reconnecting.Load() {
		go c.reconnectLoop()
	}
}

// reconnectLoop attempts to reconnect to the master with exponential backoff.
func (c *Client) reconnectLoop() {
	if c.reconnecting.Swap(true) {
		return // Already reconnecting
	}
	defer c.reconnecting.Store(false)

	backoff := c.config.ReconnectInterval

	for {
		select {
		case <-c.stopped:
			return
		default:
		}

		attempt := c.reconnectAttempt.Add(1)

		// Check max attempts
		if c.config.MaxReconnectAttempts > 0 && int(attempt) > c.config.MaxReconnectAttempts {
			fmt.Printf("max reconnection attempts reached\n")
			return
		}

		fmt.Printf("reconnection attempt %d\n", attempt)

		// Wait before reconnecting with exponential backoff
		time.Sleep(backoff)

		// Try to reconnect
		ctx := context.Background()
		if err := c.Connect(ctx); err != nil {
			fmt.Printf("reconnection failed: %v\n", err)
			// Exponential backoff, max 60 seconds
			backoff = backoff * 2
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
			continue
		}

		// Re-register
		if err := c.Register(ctx); err != nil {
			fmt.Printf("re-registration failed: %v\n", err)
			c.connected.Store(false)
			continue
		}

		// Restart loops
		if err := c.restartLoops(ctx); err != nil {
			fmt.Printf("failed to restart loops: %v\n", err)
			c.connected.Store(false)
			c.registered.Store(false)
			continue
		}

		// Reset attempt counter and backoff
		c.reconnectAttempt.Store(0)
		backoff = c.config.ReconnectInterval

		// Notify callback
		if c.onReconnect != nil {
			c.onReconnect()
		}

		return
	}
}

// restartLoops restarts heartbeat and task polling loops after reconnection.
func (c *Client) restartLoops(ctx context.Context) error {
	// Restart heartbeat if it was running
	if c.heartbeatCancel != nil {
		if err := c.StartHeartbeat(ctx, func() *types.APISlaveStatusInfo {
			return &types.APISlaveStatusInfo{
				State:    string(types.SlaveStateOnline),
				LastSeen: time.Now().UnixMilli(),
			}
		}); err != nil {
			return err
		}
	}

	// Restart task polling if it was running
	if c.taskPollCancel != nil {
		if err := c.StartTaskPolling(ctx); err != nil {
			return err
		}
	}

	return nil
}

// SetTaskHandler sets the task handler callback.
func (c *Client) SetTaskHandler(handler TaskHandler) {
	c.onTask = handler
}

// SetCommandHandler sets the command handler callback.
func (c *Client) SetCommandHandler(handler CommandHandler) {
	c.onCommand = handler
}

// SetDisconnectHandler sets the disconnect handler callback.
func (c *Client) SetDisconnectHandler(handler DisconnectHandler) {
	c.onDisconnect = handler
}

// SetReconnectHandler sets the reconnect handler callback.
func (c *Client) SetReconnectHandler(handler ReconnectHandler) {
	c.onReconnect = handler
}

// IsConnected returns whether the client is connected.
func (c *Client) IsConnected() bool {
	return c.connected.Load()
}

// IsRegistered returns whether the client is registered.
func (c *Client) IsRegistered() bool {
	return c.registered.Load()
}

// GetBufferedCount returns the number of buffered results.
func (c *Client) GetBufferedCount() int64 {
	return c.bufferedCount.Load()
}

// GetSlaveID returns the slave ID.
func (c *Client) GetSlaveID() string {
	return c.config.SlaveID
}

// GetConfig returns the client configuration.
func (c *Client) GetConfig() *Config {
	return c.config
}

// IsRetryableError checks if an HTTP status code indicates a retryable error.
func IsRetryableError(statusCode int) bool {
	switch statusCode {
	case fiber.StatusServiceUnavailable,
		fiber.StatusGatewayTimeout,
		fiber.StatusBadGateway,
		fiber.StatusTooManyRequests,
		fiber.StatusRequestTimeout:
		return true
	default:
		return false
	}
}
