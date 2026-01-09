package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
	"yqhp/gulu/internal/utils"
)

// VarLogic 变量逻辑
type VarLogic struct {
	ctx context.Context
}

// NewVarLogic 创建变量逻辑
func NewVarLogic(ctx context.Context) *VarLogic {
	return &VarLogic{ctx: ctx}
}

// 变量类型常量
const (
	VarTypeString  = "string"
	VarTypeNumber  = "number"
	VarTypeBoolean = "boolean"
	VarTypeJSON    = "json"
)

// CreateVarReq 创建变量请求
type CreateVarReq struct {
	ProjectID   int64  `json:"project_id" validate:"required"`
	EnvID       int64  `json:"env_id" validate:"required"`
	Name        string `json:"name" validate:"required,max=100"`
	Key         string `json:"key" validate:"required,max=100"`
	Value       string `json:"value"`
	Type        string `json:"type" validate:"max=20"`
	IsSensitive bool   `json:"is_sensitive"`
	Description string `json:"description" validate:"max=500"`
}

// UpdateVarReq 更新变量请求
type UpdateVarReq struct {
	Name        string `json:"name" validate:"max=100"`
	Value       string `json:"value"`
	Type        string `json:"type" validate:"max=20"`
	IsSensitive bool   `json:"is_sensitive"`
	Description string `json:"description" validate:"max=500"`
}

// VarListReq 变量列表请求
type VarListReq struct {
	Page      int    `query:"page" validate:"min=1"`
	PageSize  int    `query:"pageSize" validate:"min=1,max=100"`
	ProjectID int64  `query:"project_id"`
	EnvID     int64  `query:"env_id"`
	Name      string `query:"name"`
	Key       string `query:"key"`
	Type      string `query:"type"`
}

// VarExportItem 变量导出项
type VarExportItem struct {
	Name        string `json:"name"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	IsSensitive bool   `json:"is_sensitive"`
	Description string `json:"description"`
}

// ValidateVarType 验证变量类型
func ValidateVarType(varType, value string) error {
	switch varType {
	case VarTypeString:
		// 字符串类型，任何值都有效
		return nil
	case VarTypeNumber:
		// 数字类型，验证是否为有效数字
		if value == "" {
			return nil
		}
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return errors.New("值不是有效的数字")
		}
		return nil
	case VarTypeBoolean:
		// 布尔类型，验证是否为 true/false
		if value == "" {
			return nil
		}
		if value != "true" && value != "false" {
			return errors.New("值必须是 true 或 false")
		}
		return nil
	case VarTypeJSON:
		// JSON类型，验证是否为有效JSON
		if value == "" {
			return nil
		}
		var js interface{}
		if err := json.Unmarshal([]byte(value), &js); err != nil {
			return errors.New("值不是有效的JSON格式")
		}
		return nil
	default:
		return errors.New("不支持的变量类型")
	}
}

// Create 创建变量
func (l *VarLogic) Create(req *CreateVarReq) (*model.TVar, error) {
	// 检查环境是否存在
	envLogic := NewEnvLogic(l.ctx)
	env, err := envLogic.GetByID(req.EnvID)
	if err != nil {
		return nil, errors.New("环境不存在")
	}

	if req.ProjectID != env.ProjectID {
		return nil, errors.New("项目ID与环境所属项目不匹配")
	}

	// 设置默认类型
	if req.Type == "" {
		req.Type = VarTypeString
	}

	// 验证变量类型
	if err := ValidateVarType(req.Type, req.Value); err != nil {
		return nil, err
	}

	// 检查变量Key在环境内是否唯一
	exists, err := l.CheckKeyExists(req.EnvID, req.Key, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("变量Key在该环境中已存在")
	}

	// 敏感数据加密
	value := req.Value
	if req.IsSensitive && value != "" {
		encrypted, err := utils.Encrypt(value)
		if err != nil {
			return nil, errors.New("加密失败")
		}
		value = encrypted
	}

	now := time.Now()
	isDelete := false

	variable := &model.TVar{
		CreatedAt:   &now,
		UpdatedAt:   &now,
		IsDelete:    &isDelete,
		ProjectID:   req.ProjectID,
		EnvID:       req.EnvID,
		Name:        req.Name,
		Key:         req.Key,
		Value:       &value,
		Type:        &req.Type,
		IsSensitive: &req.IsSensitive,
		Description: &req.Description,
	}

	q := query.Use(svc.Ctx.DB)
	err = q.TVar.WithContext(l.ctx).Create(variable)
	if err != nil {
		return nil, err
	}

	return variable, nil
}

// Update 更新变量
func (l *VarLogic) Update(id int64, req *UpdateVarReq) error {
	q := query.Use(svc.Ctx.DB)
	v := q.TVar

	variable, err := v.WithContext(l.ctx).Where(v.ID.Eq(id), v.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("变量不存在")
	}

	// 验证变量类型
	varType := req.Type
	if varType == "" && variable.Type != nil {
		varType = *variable.Type
	}
	if err := ValidateVarType(varType, req.Value); err != nil {
		return err
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

	updates["is_sensitive"] = req.IsSensitive

	// 处理值更新
	value := req.Value
	if req.IsSensitive && value != "" {
		encrypted, err := utils.Encrypt(value)
		if err != nil {
			return errors.New("加密失败")
		}
		value = encrypted
	}
	updates["value"] = value

	_, err = v.WithContext(l.ctx).Where(v.ID.Eq(variable.ID)).Updates(updates)
	return err
}

// Delete 删除变量（软删除）
func (l *VarLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	v := q.TVar

	isDelete := true
	_, err := v.WithContext(l.ctx).Where(v.ID.Eq(id)).Update(v.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取变量
func (l *VarLogic) GetByID(id int64) (*model.TVar, error) {
	q := query.Use(svc.Ctx.DB)
	v := q.TVar

	return v.WithContext(l.ctx).Where(v.ID.Eq(id), v.IsDelete.Is(false)).First()
}

// List 获取变量列表
func (l *VarLogic) List(req *VarListReq) ([]*model.TVar, int64, error) {
	q := query.Use(svc.Ctx.DB)
	v := q.TVar

	qry := v.WithContext(l.ctx).Where(v.IsDelete.Is(false))

	if req.ProjectID > 0 {
		qry = qry.Where(v.ProjectID.Eq(req.ProjectID))
	}
	if req.EnvID > 0 {
		qry = qry.Where(v.EnvID.Eq(req.EnvID))
	}
	if req.Name != "" {
		qry = qry.Where(v.Name.Like("%" + req.Name + "%"))
	}
	if req.Key != "" {
		qry = qry.Where(v.Key.Like("%" + req.Key + "%"))
	}
	if req.Type != "" {
		qry = qry.Where(v.Type.Eq(req.Type))
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
	list, err := qry.Order(v.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// CheckKeyExists 检查变量Key在环境内是否存在
func (l *VarLogic) CheckKeyExists(envID int64, key string, excludeID int64) (bool, error) {
	q := query.Use(svc.Ctx.DB)
	v := q.TVar

	qry := v.WithContext(l.ctx).Where(v.EnvID.Eq(envID), v.Key.Eq(key), v.IsDelete.Is(false))
	if excludeID > 0 {
		qry = qry.Where(v.ID.Neq(excludeID))
	}

	count, err := qry.Count()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetVarsByEnvID 获取环境下所有变量
func (l *VarLogic) GetVarsByEnvID(envID int64) ([]*model.TVar, error) {
	q := query.Use(svc.Ctx.DB)
	v := q.TVar

	return v.WithContext(l.ctx).Where(
		v.EnvID.Eq(envID),
		v.IsDelete.Is(false),
	).Order(v.ID.Desc()).Find()
}

// Export 导出环境变量
func (l *VarLogic) Export(envID int64) ([]VarExportItem, error) {
	vars, err := l.GetVarsByEnvID(envID)
	if err != nil {
		return nil, err
	}

	items := make([]VarExportItem, len(vars))
	for i, v := range vars {
		value := ""
		if v.Value != nil {
			value = *v.Value
		}

		// 敏感数据解密后导出
		isSensitive := v.IsSensitive != nil && *v.IsSensitive
		if isSensitive && value != "" {
			decrypted, err := utils.Decrypt(value)
			if err == nil {
				value = decrypted
			}
		}

		varType := VarTypeString
		if v.Type != nil {
			varType = *v.Type
		}

		description := ""
		if v.Description != nil {
			description = *v.Description
		}

		items[i] = VarExportItem{
			Name:        v.Name,
			Key:         v.Key,
			Value:       value,
			Type:        varType,
			IsSensitive: isSensitive,
			Description: description,
		}
	}

	return items, nil
}

// Import 导入环境变量
func (l *VarLogic) Import(projectID, envID int64, items []VarExportItem) error {
	// 检查环境是否存在
	envLogic := NewEnvLogic(l.ctx)
	env, err := envLogic.GetByID(envID)
	if err != nil {
		return errors.New("环境不存在")
	}

	if projectID != env.ProjectID {
		return errors.New("项目ID与环境所属项目不匹配")
	}

	for _, item := range items {
		// 检查Key是否已存在
		exists, err := l.CheckKeyExists(envID, item.Key, 0)
		if err != nil {
			return err
		}

		if exists {
			// 更新已存在的变量
			q := query.Use(svc.Ctx.DB)
			v := q.TVar

			variable, err := v.WithContext(l.ctx).Where(
				v.EnvID.Eq(envID),
				v.Key.Eq(item.Key),
				v.IsDelete.Is(false),
			).First()
			if err != nil {
				continue
			}

			req := &UpdateVarReq{
				Name:        item.Name,
				Value:       item.Value,
				Type:        item.Type,
				IsSensitive: item.IsSensitive,
				Description: item.Description,
			}
			if err := l.Update(variable.ID, req); err != nil {
				return err
			}
		} else {
			// 创建新变量
			req := &CreateVarReq{
				ProjectID:   projectID,
				EnvID:       envID,
				Name:        item.Name,
				Key:         item.Key,
				Value:       item.Value,
				Type:        item.Type,
				IsSensitive: item.IsSensitive,
				Description: item.Description,
			}
			if _, err := l.Create(req); err != nil {
				return err
			}
		}
	}

	return nil
}
