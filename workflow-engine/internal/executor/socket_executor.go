package executor

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

const (
	// SocketExecutorType is the type identifier for Socket executor.
	SocketExecutorType = "socket"

	// Default buffer size for socket operations.
	defaultBufferSize = 4096

	// Default timeout for socket operations.
	defaultSocketTimeout = 30 * time.Second
)

// SocketConfig Socket 配置
type SocketConfig struct {
	Protocol   string        `yaml:"protocol" json:"protocol"`                           // 协议: tcp/udp
	Host       string        `yaml:"host" json:"host"`                                   // 目标主机
	Port       int           `yaml:"port" json:"port"`                                   // 目标端口
	TLS        bool          `yaml:"tls,omitempty" json:"tls,omitempty"`                 // 是否启用 TLS
	TLSConfig  *tls.Config   `yaml:"-" json:"-"`                                         // TLS 配置
	Timeout    time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`         // 超时时间
	BufferSize int           `yaml:"buffer_size,omitempty" json:"buffer_size,omitempty"` // 缓冲区大小
}

// SocketOperation Socket 操作
type SocketOperation struct {
	Action    string `yaml:"action" json:"action"`                           // 操作: connect/send/receive/close
	Data      string `yaml:"data,omitempty" json:"data,omitempty"`           // 发送数据
	Delimiter string `yaml:"delimiter,omitempty" json:"delimiter,omitempty"` // 接收分隔符
	Length    int    `yaml:"length,omitempty" json:"length,omitempty"`       // 固定长度接收
}

// SocketExecutor executes Socket operations.
type SocketExecutor struct {
	*BaseExecutor
	config *SocketConfig
	conn   net.Conn
	connMu sync.Mutex
	reader *bufio.Reader
}

// NewSocketExecutor creates a new Socket executor.
func NewSocketExecutor() *SocketExecutor {
	return &SocketExecutor{
		BaseExecutor: NewBaseExecutor(SocketExecutorType),
	}
}

// Init initializes the Socket executor with configuration.
func (e *SocketExecutor) Init(ctx context.Context, config map[string]any) error {
	if err := e.BaseExecutor.Init(ctx, config); err != nil {
		return err
	}

	e.config = &SocketConfig{
		Protocol:   "tcp",
		BufferSize: defaultBufferSize,
		Timeout:    defaultSocketTimeout,
	}

	// 解析配置
	if protocol, ok := config["protocol"].(string); ok {
		e.config.Protocol = strings.ToLower(protocol)
	}
	if host, ok := config["host"].(string); ok {
		e.config.Host = host
	}
	if port, ok := config["port"].(int); ok {
		e.config.Port = port
	}
	if tlsEnabled, ok := config["tls"].(bool); ok {
		e.config.TLS = tlsEnabled
	}
	if timeout, ok := config["timeout"].(string); ok {
		if d, err := time.ParseDuration(timeout); err == nil {
			e.config.Timeout = d
		}
	}
	if bufferSize, ok := config["buffer_size"].(int); ok {
		e.config.BufferSize = bufferSize
	}

	return nil
}

// Execute executes a Socket operation step.
func (e *SocketExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// 解析操作配置
	op, err := e.parseOperation(step.Config)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// 解析步骤级配置（覆盖全局配置）
	stepConfig := e.parseStepConfig(step.Config)

	// 变量解析
	if execCtx != nil {
		evalCtx := execCtx.ToEvaluationContext()
		op.Data = resolveString(op.Data, evalCtx)
		op.Delimiter = resolveString(op.Delimiter, evalCtx)
		stepConfig.Host = resolveString(stepConfig.Host, evalCtx)
	}

	// 执行操作
	var output any
	switch op.Action {
	case "connect":
		output, err = e.connect(ctx, stepConfig)
	case "send":
		output, err = e.send(ctx, op.Data)
	case "receive":
		output, err = e.receive(ctx, op, stepConfig)
	case "close":
		output, err = e.close(ctx)
	default:
		err = NewConfigError(fmt.Sprintf("unknown socket action: %s", op.Action), nil)
	}

	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	result := CreateSuccessResult(step.ID, startTime, output)
	return result, nil
}

// parseOperation 解析操作配置
func (e *SocketExecutor) parseOperation(config map[string]any) (*SocketOperation, error) {
	op := &SocketOperation{}

	if action, ok := config["action"].(string); ok {
		op.Action = strings.ToLower(action)
	} else {
		return nil, NewConfigError("socket step requires 'action' configuration", nil)
	}

	if data, ok := config["data"].(string); ok {
		op.Data = data
	}
	if delimiter, ok := config["delimiter"].(string); ok {
		op.Delimiter = delimiter
	}
	if length, ok := config["length"].(int); ok {
		op.Length = length
	}

	return op, nil
}

// parseStepConfig 解析步骤级配置
func (e *SocketExecutor) parseStepConfig(config map[string]any) *SocketConfig {
	stepConfig := &SocketConfig{
		Protocol:   e.config.Protocol,
		Host:       e.config.Host,
		Port:       e.config.Port,
		TLS:        e.config.TLS,
		Timeout:    e.config.Timeout,
		BufferSize: e.config.BufferSize,
	}

	if protocol, ok := config["protocol"].(string); ok {
		stepConfig.Protocol = strings.ToLower(protocol)
	}
	if host, ok := config["host"].(string); ok {
		stepConfig.Host = host
	}
	if port, ok := config["port"].(int); ok {
		stepConfig.Port = port
	}
	if tlsEnabled, ok := config["tls"].(bool); ok {
		stepConfig.TLS = tlsEnabled
	}
	if timeout, ok := config["timeout"].(string); ok {
		if d, err := time.ParseDuration(timeout); err == nil {
			stepConfig.Timeout = d
		}
	}
	if bufferSize, ok := config["buffer_size"].(int); ok {
		stepConfig.BufferSize = bufferSize
	}

	return stepConfig
}

// connect 建立连接
func (e *SocketExecutor) connect(ctx context.Context, config *SocketConfig) (map[string]any, error) {
	e.connMu.Lock()
	defer e.connMu.Unlock()

	// 关闭现有连接
	if e.conn != nil {
		e.conn.Close()
		e.conn = nil
		e.reader = nil
	}

	address := fmt.Sprintf("%s:%d", config.Host, config.Port)

	var conn net.Conn
	var err error

	// 创建带超时的拨号器
	dialer := &net.Dialer{
		Timeout: config.Timeout,
	}

	switch config.Protocol {
	case "tcp":
		if config.TLS {
			tlsConfig := config.TLSConfig
			if tlsConfig == nil {
				tlsConfig = &tls.Config{
					InsecureSkipVerify: true,
				}
			}
			conn, err = tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
		} else {
			conn, err = dialer.DialContext(ctx, "tcp", address)
		}
	case "udp":
		conn, err = dialer.DialContext(ctx, "udp", address)
	default:
		return nil, NewConfigError(fmt.Sprintf("unsupported protocol: %s", config.Protocol), nil)
	}

	if err != nil {
		return nil, NewExecutionError("socket", fmt.Sprintf("failed to connect to %s", address), err)
	}

	e.conn = conn
	e.reader = bufio.NewReaderSize(conn, config.BufferSize)

	return map[string]any{
		"connected":   true,
		"local_addr":  conn.LocalAddr().String(),
		"remote_addr": conn.RemoteAddr().String(),
		"protocol":    config.Protocol,
		"tls_enabled": config.TLS,
	}, nil
}

// send 发送数据
func (e *SocketExecutor) send(ctx context.Context, data string) (map[string]any, error) {
	e.connMu.Lock()
	defer e.connMu.Unlock()

	if e.conn == nil {
		return nil, NewExecutionError("socket", "not connected", nil)
	}

	// 设置写超时
	if deadline, ok := ctx.Deadline(); ok {
		e.conn.SetWriteDeadline(deadline)
	}

	n, err := e.conn.Write([]byte(data))
	if err != nil {
		return nil, NewExecutionError("socket", "failed to send data", err)
	}

	return map[string]any{
		"bytes_sent": n,
		"data":       data,
	}, nil
}

// receive 接收数据
func (e *SocketExecutor) receive(ctx context.Context, op *SocketOperation, config *SocketConfig) (map[string]any, error) {
	e.connMu.Lock()
	defer e.connMu.Unlock()

	if e.conn == nil {
		return nil, NewExecutionError("socket", "not connected", nil)
	}

	// 设置读超时
	if deadline, ok := ctx.Deadline(); ok {
		e.conn.SetReadDeadline(deadline)
	} else {
		e.conn.SetReadDeadline(time.Now().Add(config.Timeout))
	}

	var data []byte
	var err error

	if op.Length > 0 {
		// 固定长度接收
		data = make([]byte, op.Length)
		_, err = io.ReadFull(e.reader, data)
	} else if op.Delimiter != "" {
		// 分隔符接收
		delimByte := op.Delimiter[0]
		data, err = e.reader.ReadBytes(delimByte)
	} else {
		// 读取可用数据
		data = make([]byte, config.BufferSize)
		var n int
		n, err = e.reader.Read(data)
		data = data[:n]
	}

	if err != nil && err != io.EOF {
		return nil, NewExecutionError("socket", "failed to receive data", err)
	}

	return map[string]any{
		"bytes_received": len(data),
		"data":           string(data),
	}, nil
}

// close 关闭连接
func (e *SocketExecutor) close(ctx context.Context) (map[string]any, error) {
	e.connMu.Lock()
	defer e.connMu.Unlock()

	if e.conn == nil {
		return map[string]any{
			"closed":        true,
			"was_connected": false,
		}, nil
	}

	err := e.conn.Close()
	e.conn = nil
	e.reader = nil

	if err != nil {
		return nil, NewExecutionError("socket", "failed to close connection", err)
	}

	return map[string]any{
		"closed":        true,
		"was_connected": true,
	}, nil
}

// Cleanup releases resources held by the Socket executor.
func (e *SocketExecutor) Cleanup(ctx context.Context) error {
	e.connMu.Lock()
	defer e.connMu.Unlock()

	if e.conn != nil {
		e.conn.Close()
		e.conn = nil
		e.reader = nil
	}
	return nil
}

// IsConnected returns whether the socket is connected.
func (e *SocketExecutor) IsConnected() bool {
	e.connMu.Lock()
	defer e.connMu.Unlock()
	return e.conn != nil
}

// init registers the Socket executor with the default registry.
func init() {
	MustRegister(NewSocketExecutor())
}
