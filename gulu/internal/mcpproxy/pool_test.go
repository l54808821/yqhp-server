package mcpproxy

import (
	"sync"
	"testing"
)

func TestNewConnectionPool(t *testing.T) {
	pool := NewConnectionPool()
	if pool == nil {
		t.Fatal("NewConnectionPool returned nil")
	}
	if pool.Count() != 0 {
		t.Fatalf("expected empty pool, got count=%d", pool.Count())
	}
}

func TestConnectionPool_PutAndGet(t *testing.T) {
	pool := NewConnectionPool()
	conn := &mcpConnection{config: MCPServerConnConfig{Transport: "stdio", Command: "echo"}}
	config := MCPServerConnConfig{Transport: "stdio", Command: "echo"}

	pool.Put(1, conn, config)

	got, ok := pool.Get(1)
	if !ok {
		t.Fatal("expected to find connection for serverID=1")
	}
	if got != conn {
		t.Fatal("returned connection does not match stored connection")
	}
}

func TestConnectionPool_GetNotFound(t *testing.T) {
	pool := NewConnectionPool()

	_, ok := pool.Get(999)
	if ok {
		t.Fatal("expected not found for non-existent serverID")
	}
}

func TestConnectionPool_Has(t *testing.T) {
	pool := NewConnectionPool()
	conn := &mcpConnection{}
	config := MCPServerConnConfig{Transport: "sse", URL: "http://localhost"}

	if pool.Has(1) {
		t.Fatal("expected Has=false before Put")
	}

	pool.Put(1, conn, config)

	if !pool.Has(1) {
		t.Fatal("expected Has=true after Put")
	}
	if pool.Has(2) {
		t.Fatal("expected Has=false for different serverID")
	}
}

func TestConnectionPool_Count(t *testing.T) {
	pool := NewConnectionPool()

	if pool.Count() != 0 {
		t.Fatalf("expected count=0, got %d", pool.Count())
	}

	pool.Put(1, &mcpConnection{}, MCPServerConnConfig{})
	pool.Put(2, &mcpConnection{}, MCPServerConnConfig{})

	if pool.Count() != 2 {
		t.Fatalf("expected count=2, got %d", pool.Count())
	}
}

func TestConnectionPool_Remove(t *testing.T) {
	pool := NewConnectionPool()
	conn := &mcpConnection{} // client is nil, Close won't be called
	pool.Put(1, conn, MCPServerConnConfig{})

	err := pool.Remove(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.Has(1) {
		t.Fatal("expected connection removed")
	}
	if pool.Count() != 0 {
		t.Fatalf("expected count=0 after remove, got %d", pool.Count())
	}
}

func TestConnectionPool_RemoveNotFound(t *testing.T) {
	pool := NewConnectionPool()

	err := pool.Remove(999)
	if err == nil {
		t.Fatal("expected error when removing non-existent connection")
	}
}

func TestConnectionPool_CloseAll(t *testing.T) {
	pool := NewConnectionPool()
	pool.Put(1, &mcpConnection{}, MCPServerConnConfig{})
	pool.Put(2, &mcpConnection{}, MCPServerConnConfig{})

	err := pool.CloseAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.Count() != 0 {
		t.Fatalf("expected count=0 after CloseAll, got %d", pool.Count())
	}
}

func TestConnectionPool_Touch(t *testing.T) {
	pool := NewConnectionPool()
	pool.Put(1, &mcpConnection{}, MCPServerConnConfig{})

	// Get initial lastUsed
	pool.mu.RLock()
	initialTime := pool.connections[1].lastUsed
	pool.mu.RUnlock()

	// Touch should update lastUsed
	pool.Touch(1)

	pool.mu.RLock()
	updatedTime := pool.connections[1].lastUsed
	pool.mu.RUnlock()

	if updatedTime.Before(initialTime) {
		t.Fatal("Touch should not set lastUsed to an earlier time")
	}
}

func TestConnectionPool_TouchNonExistent(t *testing.T) {
	pool := NewConnectionPool()
	// Should not panic
	pool.Touch(999)
}

func TestConnectionPool_PutOverwrite(t *testing.T) {
	pool := NewConnectionPool()
	conn1 := &mcpConnection{config: MCPServerConnConfig{Transport: "stdio"}}
	conn2 := &mcpConnection{config: MCPServerConnConfig{Transport: "sse"}}

	pool.Put(1, conn1, MCPServerConnConfig{Transport: "stdio"})
	pool.Put(1, conn2, MCPServerConnConfig{Transport: "sse"})

	got, ok := pool.Get(1)
	if !ok {
		t.Fatal("expected to find connection")
	}
	if got != conn2 {
		t.Fatal("expected overwritten connection")
	}
	if pool.Count() != 1 {
		t.Fatalf("expected count=1 after overwrite, got %d", pool.Count())
	}
}

func TestConnectionPool_ConcurrentAccess(t *testing.T) {
	pool := NewConnectionPool()
	var wg sync.WaitGroup

	// Concurrent puts
	for i := int64(0); i < 50; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			pool.Put(id, &mcpConnection{}, MCPServerConnConfig{})
		}(i)
	}
	wg.Wait()

	if pool.Count() != 50 {
		t.Fatalf("expected count=50, got %d", pool.Count())
	}

	// Concurrent reads
	for i := int64(0); i < 50; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			_, ok := pool.Get(id)
			if !ok {
				t.Errorf("expected to find connection for serverID=%d", id)
			}
			pool.Has(id)
			pool.Touch(id)
		}(i)
	}
	wg.Wait()
}
