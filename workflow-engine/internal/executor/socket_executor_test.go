package executor

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// TestSocketExecutor_Connect tests socket connection.
func TestSocketExecutor_Connect(t *testing.T) {
	// 启动测试 TCP 服务器
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)

	// 接受连接的 goroutine
	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
			// 保持连接一段时间
			time.Sleep(100 * time.Millisecond)
		}
	}()

	executor := NewSocketExecutor()
	ctx := context.Background()

	err = executor.Init(ctx, map[string]any{
		"protocol": "tcp",
		"host":     "127.0.0.1",
		"port":     addr.Port,
		"timeout":  "5s",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	step := &types.Step{
		ID: "test-connect",
		Config: map[string]any{
			"action": "connect",
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusSuccess {
		t.Errorf("expected success, got %s: %s", result.Status, result.Error)
	}

	if !executor.IsConnected() {
		t.Error("expected to be connected")
	}
}

// TestSocketExecutor_SendReceive tests send and receive operations.
func TestSocketExecutor_SendReceive(t *testing.T) {
	// 启动测试 TCP 服务器
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)

	// Echo 服务器
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		conn.Write(buf[:n])
	}()

	executor := NewSocketExecutor()
	ctx := context.Background()

	err = executor.Init(ctx, map[string]any{
		"protocol": "tcp",
		"host":     "127.0.0.1",
		"port":     addr.Port,
		"timeout":  "5s",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	// 连接
	connectStep := &types.Step{
		ID: "test-connect",
		Config: map[string]any{
			"action": "connect",
		},
	}
	result, err := executor.Execute(ctx, connectStep, nil)
	if err != nil || result.Status != types.ResultStatusSuccess {
		t.Fatalf("connect failed: %v, %s", err, result.Error)
	}

	// 发送
	testData := "Hello, Socket!"
	sendStep := &types.Step{
		ID: "test-send",
		Config: map[string]any{
			"action": "send",
			"data":   testData,
		},
	}
	result, err = executor.Execute(ctx, sendStep, nil)
	if err != nil || result.Status != types.ResultStatusSuccess {
		t.Fatalf("send failed: %v, %s", err, result.Error)
	}

	output := result.Output.(map[string]any)
	if output["bytes_sent"].(int) != len(testData) {
		t.Errorf("expected %d bytes sent, got %v", len(testData), output["bytes_sent"])
	}

	// 接收
	receiveStep := &types.Step{
		ID: "test-receive",
		Config: map[string]any{
			"action": "receive",
			"length": len(testData),
		},
	}
	result, err = executor.Execute(ctx, receiveStep, nil)
	if err != nil || result.Status != types.ResultStatusSuccess {
		t.Fatalf("receive failed: %v, %s", err, result.Error)
	}

	output = result.Output.(map[string]any)
	if output["data"].(string) != testData {
		t.Errorf("expected %q, got %q", testData, output["data"])
	}

	wg.Wait()
}

// TestSocketExecutor_ReceiveWithDelimiter tests receive with delimiter.
func TestSocketExecutor_ReceiveWithDelimiter(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)

	// 服务器发送带分隔符的数据
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.Write([]byte("line1\nline2\n"))
	}()

	executor := NewSocketExecutor()
	ctx := context.Background()

	err = executor.Init(ctx, map[string]any{
		"protocol": "tcp",
		"host":     "127.0.0.1",
		"port":     addr.Port,
		"timeout":  "5s",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	// 连接
	connectStep := &types.Step{
		ID: "test-connect",
		Config: map[string]any{
			"action": "connect",
		},
	}
	executor.Execute(ctx, connectStep, nil)

	// 接收到分隔符
	receiveStep := &types.Step{
		ID: "test-receive",
		Config: map[string]any{
			"action":    "receive",
			"delimiter": "\n",
		},
	}
	result, err := executor.Execute(ctx, receiveStep, nil)
	if err != nil || result.Status != types.ResultStatusSuccess {
		t.Fatalf("receive failed: %v, %s", err, result.Error)
	}

	output := result.Output.(map[string]any)
	if output["data"].(string) != "line1\n" {
		t.Errorf("expected 'line1\\n', got %q", output["data"])
	}
}

// TestSocketExecutor_Close tests close operation.
func TestSocketExecutor_Close(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
			time.Sleep(100 * time.Millisecond)
		}
	}()

	executor := NewSocketExecutor()
	ctx := context.Background()

	err = executor.Init(ctx, map[string]any{
		"protocol": "tcp",
		"host":     "127.0.0.1",
		"port":     addr.Port,
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}

	// 连接
	connectStep := &types.Step{
		ID: "test-connect",
		Config: map[string]any{
			"action": "connect",
		},
	}
	executor.Execute(ctx, connectStep, nil)

	if !executor.IsConnected() {
		t.Error("expected to be connected")
	}

	// 关闭
	closeStep := &types.Step{
		ID: "test-close",
		Config: map[string]any{
			"action": "close",
		},
	}
	result, err := executor.Execute(ctx, closeStep, nil)
	if err != nil || result.Status != types.ResultStatusSuccess {
		t.Fatalf("close failed: %v, %s", err, result.Error)
	}

	if executor.IsConnected() {
		t.Error("expected to be disconnected")
	}

	output := result.Output.(map[string]any)
	if !output["closed"].(bool) {
		t.Error("expected closed to be true")
	}
	if !output["was_connected"].(bool) {
		t.Error("expected was_connected to be true")
	}
}

// TestSocketExecutor_CloseNotConnected tests close when not connected.
func TestSocketExecutor_CloseNotConnected(t *testing.T) {
	executor := NewSocketExecutor()
	ctx := context.Background()

	err := executor.Init(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}

	closeStep := &types.Step{
		ID: "test-close",
		Config: map[string]any{
			"action": "close",
		},
	}
	result, err := executor.Execute(ctx, closeStep, nil)
	if err != nil || result.Status != types.ResultStatusSuccess {
		t.Fatalf("close failed: %v, %s", err, result.Error)
	}

	output := result.Output.(map[string]any)
	if !output["closed"].(bool) {
		t.Error("expected closed to be true")
	}
	if output["was_connected"].(bool) {
		t.Error("expected was_connected to be false")
	}
}

// TestSocketExecutor_SendNotConnected tests send when not connected.
func TestSocketExecutor_SendNotConnected(t *testing.T) {
	executor := NewSocketExecutor()
	ctx := context.Background()

	err := executor.Init(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}

	sendStep := &types.Step{
		ID: "test-send",
		Config: map[string]any{
			"action": "send",
			"data":   "test",
		},
	}
	result, err := executor.Execute(ctx, sendStep, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusFailed {
		t.Errorf("expected failed status, got %s", result.Status)
	}
}

// TestSocketExecutor_UDP tests UDP protocol.
func TestSocketExecutor_UDP(t *testing.T) {
	// 启动 UDP 服务器
	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to resolve address: %v", err)
	}

	serverConn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		t.Fatalf("failed to start UDP server: %v", err)
	}
	defer serverConn.Close()

	addr := serverConn.LocalAddr().(*net.UDPAddr)

	// Echo 服务器
	go func() {
		buf := make([]byte, 1024)
		n, clientAddr, err := serverConn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		serverConn.WriteToUDP(buf[:n], clientAddr)
	}()

	executor := NewSocketExecutor()
	ctx := context.Background()

	err = executor.Init(ctx, map[string]any{
		"protocol": "udp",
		"host":     "127.0.0.1",
		"port":     addr.Port,
		"timeout":  "5s",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	// 连接
	connectStep := &types.Step{
		ID: "test-connect",
		Config: map[string]any{
			"action": "connect",
		},
	}
	result, err := executor.Execute(ctx, connectStep, nil)
	if err != nil || result.Status != types.ResultStatusSuccess {
		t.Fatalf("connect failed: %v, %s", err, result.Error)
	}

	output := result.Output.(map[string]any)
	if output["protocol"].(string) != "udp" {
		t.Errorf("expected udp protocol, got %s", output["protocol"])
	}
}

// TestSocketExecutor_StepConfigOverride tests step-level config override.
func TestSocketExecutor_StepConfigOverride(t *testing.T) {
	// 启动测试服务器
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
			time.Sleep(100 * time.Millisecond)
		}
	}()

	executor := NewSocketExecutor()
	ctx := context.Background()

	// 初始化时使用错误的端口
	err = executor.Init(ctx, map[string]any{
		"protocol": "tcp",
		"host":     "127.0.0.1",
		"port":     12345, // 错误端口
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	// 步骤级配置覆盖为正确的端口
	connectStep := &types.Step{
		ID: "test-connect",
		Config: map[string]any{
			"action": "connect",
			"port":   addr.Port, // 正确端口
		},
	}

	result, err := executor.Execute(ctx, connectStep, nil)
	if err != nil || result.Status != types.ResultStatusSuccess {
		t.Fatalf("connect failed: %v, %s", err, result.Error)
	}

	if !executor.IsConnected() {
		t.Error("expected to be connected")
	}
}

// TestSocketExecutor_InvalidAction tests invalid action.
func TestSocketExecutor_InvalidAction(t *testing.T) {
	executor := NewSocketExecutor()
	ctx := context.Background()

	err := executor.Init(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}

	step := &types.Step{
		ID: "test-invalid",
		Config: map[string]any{
			"action": "invalid",
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusFailed {
		t.Errorf("expected failed status, got %s", result.Status)
	}
}

// TestSocketExecutor_MissingAction tests missing action.
func TestSocketExecutor_MissingAction(t *testing.T) {
	executor := NewSocketExecutor()
	ctx := context.Background()

	err := executor.Init(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}

	step := &types.Step{
		ID:     "test-missing",
		Config: map[string]any{},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusFailed {
		t.Errorf("expected failed status, got %s", result.Status)
	}
}

// BenchmarkSocketExecutor_SendReceive benchmarks send/receive operations.
func BenchmarkSocketExecutor_SendReceive(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("failed to start test server: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)

	// Echo 服务器
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					c.Write(buf[:n])
				}
			}(conn)
		}
	}()

	executor := NewSocketExecutor()
	ctx := context.Background()

	executor.Init(ctx, map[string]any{
		"protocol": "tcp",
		"host":     "127.0.0.1",
		"port":     addr.Port,
	})
	defer executor.Cleanup(ctx)

	// 连接
	connectStep := &types.Step{
		ID: "connect",
		Config: map[string]any{
			"action": "connect",
		},
	}
	executor.Execute(ctx, connectStep, nil)

	testData := "benchmark test data"
	sendStep := &types.Step{
		ID: "send",
		Config: map[string]any{
			"action": "send",
			"data":   testData,
		},
	}
	receiveStep := &types.Step{
		ID: "receive",
		Config: map[string]any{
			"action": "receive",
			"length": len(testData),
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.Execute(ctx, sendStep, nil)
		executor.Execute(ctx, receiveStep, nil)
	}
}

// ExampleSocketExecutor demonstrates socket executor usage.
func ExampleSocketExecutor() {
	executor := NewSocketExecutor()
	ctx := context.Background()

	// 初始化
	executor.Init(ctx, map[string]any{
		"protocol": "tcp",
		"host":     "example.com",
		"port":     80,
		"timeout":  "10s",
	})
	defer executor.Cleanup(ctx)

	// 连接
	connectStep := &types.Step{
		ID: "connect",
		Config: map[string]any{
			"action": "connect",
		},
	}
	result, _ := executor.Execute(ctx, connectStep, nil)
	fmt.Printf("Connected: %v\n", result.Status)
}
