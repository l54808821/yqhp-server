// Package server implements the gRPC server for the Master node.
// Requirements: 15.2, 15.4
package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"yqhp/workflow-engine/api/grpc/converter"
	pb "yqhp/workflow-engine/api/grpc/proto"
	"yqhp/workflow-engine/internal/master"
	"yqhp/workflow-engine/pkg/types"
)

// Config holds the configuration for the gRPC server.
type Config struct {
	// Address is the address to listen on.
	Address string

	// MaxRecvMsgSize is the maximum message size in bytes the server can receive.
	MaxRecvMsgSize int

	// MaxSendMsgSize is the maximum message size in bytes the server can send.
	MaxSendMsgSize int

	// HeartbeatInterval is the interval for sending heartbeat responses.
	HeartbeatInterval time.Duration

	// ConnectionTimeout is the timeout for idle connections.
	ConnectionTimeout time.Duration

	// MasterID is the unique identifier for this master.
	MasterID string

	// Version is the version of the master.
	Version string
}

// DefaultConfig returns a default server configuration.
func DefaultConfig() *Config {
	return &Config{
		Address:           ":9090",
		MaxRecvMsgSize:    16 * 1024 * 1024, // 16MB
		MaxSendMsgSize:    16 * 1024 * 1024, // 16MB
		HeartbeatInterval: 5 * time.Second,
		ConnectionTimeout: 30 * time.Second,
		MasterID:          "master-1",
		Version:           "1.0.0",
	}
}

// Server implements the gRPC MasterService server.
// Requirements: 15.2, 15.4
type Server struct {
	pb.UnimplementedMasterServiceServer

	config     *Config
	grpcServer *grpc.Server
	registry   master.SlaveRegistry
	scheduler  master.Scheduler
	aggregator master.MetricsAggregator
	masterNode master.Master

	// Task management
	taskQueues   map[string]chan *pb.TaskAssignment // slaveID -> task queue
	taskQueuesMu sync.RWMutex

	// Command management
	commandQueues   map[string]chan *pb.ControlCommand // slaveID -> command queue
	commandQueuesMu sync.RWMutex

	// Metrics collection
	metricsHandlers   map[string]MetricsHandler // executionID -> handler
	metricsHandlersMu sync.RWMutex

	// State
	started bool
	mu      sync.RWMutex
}

// MetricsHandler handles metrics for an execution.
type MetricsHandler interface {
	HandleMetrics(ctx context.Context, slaveID string, metrics *types.Metrics) error
}

// NewServer creates a new gRPC server.
func NewServer(
	config *Config,
	registry master.SlaveRegistry,
	scheduler master.Scheduler,
	aggregator master.MetricsAggregator,
	masterNode master.Master,
) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	return &Server{
		config:          config,
		registry:        registry,
		scheduler:       scheduler,
		aggregator:      aggregator,
		masterNode:      masterNode,
		taskQueues:      make(map[string]chan *pb.TaskAssignment),
		commandQueues:   make(map[string]chan *pb.ControlCommand),
		metricsHandlers: make(map[string]MetricsHandler),
	}
}

// Start starts the gRPC server.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("server already started")
	}

	// Create listener
	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Address, err)
	}

	// Create gRPC server with options
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.config.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(s.config.MaxSendMsgSize),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    s.config.HeartbeatInterval,
			Timeout: s.config.ConnectionTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             s.config.HeartbeatInterval / 2,
			PermitWithoutStream: true,
		}),
	}

	s.grpcServer = grpc.NewServer(opts...)
	pb.RegisterMasterServiceServer(s.grpcServer, s)

	s.started = true

	// Start serving in a goroutine
	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			// Log error but don't return it since we're in a goroutine
			fmt.Printf("gRPC server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the gRPC server gracefully.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	// Close all task queues
	s.taskQueuesMu.Lock()
	for _, ch := range s.taskQueues {
		close(ch)
	}
	s.taskQueues = make(map[string]chan *pb.TaskAssignment)
	s.taskQueuesMu.Unlock()

	// Close all command queues
	s.commandQueuesMu.Lock()
	for _, ch := range s.commandQueues {
		close(ch)
	}
	s.commandQueues = make(map[string]chan *pb.ControlCommand)
	s.commandQueuesMu.Unlock()

	// Graceful stop with timeout
	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		s.grpcServer.Stop()
	}

	s.started = false
	return nil
}

// Register handles slave registration.
// Requirements: 15.2
func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Convert to internal type
	slaveInfo := converter.ProtoToSlaveInfo(req)
	if slaveInfo == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid slave info")
	}

	// Register with the registry
	if err := s.registry.Register(ctx, slaveInfo); err != nil {
		return &pb.RegisterResponse{
			Accepted: false,
			Error:    err.Error(),
		}, nil
	}

	// Create task and command queues for this slave
	s.taskQueuesMu.Lock()
	s.taskQueues[slaveInfo.ID] = make(chan *pb.TaskAssignment, 100)
	s.taskQueuesMu.Unlock()

	s.commandQueuesMu.Lock()
	s.commandQueues[slaveInfo.ID] = make(chan *pb.ControlCommand, 100)
	s.commandQueuesMu.Unlock()

	return &pb.RegisterResponse{
		Accepted:   true,
		AssignedId: slaveInfo.ID,
		MasterInfo: &pb.MasterInfo{
			MasterId:            s.config.MasterID,
			Version:             s.config.Version,
			HeartbeatIntervalMs: s.config.HeartbeatInterval.Milliseconds(),
		},
	}, nil
}

// Heartbeat handles bidirectional heartbeat streaming.
// Requirements: 15.2
func (s *Server) Heartbeat(stream pb.MasterService_HeartbeatServer) error {
	var slaveID string

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive heartbeat: %v", err)
		}

		slaveID = req.SlaveId

		// Update slave status
		slaveStatus := converter.ProtoToSlaveStatus(req.Status)
		if slaveStatus != nil {
			slaveStatus.LastSeen = time.Now()
			if err := s.registry.UpdateStatus(stream.Context(), slaveID, slaveStatus); err != nil {
				// Log error but continue
				fmt.Printf("failed to update slave status: %v\n", err)
			}
		}

		// Get pending commands for this slave
		var commands []*pb.ControlCommand
		s.commandQueuesMu.RLock()
		cmdQueue, ok := s.commandQueues[slaveID]
		s.commandQueuesMu.RUnlock()

		if ok {
			// Drain available commands (non-blocking)
			for {
				select {
				case cmd := <-cmdQueue:
					commands = append(commands, cmd)
				default:
					goto sendResponse
				}
			}
		}

	sendResponse:
		// Send response with any pending commands
		resp := &pb.HeartbeatResponse{
			Commands:  commands,
			Timestamp: time.Now().UnixNano(),
		}

		if err := stream.Send(resp); err != nil {
			return status.Errorf(codes.Internal, "failed to send heartbeat response: %v", err)
		}
	}
}

// StreamTasks handles bidirectional task streaming.
// Requirements: 15.2, 15.4
func (s *Server) StreamTasks(stream pb.MasterService_StreamTasksServer) error {
	var slaveID string

	// Start a goroutine to send task assignments
	ctx := stream.Context()
	errCh := make(chan error, 1)

	// First, receive an initial message to identify the slave
	initialUpdate, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to receive initial task update: %v", err)
	}

	// Extract slave ID from the task update (we need to add this to the protocol)
	// For now, we'll use the execution ID to look up the slave
	slaveID = s.getSlaveIDFromTaskUpdate(initialUpdate)
	if slaveID == "" {
		return status.Error(codes.InvalidArgument, "could not determine slave ID")
	}

	// Get or create task queue for this slave
	s.taskQueuesMu.Lock()
	taskQueue, ok := s.taskQueues[slaveID]
	if !ok {
		taskQueue = make(chan *pb.TaskAssignment, 100)
		s.taskQueues[slaveID] = taskQueue
	}
	s.taskQueuesMu.Unlock()

	// Start goroutine to send tasks
	go func() {
		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case task, ok := <-taskQueue:
				if !ok {
					errCh <- nil
					return
				}
				if err := stream.Send(task); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	// Process the initial update
	if err := s.processTaskUpdate(ctx, slaveID, initialUpdate); err != nil {
		return err
	}

	// Continue receiving task updates
	for {
		select {
		case err := <-errCh:
			return err
		default:
		}

		update, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive task update: %v", err)
		}

		if err := s.processTaskUpdate(ctx, slaveID, update); err != nil {
			return err
		}
	}
}

// processTaskUpdate processes a task update from a slave.
func (s *Server) processTaskUpdate(ctx context.Context, slaveID string, update *pb.TaskUpdate) error {
	if update == nil {
		return nil
	}

	// Convert to internal type
	result, err := converter.ProtoToTaskResult(update, slaveID)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to convert task result: %v", err)
	}

	// Handle the task result (update execution state, etc.)
	// This would typically be handled by the master node
	if s.masterNode != nil {
		// The master node would process this result
		// For now, we just log it
		fmt.Printf("Received task update from slave %s: task=%s, status=%s\n",
			slaveID, result.TaskID, result.Status)
	}

	return nil
}

// getSlaveIDFromTaskUpdate extracts the slave ID from a task update.
// In a real implementation, this would be part of the protocol.
func (s *Server) getSlaveIDFromTaskUpdate(update *pb.TaskUpdate) string {
	// For now, we'll look up the slave by execution ID
	// This is a simplified implementation
	if update == nil {
		return ""
	}

	// In a real implementation, the slave ID would be included in the message
	// or determined from the connection context
	return update.ExecutionId // Placeholder - should be slave ID
}

// StreamMetrics handles metrics streaming from slaves.
// Requirements: 15.4
func (s *Server) StreamMetrics(stream pb.MasterService_StreamMetricsServer) error {
	for {
		report, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive metrics: %v", err)
		}

		// Convert to internal type
		metrics, err := converter.ProtoToMetrics(report)
		if err != nil {
			// Send error acknowledgment
			if sendErr := stream.Send(&pb.MetricsAck{
				ExecutionId: report.ExecutionId,
				Timestamp:   time.Now().UnixNano(),
				Success:     false,
				Error:       err.Error(),
			}); sendErr != nil {
				return status.Errorf(codes.Internal, "failed to send metrics ack: %v", sendErr)
			}
			continue
		}

		// Handle metrics
		s.metricsHandlersMu.RLock()
		handler, ok := s.metricsHandlers[report.ExecutionId]
		s.metricsHandlersMu.RUnlock()

		if ok && handler != nil {
			if err := handler.HandleMetrics(stream.Context(), report.SlaveId, metrics); err != nil {
				// Send error acknowledgment
				if sendErr := stream.Send(&pb.MetricsAck{
					ExecutionId: report.ExecutionId,
					Timestamp:   time.Now().UnixNano(),
					Success:     false,
					Error:       err.Error(),
				}); sendErr != nil {
					return status.Errorf(codes.Internal, "failed to send metrics ack: %v", sendErr)
				}
				continue
			}
		}

		// Send success acknowledgment
		if err := stream.Send(&pb.MetricsAck{
			ExecutionId: report.ExecutionId,
			Timestamp:   time.Now().UnixNano(),
			Success:     true,
		}); err != nil {
			return status.Errorf(codes.Internal, "failed to send metrics ack: %v", err)
		}
	}
}

// AssignTask assigns a task to a slave.
func (s *Server) AssignTask(slaveID string, task *types.Task) error {
	s.taskQueuesMu.RLock()
	taskQueue, ok := s.taskQueues[slaveID]
	s.taskQueuesMu.RUnlock()

	if !ok {
		return fmt.Errorf("slave not connected: %s", slaveID)
	}

	// Convert to protobuf
	assignment, err := converter.TaskToProto(task)
	if err != nil {
		return fmt.Errorf("failed to convert task: %w", err)
	}

	// Send to queue (non-blocking with timeout)
	select {
	case taskQueue <- assignment:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending task to slave: %s", slaveID)
	}
}

// SendCommand sends a control command to a slave.
func (s *Server) SendCommand(slaveID string, cmdType string, executionID string, params map[string]string) error {
	s.commandQueuesMu.RLock()
	cmdQueue, ok := s.commandQueues[slaveID]
	s.commandQueuesMu.RUnlock()

	if !ok {
		return fmt.Errorf("slave not connected: %s", slaveID)
	}

	cmd := &pb.ControlCommand{
		Type:        converter.CommandTypeToProto(cmdType),
		ExecutionId: executionID,
		Params:      params,
	}

	// Send to queue (non-blocking with timeout)
	select {
	case cmdQueue <- cmd:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending command to slave: %s", slaveID)
	}
}

// RegisterMetricsHandler registers a metrics handler for an execution.
func (s *Server) RegisterMetricsHandler(executionID string, handler MetricsHandler) {
	s.metricsHandlersMu.Lock()
	s.metricsHandlers[executionID] = handler
	s.metricsHandlersMu.Unlock()
}

// UnregisterMetricsHandler unregisters a metrics handler for an execution.
func (s *Server) UnregisterMetricsHandler(executionID string) {
	s.metricsHandlersMu.Lock()
	delete(s.metricsHandlers, executionID)
	s.metricsHandlersMu.Unlock()
}

// GetAddress returns the server address.
func (s *Server) GetAddress() string {
	return s.config.Address
}

// IsStarted returns whether the server is started.
func (s *Server) IsStarted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}
