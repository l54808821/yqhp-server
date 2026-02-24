package types

import "encoding/json"

// WSMessageType defines WebSocket message types for Master-Slave communication.
type WSMessageType string

const (
	// Master -> Slave
	WSMsgTaskAssign  WSMessageType = "task_assign"
	WSMsgCommand     WSMessageType = "command"
	WSMsgPing        WSMessageType = "ping"
	WSMsgRegisterAck WSMessageType = "register_ack"

	// Slave -> Master
	WSMsgRegister   WSMessageType = "register"
	WSMsgHeartbeat  WSMessageType = "heartbeat"
	WSMsgTaskResult WSMessageType = "task_result"
	WSMsgMetrics    WSMessageType = "metrics"
	WSMsgPong       WSMessageType = "pong"
)

// WSMessage is the unified envelope for all WebSocket messages.
type WSMessage struct {
	Type WSMessageType   `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}
