package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	fiberws "github.com/gofiber/websocket/v2"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"
)

// SlaveWSConn wraps a single WebSocket connection from a slave.
type SlaveWSConn struct {
	slaveID string
	conn    *fiberws.Conn
	send    chan []byte
	hub     *SlaveWSHub
	done    chan struct{}
	once    sync.Once
}

// SlaveWSHub manages all slave WebSocket connections.
type SlaveWSHub struct {
	conns        map[string]*SlaveWSConn
	mu           sync.RWMutex
	server       *Server
	pingInterval time.Duration
}

// NewSlaveWSHub creates a new hub.
func NewSlaveWSHub(server *Server) *SlaveWSHub {
	return &SlaveWSHub{
		conns:        make(map[string]*SlaveWSConn),
		server:       server,
		pingInterval: 20 * time.Second,
	}
}

// HasConn returns true if the slave has an active WebSocket connection.
func (h *SlaveWSHub) HasConn(slaveID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.conns[slaveID]
	return ok
}

// PushTask sends a task assignment to a slave via WebSocket.
func (h *SlaveWSHub) PushTask(slaveID string, task *TaskAssignment) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return h.sendToSlave(slaveID, &types.WSMessage{Type: types.WSMsgTaskAssign, Data: data})
}

// PushCommand sends a control command to a slave via WebSocket.
func (h *SlaveWSHub) PushCommand(slaveID string, cmd *ControlCommand) error {
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	return h.sendToSlave(slaveID, &types.WSMessage{Type: types.WSMsgCommand, Data: data})
}

func (h *SlaveWSHub) sendToSlave(slaveID string, msg *types.WSMessage) error {
	h.mu.RLock()
	conn, ok := h.conns[slaveID]
	h.mu.RUnlock()

	if !ok {
		return fmt.Errorf("slave %s not connected via WebSocket", slaveID)
	}

	envelope, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case conn.send <- envelope:
		return nil
	default:
		return fmt.Errorf("send buffer full for slave %s", slaveID)
	}
}

func (h *SlaveWSHub) register(conn *SlaveWSConn) {
	h.mu.Lock()
	if old, ok := h.conns[conn.slaveID]; ok {
		old.close()
	}
	h.conns[conn.slaveID] = conn
	h.mu.Unlock()
}

func (h *SlaveWSHub) unregister(slaveID string) {
	h.mu.Lock()
	delete(h.conns, slaveID)
	h.mu.Unlock()
}

// setupSlaveWSRoute registers the Fiber-native WebSocket endpoint.
func (s *Server) setupSlaveWSRoute() {
	s.app.Use("/api/v1/slave-ws", func(c *fiber.Ctx) error {
		if fiberws.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	s.app.Get("/api/v1/slave-ws", fiberws.New(func(c *fiberws.Conn) {
		s.wsHub.handleConnection(c)
	}))
}

// handleConnection handles a newly established slave WebSocket connection.
func (h *SlaveWSHub) handleConnection(c *fiberws.Conn) {
	// The first message must be a register message.
	var firstMsg types.WSMessage
	if err := c.ReadJSON(&firstMsg); err != nil {
		logger.Error("ws: read first message failed", "err", err)
		return
	}

	if firstMsg.Type != types.WSMsgRegister {
		logger.Error("ws: expected register message", "got", firstMsg.Type)
		return
	}

	var regReq types.SlaveRegisterRequest
	if err := json.Unmarshal(firstMsg.Data, &regReq); err != nil {
		logger.Error("ws: parse register request failed", "err", err)
		return
	}

	slaveID := regReq.SlaveID
	if slaveID == "" {
		logger.Error("ws: empty slave ID")
		return
	}

	// Register the slave in the registry.
	ctx := context.Background()
	if h.server.registry != nil {
		slaveInfo := &types.SlaveInfo{
			ID:           regReq.SlaveID,
			Type:         types.SlaveType(regReq.SlaveType),
			Address:      regReq.Address,
			Capabilities: regReq.Capabilities,
			Labels:       regReq.Labels,
		}
		if regReq.Resources != nil {
			slaveInfo.Resources = &types.ResourceInfo{
				CPUCores:    regReq.Resources.CPUCores,
				MemoryMB:    regReq.Resources.MemoryMB,
				MaxVUs:      regReq.Resources.MaxVUs,
				CurrentLoad: regReq.Resources.CurrentLoad,
			}
		}
		if err := h.server.registry.Register(ctx, slaveInfo); err != nil {
			logger.Error("ws: register slave failed", "slave", slaveID, "err", err)
			ackData, _ := json.Marshal(types.SlaveRegisterResponse{
				Accepted: false,
				Error:    "Registration failed: " + err.Error(),
			})
			_ = c.WriteJSON(&types.WSMessage{Type: types.WSMsgRegisterAck, Data: ackData})
			return
		}
	}

	// Send register ack.
	ackData, _ := json.Marshal(types.SlaveRegisterResponse{
		Accepted:          true,
		AssignedID:        slaveID,
		HeartbeatInterval: 30000,
		MasterID:          "master-1",
		Version:           "1.0.0",
	})
	if err := c.WriteJSON(&types.WSMessage{Type: types.WSMsgRegisterAck, Data: ackData}); err != nil {
		logger.Error("ws: send register ack failed", "err", err)
		return
	}

	conn := &SlaveWSConn{
		slaveID: slaveID,
		conn:    c,
		send:    make(chan []byte, 256),
		hub:     h,
		done:    make(chan struct{}),
	}

	h.register(conn)
	defer h.unregister(slaveID)

	logger.Info("ws: slave connected", "slave", slaveID)

	// Drain any tasks/commands that were queued before the WebSocket connected.
	h.drainPendingToWS(slaveID, conn)

	go conn.writePump()

	// readPump blocks until the connection closes.
	conn.readPump()

	logger.Info("ws: slave disconnected", "slave", slaveID)
}

// drainPendingToWS pushes any queued tasks/commands through the WebSocket.
func (h *SlaveWSHub) drainPendingToWS(slaveID string, conn *SlaveWSConn) {
	h.server.taskQueuesMu.RLock()
	taskQueue, hasTask := h.server.taskQueues[slaveID]
	h.server.taskQueuesMu.RUnlock()
	if hasTask {
		for {
			select {
			case task := <-taskQueue:
				data, _ := json.Marshal(task)
				envelope, _ := json.Marshal(&types.WSMessage{Type: types.WSMsgTaskAssign, Data: data})
				select {
				case conn.send <- envelope:
				default:
				}
			default:
				goto doneTask
			}
		}
	}
doneTask:

	h.server.commandQueuesMu.RLock()
	cmdQueue, hasCmd := h.server.commandQueues[slaveID]
	h.server.commandQueuesMu.RUnlock()
	if hasCmd {
		for {
			select {
			case cmd := <-cmdQueue:
				data, _ := json.Marshal(cmd)
				envelope, _ := json.Marshal(&types.WSMessage{Type: types.WSMsgCommand, Data: data})
				select {
				case conn.send <- envelope:
				default:
				}
			default:
				return
			}
		}
	}
}

// ─── conn read / write ──────────────────────────────────────────────────────

func (c *SlaveWSConn) readPump() {
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var msg types.WSMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			logger.Error("ws: invalid message", "slave", c.slaveID, "err", err)
			continue
		}

		c.handleMessage(&msg)
	}
}

func (c *SlaveWSConn) handleMessage(msg *types.WSMessage) {
	ctx := context.Background()

	switch msg.Type {
	case types.WSMsgHeartbeat:
		var req types.SlaveHeartbeatRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return
		}
		if c.hub.server.registry != nil && req.Status != nil {
			status := &types.SlaveStatus{
				State:       types.SlaveState(req.Status.State),
				Load:        req.Status.Load,
				ActiveTasks: req.Status.ActiveTasks,
				LastSeen:    time.Now(),
			}
			if req.Status.Metrics != nil {
				status.Metrics = &types.SlaveMetrics{
					CPUUsage:    req.Status.Metrics.CPUUsage,
					MemoryUsage: req.Status.Metrics.MemoryUsage,
					ActiveVUs:   req.Status.Metrics.ActiveVUs,
					Throughput:  req.Status.Metrics.Throughput,
				}
			}
			_ = c.hub.server.registry.UpdateStatus(ctx, c.slaveID, status)
		}

	case types.WSMsgTaskResult:
		_ = ctx

	case types.WSMsgMetrics:
		_ = ctx

	case types.WSMsgPong:
		// keepalive acknowledged
	}
}

func (c *SlaveWSConn) writePump() {
	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.conn.WriteMessage(fiberws.TextMessage, data); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

func (c *SlaveWSConn) close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}
