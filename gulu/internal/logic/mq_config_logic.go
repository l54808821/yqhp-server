package logic

import (
	"context"
	"errors"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
	"yqhp/gulu/internal/utils"
)

// MQConfigLogic MQ配置逻辑
type MQConfigLogic struct {
	ctx context.Context
}

// NewMQConfigLogic 创建MQ配置逻辑
func NewMQConfigLogic(ctx context.Context) *MQConfigLogic {
	return &MQConfigLogic{ctx: ctx}
}

// MQ类型常量
const (
	MQTypeRabbitMQ = "rabbitmq"
	MQTypeKafka    = "kafka"
	MQTypeRocketMQ = "rocketmq"
)

// CreateMQConfigReq 创建MQ配置请求
type CreateMQConfigReq struct {
	ProjectID   int64  `json:"project_id" validate:"required"`
	EnvID       int64  `json:"env_id" validate:"required"`
	Name        string `json:"name" validate:"required,max=100"`
	Code        string `json:"code" validate:"required,max=50"`
	Type        string `json:"type" validate:"required,max=20"`
	Host        string `json:"host" validate:"required,max=255"`
	Port        int32  `json:"port" validate:"required"`
	Username    string `json:"username" validate:"max=100"`
	Password    string `json:"password" validate:"max=500"`
	Vhost       string `json:"vhost" validate:"max=100"`
	Options     string `json:"options"`
	Description string `json:"description" validate:"max=500"`
	Status      int32  `json:"status"`
}

// UpdateMQConfigReq 更新MQ配置请求
type UpdateMQConfigReq struct {
	Name        string `json:"name" validate:"max=100"`
	Type        string `json:"type" validate:"max=20"`
	Host        string `json:"host" validate:"max=255"`
	Port        int32  `json:"port"`
	Username    string `json:"username" validate:"max=100"`
	Password    string `json:"password" validate:"max=500"`
	Vhost       string `json:"vhost" validate:"max=100"`
	Options     string `json:"options"`
	Description string `json:"description" validate:"max=500"`
	Status      int32  `json:"status"`
}

// MQConfigListReq MQ配置列表请求
type MQConfigListReq struct {
	Page      int    `query:"page" validate:"min=1"`
	PageSize  int    `query:"pageSize" validate:"min=1,max=100"`
	ProjectID int64  `query:"project_id"`
	EnvID     int64  `query:"env_id"`
	Name      string `query:"name"`
	Code      string `query:"code"`
	Type      string `query:"type"`
	Status    *int32 `query:"status"`
}

// Create 创建MQ配置
func (l *MQConfigLogic) Create(req *CreateMQConfigReq) (*model.TMqConfig, error) {
	// 检查环境是否存在
	envLogic := NewEnvLogic(l.ctx)
	env, err := envLogic.GetByID(req.EnvID)
	if err != nil {
		return nil, errors.New("环境不存在")
	}

	if req.ProjectID != env.ProjectID {
		return nil, errors.New("项目ID与环境所属项目不匹配")
	}

	// 验证MQ类型
	if req.Type != MQTypeRabbitMQ && req.Type != MQTypeKafka && req.Type != MQTypeRocketMQ {
		return nil, errors.New("不支持的MQ类型")
	}

	// 检查代码是否唯一
	exists, err := l.CheckCodeExists(req.EnvID, req.Code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("配置代码在该环境中已存在")
	}

	// 加密密码
	password := req.Password
	if password != "" {
		encrypted, err := utils.Encrypt(password)
		if err != nil {
			return nil, errors.New("密码加密失败")
		}
		password = encrypted
	}

	now := time.Now()
	isDelete := false
	status := req.Status
	if status == 0 {
		status = 1
	}

	config := &model.TMqConfig{
		CreatedAt:   &now,
		UpdatedAt:   &now,
		IsDelete:    &isDelete,
		ProjectID:   req.ProjectID,
		EnvID:       req.EnvID,
		Name:        req.Name,
		Code:        req.Code,
		Type:        req.Type,
		Host:        req.Host,
		Port:        req.Port,
		Username:    &req.Username,
		Password:    &password,
		Vhost:       &req.Vhost,
		Options:     &req.Options,
		Description: &req.Description,
		Status:      &status,
	}

	q := query.Use(svc.Ctx.DB)
	err = q.TMqConfig.WithContext(l.ctx).Create(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// Update 更新MQ配置
func (l *MQConfigLogic) Update(id int64, req *UpdateMQConfigReq) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TMqConfig

	config, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("配置不存在")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Type != "" {
		if req.Type != MQTypeRabbitMQ && req.Type != MQTypeKafka && req.Type != MQTypeRocketMQ {
			return errors.New("不支持的MQ类型")
		}
		updates["type"] = req.Type
	}
	if req.Host != "" {
		updates["host"] = req.Host
	}
	if req.Port > 0 {
		updates["port"] = req.Port
	}
	updates["username"] = req.Username
	updates["vhost"] = req.Vhost
	updates["options"] = req.Options
	updates["description"] = req.Description
	updates["status"] = req.Status

	// 加密密码
	if req.Password != "" {
		encrypted, err := utils.Encrypt(req.Password)
		if err != nil {
			return errors.New("密码加密失败")
		}
		updates["password"] = encrypted
	}

	_, err = m.WithContext(l.ctx).Where(m.ID.Eq(config.ID)).Updates(updates)
	return err
}

// Delete 删除MQ配置（软删除）
func (l *MQConfigLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TMqConfig

	isDelete := true
	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(id)).Update(m.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取MQ配置
func (l *MQConfigLogic) GetByID(id int64) (*model.TMqConfig, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TMqConfig

	return m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
}

// List 获取MQ配置列表
func (l *MQConfigLogic) List(req *MQConfigListReq) ([]*model.TMqConfig, int64, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TMqConfig

	qry := m.WithContext(l.ctx).Where(m.IsDelete.Is(false))

	if req.ProjectID > 0 {
		qry = qry.Where(m.ProjectID.Eq(req.ProjectID))
	}
	if req.EnvID > 0 {
		qry = qry.Where(m.EnvID.Eq(req.EnvID))
	}
	if req.Name != "" {
		qry = qry.Where(m.Name.Like("%" + req.Name + "%"))
	}
	if req.Code != "" {
		qry = qry.Where(m.Code.Like("%" + req.Code + "%"))
	}
	if req.Type != "" {
		qry = qry.Where(m.Type.Eq(req.Type))
	}
	if req.Status != nil {
		qry = qry.Where(m.Status.Eq(*req.Status))
	}

	total, err := qry.Count()
	if err != nil {
		return nil, 0, err
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	offset := (req.Page - 1) * req.PageSize
	list, err := qry.Order(m.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// CheckCodeExists 检查配置代码在环境内是否存在
func (l *MQConfigLogic) CheckCodeExists(envID int64, code string, excludeID int64) (bool, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TMqConfig

	qry := m.WithContext(l.ctx).Where(m.EnvID.Eq(envID), m.Code.Eq(code), m.IsDelete.Is(false))
	if excludeID > 0 {
		qry = qry.Where(m.ID.Neq(excludeID))
	}

	count, err := qry.Count()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetConfigsByEnvID 获取环境下所有MQ配置
func (l *MQConfigLogic) GetConfigsByEnvID(envID int64) ([]*model.TMqConfig, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TMqConfig

	status := int32(1)
	return m.WithContext(l.ctx).Where(
		m.EnvID.Eq(envID),
		m.IsDelete.Is(false),
		m.Status.Eq(status),
	).Order(m.ID.Desc()).Find()
}
