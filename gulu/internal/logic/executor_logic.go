package logic

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"yqhp/gulu/internal/client"
	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

// ExecutorLogic 执行机逻辑
type ExecutorLogic struct {
	ctx context.Context
}

// NewExecutorLogic 创建执行机逻辑
func NewExecutorLogic(ctx context.Context) *ExecutorLogic {
	return &ExecutorLogic{ctx: ctx}
}

// CreateExecutorReq 创建执行机请求
type CreateExecutorReq struct {
	SlaveID     string            `json:"slave_id" validate:"required,max=100"`
	Name        string            `json:"name" validate:"required,max=100"`
	Type        string            `json:"type" validate:"required,max=50"` // performance, normal, debug
	Description string            `json:"description" validate:"max=500"`
	Labels      map[string]string `json:"labels"`
	MaxVUs      int32             `json:"max_vus"`
	Priority    int32             `json:"priority"`
	Status      int32             `json:"status"`
}

// UpdateExecutorReq 更新执行机请求
type UpdateExecutorReq struct {
	Name        string            `json:"name" validate:"max=100"`
	Type        string            `json:"type" validate:"max=50"`
	Description string            `json:"description" validate:"max=500"`
	Labels      map[string]string `json:"labels"`
	MaxVUs      int32             `json:"max_vus"`
	Priority    int32             `json:"priority"`
	Status      int32             `json:"status"`
}

// ExecutorListReq 执行机列表请求
type ExecutorListReq struct {
	Page     int               `query:"page" validate:"min=1"`
	PageSize int               `query:"pageSize" validate:"min=1,max=100"`
	Name     string            `query:"name"`
	Type     string            `query:"type"`
	Status   *int32            `query:"status"`
	Labels   map[string]string `query:"labels"` // 标签筛选
}

// ExecutorInfo 执行机完整信息（合并持久化配置和运行时状态）
type ExecutorInfo struct {
	// 持久化配置
	ID          int64             `json:"id"`
	SlaveID     string            `json:"slave_id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
	MaxVUs      int32             `json:"max_vus"`
	Priority    int32             `json:"priority"`
	Status      int32             `json:"status"`
	CreatedAt   *time.Time        `json:"created_at"`
	UpdatedAt   *time.Time        `json:"updated_at"`

	// 运行时状态
	Address      string    `json:"address"`
	Capabilities []string  `json:"capabilities"`
	State        string    `json:"state"`
	Load         float64   `json:"load"`
	ActiveTasks  int       `json:"active_tasks"`
	CurrentVUs   int       `json:"current_vus"`
	LastSeen     time.Time `json:"last_seen"`
	IsOnline     bool      `json:"is_online"`
}

// Create 创建执行机
func (l *ExecutorLogic) Create(req *CreateExecutorReq) (*model.TExecutor, error) {
	// 验证类型
	if !isValidExecutorType(req.Type) {
		return nil, errors.New("无效的执行机类型，必须是 performance、normal 或 debug")
	}

	// 检查 SlaveID 是否已存在
	exists, err := l.CheckSlaveIDExists(req.SlaveID, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("执行机 SlaveID 已存在")
	}

	now := time.Now()
	isDelete := false
	status := req.Status
	if status == 0 {
		status = 1 // 默认启用
	}

	// 序列化 Labels
	var labelsJSON *string
	if len(req.Labels) > 0 {
		labelsBytes, err := json.Marshal(req.Labels)
		if err != nil {
			return nil, errors.New("标签序列化失败")
		}
		labelsStr := string(labelsBytes)
		labelsJSON = &labelsStr
	}

	executor := &model.TExecutor{
		CreatedAt:   &now,
		UpdatedAt:   &now,
		IsDelete:    &isDelete,
		SlaveID:     req.SlaveID,
		Name:        req.Name,
		Type:        req.Type,
		Description: &req.Description,
		Labels:      labelsJSON,
		MaxVus:      &req.MaxVUs,
		Priority:    &req.Priority,
		Status:      &status,
	}

	q := query.Use(svc.Ctx.DB)
	err = q.TExecutor.WithContext(l.ctx).Create(executor)
	if err != nil {
		return nil, err
	}

	return executor, nil
}

// Update 更新执行机
func (l *ExecutorLogic) Update(id int64, req *UpdateExecutorReq) error {
	// 验证类型
	if req.Type != "" && !isValidExecutorType(req.Type) {
		return errors.New("无效的执行机类型，必须是 performance、normal 或 debug")
	}

	q := query.Use(svc.Ctx.DB)
	e := q.TExecutor

	// 检查执行机是否存在
	_, err := e.WithContext(l.ctx).Where(e.ID.Eq(id), e.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("执行机不存在")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if len(req.Labels) > 0 {
		labelsBytes, err := json.Marshal(req.Labels)
		if err != nil {
			return errors.New("标签序列化失败")
		}
		updates["labels"] = string(labelsBytes)
	}
	updates["max_vus"] = req.MaxVUs
	updates["priority"] = req.Priority
	updates["status"] = req.Status

	_, err = e.WithContext(l.ctx).Where(e.ID.Eq(id)).Updates(updates)
	return err
}

// Delete 删除执行机（软删除）
func (l *ExecutorLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	e := q.TExecutor

	isDelete := true
	_, err := e.WithContext(l.ctx).Where(e.ID.Eq(id)).Update(e.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取执行机（含运行时状态）
func (l *ExecutorLogic) GetByID(id int64) (*ExecutorInfo, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TExecutor

	executor, err := e.WithContext(l.ctx).Where(e.ID.Eq(id), e.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}

	return l.mergeWithRuntimeStatus(executor), nil
}

// List 获取执行机列表（含运行时状态）
func (l *ExecutorLogic) List(req *ExecutorListReq) ([]*ExecutorInfo, int64, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TExecutor

	// 构建查询条件
	queryBuilder := e.WithContext(l.ctx).Where(e.IsDelete.Is(false))

	if req.Name != "" {
		queryBuilder = queryBuilder.Where(e.Name.Like("%" + req.Name + "%"))
	}
	if req.Type != "" {
		queryBuilder = queryBuilder.Where(e.Type.Eq(req.Type))
	}
	if req.Status != nil {
		queryBuilder = queryBuilder.Where(e.Status.Eq(*req.Status))
	}

	// 获取总数
	total, err := queryBuilder.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	offset := (req.Page - 1) * req.PageSize
	list, err := queryBuilder.Order(e.Priority.Desc(), e.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	// 合并运行时状态并进行标签筛选
	result := make([]*ExecutorInfo, 0, len(list))
	for _, executor := range list {
		info := l.mergeWithRuntimeStatus(executor)
		// 标签筛选
		if len(req.Labels) > 0 && !matchLabels(info.Labels, req.Labels) {
			continue
		}
		result = append(result, info)
	}

	return result, total, nil
}

// ListByLabels 根据标签筛选执行机
func (l *ExecutorLogic) ListByLabels(labels map[string]string) ([]*ExecutorInfo, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TExecutor

	status := int32(1)
	list, err := e.WithContext(l.ctx).Where(e.IsDelete.Is(false), e.Status.Eq(status)).Order(e.Priority.Desc()).Find()
	if err != nil {
		return nil, err
	}

	result := make([]*ExecutorInfo, 0)
	for _, executor := range list {
		info := l.mergeWithRuntimeStatus(executor)
		if matchLabels(info.Labels, labels) {
			result = append(result, info)
		}
	}

	return result, nil
}

// Sync 同步 workflow-engine 的执行机列表
func (l *ExecutorLogic) Sync() (int, error) {
	weClient := client.NewWorkflowEngineClient()
	executorStatuses, err := weClient.GetExecutorList()
	if err != nil {
		return 0, err
	}

	syncCount := 0
	for _, status := range executorStatuses {
		// 检查是否已存在
		exists, err := l.CheckSlaveIDExists(status.SlaveID, 0)
		if err != nil {
			continue
		}

		if !exists {
			// 创建新执行机
			now := time.Now()
			isDelete := false
			defaultStatus := int32(1)
			defaultPriority := int32(0)

			executor := &model.TExecutor{
				CreatedAt: &now,
				UpdatedAt: &now,
				IsDelete:  &isDelete,
				SlaveID:   status.SlaveID,
				Name:      status.SlaveID, // 默认使用 SlaveID 作为名称
				Type:      "normal",       // 默认类型
				Status:    &defaultStatus,
				Priority:  &defaultPriority,
			}

			q := query.Use(svc.Ctx.DB)
			if err := q.TExecutor.WithContext(l.ctx).Create(executor); err == nil {
				syncCount++
			}
		}
	}

	return syncCount, nil
}

// UpdateStatus 更新执行机状态
func (l *ExecutorLogic) UpdateStatus(id int64, status int32) error {
	q := query.Use(svc.Ctx.DB)
	e := q.TExecutor

	now := time.Now()
	_, err := e.WithContext(l.ctx).Where(e.ID.Eq(id), e.IsDelete.Is(false)).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": now,
	})
	return err
}

// CheckSlaveIDExists 检查 SlaveID 是否存在
func (l *ExecutorLogic) CheckSlaveIDExists(slaveID string, excludeID int64) (bool, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TExecutor

	queryBuilder := e.WithContext(l.ctx).Where(e.SlaveID.Eq(slaveID), e.IsDelete.Is(false))
	if excludeID > 0 {
		queryBuilder = queryBuilder.Where(e.ID.Neq(excludeID))
	}

	count, err := queryBuilder.Count()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// mergeWithRuntimeStatus 合并持久化配置和运行时状态
func (l *ExecutorLogic) mergeWithRuntimeStatus(executor *model.TExecutor) *ExecutorInfo {
	info := &ExecutorInfo{
		ID:        executor.ID,
		SlaveID:   executor.SlaveID,
		Name:      executor.Name,
		Type:      executor.Type,
		CreatedAt: executor.CreatedAt,
		UpdatedAt: executor.UpdatedAt,
	}

	if executor.Description != nil {
		info.Description = *executor.Description
	}
	if executor.MaxVus != nil {
		info.MaxVUs = *executor.MaxVus
	}
	if executor.Priority != nil {
		info.Priority = *executor.Priority
	}
	if executor.Status != nil {
		info.Status = *executor.Status
	}

	// 解析 Labels
	if executor.Labels != nil && *executor.Labels != "" {
		var labels map[string]string
		if err := json.Unmarshal([]byte(*executor.Labels), &labels); err == nil {
			info.Labels = labels
		}
	}

	// 获取运行时状态
	weClient := client.NewWorkflowEngineClient()
	status, err := weClient.GetExecutorStatus(executor.SlaveID)
	if err == nil && status != nil {
		info.Address = status.Address
		info.Capabilities = status.Capabilities
		info.State = status.State
		info.Load = status.Load
		info.ActiveTasks = status.ActiveTasks
		info.CurrentVUs = status.CurrentVUs
		info.LastSeen = status.LastSeen
		info.IsOnline = status.State == "online" || status.State == "busy"
	} else {
		info.State = "offline"
		info.IsOnline = false
	}

	return info
}

// matchLabels 检查执行机标签是否匹配筛选条件
func matchLabels(executorLabels, filterLabels map[string]string) bool {
	if len(filterLabels) == 0 {
		return true
	}
	if len(executorLabels) == 0 {
		return false
	}

	for key, value := range filterLabels {
		if executorLabels[key] != value {
			return false
		}
	}
	return true
}

// MatchLabels 导出的标签匹配函数（用于测试）
func MatchLabels(executorLabels, filterLabels map[string]string) bool {
	return matchLabels(executorLabels, filterLabels)
}

// isValidExecutorType 验证执行机类型
func isValidExecutorType(t string) bool {
	return t == "performance" || t == "normal" || t == "debug"
}

// findOrphanedExecutor 查找孤儿执行机记录
// 孤儿记录 = 数据库中存在但运行时状态为离线的记录（Slave 重启生成了新 ID，旧记录已离线）
func (l *ExecutorLogic) findOrphanedExecutor() *model.TExecutor {
	q := query.Use(svc.Ctx.DB)
	e := q.TExecutor
	allExecutors, err := e.WithContext(l.ctx).Where(e.IsDelete.Is(false)).Order(e.UpdatedAt.Desc()).Find()
	if err != nil {
		return nil
	}

	for _, ex := range allExecutors {
		info := l.mergeWithRuntimeStatus(ex)
		if !info.IsOnline {
			return ex
		}
	}
	return nil
}

// RegisterExecutorReq 执行机注册请求（自动注册 / 手动简化注册）
type RegisterExecutorReq struct {
	SlaveID      string            `json:"slave_id" validate:"required"`
	Name         string            `json:"name"`
	Address      string            `json:"address"`
	Type         string            `json:"type"`
	Capabilities []string          `json:"capabilities"`
	Labels       map[string]string `json:"labels"`
}

// Register 注册执行机（已存在则更新状态，不存在则自动创建）
func (l *ExecutorLogic) Register(req *RegisterExecutorReq) (*ExecutorInfo, error) {
	if req.SlaveID == "" {
		return nil, errors.New("SlaveID 不能为空")
	}

	q := query.Use(svc.Ctx.DB)
	e := q.TExecutor

	// 1. 按 slave_id 精确匹配
	existing, err := e.WithContext(l.ctx).Where(e.SlaveID.Eq(req.SlaveID), e.IsDelete.Is(false)).First()

	if err != nil {
		// 2. slave_id 没匹配到，查找孤儿记录（slave_id 已不在引擎中的旧记录）
		// 这处理了 Slave 重启后生成新 ID 的情况
		existing = l.findOrphanedExecutor()
	}

	if existing != nil {
		// 已存在：更新 slave_id 和其他信息
		now := time.Now()
		updates := map[string]interface{}{
			"updated_at": now,
			"slave_id":   req.SlaveID,
		}
		if req.Name != "" {
			updates["name"] = req.Name
		}
		if req.Type != "" && req.Type != existing.Type {
			updates["type"] = req.Type
		}
		if len(req.Labels) > 0 {
			labelsBytes, _ := json.Marshal(req.Labels)
			updates["labels"] = string(labelsBytes)
		}
		e.WithContext(l.ctx).Where(e.ID.Eq(existing.ID)).Updates(updates)
		existing.SlaveID = req.SlaveID
		return l.mergeWithRuntimeStatus(existing), nil
	}

	// 不存在：自动创建
	name := req.Name
	if name == "" {
		name = req.SlaveID
	}
	executorType := req.Type
	if executorType == "" {
		executorType = "normal"
	}

	now := time.Now()
	isDelete := false
	defaultStatus := int32(1)
	defaultPriority := int32(0)

	var labelsJSON *string
	if len(req.Labels) > 0 {
		labelsBytes, _ := json.Marshal(req.Labels)
		s := string(labelsBytes)
		labelsJSON = &s
	}

	executor := &model.TExecutor{
		CreatedAt: &now,
		UpdatedAt: &now,
		IsDelete:  &isDelete,
		SlaveID:   req.SlaveID,
		Name:      name,
		Type:      executorType,
		Labels:    labelsJSON,
		Status:    &defaultStatus,
		Priority:  &defaultPriority,
	}

	if err := q.TExecutor.WithContext(l.ctx).Create(executor); err != nil {
		return nil, err
	}

	return l.mergeWithRuntimeStatus(executor), nil
}

// ListAvailable 获取可用的执行机列表（启用且在线的，带运行时状态）
func (l *ExecutorLogic) ListAvailable() ([]*ExecutorInfo, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TExecutor

	status := int32(1)
	list, err := e.WithContext(l.ctx).Where(e.IsDelete.Is(false), e.Status.Eq(status)).Order(e.Priority.Desc()).Find()
	if err != nil {
		return nil, err
	}

	result := make([]*ExecutorInfo, 0)
	for _, executor := range list {
		info := l.mergeWithRuntimeStatus(executor)
		result = append(result, info)
	}

	return result, nil
}

// ExecutorStrategy 执行策略
type ExecutorStrategy struct {
	Strategy   string            `json:"strategy"`    // local, manual, auto
	ExecutorID int64             `json:"executor_id"` // manual 模式下指定的执行机 ID
	Labels     map[string]string `json:"labels"`      // auto 模式下的标签匹配
}

// SelectByStrategy 根据策略选择执行机
func (l *ExecutorLogic) SelectByStrategy(strategy *ExecutorStrategy) (*ExecutorInfo, error) {
	if strategy == nil {
		return nil, nil
	}

	switch strategy.Strategy {
	case "local", "":
		return nil, nil
	case "manual":
		if strategy.ExecutorID <= 0 {
			return nil, errors.New("手动模式下必须指定执行机 ID")
		}
		info, err := l.GetByID(strategy.ExecutorID)
		if err != nil {
			return nil, errors.New("指定的执行机不存在")
		}
		if info.Status != 1 {
			return nil, errors.New("指定的执行机已禁用")
		}
		if info.State != "online" && info.State != "busy" {
			return nil, errors.New("指定的执行机不在线")
		}
		return info, nil
	case "auto":
		return l.selectBestExecutor(strategy.Labels)
	default:
		return nil, nil
	}
}

// selectBestExecutor 自动选择最优执行机（负载最低、标签匹配的在线执行机）
func (l *ExecutorLogic) selectBestExecutor(labels map[string]string) (*ExecutorInfo, error) {
	available, err := l.ListAvailable()
	if err != nil {
		return nil, err
	}

	var best *ExecutorInfo
	for _, info := range available {
		if info.State != "online" && info.State != "busy" {
			continue
		}
		if len(labels) > 0 && !matchLabels(info.Labels, labels) {
			continue
		}
		if best == nil || info.Load < best.Load {
			best = info
		}
	}

	if best == nil {
		return nil, errors.New("没有可用的执行机")
	}

	return best, nil
}
