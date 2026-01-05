// Package rest provides the REST API server for the workflow execution engine.
package rest

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/grafana/k6/workflow-engine/pkg/types"
	"golang.org/x/net/websocket"
)

// MetricsStreamConfig holds configuration for metrics streaming.
type MetricsStreamConfig struct {
	// Interval is the interval between metrics updates.
	Interval time.Duration `yaml:"interval"`

	// BufferSize is the size of the metrics buffer.
	BufferSize int `yaml:"buffer_size"`
}

// DefaultMetricsStreamConfig returns a default metrics stream configuration.
func DefaultMetricsStreamConfig() *MetricsStreamConfig {
	return &MetricsStreamConfig{
		Interval:   time.Second,
		BufferSize: 100,
	}
}

// MetricsStreamer handles real-time metrics streaming.
type MetricsStreamer struct {
	server *Server
	config *MetricsStreamConfig

	// Active connections per execution
	connections map[string]map[*websocket.Conn]bool
	mu          sync.RWMutex
}

// NewMetricsStreamer creates a new metrics streamer.
func NewMetricsStreamer(server *Server, config *MetricsStreamConfig) *MetricsStreamer {
	if config == nil {
		config = DefaultMetricsStreamConfig()
	}
	return &MetricsStreamer{
		server:      server,
		config:      config,
		connections: make(map[string]map[*websocket.Conn]bool),
	}
}

// MetricsStreamMessage represents a message sent over the WebSocket.
type MetricsStreamMessage struct {
	Type        string           `json:"type"`
	ExecutionID string           `json:"execution_id"`
	Timestamp   string           `json:"timestamp"`
	Metrics     *MetricsResponse `json:"metrics,omitempty"`
	Error       string           `json:"error,omitempty"`
}

// setupWebSocketRoutes sets up WebSocket routes.
func (s *Server) setupWebSocketRoutes() {
	if !s.config.EnableWebSocket {
		return
	}

	streamer := NewMetricsStreamer(s, nil)

	// WebSocket endpoint for metrics streaming
	s.app.Get("/api/v1/executions/:id/metrics/stream", adaptor.HTTPHandler(
		websocket.Handler(func(ws *websocket.Conn) {
			streamer.handleMetricsStream(ws)
		}),
	))
}

// handleMetricsStream handles a WebSocket connection for metrics streaming.
func (ms *MetricsStreamer) handleMetricsStream(ws *websocket.Conn) {
	defer ws.Close()

	// Extract execution ID from the URL path
	// The path format is /api/v1/executions/:id/metrics/stream
	path := ws.Request().URL.Path
	executionID := extractExecutionID(path)

	if executionID == "" {
		msg := MetricsStreamMessage{
			Type:      "error",
			Timestamp: time.Now().Format(time.RFC3339),
			Error:     "execution ID is required",
		}
		data, _ := json.Marshal(msg)
		websocket.Message.Send(ws, string(data))
		return
	}

	// Register connection
	ms.registerConnection(executionID, ws)
	defer ms.unregisterConnection(executionID, ws)

	// Send initial metrics
	ctx := context.Background()
	metrics, err := ms.server.master.GetMetrics(ctx, executionID)
	if err != nil {
		msg := MetricsStreamMessage{
			Type:        "error",
			ExecutionID: executionID,
			Timestamp:   time.Now().Format(time.RFC3339),
			Error:       "failed to get metrics: " + err.Error(),
		}
		data, _ := json.Marshal(msg)
		websocket.Message.Send(ws, string(data))
		return
	}

	// Send initial metrics
	msg := MetricsStreamMessage{
		Type:        "metrics",
		ExecutionID: executionID,
		Timestamp:   time.Now().Format(time.RFC3339),
		Metrics:     toMetricsResponse(metrics),
	}
	data, _ := json.Marshal(msg)
	if err := websocket.Message.Send(ws, string(data)); err != nil {
		return
	}

	// Start streaming metrics
	ticker := time.NewTicker(ms.config.Interval)
	defer ticker.Stop()

	done := make(chan struct{})

	// Read messages from client (for ping/pong and close)
	go func() {
		for {
			var msg string
			if err := websocket.Message.Receive(ws, &msg); err != nil {
				close(done)
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			// Get latest metrics
			metrics, err := ms.server.master.GetMetrics(ctx, executionID)
			if err != nil {
				msg := MetricsStreamMessage{
					Type:        "error",
					ExecutionID: executionID,
					Timestamp:   time.Now().Format(time.RFC3339),
					Error:       "failed to get metrics: " + err.Error(),
				}
				data, _ := json.Marshal(msg)
				websocket.Message.Send(ws, string(data))
				continue
			}

			// Check execution status
			state, err := ms.server.master.GetExecutionStatus(ctx, executionID)
			if err != nil {
				continue
			}

			// Send metrics update
			msg := MetricsStreamMessage{
				Type:        "metrics",
				ExecutionID: executionID,
				Timestamp:   time.Now().Format(time.RFC3339),
				Metrics:     toMetricsResponse(metrics),
			}
			data, _ := json.Marshal(msg)
			if err := websocket.Message.Send(ws, string(data)); err != nil {
				return
			}

			// If execution is complete, send final message and close
			if state.Status == types.ExecutionStatusCompleted ||
				state.Status == types.ExecutionStatusFailed ||
				state.Status == types.ExecutionStatusAborted {
				msg := MetricsStreamMessage{
					Type:        "complete",
					ExecutionID: executionID,
					Timestamp:   time.Now().Format(time.RFC3339),
					Metrics:     toMetricsResponse(metrics),
				}
				data, _ := json.Marshal(msg)
				websocket.Message.Send(ws, string(data))
				return
			}
		}
	}
}

// registerConnection registers a WebSocket connection for an execution.
func (ms *MetricsStreamer) registerConnection(executionID string, ws *websocket.Conn) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.connections[executionID] == nil {
		ms.connections[executionID] = make(map[*websocket.Conn]bool)
	}
	ms.connections[executionID][ws] = true
}

// unregisterConnection unregisters a WebSocket connection.
func (ms *MetricsStreamer) unregisterConnection(executionID string, ws *websocket.Conn) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if conns, ok := ms.connections[executionID]; ok {
		delete(conns, ws)
		if len(conns) == 0 {
			delete(ms.connections, executionID)
		}
	}
}

// BroadcastMetrics broadcasts metrics to all connected clients for an execution.
func (ms *MetricsStreamer) BroadcastMetrics(executionID string, metrics *types.AggregatedMetrics) {
	ms.mu.RLock()
	conns := ms.connections[executionID]
	ms.mu.RUnlock()

	if len(conns) == 0 {
		return
	}

	msg := MetricsStreamMessage{
		Type:        "metrics",
		ExecutionID: executionID,
		Timestamp:   time.Now().Format(time.RFC3339),
		Metrics:     toMetricsResponse(metrics),
	}
	data, _ := json.Marshal(msg)

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	for ws := range conns {
		go func(conn *websocket.Conn) {
			websocket.Message.Send(conn, string(data))
		}(ws)
	}
}

// extractExecutionID extracts the execution ID from the WebSocket URL path.
func extractExecutionID(path string) string {
	// Path format: /api/v1/executions/:id/metrics/stream
	// We need to extract the :id part
	const prefix = "/api/v1/executions/"
	const suffix = "/metrics/stream"

	if len(path) <= len(prefix)+len(suffix) {
		return ""
	}

	start := len(prefix)
	end := len(path) - len(suffix)

	if end <= start {
		return ""
	}

	return path[start:end]
}
