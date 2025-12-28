package logic

import (
	"errors"

	"yqhp/admin/internal/model"
	"yqhp/admin/internal/types"

	"gorm.io/gorm"
)

// ApplicationLogic 应用逻辑
type ApplicationLogic struct {
	db *gorm.DB
}

// NewApplicationLogic 创建应用逻辑
func NewApplicationLogic(db *gorm.DB) *ApplicationLogic {
	return &ApplicationLogic{db: db}
}

// CreateApplication 创建应用
func (l *ApplicationLogic) CreateApplication(req *types.CreateApplicationRequest) (*model.SysApplication, error) {
	// 检查应用编码是否存在
	var count int64
	l.db.Model(&model.SysApplication{}).Where("code = ? AND is_delete = ?", req.Code, false).Count(&count)
	if count > 0 {
		return nil, errors.New("应用编码已存在")
	}

	app := &model.SysApplication{
		Name:        req.Name,
		Code:        req.Code,
		Description: model.StringPtr(req.Description),
		Icon:        model.StringPtr(req.Icon),
		Sort:        model.Int64Ptr(int64(req.Sort)),
		Status:      model.Int32Ptr(int32(req.Status)),
		IsDelete:    model.BoolPtr(false),
	}

	if err := l.db.Create(app).Error; err != nil {
		return nil, err
	}

	return app, nil
}

// UpdateApplication 更新应用
func (l *ApplicationLogic) UpdateApplication(req *types.UpdateApplicationRequest) error {
	updates := map[string]any{
		"name":        req.Name,
		"description": req.Description,
		"icon":        req.Icon,
		"sort":        req.Sort,
		"status":      req.Status,
	}

	return l.db.Model(&model.SysApplication{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteApplication 删除应用（软删除）
func (l *ApplicationLogic) DeleteApplication(id int64) error {
	// 检查是否有角色使用该应用
	var roleCount int64
	l.db.Model(&model.SysRole{}).Where("app_id = ? AND is_delete = ?", id, false).Count(&roleCount)
	if roleCount > 0 {
		return errors.New("该应用下有角色，无法删除")
	}

	// 检查是否有资源使用该应用
	var resourceCount int64
	l.db.Model(&model.SysResource{}).Where("app_id = ? AND is_delete = ?", id, false).Count(&resourceCount)
	if resourceCount > 0 {
		return errors.New("该应用下有资源，无法删除")
	}

	return l.db.Model(&model.SysApplication{}).Where("id = ?", id).Update("is_delete", true).Error
}

// GetApplication 获取应用详情
func (l *ApplicationLogic) GetApplication(id int64) (*model.SysApplication, error) {
	var app model.SysApplication
	if err := l.db.Where("is_delete = ?", false).First(&app, id).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// GetApplicationByCode 根据编码获取应用
func (l *ApplicationLogic) GetApplicationByCode(code string) (*model.SysApplication, error) {
	var app model.SysApplication
	if err := l.db.Where("code = ? AND is_delete = ?", code, false).First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// ListApplications 获取应用列表
func (l *ApplicationLogic) ListApplications(req *types.ListApplicationsRequest) ([]model.SysApplication, int64, error) {
	var apps []model.SysApplication
	var total int64

	query := l.db.Model(&model.SysApplication{}).Where("is_delete = ?", false)

	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Code != "" {
		query = query.Where("code LIKE ?", "%"+req.Code+"%")
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}

	query.Count(&total)

	if req.Page > 0 && req.PageSize > 0 {
		offset := (req.Page - 1) * req.PageSize
		query = query.Offset(offset).Limit(req.PageSize)
	}

	if err := query.Order("sort ASC").Find(&apps).Error; err != nil {
		return nil, 0, err
	}

	return apps, total, nil
}

// GetAllApplications 获取所有应用(用于下拉选择)
func (l *ApplicationLogic) GetAllApplications() ([]model.SysApplication, error) {
	var apps []model.SysApplication
	if err := l.db.Where("status = 1 AND is_delete = ?", false).Order("sort ASC").Find(&apps).Error; err != nil {
		return nil, err
	}
	return apps, nil
}
