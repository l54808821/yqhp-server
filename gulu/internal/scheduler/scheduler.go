package scheduler

import (
	"context"
	"errors"
	"fmt"

	"yqhp/gulu/internal/client"
	"yqhp/gulu/internal/model"
)

// ExecutionMode 执行模式
type ExecutionMode string

const (
	ModeDebug   ExecutionMode = "debug"   // 调试模式，走 Master
	ModeExecute ExecutionMode = "execute" // 执行模式，走 Slave
)

// ScheduleRequest 调度请求
type ScheduleRequest struct {
	WorkflowID   int64
	WorkflowType string // normal, performance, data_generation
	Mode         ExecutionMode
	EnvID        int64
	ExecutorID   string // 仅执行模式需要
	SessionID    string // 调试会话ID
	UserID       int64
}

// ScheduleResult 调度结果
type ScheduleResult struct {
	ExecutionID string
	TargetType  string // master, slave
	TargetID    string
}

// SlaveInfo Slave 信息
type SlaveInfo struct {
	ID           string   `json:"id"`
	Address      string   `json:"address"`
	Capabilities []string `json:"capabilities"`
	State        string   `json:"state"`
	Load         float64  `json:"load"`
	ActiveTasks  int      `json:"active_tasks"`
}

// Scheduler 调度器接口
type Scheduler interface {
	// Schedule 调度执行任务
	Schedule(ctx context.Context, req *ScheduleRequest) (*ScheduleResult, error)

	// GetAvailableSlaves 获取可用的 Slave 列表
	GetAvailableSlaves(ctx context.Context) ([]*SlaveInfo, error)

	// CheckSlaveHealth 检查 Slave 健康状态
	CheckSlaveHealth(ctx context.Context, slaveID string) error

	// ValidateRequest 验证调度请求
	ValidateRequest(req *ScheduleRequest) error
}

// DefaultScheduler 默认调度器实现
type DefaultScheduler struct {
	engineClient *client.WorkflowEngineClient
}

// NewScheduler 创建调度器
func NewScheduler(engineClient *client.WorkflowEngineClient) *DefaultScheduler {
	return &DefaultScheduler{
		engineClient: engineClient,
	}
}

// ValidateRequest 验证调度请求
func (s *DefaultScheduler) ValidateRequest(req *ScheduleRequest) error {
	if req.WorkflowID <= 0 {
		return errors.New("工作流ID无效")
	}

	if req.Mode == "" {
		return errors.New("执行模式不能为空")
	}

	if req.Mode != ModeDebug && req.Mode != ModeExecute {
		return fmt.Errorf("无效的执行模式: %s", req.Mode)
	}

	// 验证工作流类型
	validTypes := map[string]bool{
		string(model.WorkflowTypeNormal):         true,
		string(model.WorkflowTypePerformance):    true,
		string(model.WorkflowTypeDataGeneration): true,
	}
	if !validTypes[req.WorkflowType] {
		return fmt.Errorf("无效的工作流类型: %s", req.WorkflowType)
	}

	// 执行模式下，普通流程不支持
	if req.Mode == ModeExecute && req.WorkflowType == string(model.WorkflowTypeNormal) {
		return errors.New("普通流程仅支持调试模式")
	}

	// 执行模式下需要指定执行器
	if req.Mode == ModeExecute && req.ExecutorID == "" {
		return errors.New("执行模式需要指定执行器")
	}

	// 调试模式需要会话ID
	if req.Mode == ModeDebug && req.SessionID == "" {
		return errors.New("调试模式需要会话ID")
	}

	return nil
}

// Schedule 实现调度逻辑
func (s *DefaultScheduler) Schedule(ctx context.Context, req *ScheduleRequest) (*ScheduleResult, error) {
	// 验证请求
	if err := s.ValidateRequest(req); err != nil {
		return nil, err
	}

	// 调试模式：所有类型都走 Master
	if req.Mode == ModeDebug {
		return s.scheduleToMaster(ctx, req)
	}

	// 执行模式：压测和造数走 Slave
	return s.scheduleToSlave(ctx, req)
}

// scheduleToMaster 调度到 Master 执行器
func (s *DefaultScheduler) scheduleToMaster(ctx context.Context, req *ScheduleRequest) (*ScheduleResult, error) {
	return &ScheduleResult{
		ExecutionID: req.SessionID, // 调试模式使用会话ID作为执行ID
		TargetType:  "master",
		TargetID:    "embedded",
	}, nil
}

// scheduleToSlave 调度到 Slave 执行器
func (s *DefaultScheduler) scheduleToSlave(ctx context.Context, req *ScheduleRequest) (*ScheduleResult, error) {
	// 检查 Slave 健康状态
	if err := s.CheckSlaveHealth(ctx, req.ExecutorID); err != nil {
		return nil, fmt.Errorf("执行器不可用: %w", err)
	}

	return &ScheduleResult{
		ExecutionID: "", // 执行ID由 Slave 返回
		TargetType:  "slave",
		TargetID:    req.ExecutorID,
	}, nil
}

// GetAvailableSlaves 获取可用的 Slave 列表
func (s *DefaultScheduler) GetAvailableSlaves(ctx context.Context) ([]*SlaveInfo, error) {
	executors, err := s.engineClient.GetExecutorList()
	if err != nil {
		return nil, err
	}

	result := make([]*SlaveInfo, 0, len(executors))
	for _, exec := range executors {
		if exec.State == "online" {
			result = append(result, &SlaveInfo{
				ID:           exec.SlaveID,
				Address:      exec.Address,
				Capabilities: exec.Capabilities,
				State:        exec.State,
				Load:         exec.Load,
				ActiveTasks:  exec.ActiveTasks,
			})
		}
	}

	return result, nil
}

// CheckSlaveHealth 检查 Slave 健康状态
func (s *DefaultScheduler) CheckSlaveHealth(ctx context.Context, slaveID string) error {
	status, err := s.engineClient.GetExecutorStatus(slaveID)
	if err != nil {
		return err
	}

	if status.State != "online" {
		return fmt.Errorf("执行器状态异常: %s", status.State)
	}

	return nil
}

// CanExecute 检查工作流类型是否支持执行模式
func CanExecute(workflowType string) bool {
	return workflowType == string(model.WorkflowTypePerformance) ||
		workflowType == string(model.WorkflowTypeDataGeneration)
}

// CanDebug 检查工作流类型是否支持调试模式（所有类型都支持）
func CanDebug(workflowType string) bool {
	return true
}
