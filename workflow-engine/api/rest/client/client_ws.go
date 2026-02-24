package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"yqhp/workflow-engine/pkg/types"
)

// ConnectWS establishes a WebSocket connection to the master, replacing
// HTTP heartbeat polling and task polling with a single persistent connection.
func (c *Client) ConnectWS(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected.Load() {
		return fmt.Errorf("already connected")
	}

	wsURL := toWebSocketURL(c.config.MasterURL) + "/api/v1/slave-ws"

	dialer := websocket.Dialer{
		HandshakeTimeout: c.config.RequestTimeout,
	}
	ws, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("WebSocket dial failed: %w", err)
	}

	// Build registration request.
	regReq := &types.SlaveRegisterRequest{
		SlaveID:      c.config.SlaveID,
		SlaveType:    string(c.config.SlaveType),
		Capabilities: c.config.Capabilities,
		Labels:       c.config.Labels,
		Address:      c.config.Address,
		Resources:    c.config.Resources,
	}
	regData, _ := json.Marshal(regReq)
	regMsg := types.WSMessage{Type: types.WSMsgRegister, Data: regData}

	if err := ws.WriteJSON(&regMsg); err != nil {
		ws.Close()
		return fmt.Errorf("send register message failed: %w", err)
	}

	// Read register ack.
	var ackMsg types.WSMessage
	if err := ws.ReadJSON(&ackMsg); err != nil {
		ws.Close()
		return fmt.Errorf("read register ack failed: %w", err)
	}
	if ackMsg.Type != types.WSMsgRegisterAck {
		ws.Close()
		return fmt.Errorf("unexpected ack type: %s", ackMsg.Type)
	}

	var ackResp types.SlaveRegisterResponse
	if err := json.Unmarshal(ackMsg.Data, &ackResp); err != nil {
		ws.Close()
		return fmt.Errorf("parse register ack failed: %w", err)
	}
	if !ackResp.Accepted {
		ws.Close()
		return fmt.Errorf("registration rejected: %s", ackResp.Error)
	}
	if ackResp.AssignedID != "" {
		c.config.SlaveID = ackResp.AssignedID
	}

	c.wsConn = ws
	c.wsSend = make(chan []byte, 256)
	c.wsDone = make(chan struct{})
	c.connected.Store(true)
	c.registered.Store(true)

	go c.wsWritePump()
	go c.wsReadPump(ctx)
	go c.wsHeartbeatPump(ctx)

	return nil
}

// DisconnectWS closes the WebSocket connection gracefully.
func (c *Client) DisconnectWS() {
	c.wsCloseOnce.Do(func() {
		if c.wsDone != nil {
			close(c.wsDone)
		}
		if c.wsConn != nil {
			c.wsConn.Close()
		}
		c.connected.Store(false)
		c.registered.Store(false)
	})
}

// WSSendTaskResult sends a task result through the WebSocket.
func (c *Client) WSSendTaskResult(result *BufferedResult) error {
	req := &types.TaskResultRequest{
		TaskID:      result.TaskID,
		ExecutionID: result.ExecutionID,
		SlaveID:     c.config.SlaveID,
		Status:      result.Status,
		Result:      result.Result,
		Errors:      result.Errors,
	}
	return c.wsSendMsg(types.WSMsgTaskResult, req)
}

// WSSendMetrics sends metrics through the WebSocket.
func (c *Client) WSSendMetrics(executionID string, metrics *types.APIMetricsData, stepMetrics map[string]*types.StepMetricsData) error {
	req := &types.MetricsReportRequest{
		SlaveID:     c.config.SlaveID,
		ExecutionID: executionID,
		Timestamp:   time.Now().UnixMilli(),
		Metrics:     metrics,
		StepMetrics: stepMetrics,
	}
	return c.wsSendMsg(types.WSMsgMetrics, req)
}

// IsWebSocket returns true if the client is connected via WebSocket.
func (c *Client) IsWebSocket() bool {
	return c.wsConn != nil && c.connected.Load()
}

// ─── internal pumps ─────────────────────────────────────────────────────────

func (c *Client) wsReadPump(ctx context.Context) {
	defer c.handleWSDisconnect(fmt.Errorf("read pump closed"))

	for {
		_, raw, err := c.wsConn.ReadMessage()
		if err != nil {
			return
		}

		var msg types.WSMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case types.WSMsgTaskAssign:
			var task types.TaskAssignment
			if err := json.Unmarshal(msg.Data, &task); err == nil && c.onTask != nil {
				go c.onTask(ctx, &task)
			}

		case types.WSMsgCommand:
			var cmd types.ControlCommand
			if err := json.Unmarshal(msg.Data, &cmd); err == nil && c.onCommand != nil {
				go c.onCommand(ctx, &cmd)
			}

		case types.WSMsgPing:
			c.wsSendMsg(types.WSMsgPong, nil)
		}
	}
}

func (c *Client) wsWritePump() {
	for {
		select {
		case data, ok := <-c.wsSend:
			if !ok {
				return
			}
			if err := c.wsConn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-c.wsDone:
			return
		}
	}
}

func (c *Client) wsHeartbeatPump(ctx context.Context) {
	interval := 30 * time.Second
	if c.config.HeartbeatInterval > 0 {
		interval = c.config.HeartbeatInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hb := &types.SlaveHeartbeatRequest{
				SlaveID: c.config.SlaveID,
				Status: &types.APISlaveStatusInfo{
					State:    string(types.SlaveStateOnline),
					LastSeen: time.Now().UnixMilli(),
				},
				Timestamp: time.Now().UnixMilli(),
			}
			c.wsSendMsg(types.WSMsgHeartbeat, hb)
		case <-c.wsDone:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) wsSendMsg(msgType types.WSMessageType, payload interface{}) error {
	var data json.RawMessage
	if payload != nil {
		var err error
		data, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	envelope, err := json.Marshal(&types.WSMessage{Type: msgType, Data: data})
	if err != nil {
		return err
	}

	select {
	case c.wsSend <- envelope:
		return nil
	default:
		return fmt.Errorf("ws send buffer full")
	}
}

func (c *Client) handleWSDisconnect(err error) {
	c.DisconnectWS()

	if c.onDisconnect != nil {
		c.onDisconnect(err)
	}

	if !c.reconnecting.Load() {
		go c.wsReconnectLoop()
	}
}

func (c *Client) wsReconnectLoop() {
	if c.reconnecting.Swap(true) {
		return
	}
	defer c.reconnecting.Store(false)

	c.wsCloseOnce = sync.Once{} // reset for next disconnect

	backoff := c.config.ReconnectInterval
	for {
		select {
		case <-c.stopped:
			return
		default:
		}

		attempt := c.reconnectAttempt.Add(1)
		if c.config.MaxReconnectAttempts > 0 && int(attempt) > c.config.MaxReconnectAttempts {
			fmt.Printf("ws: max reconnection attempts reached\n")
			return
		}

		fmt.Printf("ws: reconnection attempt %d\n", attempt)
		time.Sleep(backoff)

		ctx := context.Background()
		if err := c.ConnectWS(ctx); err != nil {
			fmt.Printf("ws: reconnection failed: %v\n", err)
			backoff = min(backoff*2, 60*time.Second)
			continue
		}

		c.reconnectAttempt.Store(0)
		if c.onReconnect != nil {
			c.onReconnect()
		}
		return
	}
}

// toWebSocketURL converts an HTTP(s) URL or bare host:port to a ws:// URL.
func toWebSocketURL(raw string) string {
	if strings.HasPrefix(raw, "https://") {
		return "wss://" + strings.TrimPrefix(raw, "https://")
	}
	if strings.HasPrefix(raw, "http://") {
		return "ws://" + strings.TrimPrefix(raw, "http://")
	}
	return "ws://" + raw
}
