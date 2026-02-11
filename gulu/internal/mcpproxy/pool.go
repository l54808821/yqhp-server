package mcpproxy

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
)

// poolEntry 连接池条目，包装 mcpConnection 并附加池管理元数据
type poolEntry struct {
	conn         *mcpConnection
	config       MCPServerConnConfig
	lastUsed     time.Time
	reconnecting bool
}

// ConnectionPool MCP 连接池，按 serverID 管理连接
type ConnectionPool struct {
	connections map[int64]*poolEntry
	mu          sync.RWMutex
}

// NewConnectionPool 创建新的连接池
func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		connections: make(map[int64]*poolEntry),
	}
}

// Get 获取指定 serverID 的连接
func (p *ConnectionPool) Get(serverID int64) (*mcpConnection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	entry, ok := p.connections[serverID]
	if !ok {
		return nil, false
	}
	return entry.conn, true
}

// Put 存储连接到池中
func (p *ConnectionPool) Put(serverID int64, conn *mcpConnection, config MCPServerConnConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.connections[serverID] = &poolEntry{
		conn:     conn,
		config:   config,
		lastUsed: time.Now(),
	}
}

// Remove 移除并关闭指定 serverID 的连接
func (p *ConnectionPool) Remove(serverID int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry, ok := p.connections[serverID]
	if !ok {
		return fmt.Errorf("连接池中不存在 serverID=%d 的连接", serverID)
	}

	var err error
	if entry.conn != nil && entry.conn.client != nil {
		err = entry.conn.client.Close()
	}
	delete(p.connections, serverID)

	log.Printf("[ConnectionPool] 已移除 serverID=%d 的连接", serverID)
	return err
}

// CloseAll 关闭池中所有连接
func (p *ConnectionPool) CloseAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error
	for id, entry := range p.connections {
		if entry.conn != nil && entry.conn.client != nil {
			if err := entry.conn.client.Close(); err != nil {
				log.Printf("[ConnectionPool] 关闭 serverID=%d 连接失败: %v", id, err)
				lastErr = err
			}
		}
	}
	p.connections = make(map[int64]*poolEntry)

	log.Printf("[ConnectionPool] 已关闭所有连接")
	return lastErr
}

// Has 检查指定 serverID 的连接是否存在
func (p *ConnectionPool) Has(serverID int64) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, ok := p.connections[serverID]
	return ok
}

// Count 返回池中活跃连接数量
func (p *ConnectionPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.connections)
}

// Touch 更新指定 serverID 连接的最后使用时间
func (p *ConnectionPool) Touch(serverID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if entry, ok := p.connections[serverID]; ok {
		entry.lastUsed = time.Now()
	}
}

// TryReconnect 尝试重新连接指定 serverID 的 MCP 服务器
// createFn 用于根据配置创建新的 MCP 客户端
// initFn 用于初始化新创建的客户端连接
func (p *ConnectionPool) TryReconnect(
	ctx context.Context,
	serverID int64,
	createFn func(MCPServerConnConfig) (*client.Client, error),
	initFn func(context.Context, *client.Client) error,
) error {
	p.mu.Lock()
	entry, ok := p.connections[serverID]
	if !ok {
		p.mu.Unlock()
		return fmt.Errorf("连接池中不存在 serverID=%d 的连接", serverID)
	}

	// 防止并发重连
	if entry.reconnecting {
		p.mu.Unlock()
		return fmt.Errorf("serverID=%d 正在重连中", serverID)
	}
	entry.reconnecting = true
	config := entry.config
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		if e, ok := p.connections[serverID]; ok {
			e.reconnecting = false
		}
		p.mu.Unlock()
	}()

	log.Printf("[ConnectionPool] 尝试重连 serverID=%d (transport=%s)", serverID, config.Transport)

	// 关闭旧连接（忽略错误，可能已断开）
	p.mu.RLock()
	if entry.conn != nil && entry.conn.client != nil {
		_ = entry.conn.client.Close()
	}
	p.mu.RUnlock()

	// 创建新客户端
	newClient, err := createFn(config)
	if err != nil {
		p.mu.Lock()
		if e, ok := p.connections[serverID]; ok {
			e.conn.err = fmt.Sprintf("重连失败: %v", err)
		}
		p.mu.Unlock()
		return fmt.Errorf("重连创建客户端失败: %w", err)
	}

	// 初始化新连接
	if err := initFn(ctx, newClient); err != nil {
		_ = newClient.Close()
		p.mu.Lock()
		if e, ok := p.connections[serverID]; ok {
			e.conn.err = fmt.Sprintf("重连初始化失败: %v", err)
		}
		p.mu.Unlock()
		return fmt.Errorf("重连初始化失败: %w", err)
	}

	// 更新连接
	p.mu.Lock()
	if e, ok := p.connections[serverID]; ok {
		e.conn.client = newClient
		e.conn.err = ""
		e.lastUsed = time.Now()
	}
	p.mu.Unlock()

	log.Printf("[ConnectionPool] 重连 serverID=%d 成功", serverID)
	return nil
}
