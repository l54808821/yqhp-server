package logic

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

// McpServerLogic MCP服务器逻辑
type McpServerLogic struct {
	ctx context.Context
}

// NewMcpServerLogic 创建MCP服务器逻辑
func NewMcpServerLogic(ctx context.Context) *McpServerLogic {
	return &McpServerLogic{ctx: ctx}
}

// CreateMcpServerReq 创建MCP服务器请求
type CreateMcpServerReq struct {
	Name        string            `json:"name" validate:"required,max=100"`
	Description string            `json:"description" validate:"max=500"`
	Transport   string            `json:"transport" validate:"required,oneof=stdio sse"`
	Command     string            `json:"command" validate:"max=500"`
	Args        []string          `json:"args"`
	URL         string            `json:"url" validate:"max=500"`
	Env         map[string]string `json:"env"`
	Timeout     int32             `json:"timeout"`
	Sort        int32             `json:"sort"`
	Status      int32             `json:"status"`
}

// UpdateMcpServerReq 更新MCP服务器请求
type UpdateMcpServerReq struct {
	Name        string            `json:"name" validate:"max=100"`
	Description string            `json:"description" validate:"max=500"`
	Transport   string            `json:"transport" validate:"oneof=stdio sse"`
	Command     string            `json:"command" validate:"max=500"`
	Args        []string          `json:"args"`
	URL         string            `json:"url" validate:"max=500"`
	Env         map[string]string `json:"env"`
	Timeout     int32             `json:"timeout"`
	Sort        int32             `json:"sort"`
	Status      int32             `json:"status"`
}

// McpServerListReq MCP服务器列表请求
type McpServerListReq struct {
	Page      int    `query:"page" validate:"min=1"`
	PageSize  int    `query:"pageSize" validate:"min=1,max=100"`
	Name      string `query:"name"`
	Transport string `query:"transport"`
	Status    *int32 `query:"status"`
}

// McpServerInfo MCP服务器返回信息
type McpServerInfo struct {
	ID          int64             `json:"id"`
	CreatedAt   *time.Time        `json:"created_at"`
	UpdatedAt   *time.Time        `json:"updated_at"`
	CreatedBy   *int64            `json:"created_by"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Transport   string            `json:"transport"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	URL         string            `json:"url"`
	Env         map[string]string `json:"env"`
	Timeout     int32             `json:"timeout"`
	Sort        int32             `json:"sort"`
	Status      int32             `json:"status"`
}

// Create 创建MCP服务器
func (l *McpServerLogic) Create(req *CreateMcpServerReq) (*McpServerInfo, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TMcpServer

	// 验证名称唯一性
	existing, err := m.WithContext(l.ctx).Where(m.Name.Eq(req.Name), m.IsDelete.Is(false)).First()
	if err == nil && existing != nil {
		return nil, errors.New("MCP服务器名称已存在")
	}

	now := time.Now()
	isDelete := false
	status := req.Status
	if status == 0 {
		status = 1 // 默认启用
	}
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 30 // 默认超时30秒
	}

	// 序列化 Args
	var argsJSON *string
	if len(req.Args) > 0 {
		b, err := json.Marshal(req.Args)
		if err != nil {
			return nil, errors.New("参数序列化失败")
		}
		s := string(b)
		argsJSON = &s
	}

	// 序列化 Env
	var envJSON *string
	if len(req.Env) > 0 {
		b, err := json.Marshal(req.Env)
		if err != nil {
			return nil, errors.New("环境变量序列化失败")
		}
		s := string(b)
		envJSON = &s
	}

	mcpServer := &model.TMcpServer{
		CreatedAt:   &now,
		UpdatedAt:   &now,
		IsDelete:    &isDelete,
		Name:        req.Name,
		Description: &req.Description,
		Transport:   req.Transport,
		Command:     &req.Command,
		Args:        argsJSON,
		URL:         &req.URL,
		Env:         envJSON,
		Timeout:     &timeout,
		Sort:        &req.Sort,
		Status:      &status,
	}

	err = q.TMcpServer.WithContext(l.ctx).Create(mcpServer)
	if err != nil {
		return nil, err
	}

	return l.toMcpServerInfo(mcpServer), nil
}

// Update 更新MCP服务器
func (l *McpServerLogic) Update(id int64, req *UpdateMcpServerReq) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TMcpServer

	// 检查是否存在
	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("MCP服务器不存在")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	updates["description"] = req.Description
	if req.Transport != "" {
		updates["transport"] = req.Transport
	}
	updates["command"] = req.Command
	updates["url"] = req.URL
	updates["timeout"] = req.Timeout
	updates["sort"] = req.Sort
	updates["status"] = req.Status

	// 序列化 Args
	if req.Args != nil {
		b, _ := json.Marshal(req.Args)
		updates["args"] = string(b)
	}

	// 序列化 Env
	if req.Env != nil {
		b, _ := json.Marshal(req.Env)
		updates["env"] = string(b)
	}

	_, err = m.WithContext(l.ctx).Where(m.ID.Eq(id)).Updates(updates)
	return err
}

// Delete 删除MCP服务器（软删除）
func (l *McpServerLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TMcpServer

	isDelete := true
	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(id)).Update(m.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取MCP服务器
func (l *McpServerLogic) GetByID(id int64) (*McpServerInfo, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TMcpServer

	mcpServer, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}

	return l.toMcpServerInfo(mcpServer), nil
}

// List 获取MCP服务器列表
func (l *McpServerLogic) List(req *McpServerListReq) ([]*McpServerInfo, int64, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TMcpServer

	queryBuilder := m.WithContext(l.ctx).Where(m.IsDelete.Is(false))

	if req.Name != "" {
		queryBuilder = queryBuilder.Where(m.Name.Like("%" + req.Name + "%"))
	}
	if req.Transport != "" {
		queryBuilder = queryBuilder.Where(m.Transport.Eq(req.Transport))
	}
	if req.Status != nil {
		queryBuilder = queryBuilder.Where(m.Status.Eq(*req.Status))
	}

	// 获取总数
	total, err := queryBuilder.Count()
	if err != nil {
		return nil, 0, err
	}

	// 分页
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	offset := (req.Page - 1) * req.PageSize
	list, err := queryBuilder.Order(m.Sort.Desc(), m.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	result := make([]*McpServerInfo, 0, len(list))
	for _, item := range list {
		result = append(result, l.toMcpServerInfo(item))
	}

	return result, total, nil
}

// UpdateStatus 更新MCP服务器状态
func (l *McpServerLogic) UpdateStatus(id int64, status int32) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TMcpServer

	now := time.Now()
	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": now,
	})
	return err
}

// toMcpServerInfo 转换为返回信息
func (l *McpServerLogic) toMcpServerInfo(m *model.TMcpServer) *McpServerInfo {
	info := &McpServerInfo{
		ID:        m.ID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
		CreatedBy: m.CreatedBy,
		Name:      m.Name,
		Transport: m.Transport,
	}

	if m.Description != nil {
		info.Description = *m.Description
	}
	if m.Command != nil {
		info.Command = *m.Command
	}
	if m.URL != nil {
		info.URL = *m.URL
	}
	if m.Timeout != nil {
		info.Timeout = *m.Timeout
	}
	if m.Sort != nil {
		info.Sort = *m.Sort
	}
	if m.Status != nil {
		info.Status = *m.Status
	}

	// 解析 Args
	if m.Args != nil && *m.Args != "" {
		var args []string
		if err := json.Unmarshal([]byte(*m.Args), &args); err == nil {
			info.Args = args
		}
	}
	if info.Args == nil {
		info.Args = []string{}
	}

	// 解析 Env
	if m.Env != nil && *m.Env != "" {
		var env map[string]string
		if err := json.Unmarshal([]byte(*m.Env), &env); err == nil {
			info.Env = env
		}
	}
	if info.Env == nil {
		info.Env = map[string]string{}
	}

	return info
}
