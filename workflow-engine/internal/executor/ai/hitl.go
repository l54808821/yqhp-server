package ai

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/cloudwego/eino/compose"
)

// InMemoryCheckPointStore 内存版 CheckPointStore（开发/测试用，生产环境需替换为 Redis）
// 实现 core.CheckPointStore 接口：Get(ctx, id) ([]byte, bool, error) + Set(ctx, id, data) error
type InMemoryCheckPointStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func NewInMemoryCheckPointStore() compose.CheckPointStore {
	return &InMemoryCheckPointStore{
		data: make(map[string][]byte),
	}
}

func (s *InMemoryCheckPointStore) Get(_ context.Context, checkPointID string) ([]byte, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.data[checkPointID]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), data...), true, nil
}

func (s *InMemoryCheckPointStore) Set(_ context.Context, checkPointID string, checkPoint []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[checkPointID] = append([]byte(nil), checkPoint...)
	return nil
}

// InterruptData 中断时传递给前端的数据
type InterruptData struct {
	InterruptID string `json:"interrupt_id"`
	AgentName   string `json:"agent_name"`
	ToolName    string `json:"tool_name,omitempty"`
	Arguments   string `json:"arguments,omitempty"`
	Info        any    `json:"info,omitempty"`
}

func (d *InterruptData) Marshal() ([]byte, error) {
	return json.Marshal(d)
}

// ResumeData 前端提交的恢复数据
type ResumeData struct {
	InterruptID string `json:"interrupt_id"`
	Data        any    `json:"data"`
	Approved    bool   `json:"approved"`
	Reason      string `json:"reason,omitempty"`
}
