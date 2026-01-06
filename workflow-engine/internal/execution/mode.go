package execution

import (
	"context"
	"sync"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

// Mode 定义执行模式的接口。
// 每种模式控制 VU 的管理方式和迭代的执行方式。
type Mode interface {
	// Name 返回执行模式的名称。
	Name() types.ExecutionMode

	// Run 使用给定配置启动执行模式。
	// 阻塞直到执行完成或上下文被取消。
	Run(ctx context.Context, config *ModeConfig) error

	// Stop 优雅地停止执行模式。
	Stop(ctx context.Context) error

	// GetState 返回当前执行状态。
	GetState() *ModeState
}

// ModeConfig 包含执行模式的配置。
type ModeConfig struct {
	// VUs 是虚拟用户数量（用于基于 VU 的模式）。
	VUs int

	// Duration 是总执行时长。
	Duration time.Duration

	// Iterations 是总迭代次数。
	Iterations int

	// Stages 定义执行阶段（用于递增模式）。
	Stages []types.Stage

	// Rate 是请求速率（用于到达率模式）。
	Rate int

	// TimeUnit 是速率计算的时间单位。
	TimeUnit time.Duration

	// PreAllocatedVUs 是预分配的 VU 数量（用于到达率模式）。
	PreAllocatedVUs int

	// MaxVUs 是最大 VU 数量（用于到达率模式）。
	MaxVUs int

	// GracefulStop 是等待优雅关闭的时长。
	GracefulStop time.Duration

	// IterationFunc 是每次迭代执行的函数。
	IterationFunc IterationFunc

	// OnVUStart 在 VU 启动时调用。
	OnVUStart func(vuID int)

	// OnVUStop 在 VU 停止时调用。
	OnVUStop func(vuID int)

	// OnIterationComplete 在迭代完成时调用。
	OnIterationComplete func(vuID int, iteration int, duration time.Duration, err error)
}

// IterationFunc 是执行单次迭代的函数签名。
type IterationFunc func(ctx context.Context, vuID int, iteration int) error

// ModeState 表示执行模式的当前状态。
type ModeState struct {
	// ActiveVUs 是当前活跃的 VU 数量。
	ActiveVUs int

	// TargetVUs 是目标 VU 数量。
	TargetVUs int

	// CompletedIterations 是已完成的迭代次数。
	CompletedIterations int64

	// CurrentRate 是当前请求速率（用于到达率模式）。
	CurrentRate float64

	// Running 表示模式是否正在运行。
	Running bool

	// Paused 表示模式是否已暂停。
	Paused bool

	// StartTime 是执行开始时间。
	StartTime time.Time

	// ElapsedTime 是已执行时长。
	ElapsedTime time.Duration
}

// BaseMode 为执行模式提供通用功能。
type BaseMode struct {
	name    types.ExecutionMode
	state   ModeState
	stateMu sync.RWMutex
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewBaseMode 创建一个新的基础模式。
func NewBaseMode(name types.ExecutionMode) *BaseMode {
	return &BaseMode{
		name:   name,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Name 返回模式名称。
func (b *BaseMode) Name() types.ExecutionMode {
	return b.name
}

// GetState 返回当前状态。
func (b *BaseMode) GetState() *ModeState {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	state := b.state
	return &state
}

// SetState 更新状态。
func (b *BaseMode) SetState(fn func(*ModeState)) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	fn(&b.state)
}

// IsStopped 如果已请求停止则返回 true。
func (b *BaseMode) IsStopped() bool {
	select {
	case <-b.stopCh:
		return true
	default:
		return false
	}
}

// RequestStop 发送停止信号。
func (b *BaseMode) RequestStop() {
	select {
	case <-b.stopCh:
		// 已停止
	default:
		close(b.stopCh)
	}
}

// SignalDone 发送完成信号。
func (b *BaseMode) SignalDone() {
	select {
	case <-b.doneCh:
		// 已完成
	default:
		close(b.doneCh)
	}
}

// WaitDone 等待模式完成。
func (b *BaseMode) WaitDone(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-b.doneCh:
		return nil
	}
}
