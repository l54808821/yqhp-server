// Package client implements the gRPC client for Slave nodes.
// Requirements: 15.5
package client

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"yqhp/workflow-engine/api/grpc/converter"
	pb "yqhp/workflow-engine/api/grpc/proto"
	"yqhp/workflow-engine/pkg/types"
)

// Config holds the configuration for the gRPC client.
type Config struct {
	// MasterAddress is the address of the master node.
	MasterAddress string

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
	Resources *types.ResourceInfo

	// HeartbeatInterval is the interval between heartbeats.
	HeartbeatInterval time.Duration

	// ReconnectInterval is the interval between reconnection attempts.
	ReconnectInterval time.Duration

	// MaxReconnectAttempts is the maximum number of reconnection attempts.
	// 0 means unlimited.
	MaxReconnectAttempts int

	// ResultBufferSize is the size of the result buffer.
	ResultBufferSize int

	// MetricsBufferSize is the size of the metrics buffer.
	MetricsBufferSize int

	// ConnectionTimeout is the timeout for establishing a connection.
	ConnectionTimeout time.Duration
}

// DefaultConfig returns a default client configuration.
func DefaultConfig() *Config {
	return &Config{
		MasterAddress:        "localhost:9090",
		SlaveType:            types.SlaveTypeWorker,
		HeartbeatInterval:    5 * time.Second,
		ReconnectInterval:    5 * time.Second,
		MaxReconnectAttempts: 0, // Unlimited
		ResultBufferSize:     1000,
		MetricsBufferSize:    1000,
		ConnectionTimeout:    30 * time.Second,
	}
}

// Client implements the gRPC client for slave nodes.
// Requirements: 15.5
type Client struct {
	config *Config

	// gRPC connection and client
	conn   *grpc.ClientConn
	client pb.MasterServiceClient

	// State management
	connected        atomic.Bool
	registered       atomic.Bool
	reconnecting     atomic.Bool
	reconnectAttempt atomic.Int32

	// Heartbeat management
	heartbeatCtx    context.Context
	heartbeatCancel context.CancelFunc
	heartbeatStream pb.MasterService_HeartbeatClient

	// Task streaming
	taskCtx    context.Context
	taskCancel context.CancelFunc
	taskStream pb.MasterService_StreamTasksClient

	// Metrics streaming
	metricsCtx    context.Context
	metricsCancel context.CancelFunc
	metricsStream pb.MasterService_StreamMetricsClient

	// Result buffering for network resilience
	resultBuffer  chan *types.TaskResult
	metricsBuffer chan *types.Metrics
	bufferMu      sync.RWMutex
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

// TaskHandler handles incoming task assignments.
type TaskHandler func(ctx context.Context, task *types.Task) error

// CommandHandler handles incoming control commands.
type CommandHandler func(ctx context.Context, cmdType string, executionID string, params map[string]string) error

// DisconnectHandler is called when the client disconnects.
type DisconnectHandler func(err error)

// ReconnectHandler is called when the client reconnects.
type ReconnectHandler func()

// NewClient creates a new gRPC client.
func NewClient(config *Config) *Client {
	if config == nil {
		config = DefaultConfig()
	}

	return &Client{
		config:        config,
		resultBuffer:  make(chan *types.TaskResult, config.ResultBufferSize),
		metricsBuffer: make(chan *types.Metrics, config.MetricsBufferSize),
		stopped:       make(chan struct{}),
	}
}

// Connect connects to the master node.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected.Load() {
		return fmt.Errorf("already connected")
	}

	// Create connection with options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                c.config.HeartbeatInterval,
			Timeout:             c.config.ConnectionTimeout,
			PermitWithoutStream: true,
		}),
	}

	// Connect with timeout
	dialCtx, cancel := context.WithTimeout(ctx, c.config.ConnectionTimeout)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, c.config.MasterAddress, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to master: %w", err)
	}

	c.conn = conn
	c.client = pb.NewMasterServiceClient(conn)
	c.connected.Store(true)

	return nil
}

// Register registers this slave with the master.
func (c *Client) Register(ctx context.Context) error {
	if !c.connected.Load() {
		return fmt.Errorf("not connected")
	}

	req := converter.SlaveInfoToProto(&types.SlaveInfo{
		ID:           c.config.SlaveID,
		Type:         c.config.SlaveType,
		Address:      c.config.Address,
		Capabilities: c.config.Capabilities,
		Labels:       c.config.Labels,
		Resources:    c.config.Resources,
	})

	resp, err := c.client.Register(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}

	if !resp.Accepted {
		return fmt.Errorf("registration rejected: %s", resp.Error)
	}

	// Update slave ID if assigned
	if resp.AssignedId != "" {
		c.config.SlaveID = resp.AssignedId
	}

	// Update heartbeat interval if provided
	if resp.MasterInfo != nil && resp.MasterInfo.HeartbeatIntervalMs > 0 {
		c.config.HeartbeatInterval = time.Duration(resp.MasterInfo.HeartbeatIntervalMs) * time.Millisecond
	}

	c.registered.Store(true)
	return nil
}

// StartHeartbeat starts the heartbeat loop.
func (c *Client) StartHeartbeat(ctx context.Context, statusProvider func() *types.SlaveStatus) error {
	if !c.registered.Load() {
		return fmt.Errorf("not registered")
	}

	c.heartbeatCtx, c.heartbeatCancel = context.WithCancel(ctx)

	// Create heartbeat stream
	stream, err := c.client.Heartbeat(c.heartbeatCtx)
	if err != nil {
		return fmt.Errorf("failed to create heartbeat stream: %w", err)
	}
	c.heartbeatStream = stream

	// Start heartbeat goroutine
	go c.heartbeatLoop(statusProvider)

	// Start receiving commands
	go c.receiveCommands()

	return nil
}

// heartbeatLoop sends periodic heartbeats.
func (c *Client) heartbeatLoop(statusProvider func() *types.SlaveStatus) {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.heartbeatCtx.Done():
			return
		case <-c.stopped:
			return
		case <-ticker.C:
			if err := c.sendHeartbeat(statusProvider()); err != nil {
				c.handleDisconnect(err)
				return
			}
		}
	}
}

// sendHeartbeat sends a single heartbeat.
func (c *Client) sendHeartbeat(status *types.SlaveStatus) error {
	if c.heartbeatStream == nil {
		return fmt.Errorf("heartbeat stream not initialized")
	}

	req := &pb.HeartbeatRequest{
		SlaveId:   c.config.SlaveID,
		Status:    converter.SlaveStatusToProto(status),
		Timestamp: time.Now().UnixNano(),
	}

	return c.heartbeatStream.Send(req)
}

// receiveCommands receives commands from the heartbeat stream.
func (c *Client) receiveCommands() {
	for {
		select {
		case <-c.heartbeatCtx.Done():
			return
		case <-c.stopped:
			return
		default:
		}

		if c.heartbeatStream == nil {
			return
		}

		resp, err := c.heartbeatStream.Recv()
		if err != nil {
			if err != io.EOF {
				c.handleDisconnect(err)
			}
			return
		}

		// Process commands
		for _, cmd := range resp.Commands {
			if c.onCommand != nil {
				cmdType := converter.ProtoToCommandType(cmd.Type)
				if err := c.onCommand(c.heartbeatCtx, cmdType, cmd.ExecutionId, cmd.Params); err != nil {
					// Log error but continue
					fmt.Printf("failed to handle command: %v\n", err)
				}
			}
		}
	}
}

// StartTaskStream starts the task streaming.
func (c *Client) StartTaskStream(ctx context.Context) error {
	if !c.registered.Load() {
		return fmt.Errorf("not registered")
	}

	c.taskCtx, c.taskCancel = context.WithCancel(ctx)

	// Create task stream
	stream, err := c.client.StreamTasks(c.taskCtx)
	if err != nil {
		return fmt.Errorf("failed to create task stream: %w", err)
	}
	c.taskStream = stream

	// Send initial message to identify this slave
	initialUpdate := &pb.TaskUpdate{
		ExecutionId: c.config.SlaveID, // Using slave ID as identifier
		Status:      pb.TaskStatus_TASK_STATUS_PENDING,
	}
	if err := stream.Send(initialUpdate); err != nil {
		return fmt.Errorf("failed to send initial task update: %w", err)
	}

	// Start receiving tasks
	go c.receiveTasks()

	// Start sending buffered results
	go c.sendBufferedResults()

	return nil
}

// receiveTasks receives task assignments from the stream.
func (c *Client) receiveTasks() {
	for {
		select {
		case <-c.taskCtx.Done():
			return
		case <-c.stopped:
			return
		default:
		}

		if c.taskStream == nil {
			return
		}

		assignment, err := c.taskStream.Recv()
		if err != nil {
			if err != io.EOF {
				c.handleDisconnect(err)
			}
			return
		}

		// Convert and handle task
		task, err := converter.ProtoToTask(assignment)
		if err != nil {
			fmt.Printf("failed to convert task: %v\n", err)
			continue
		}

		if c.onTask != nil {
			if err := c.onTask(c.taskCtx, task); err != nil {
				fmt.Printf("failed to handle task: %v\n", err)
			}
		}
	}
}

// SendTaskResult sends a task result to the master.
// If not connected, the result is buffered for later delivery.
// Requirements: 15.5
func (c *Client) SendTaskResult(result *types.TaskResult) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}

	// If connected and stream is available, send directly
	if c.connected.Load() && c.taskStream != nil {
		update, err := converter.TaskResultToProto(result)
		if err != nil {
			return fmt.Errorf("failed to convert result: %w", err)
		}

		if err := c.taskStream.Send(update); err != nil {
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

// bufferResult adds a result to the buffer.
func (c *Client) bufferResult(result *types.TaskResult) {
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
		case <-c.taskCtx.Done():
			return
		case <-c.stopped:
			return
		case result := <-c.resultBuffer:
			if c.connected.Load() && c.taskStream != nil {
				update, err := converter.TaskResultToProto(result)
				if err != nil {
					continue
				}

				if err := c.taskStream.Send(update); err != nil {
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

// StartMetricsStream starts the metrics streaming.
func (c *Client) StartMetricsStream(ctx context.Context) error {
	if !c.registered.Load() {
		return fmt.Errorf("not registered")
	}

	c.metricsCtx, c.metricsCancel = context.WithCancel(ctx)

	// Create metrics stream
	stream, err := c.client.StreamMetrics(c.metricsCtx)
	if err != nil {
		return fmt.Errorf("failed to create metrics stream: %w", err)
	}
	c.metricsStream = stream

	// Start receiving acknowledgments
	go c.receiveMetricsAcks()

	// Start sending buffered metrics
	go c.sendBufferedMetrics()

	return nil
}

// receiveMetricsAcks receives metrics acknowledgments.
func (c *Client) receiveMetricsAcks() {
	for {
		select {
		case <-c.metricsCtx.Done():
			return
		case <-c.stopped:
			return
		default:
		}

		if c.metricsStream == nil {
			return
		}

		ack, err := c.metricsStream.Recv()
		if err != nil {
			if err != io.EOF {
				c.handleDisconnect(err)
			}
			return
		}

		if !ack.Success {
			fmt.Printf("metrics rejected: %s\n", ack.Error)
		}
	}
}

// SendMetrics sends metrics to the master.
// If not connected, the metrics are buffered for later delivery.
// Requirements: 15.5
func (c *Client) SendMetrics(executionID string, metrics *types.Metrics) error {
	if metrics == nil {
		return fmt.Errorf("metrics cannot be nil")
	}

	// If connected and stream is available, send directly
	if c.connected.Load() && c.metricsStream != nil {
		report, err := converter.MetricsToProto(c.config.SlaveID, executionID, metrics)
		if err != nil {
			return fmt.Errorf("failed to convert metrics: %w", err)
		}

		if err := c.metricsStream.Send(report); err != nil {
			// Buffer the metrics for retry
			c.bufferMetrics(metrics)
			return err
		}
		return nil
	}

	// Buffer the metrics for later delivery
	c.bufferMetrics(metrics)
	return nil
}

// bufferMetrics adds metrics to the buffer.
func (c *Client) bufferMetrics(metrics *types.Metrics) {
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
		case <-c.metricsCtx.Done():
			return
		case <-c.stopped:
			return
		case metrics := <-c.metricsBuffer:
			if c.connected.Load() && c.metricsStream != nil {
				report, err := converter.MetricsToProto(c.config.SlaveID, "", metrics)
				if err != nil {
					continue
				}

				if err := c.metricsStream.Send(report); err != nil {
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

// reconnectLoop attempts to reconnect to the master.
// Requirements: 15.5
func (c *Client) reconnectLoop() {
	if c.reconnecting.Swap(true) {
		return // Already reconnecting
	}
	defer c.reconnecting.Store(false)

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

		// Close existing connection
		c.closeConnection()

		// Wait before reconnecting
		time.Sleep(c.config.ReconnectInterval)

		// Try to reconnect
		ctx := context.Background()
		if err := c.Connect(ctx); err != nil {
			fmt.Printf("reconnection failed: %v\n", err)
			continue
		}

		// Re-register
		if err := c.Register(ctx); err != nil {
			fmt.Printf("re-registration failed: %v\n", err)
			c.closeConnection()
			continue
		}

		// Restart streams
		if err := c.restartStreams(ctx); err != nil {
			fmt.Printf("failed to restart streams: %v\n", err)
			c.closeConnection()
			continue
		}

		// Reset attempt counter
		c.reconnectAttempt.Store(0)

		// Notify callback
		if c.onReconnect != nil {
			c.onReconnect()
		}

		return
	}
}

// restartStreams restarts all streams after reconnection.
func (c *Client) restartStreams(ctx context.Context) error {
	// Restart heartbeat if it was running
	if c.heartbeatCancel != nil {
		if err := c.StartHeartbeat(ctx, func() *types.SlaveStatus {
			return &types.SlaveStatus{
				State:    types.SlaveStateOnline,
				LastSeen: time.Now(),
			}
		}); err != nil {
			return err
		}
	}

	// Restart task stream if it was running
	if c.taskCancel != nil {
		if err := c.StartTaskStream(ctx); err != nil {
			return err
		}
	}

	// Restart metrics stream if it was running
	if c.metricsCancel != nil {
		if err := c.StartMetricsStream(ctx); err != nil {
			return err
		}
	}

	return nil
}

// closeConnection closes the gRPC connection.
func (c *Client) closeConnection() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Cancel all contexts
	if c.heartbeatCancel != nil {
		c.heartbeatCancel()
	}
	if c.taskCancel != nil {
		c.taskCancel()
	}
	if c.metricsCancel != nil {
		c.metricsCancel()
	}

	// Close connection
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	c.client = nil
	c.heartbeatStream = nil
	c.taskStream = nil
	c.metricsStream = nil
	c.connected.Store(false)
}

// Disconnect disconnects from the master.
func (c *Client) Disconnect(ctx context.Context) error {
	c.stopOnce.Do(func() {
		close(c.stopped)
		c.closeConnection()
	})
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

// IsRetryableError checks if an error is retryable.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	st, ok := status.FromError(err)
	if !ok {
		return true // Assume retryable for non-gRPC errors
	}

	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Aborted, codes.ResourceExhausted:
		return true
	default:
		return false
	}
}
