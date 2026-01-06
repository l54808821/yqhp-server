package slave

import (
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// VUPool 管理虚拟用户池。
// Requirements: 6.1, 6.3
type VUPool struct {
	maxVUs int
	vus    map[int]*types.VirtualUser
	inUse  map[int]bool
	mu     sync.Mutex
}

// NewVUPool 创建一个新的 VU 池。
func NewVUPool(maxVUs int) *VUPool {
	return &VUPool{
		maxVUs: maxVUs,
		vus:    make(map[int]*types.VirtualUser),
		inUse:  make(map[int]bool),
	}
}

// Acquire 从池中获取一个 VU。
// 如果池已耗尽则返回 nil。
func (p *VUPool) Acquire(id int) *types.VirtualUser {
	p.mu.Lock()
	defer p.mu.Unlock()

	if id >= p.maxVUs {
		return nil
	}

	if p.inUse[id] {
		return nil
	}

	vu, exists := p.vus[id]
	if !exists {
		vu = &types.VirtualUser{
			ID:        id,
			Iteration: 0,
			StartTime: time.Now(),
		}
		p.vus[id] = vu
	}

	p.inUse[id] = true
	vu.StartTime = time.Now()
	vu.Iteration = 0

	return vu
}

// Release 将 VU 释放回池中。
func (p *VUPool) Release(vu *types.VirtualUser) {
	if vu == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.inUse[vu.ID] = false
}

// ActiveCount 返回活跃 VU 的数量。
func (p *VUPool) ActiveCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := 0
	for _, inUse := range p.inUse {
		if inUse {
			count++
		}
	}
	return count
}

// StopAll 将所有 VU 标记为未使用状态。
func (p *VUPool) StopAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id := range p.inUse {
		p.inUse[id] = false
	}
}

// Reset 将池重置为初始状态。
func (p *VUPool) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.vus = make(map[int]*types.VirtualUser)
	p.inUse = make(map[int]bool)
}

// GetVU 根据 ID 获取 VU，但不获取其使用权。
func (p *VUPool) GetVU(id int) *types.VirtualUser {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.vus[id]
}

// IsInUse 检查指定 VU 是否正在使用中。
func (p *VUPool) IsInUse(id int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.inUse[id]
}
