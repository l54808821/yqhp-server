package master

import (
	"context"
	"fmt"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// InMemorySlaveRegistry 使用内存存储实现 SlaveRegistry。
// Requirements: 5.2, 12.2, 13.1
type InMemorySlaveRegistry struct {
	// Slave 存储
	slaves map[string]*types.SlaveInfo
	status map[string]*types.SlaveStatus

	// 事件订阅者
	subscribers []chan *types.SlaveEvent
	subMu       sync.RWMutex

	// 同步
	mu sync.RWMutex
}

// NewInMemorySlaveRegistry 创建一个新的内存 Slave 注册表。
func NewInMemorySlaveRegistry() *InMemorySlaveRegistry {
	return &InMemorySlaveRegistry{
		slaves:      make(map[string]*types.SlaveInfo),
		status:      make(map[string]*types.SlaveStatus),
		subscribers: make([]chan *types.SlaveEvent, 0),
	}
}

// Register 注册一个新的 Slave。
// Requirements: 5.2, 12.2
func (r *InMemorySlaveRegistry) Register(ctx context.Context, slave *types.SlaveInfo) error {
	if slave == nil {
		return fmt.Errorf("Slave 不能为空")
	}
	if slave.ID == "" {
		return fmt.Errorf("Slave ID 不能为空")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否已注册
	if _, exists := r.slaves[slave.ID]; exists {
		return fmt.Errorf("Slave 已注册: %s", slave.ID)
	}

	// 存储 Slave 信息
	r.slaves[slave.ID] = slave

	// 初始化状态
	r.status[slave.ID] = &types.SlaveStatus{
		State:       types.SlaveStateOnline,
		Load:        0,
		ActiveTasks: 0,
		LastSeen:    time.Now(),
	}

	// 通知订阅者
	r.notifyEvent(&types.SlaveEvent{
		Type:    types.SlaveEventRegistered,
		SlaveID: slave.ID,
		Slave:   slave,
	})

	return nil
}

// Unregister 注销一个 Slave。
// Requirements: 5.2
func (r *InMemorySlaveRegistry) Unregister(ctx context.Context, slaveID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	slave, exists := r.slaves[slaveID]
	if !exists {
		return fmt.Errorf("Slave 未找到: %s", slaveID)
	}

	delete(r.slaves, slaveID)
	delete(r.status, slaveID)

	// 通知订阅者
	r.notifyEvent(&types.SlaveEvent{
		Type:    types.SlaveEventUnregistered,
		SlaveID: slaveID,
		Slave:   slave,
	})

	return nil
}

// UpdateStatus 更新 Slave 的状态。
// Requirements: 12.2
func (r *InMemorySlaveRegistry) UpdateStatus(ctx context.Context, slaveID string, status *types.SlaveStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.slaves[slaveID]; !exists {
		return fmt.Errorf("Slave 未找到: %s", slaveID)
	}

	oldStatus := r.status[slaveID]
	r.status[slaveID] = status

	// 检查状态变化并通知
	if oldStatus != nil && oldStatus.State != status.State {
		var eventType types.SlaveEventType
		switch status.State {
		case types.SlaveStateOnline:
			eventType = types.SlaveEventOnline
		case types.SlaveStateOffline:
			eventType = types.SlaveEventOffline
		default:
			eventType = types.SlaveEventUpdated
		}

		r.notifyEvent(&types.SlaveEvent{
			Type:    eventType,
			SlaveID: slaveID,
			Slave:   r.slaves[slaveID],
		})
	}

	return nil
}

// GetSlave 返回单个 Slave 的信息。
func (r *InMemorySlaveRegistry) GetSlave(ctx context.Context, slaveID string) (*types.SlaveInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	slave, exists := r.slaves[slaveID]
	if !exists {
		return nil, fmt.Errorf("Slave 未找到: %s", slaveID)
	}

	return slave, nil
}

// GetSlaveStatus 返回 Slave 的当前状态。
func (r *InMemorySlaveRegistry) GetSlaveStatus(ctx context.Context, slaveID string) (*types.SlaveStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status, exists := r.status[slaveID]
	if !exists {
		return nil, fmt.Errorf("Slave 未找到: %s", slaveID)
	}

	return status, nil
}

// ListSlaves 列出所有匹配过滤条件的 Slave。
// Requirements: 13.1
func (r *InMemorySlaveRegistry) ListSlaves(ctx context.Context, filter *SlaveFilter) ([]*types.SlaveInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*types.SlaveInfo, 0, len(r.slaves))

	for id, slave := range r.slaves {
		if filter != nil && !r.matchesFilter(id, slave, filter) {
			continue
		}
		result = append(result, slave)
	}

	return result, nil
}

// matchesFilter 检查 Slave 是否匹配给定的过滤条件。
func (r *InMemorySlaveRegistry) matchesFilter(slaveID string, slave *types.SlaveInfo, filter *SlaveFilter) bool {
	// 按类型过滤
	if len(filter.Types) > 0 {
		found := false
		for _, t := range filter.Types {
			if slave.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 按状态过滤
	if len(filter.States) > 0 {
		status := r.status[slaveID]
		if status == nil {
			return false
		}
		found := false
		for _, s := range filter.States {
			if status.State == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 按标签过滤
	if len(filter.Labels) > 0 {
		for key, value := range filter.Labels {
			if slave.Labels == nil {
				return false
			}
			if slaveValue, ok := slave.Labels[key]; !ok || slaveValue != value {
				return false
			}
		}
	}

	// 按能力过滤
	if len(filter.Capabilities) > 0 {
		for _, required := range filter.Capabilities {
			found := false
			for _, cap := range slave.Capabilities {
				if cap == required {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}

// GetOnlineSlaves 返回所有在线的 Slave。
// Requirements: 13.1
func (r *InMemorySlaveRegistry) GetOnlineSlaves(ctx context.Context) ([]*types.SlaveInfo, error) {
	return r.ListSlaves(ctx, &SlaveFilter{
		States: []types.SlaveState{types.SlaveStateOnline},
	})
}

// WatchSlaves 监听 Slave 事件。
// Requirements: 13.1
func (r *InMemorySlaveRegistry) WatchSlaves(ctx context.Context) (<-chan *types.SlaveEvent, error) {
	ch := make(chan *types.SlaveEvent, 100)

	r.subMu.Lock()
	r.subscribers = append(r.subscribers, ch)
	r.subMu.Unlock()

	// 上下文结束时清理
	go func() {
		<-ctx.Done()
		r.removeSubscriber(ch)
		close(ch)
	}()

	return ch, nil
}

// notifyEvent 向所有订阅者发送事件。
func (r *InMemorySlaveRegistry) notifyEvent(event *types.SlaveEvent) {
	r.subMu.RLock()
	defer r.subMu.RUnlock()

	for _, ch := range r.subscribers {
		select {
		case ch <- event:
		default:
			// 通道已满，跳过
		}
	}
}

// removeSubscriber 移除订阅者通道。
func (r *InMemorySlaveRegistry) removeSubscriber(ch chan *types.SlaveEvent) {
	r.subMu.Lock()
	defer r.subMu.Unlock()

	for i, sub := range r.subscribers {
		if sub == ch {
			r.subscribers = append(r.subscribers[:i], r.subscribers[i+1:]...)
			break
		}
	}
}

// Count 返回已注册 Slave 的数量。
func (r *InMemorySlaveRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.slaves)
}

// CountOnline 返回在线 Slave 的数量。
func (r *InMemorySlaveRegistry) CountOnline() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for id := range r.slaves {
		if status, ok := r.status[id]; ok && status.State == types.SlaveStateOnline {
			count++
		}
	}
	return count
}

// UpdateHeartbeat 更新 Slave 的最后心跳时间。
func (r *InMemorySlaveRegistry) UpdateHeartbeat(ctx context.Context, slaveID string, metrics *types.SlaveMetrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, exists := r.status[slaveID]
	if !exists {
		return fmt.Errorf("Slave 未找到: %s", slaveID)
	}

	status.LastSeen = time.Now()
	if metrics != nil {
		status.Metrics = metrics
		status.Load = metrics.CPUUsage
	}

	// 如果 Slave 之前离线，标记为在线
	if status.State == types.SlaveStateOffline {
		status.State = types.SlaveStateOnline
		r.notifyEvent(&types.SlaveEvent{
			Type:    types.SlaveEventOnline,
			SlaveID: slaveID,
			Slave:   r.slaves[slaveID],
		})
	}

	return nil
}

// MarkOffline 将 Slave 标记为离线。
func (r *InMemorySlaveRegistry) MarkOffline(ctx context.Context, slaveID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, exists := r.status[slaveID]
	if !exists {
		return fmt.Errorf("Slave 未找到: %s", slaveID)
	}

	if status.State != types.SlaveStateOffline {
		status.State = types.SlaveStateOffline
		r.notifyEvent(&types.SlaveEvent{
			Type:    types.SlaveEventOffline,
			SlaveID: slaveID,
			Slave:   r.slaves[slaveID],
		})
	}

	return nil
}

// DrainSlave 将 Slave 标记为正在排空。
func (r *InMemorySlaveRegistry) DrainSlave(ctx context.Context, slaveID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, exists := r.status[slaveID]
	if !exists {
		return fmt.Errorf("Slave 未找到: %s", slaveID)
	}

	status.State = types.SlaveStateDraining
	r.notifyEvent(&types.SlaveEvent{
		Type:    types.SlaveEventUpdated,
		SlaveID: slaveID,
		Slave:   r.slaves[slaveID],
	})

	return nil
}
