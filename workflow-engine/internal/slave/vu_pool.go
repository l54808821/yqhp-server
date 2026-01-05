package slave

import (
	"sync"
	"time"

	"github.com/grafana/k6/workflow-engine/pkg/types"
)

// VUPool manages a pool of virtual users.
// Requirements: 6.1, 6.3
type VUPool struct {
	maxVUs int
	vus    map[int]*types.VirtualUser
	inUse  map[int]bool
	mu     sync.Mutex
}

// NewVUPool creates a new VU pool.
func NewVUPool(maxVUs int) *VUPool {
	return &VUPool{
		maxVUs: maxVUs,
		vus:    make(map[int]*types.VirtualUser),
		inUse:  make(map[int]bool),
	}
}

// Acquire acquires a VU from the pool.
// Returns nil if the pool is exhausted.
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

// Release releases a VU back to the pool.
func (p *VUPool) Release(vu *types.VirtualUser) {
	if vu == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.inUse[vu.ID] = false
}

// ActiveCount returns the number of active VUs.
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

// StopAll marks all VUs as not in use.
func (p *VUPool) StopAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id := range p.inUse {
		p.inUse[id] = false
	}
}

// Reset resets the pool to its initial state.
func (p *VUPool) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.vus = make(map[int]*types.VirtualUser)
	p.inUse = make(map[int]bool)
}

// GetVU returns a VU by ID without acquiring it.
func (p *VUPool) GetVU(id int) *types.VirtualUser {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.vus[id]
}

// IsInUse checks if a VU is currently in use.
func (p *VUPool) IsInUse(id int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.inUse[id]
}
