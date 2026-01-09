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

// DatabaseConfigLogic 数据库配置逻辑
type DatabaseConfigLogic struct {
	ctx context.Context
}

// NewDatabaseConfigLogic 创建数据库配置逻辑
func NewDatabaseConfigLogic(ctx context.Context) *DatabaseConfigLogic {
	return &DatabaseConfigLogic{ctx: ctx}
}

// 数据库类型常量
const (
	DBTypeMySQL   = "mysql"
	DBTypeRedis   = "redis"
	DBTypeMongoDB = "mongodb"
)

// CreateDatabaseConfigReq 创建数据库配置请求
type CreateDatabaseConfigReq struct {
	ProjectID   int64  `json:"project_id" validate:"required"`
	EnvID       int64  `json:"env_id" validate:"required"`
	Name        string `json:"name" validate:"required,max=100"`
	Code        string `json:"code" validate:"required,max=50"`
	Type        string `json:"type" validate:"required,max=20"`
	Host        string `json:"host" validate:"required,max=255"`
	Port        int32  `json:"port" validate:"required"`
	Database    string `json:"database" validate:"max=100"`
	Username    string `json:"username" validate:"max=100"`
	Password    string `json:"password" validate:"max=500"`
	Options     string `json:"options"`
	Description string `json:"description" validate:"max=500"`
	Status      int32  `json:"status"`
}

// UpdateDatabaseConfigReq 更新数据库配置请求
type UpdateDatabaseConfigReq struct {
	Name        string `json:"name" validate:"max=100"`
	Type        string `json:"type" validate:"max=20"`
	Host        string `json:"host" validate:"max=255"`
	Port        int32  `json:"port"`
	Database    string `json:"database" validate:"max=100"`
	Username    string `json:"username" validate:"max=100"`
	Password    string `json:"password" validate:"max=500"`
	Options     string `json:"options"`
	Description string `json:"description" validate:"max=500"`
	Status      int32  `json:"status"`
}

// DatabaseConfigListReq 数据库配置列表请求
type DatabaseConfigListReq struct {
	Page      int    `query:"page" validate:"min=1"`
	PageSize  int    `query:"pageSize" validate:"min=1,max=100"`
	ProjectID int64  `query:"project_id"`
	EnvID     int64  `query:"env_id"`
	Name      string `query:"name"`
	Code      string `query:"code"`
	Type      string `query:"type"`
	Status    *int32 `query:"status"`
}

// Create 创建数据库配置
func (l *DatabaseConfigLogic) Create(req *CreateDatabaseConfigReq) (*model.TDatabaseConfig, error) {
	// 检查环境是否存在
	envLogic := NewEnvLogic(l.ctx)
	env, err := envLogic.GetByID(req.EnvID)
	if err != nil {
		return nil, errors.New("环境不存在")
	}

	if req.ProjectID != env.ProjectID {
		return nil, errors.New("项目ID与环境所属项目不匹配")
	}

	// 验证数据库类型
	if req.Type != DBTypeMySQL && req.Type != DBTypeRedis && req.Type != DBTypeMongoDB {
		return nil, errors.New("不支持的数据库类型")
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

	config := &model.TDatabaseConfig{
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
		Database:    &req.Database,
		Username:    &req.Username,
		Password:    &password,
		Options:     &req.Options,
		Description: &req.Description,
		Status:      &status,
	}

	q := query.Use(svc.Ctx.DB)
	err = q.TDatabaseConfig.WithContext(l.ctx).Create(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// Update 更新数据库配置
func (l *DatabaseConfigLogic) Update(id int64, req *UpdateDatabaseConfigReq) error {
	q := query.Use(svc.Ctx.DB)
	d := q.TDatabaseConfig

	config, err := d.WithContext(l.ctx).Where(d.ID.Eq(id), d.IsDelete.Is(false)).First()
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
		if req.Type != DBTypeMySQL && req.Type != DBTypeRedis && req.Type != DBTypeMongoDB {
			return errors.New("不支持的数据库类型")
		}
		updates["type"] = req.Type
	}
	if req.Host != "" {
		updates["host"] = req.Host
	}
	if req.Port > 0 {
		updates["port"] = req.Port
	}
	updates["database"] = req.Database
	updates["username"] = req.Username
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

	_, err = d.WithContext(l.ctx).Where(d.ID.Eq(config.ID)).Updates(updates)
	return err
}

// Delete 删除数据库配置（软删除）
func (l *DatabaseConfigLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	d := q.TDatabaseConfig

	isDelete := true
	_, err := d.WithContext(l.ctx).Where(d.ID.Eq(id)).Update(d.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取数据库配置
func (l *DatabaseConfigLogic) GetByID(id int64) (*model.TDatabaseConfig, error) {
	q := query.Use(svc.Ctx.DB)
	d := q.TDatabaseConfig

	return d.WithContext(l.ctx).Where(d.ID.Eq(id), d.IsDelete.Is(false)).First()
}

// List 获取数据库配置列表
func (l *DatabaseConfigLogic) List(req *DatabaseConfigListReq) ([]*model.TDatabaseConfig, int64, error) {
	q := query.Use(svc.Ctx.DB)
	d := q.TDatabaseConfig

	qry := d.WithContext(l.ctx).Where(d.IsDelete.Is(false))

	if req.ProjectID > 0 {
		qry = qry.Where(d.ProjectID.Eq(req.ProjectID))
	}
	if req.EnvID > 0 {
		qry = qry.Where(d.EnvID.Eq(req.EnvID))
	}
	if req.Name != "" {
		qry = qry.Where(d.Name.Like("%" + req.Name + "%"))
	}
	if req.Code != "" {
		qry = qry.Where(d.Code.Like("%" + req.Code + "%"))
	}
	if req.Type != "" {
		qry = qry.Where(d.Type.Eq(req.Type))
	}
	if req.Status != nil {
		qry = qry.Where(d.Status.Eq(*req.Status))
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
	list, err := qry.Order(d.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// CheckCodeExists 检查配置代码在环境内是否存在
func (l *DatabaseConfigLogic) CheckCodeExists(envID int64, code string, excludeID int64) (bool, error) {
	q := query.Use(svc.Ctx.DB)
	d := q.TDatabaseConfig

	qry := d.WithContext(l.ctx).Where(d.EnvID.Eq(envID), d.Code.Eq(code), d.IsDelete.Is(false))
	if excludeID > 0 {
		qry = qry.Where(d.ID.Neq(excludeID))
	}

	count, err := qry.Count()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetConfigsByEnvID 获取环境下所有数据库配置
func (l *DatabaseConfigLogic) GetConfigsByEnvID(envID int64) ([]*model.TDatabaseConfig, error) {
	q := query.Use(svc.Ctx.DB)
	d := q.TDatabaseConfig

	status := int32(1)
	return d.WithContext(l.ctx).Where(
		d.EnvID.Eq(envID),
		d.IsDelete.Is(false),
		d.Status.Eq(status),
	).Order(d.ID.Desc()).Find()
}
