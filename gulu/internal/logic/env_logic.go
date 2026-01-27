package logic

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
	"yqhp/gulu/internal/utils"

	"gorm.io/gorm"
)

// EnvLogic 环境逻辑
type EnvLogic struct {
	ctx context.Context
}

// NewEnvLogic 创建环境逻辑
func NewEnvLogic(ctx context.Context) *EnvLogic {
	return &EnvLogic{ctx: ctx}
}

// CreateEnvReq 创建环境请求
type CreateEnvReq struct {
	ProjectID   int64  `json:"project_id" validate:"required"`
	Name        string `json:"name" validate:"required,max=100"`
	Description string `json:"description" validate:"max=500"`
	Sort        int64  `json:"sort"`
	Status      int32  `json:"status"`
}

// UpdateEnvReq 更新环境请求
type UpdateEnvReq struct {
	Name        string `json:"name" validate:"max=100"`
	Description string `json:"description" validate:"max=500"`
	Sort        int64  `json:"sort"`
	Status      int32  `json:"status"`
}

// EnvListReq 环境列表请求
type EnvListReq struct {
	Page      int    `query:"page" validate:"min=1"`
	PageSize  int    `query:"pageSize" validate:"min=1,max=100"`
	ProjectID int64  `query:"project_id"`
	Name      string `query:"name"`
	Status    *int32 `query:"status"`
}

// EnvUpdateSortReq 环境排序请求
type EnvUpdateSortReq struct {
	ID       int64  `json:"id" validate:"required"`        // 被拖动的环境ID
	TargetID int64  `json:"target_id" validate:"required"` // 目标位置的环境ID
	Position string `json:"position" validate:"required"`  // before 或 after
}

// Create 创建环境
func (l *EnvLogic) Create(req *CreateEnvReq, userID int64) (*model.TEnv, error) {
	// 检查项目是否存在
	projectLogic := NewProjectLogic(l.ctx)
	_, err := projectLogic.GetByID(req.ProjectID)
	if err != nil {
		return nil, errors.New("项目不存在")
	}

	now := time.Now()
	isDelete := false
	status := req.Status
	if status == 0 {
		status = 1
	}

	env := &model.TEnv{
		CreatedAt:   &now,
		UpdatedAt:   &now,
		IsDelete:    &isDelete,
		CreatedBy:   &userID,
		UpdatedBy:   &userID,
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		Description: &req.Description,
		Sort:        &req.Sort,
		Status:      &status,
	}

	q := query.Use(svc.Ctx.DB)
	err = q.TEnv.WithContext(l.ctx).Create(env)
	if err != nil {
		return nil, err
	}

	return env, nil
}

// Update 更新环境
func (l *EnvLogic) Update(id int64, req *UpdateEnvReq, userID int64) error {
	q := query.Use(svc.Ctx.DB)
	e := q.TEnv

	env, err := e.WithContext(l.ctx).Where(e.ID.Eq(id), e.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("环境不存在")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
		"updated_by": userID,
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	updates["sort"] = req.Sort
	updates["status"] = req.Status

	_, err = e.WithContext(l.ctx).Where(e.ID.Eq(env.ID)).Updates(updates)
	return err
}

// Delete 删除环境（软删除）
func (l *EnvLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	e := q.TEnv

	isDelete := true
	_, err := e.WithContext(l.ctx).Where(e.ID.Eq(id)).Update(e.IsDelete, isDelete)
	return err
}

// GetByID 根据ID获取环境
func (l *EnvLogic) GetByID(id int64) (*model.TEnv, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TEnv

	return e.WithContext(l.ctx).Where(e.ID.Eq(id), e.IsDelete.Is(false)).First()
}

// List 获取环境列表
func (l *EnvLogic) List(req *EnvListReq) ([]*model.TEnv, int64, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TEnv

	qry := e.WithContext(l.ctx).Where(e.IsDelete.Is(false))

	if req.ProjectID > 0 {
		qry = qry.Where(e.ProjectID.Eq(req.ProjectID))
	}
	if req.Name != "" {
		qry = qry.Where(e.Name.Like("%" + req.Name + "%"))
	}
	if req.Status != nil {
		qry = qry.Where(e.Status.Eq(*req.Status))
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
	list, err := qry.Order(e.Sort.Desc(), e.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// GetEnvsByProjectID 获取项目下所有环境
func (l *EnvLogic) GetEnvsByProjectID(projectID int64) ([]*model.TEnv, error) {
	q := query.Use(svc.Ctx.DB)
	e := q.TEnv

	return e.WithContext(l.ctx).Where(
		e.ProjectID.Eq(projectID),
		e.IsDelete.Is(false),
	).Order(e.Sort, e.ID).Find()
}

// UpdateSort 更新环境排序
func (l *EnvLogic) UpdateSort(req *EnvUpdateSortReq) error {
	q := query.Use(svc.Ctx.DB)
	e := q.TEnv

	// 获取被拖动的环境
	draggedEnv, err := e.WithContext(l.ctx).Where(e.ID.Eq(req.ID), e.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("被拖动的环境不存在")
	}

	// 获取目标位置的环境
	targetEnv, err := e.WithContext(l.ctx).Where(e.ID.Eq(req.TargetID), e.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("目标环境不存在")
	}

	// 确保是同一个项目
	if draggedEnv.ProjectID != targetEnv.ProjectID {
		return errors.New("只能在同一项目内排序")
	}

	// 获取项目下所有环境，按 sort 排序
	envs, err := e.WithContext(l.ctx).Where(
		e.ProjectID.Eq(draggedEnv.ProjectID),
		e.IsDelete.Is(false),
	).Order(e.Sort, e.ID).Find()
	if err != nil {
		return err
	}

	// 找到拖动项和目标项的索引
	var draggedIdx, targetIdx int
	for i, env := range envs {
		if env.ID == req.ID {
			draggedIdx = i
		}
		if env.ID == req.TargetID {
			targetIdx = i
		}
	}

	// 从列表中移除拖动项
	envs = append(envs[:draggedIdx], envs[draggedIdx+1:]...)

	// 重新计算目标索引（因为移除了一项）
	if draggedIdx < targetIdx {
		targetIdx--
	}

	// 根据 position 插入到正确位置
	var insertIdx int
	if req.Position == "before" {
		insertIdx = targetIdx
	} else {
		insertIdx = targetIdx + 1
	}

	// 插入拖动项到新位置
	envs = append(envs[:insertIdx], append([]*model.TEnv{draggedEnv}, envs[insertIdx:]...)...)

	// 批量更新排序值
	for i, env := range envs {
		sort := int64(i)
		_, err := e.WithContext(l.ctx).Where(e.ID.Eq(env.ID)).Update(e.Sort, sort)
		if err != nil {
			return err
		}
	}

	return nil
}

// CopyEnv 复制环境（包含所有配置）
func (l *EnvLogic) CopyEnv(sourceEnvID int64, newName string, userID int64) (*model.TEnv, error) {
	// 获取源环境
	sourceEnv, err := l.GetByID(sourceEnvID)
	if err != nil {
		return nil, errors.New("源环境不存在")
	}

	// 开启事务
	var newEnv *model.TEnv
	err = svc.Ctx.DB.Transaction(func(tx *gorm.DB) error {
		q := query.Use(tx)

		// 1. 创建新环境（直接复制 JSON 配置）
		now := time.Now()
		isDelete := false
		status := int32(1)
		zeroVersion := int32(0)
		newEnv = &model.TEnv{
			CreatedAt:      &now,
			UpdatedAt:      &now,
			IsDelete:       &isDelete,
			CreatedBy:      &userID,
			UpdatedBy:      &userID,
			ProjectID:      sourceEnv.ProjectID,
			Name:           newName,
			Description:    sourceEnv.Description,
			Sort:           sourceEnv.Sort,
			Status:         &status,
			Domains:        sourceEnv.Domains,
			Vars:           sourceEnv.Vars,
			DomainsVersion: &zeroVersion,
			VarsVersion:    &zeroVersion,
		}
		if err := q.TEnv.WithContext(l.ctx).Create(newEnv); err != nil {
			return err
		}

		// 2. 复制数据库配置
		dbConfigs, err := q.TDatabaseConfig.WithContext(l.ctx).Where(
			q.TDatabaseConfig.EnvID.Eq(sourceEnvID),
			q.TDatabaseConfig.IsDelete.Is(false),
		).Find()
		if err != nil {
			return err
		}
		for _, db := range dbConfigs {
			newDB := &model.TDatabaseConfig{
				CreatedAt:   &now,
				UpdatedAt:   &now,
				IsDelete:    &isDelete,
				ProjectID:   db.ProjectID,
				EnvID:       newEnv.ID,
				Name:        db.Name,
				Code:        db.Code,
				Type:        db.Type,
				Host:        db.Host,
				Port:        db.Port,
				Database:    db.Database,
				Username:    db.Username,
				Password:    db.Password,
				Options:     db.Options,
				Description: db.Description,
				Status:      db.Status,
			}
			if err := q.TDatabaseConfig.WithContext(l.ctx).Create(newDB); err != nil {
				return err
			}
		}

		// 3. 复制MQ配置
		mqConfigs, err := q.TMqConfig.WithContext(l.ctx).Where(
			q.TMqConfig.EnvID.Eq(sourceEnvID),
			q.TMqConfig.IsDelete.Is(false),
		).Find()
		if err != nil {
			return err
		}
		for _, mq := range mqConfigs {
			newMQ := &model.TMqConfig{
				CreatedAt:   &now,
				UpdatedAt:   &now,
				IsDelete:    &isDelete,
				ProjectID:   mq.ProjectID,
				EnvID:       newEnv.ID,
				Name:        mq.Name,
				Code:        mq.Code,
				Type:        mq.Type,
				Host:        mq.Host,
				Port:        mq.Port,
				Username:    mq.Username,
				Password:    mq.Password,
				Vhost:       mq.Vhost,
				Options:     mq.Options,
				Description: mq.Description,
				Status:      mq.Status,
			}
			if err := q.TMqConfig.WithContext(l.ctx).Create(newMQ); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return newEnv, nil
}

// ============================================
// 域名配置管理（存储在 DomainsJSON 字段）
// ============================================

// 变量类型常量
const (
	VarTypeString  = "string"
	VarTypeNumber  = "number"
	VarTypeBoolean = "boolean"
	VarTypeJSON    = "json"
)

// ErrVersionConflict 版本冲突错误
var ErrVersionConflict = errors.New("配置已被他人修改，请刷新后重试")

// UpdateDomainsReq 更新域名配置请求
type UpdateDomainsReq struct {
	Version int                `json:"version"` // 当前版本号
	Domains []model.DomainItem `json:"domains"` // 域名配置列表
}

// UpdateDomainsResp 更新域名配置响应
type UpdateDomainsResp struct {
	Version int                `json:"version"` // 新版本号
	Domains []model.DomainItem `json:"domains"` // 域名配置列表
}

// GetDomains 获取环境的域名配置
func (l *EnvLogic) GetDomains(envID int64) ([]model.DomainItem, int, error) {
	env, err := l.GetByID(envID)
	if err != nil {
		return nil, 0, errors.New("环境不存在")
	}

	var domains []model.DomainItem
	if env.Domains != nil && *env.Domains != "" {
		if err := json.Unmarshal([]byte(*env.Domains), &domains); err != nil {
			return nil, 0, errors.New("解析域名配置失败")
		}
	}

	version := 0
	if env.DomainsVersion != nil {
		version = int(*env.DomainsVersion)
	}
	return domains, version, nil
}

// ValidateURL 验证URL格式
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("URL不能为空")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("URL格式无效")
	}

	if parsed.Scheme == "" {
		return errors.New("URL必须包含协议（如 http:// 或 https://）")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL协议必须是 http 或 https")
	}

	if parsed.Host == "" {
		return errors.New("URL必须包含主机名")
	}

	return nil
}

// UpdateDomains 更新环境的域名配置（带乐观锁）
func (l *EnvLogic) UpdateDomains(envID int64, req *UpdateDomainsReq, userID int64) (*UpdateDomainsResp, error) {
	// 验证域名配置
	codeMap := make(map[string]bool)
	for _, d := range req.Domains {
		if d.Code == "" {
			return nil, errors.New("域名代码不能为空")
		}
		if d.Name == "" {
			return nil, errors.New("域名名称不能为空")
		}
		if err := ValidateURL(d.BaseURL); err != nil {
			return nil, err
		}
		if codeMap[d.Code] {
			return nil, errors.New("域名代码不能重复: " + d.Code)
		}
		codeMap[d.Code] = true
	}

	// 序列化 JSON
	domainsJSON, err := json.Marshal(req.Domains)
	if err != nil {
		return nil, errors.New("序列化域名配置失败")
	}

	now := time.Now()
	domainsStr := string(domainsJSON)

	// 使用乐观锁更新
	result := svc.Ctx.DB.WithContext(l.ctx).Exec(
		`UPDATE t_env SET domains = ?, domains_version = domains_version + 1, updated_at = ?, updated_by = ? 
		 WHERE id = ? AND domains_version = ? AND is_delete = 0`,
		domainsStr, now, userID, envID, req.Version,
	)

	if result.Error != nil {
		return nil, result.Error
	}

	if result.RowsAffected == 0 {
		return nil, ErrVersionConflict
	}

	return &UpdateDomainsResp{
		Version: req.Version + 1,
		Domains: req.Domains,
	}, nil
}

// ============================================
// 变量配置管理（存储在 VarsJSON 字段）
// ============================================

// UpdateVarsReq 更新变量配置请求
type UpdateVarsReq struct {
	Version int             `json:"version"` // 当前版本号
	Vars    []model.VarItem `json:"vars"`    // 变量配置列表
}

// UpdateVarsResp 更新变量配置响应
type UpdateVarsResp struct {
	Version int             `json:"version"` // 新版本号
	Vars    []model.VarItem `json:"vars"`    // 变量配置列表
}

// GetVars 获取环境的变量配置
func (l *EnvLogic) GetVars(envID int64) ([]model.VarItem, int, error) {
	env, err := l.GetByID(envID)
	if err != nil {
		return nil, 0, errors.New("环境不存在")
	}

	var vars []model.VarItem
	if env.Vars != nil && *env.Vars != "" {
		if err := json.Unmarshal([]byte(*env.Vars), &vars); err != nil {
			return nil, 0, errors.New("解析变量配置失败")
		}
	}

	// 解密敏感数据用于显示（脱敏处理）
	for i := range vars {
		if vars[i].IsSensitive && vars[i].Value != "" {
			// 敏感数据显示为 ******
			vars[i].Value = "******"
		}
	}

	version := 0
	if env.VarsVersion != nil {
		version = int(*env.VarsVersion)
	}
	return vars, version, nil
}

// GetVarsForExecution 获取环境的变量配置（用于工作流执行，解密敏感数据）
func (l *EnvLogic) GetVarsForExecution(envID int64) ([]model.VarItem, error) {
	env, err := l.GetByID(envID)
	if err != nil {
		return nil, errors.New("环境不存在")
	}

	var vars []model.VarItem
	if env.Vars != nil && *env.Vars != "" {
		if err := json.Unmarshal([]byte(*env.Vars), &vars); err != nil {
			return nil, errors.New("解析变量配置失败")
		}
	}

	// 解密敏感数据
	for i := range vars {
		if vars[i].IsSensitive && vars[i].Value != "" {
			decrypted, err := utils.Decrypt(vars[i].Value)
			if err == nil {
				vars[i].Value = decrypted
			}
		}
	}

	return vars, nil
}

// ValidateVarType 验证变量类型
func ValidateVarType(varType, value string) error {
	switch varType {
	case VarTypeString:
		return nil
	case VarTypeNumber:
		if value == "" {
			return nil
		}
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return errors.New("值不是有效的数字")
		}
		return nil
	case VarTypeBoolean:
		if value == "" {
			return nil
		}
		if value != "true" && value != "false" {
			return errors.New("值必须是 true 或 false")
		}
		return nil
	case VarTypeJSON:
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

// UpdateVars 更新环境的变量配置（带乐观锁）
func (l *EnvLogic) UpdateVars(envID int64, req *UpdateVarsReq, userID int64) (*UpdateVarsResp, error) {
	// 获取当前环境的变量配置，用于保留未修改的敏感数据值
	env, err := l.GetByID(envID)
	if err != nil {
		return nil, errors.New("环境不存在")
	}

	// 解析当前的变量配置
	var currentVars []model.VarItem
	if env.Vars != nil && *env.Vars != "" {
		json.Unmarshal([]byte(*env.Vars), &currentVars)
	}
	currentVarsMap := make(map[string]model.VarItem)
	for _, v := range currentVars {
		currentVarsMap[v.Key] = v
	}

	// 验证并处理变量配置
	keyMap := make(map[string]bool)
	for i := range req.Vars {
		v := &req.Vars[i]
		if v.Key == "" {
			return nil, errors.New("变量Key不能为空")
		}
		if v.Name == "" {
			return nil, errors.New("变量名称不能为空")
		}
		if v.Type == "" {
			v.Type = VarTypeString
		}

		// 如果敏感数据值为 ****** 或空，保留原值
		if v.IsSensitive {
			if v.Value == "******" || v.Value == "" {
				if old, ok := currentVarsMap[v.Key]; ok {
					v.Value = old.Value
				}
			} else {
				// 新的敏感数据需要加密
				if err := ValidateVarType(v.Type, v.Value); err != nil {
					return nil, err
				}
				encrypted, err := utils.Encrypt(v.Value)
				if err != nil {
					return nil, errors.New("加密失败")
				}
				v.Value = encrypted
			}
		} else {
			// 非敏感数据验证类型
			if err := ValidateVarType(v.Type, v.Value); err != nil {
				return nil, err
			}
		}

		if keyMap[v.Key] {
			return nil, errors.New("变量Key不能重复: " + v.Key)
		}
		keyMap[v.Key] = true
	}

	// 序列化 JSON
	varsJSON, err := json.Marshal(req.Vars)
	if err != nil {
		return nil, errors.New("序列化变量配置失败")
	}

	now := time.Now()
	varsStr := string(varsJSON)

	// 使用乐观锁更新
	result := svc.Ctx.DB.WithContext(l.ctx).Exec(
		`UPDATE t_env SET vars = ?, vars_version = vars_version + 1, updated_at = ?, updated_by = ? 
		 WHERE id = ? AND vars_version = ? AND is_delete = 0`,
		varsStr, now, userID, envID, req.Version,
	)

	if result.Error != nil {
		return nil, result.Error
	}

	if result.RowsAffected == 0 {
		return nil, ErrVersionConflict
	}

	// 返回时脱敏显示
	respVars := make([]model.VarItem, len(req.Vars))
	copy(respVars, req.Vars)
	for i := range respVars {
		if respVars[i].IsSensitive {
			respVars[i].Value = "******"
		}
	}

	return &UpdateVarsResp{
		Version: req.Version + 1,
		Vars:    respVars,
	}, nil
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

// ExportVars 导出环境变量
func (l *EnvLogic) ExportVars(envID int64) ([]VarExportItem, error) {
	env, err := l.GetByID(envID)
	if err != nil {
		return nil, errors.New("环境不存在")
	}

	var vars []model.VarItem
	if env.Vars != nil && *env.Vars != "" {
		if err := json.Unmarshal([]byte(*env.Vars), &vars); err != nil {
			return nil, errors.New("解析变量配置失败")
		}
	}

	items := make([]VarExportItem, len(vars))
	for i, v := range vars {
		value := v.Value
		// 敏感数据解密后导出
		if v.IsSensitive && value != "" {
			decrypted, err := utils.Decrypt(value)
			if err == nil {
				value = decrypted
			}
		}

		items[i] = VarExportItem{
			Name:        v.Name,
			Key:         v.Key,
			Value:       value,
			Type:        v.Type,
			IsSensitive: v.IsSensitive,
			Description: v.Description,
		}
	}

	return items, nil
}

// ImportVars 导入环境变量
func (l *EnvLogic) ImportVars(envID int64, items []VarExportItem, userID int64) error {
	// 获取当前环境
	env, err := l.GetByID(envID)
	if err != nil {
		return errors.New("环境不存在")
	}

	// 转换为 VarItem
	vars := make([]model.VarItem, len(items))
	for i, item := range items {
		value := item.Value
		// 敏感数据需要加密
		if item.IsSensitive && value != "" {
			encrypted, err := utils.Encrypt(value)
			if err != nil {
				return errors.New("加密失败")
			}
			value = encrypted
		}

		vars[i] = model.VarItem{
			Key:         item.Key,
			Name:        item.Name,
			Value:       value,
			Type:        item.Type,
			IsSensitive: item.IsSensitive,
			Description: item.Description,
		}
	}

	// 序列化 JSON
	varsJSON, err := json.Marshal(vars)
	if err != nil {
		return errors.New("序列化变量配置失败")
	}

	now := time.Now()
	varsStr := string(varsJSON)

	// 直接更新（导入操作不使用乐观锁，强制覆盖）
	result := svc.Ctx.DB.WithContext(l.ctx).Exec(
		`UPDATE t_env SET vars = ?, vars_version = vars_version + 1, updated_at = ?, updated_by = ? 
		 WHERE id = ? AND is_delete = 0`,
		varsStr, now, userID, env.ID,
	)

	return result.Error
}
