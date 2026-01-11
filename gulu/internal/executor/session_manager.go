package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"yqhp/gulu/internal/sse"
)

// SessionStatus 会话状态
type SessionStatus string

const (
	SessionStatusRunning   SessionStatus = "running"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusStopped   SessionStatus = "stopped"
	SessionStatusWaiting   SessionStatus = "waiting_interaction"
)

// InteractionResponse 交互响应
type InteractionResponse struct {
	Value   string `json:"value"`
	Skipped bool   `json:"skipped"`
}

// Session 执行会话
type Session struct {
	ID            string
	WorkflowID    int64
	Status        SessionStatus
	StartTime     time.Time
	Cancel        context.CancelFunc
	SSEWriter     *sse.Writer
	InteractionCh chan *InteractionResponse

	// 执行统计
	TotalSteps   int
	SuccessSteps int
	FailedSteps  int

	mu sync.Mutex
}

// SessionManager 会话管理器
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewSessionManager 创建会话管理器
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

// CreateSession 创建会话
func (m *SessionManager) CreateSession(workflowID int64, writer *sse.Writer) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionID := writer.SessionID()
	if sessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	// 检查是否已存在
	if _, exists := m.sessions[sessionID]; exists {
		return nil, fmt.Errorf("session already exists: %s", sessionID)
	}

	session := &Session{
		ID:            sessionID,
		WorkflowID:    workflowID,
		Status:        SessionStatusRunning,
		StartTime:     time.Now(),
		SSEWriter:     writer,
		InteractionCh: make(chan *InteractionResponse, 1),
	}

	m.sessions[sessionID] = session
	return session, nil
}

// GetSession 获取会话
func (m *SessionManager) GetSession(sessionID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[sessionID]
	return session, ok
}

// CleanupSession 清理会话
func (m *SessionManager) CleanupSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[sessionID]; ok {
		// 关闭交互通道
		close(session.InteractionCh)
		// 关闭 SSE Writer
		if session.SSEWriter != nil {
			session.SSEWriter.Close()
		}
		delete(m.sessions, sessionID)
	}
}

// SubmitInteraction 提交交互响应
func (m *SessionManager) SubmitInteraction(sessionID string, response *InteractionResponse) error {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	if session.Status != SessionStatusWaiting {
		session.mu.Unlock()
		return fmt.Errorf("session is not waiting for interaction")
	}
	session.mu.Unlock()

	// 非阻塞发送
	select {
	case session.InteractionCh <- response:
		return nil
	default:
		return fmt.Errorf("interaction channel is full")
	}
}

// UpdateStatus 更新会话状态
func (m *SessionManager) UpdateStatus(sessionID string, status SessionStatus) error {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	session.Status = status
	session.mu.Unlock()
	return nil
}

// StopSession 停止会话
func (m *SessionManager) StopSession(sessionID string) error {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	session.Status = SessionStatusStopped
	if session.Cancel != nil {
		session.Cancel()
	}
	session.mu.Unlock()
	return nil
}

// SetCancel 设置取消函数
func (m *SessionManager) SetCancel(sessionID string, cancel context.CancelFunc) error {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	session.Cancel = cancel
	session.mu.Unlock()
	return nil
}

// GetActiveSessions 获取活跃会话数
func (m *SessionManager) GetActiveSessions() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, session := range m.sessions {
		session.mu.Lock()
		if session.Status == SessionStatusRunning || session.Status == SessionStatusWaiting {
			count++
		}
		session.mu.Unlock()
	}
	return count
}

// GetSessionsByWorkflow 获取工作流的所有会话
func (m *SessionManager) GetSessionsByWorkflow(workflowID int64) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*Session
	for _, session := range m.sessions {
		if session.WorkflowID == workflowID {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// IsRunning 检查会话是否在运行
func (m *SessionManager) IsRunning(sessionID string) bool {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return false
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	return session.Status == SessionStatusRunning || session.Status == SessionStatusWaiting
}

// ============ Session 方法 ============

// IncrementSuccess 增加成功步骤计数
func (s *Session) IncrementSuccess() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalSteps++
	s.SuccessSteps++
}

// IncrementFailed 增加失败步骤计数
func (s *Session) IncrementFailed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalSteps++
	s.FailedSteps++
}

// GetStats 获取统计信息
func (s *Session) GetStats() (total, success, failed int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.TotalSteps, s.SuccessSteps, s.FailedSteps
}

// SetStatus 设置状态
func (s *Session) SetStatus(status SessionStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
}

// GetStatus 获取状态
func (s *Session) GetStatus() SessionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Status
}

// WaitForInteraction 等待交互响应
func (s *Session) WaitForInteraction(ctx context.Context, timeout time.Duration) (*InteractionResponse, error) {
	s.mu.Lock()
	s.Status = SessionStatusWaiting
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if s.Status == SessionStatusWaiting {
			s.Status = SessionStatusRunning
		}
		s.mu.Unlock()
	}()

	var timeoutCh <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timeoutCh:
		return &InteractionResponse{Skipped: true}, nil
	case resp := <-s.InteractionCh:
		if resp == nil {
			return nil, fmt.Errorf("interaction channel closed")
		}
		return resp, nil
	}
}
