package master

import (
	"context"
	"fmt"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// InMemorySlaveRegistry implements SlaveRegistry using in-memory storage.
// Requirements: 5.2, 12.2, 13.1
type InMemorySlaveRegistry struct {
	// Slave storage
	slaves map[string]*types.SlaveInfo
	status map[string]*types.SlaveStatus

	// Event subscribers
	subscribers []chan *types.SlaveEvent
	subMu       sync.RWMutex

	// Synchronization
	mu sync.RWMutex
}

// NewInMemorySlaveRegistry creates a new in-memory slave registry.
func NewInMemorySlaveRegistry() *InMemorySlaveRegistry {
	return &InMemorySlaveRegistry{
		slaves:      make(map[string]*types.SlaveInfo),
		status:      make(map[string]*types.SlaveStatus),
		subscribers: make([]chan *types.SlaveEvent, 0),
	}
}

// Register registers a new slave.
// Requirements: 5.2, 12.2
func (r *InMemorySlaveRegistry) Register(ctx context.Context, slave *types.SlaveInfo) error {
	if slave == nil {
		return fmt.Errorf("slave cannot be nil")
	}
	if slave.ID == "" {
		return fmt.Errorf("slave ID cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already registered
	if _, exists := r.slaves[slave.ID]; exists {
		return fmt.Errorf("slave already registered: %s", slave.ID)
	}

	// Store slave info
	r.slaves[slave.ID] = slave

	// Initialize status
	r.status[slave.ID] = &types.SlaveStatus{
		State:       types.SlaveStateOnline,
		Load:        0,
		ActiveTasks: 0,
		LastSeen:    time.Now(),
	}

	// Notify subscribers
	r.notifyEvent(&types.SlaveEvent{
		Type:    types.SlaveEventRegistered,
		SlaveID: slave.ID,
		Slave:   slave,
	})

	return nil
}

// Unregister unregisters a slave.
// Requirements: 5.2
func (r *InMemorySlaveRegistry) Unregister(ctx context.Context, slaveID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	slave, exists := r.slaves[slaveID]
	if !exists {
		return fmt.Errorf("slave not found: %s", slaveID)
	}

	delete(r.slaves, slaveID)
	delete(r.status, slaveID)

	// Notify subscribers
	r.notifyEvent(&types.SlaveEvent{
		Type:    types.SlaveEventUnregistered,
		SlaveID: slaveID,
		Slave:   slave,
	})

	return nil
}

// UpdateStatus updates a slave's status.
// Requirements: 12.2
func (r *InMemorySlaveRegistry) UpdateStatus(ctx context.Context, slaveID string, status *types.SlaveStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.slaves[slaveID]; !exists {
		return fmt.Errorf("slave not found: %s", slaveID)
	}

	oldStatus := r.status[slaveID]
	r.status[slaveID] = status

	// Check for state changes and notify
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

// GetSlave returns a single slave's information.
func (r *InMemorySlaveRegistry) GetSlave(ctx context.Context, slaveID string) (*types.SlaveInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	slave, exists := r.slaves[slaveID]
	if !exists {
		return nil, fmt.Errorf("slave not found: %s", slaveID)
	}

	return slave, nil
}

// GetSlaveStatus returns a slave's current status.
func (r *InMemorySlaveRegistry) GetSlaveStatus(ctx context.Context, slaveID string) (*types.SlaveStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status, exists := r.status[slaveID]
	if !exists {
		return nil, fmt.Errorf("slave not found: %s", slaveID)
	}

	return status, nil
}

// ListSlaves lists all slaves matching the filter.
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

// matchesFilter checks if a slave matches the given filter.
func (r *InMemorySlaveRegistry) matchesFilter(slaveID string, slave *types.SlaveInfo, filter *SlaveFilter) bool {
	// Filter by type
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

	// Filter by state
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

	// Filter by labels
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

	// Filter by capabilities
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

// GetOnlineSlaves returns all online slaves.
// Requirements: 13.1
func (r *InMemorySlaveRegistry) GetOnlineSlaves(ctx context.Context) ([]*types.SlaveInfo, error) {
	return r.ListSlaves(ctx, &SlaveFilter{
		States: []types.SlaveState{types.SlaveStateOnline},
	})
}

// WatchSlaves watches for slave events.
// Requirements: 13.1
func (r *InMemorySlaveRegistry) WatchSlaves(ctx context.Context) (<-chan *types.SlaveEvent, error) {
	ch := make(chan *types.SlaveEvent, 100)

	r.subMu.Lock()
	r.subscribers = append(r.subscribers, ch)
	r.subMu.Unlock()

	// Clean up when context is done
	go func() {
		<-ctx.Done()
		r.removeSubscriber(ch)
		close(ch)
	}()

	return ch, nil
}

// notifyEvent sends an event to all subscribers.
func (r *InMemorySlaveRegistry) notifyEvent(event *types.SlaveEvent) {
	r.subMu.RLock()
	defer r.subMu.RUnlock()

	for _, ch := range r.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
}

// removeSubscriber removes a subscriber channel.
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

// Count returns the number of registered slaves.
func (r *InMemorySlaveRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.slaves)
}

// CountOnline returns the number of online slaves.
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

// UpdateHeartbeat updates the last seen time for a slave.
func (r *InMemorySlaveRegistry) UpdateHeartbeat(ctx context.Context, slaveID string, metrics *types.SlaveMetrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, exists := r.status[slaveID]
	if !exists {
		return fmt.Errorf("slave not found: %s", slaveID)
	}

	status.LastSeen = time.Now()
	if metrics != nil {
		status.Metrics = metrics
		status.Load = metrics.CPUUsage
	}

	// If slave was offline, mark as online
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

// MarkOffline marks a slave as offline.
func (r *InMemorySlaveRegistry) MarkOffline(ctx context.Context, slaveID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, exists := r.status[slaveID]
	if !exists {
		return fmt.Errorf("slave not found: %s", slaveID)
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

// DrainSlave marks a slave for draining.
func (r *InMemorySlaveRegistry) DrainSlave(ctx context.Context, slaveID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, exists := r.status[slaveID]
	if !exists {
		return fmt.Errorf("slave not found: %s", slaveID)
	}

	status.State = types.SlaveStateDraining
	r.notifyEvent(&types.SlaveEvent{
		Type:    types.SlaveEventUpdated,
		SlaveID: slaveID,
		Slave:   r.slaves[slaveID],
	})

	return nil
}
