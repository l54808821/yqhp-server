package logic

import (
	"errors"

	"yqhp/admin/internal/model"
	"yqhp/admin/internal/types"

	"gorm.io/gorm"
)

// DictLogic 字典逻辑
type DictLogic struct {
	db *gorm.DB
}

// NewDictLogic 创建字典逻辑
func NewDictLogic(db *gorm.DB) *DictLogic {
	return &DictLogic{db: db}
}

// CreateDictType 创建字典类型
func (l *DictLogic) CreateDictType(req *types.CreateDictTypeRequest) (*model.DictType, error) {
	// 检查编码是否存在
	var count int64
	l.db.Model(&model.DictType{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		return nil, errors.New("字典类型编码已存在")
	}

	dictType := &model.DictType{
		Name:   req.Name,
		Code:   req.Code,
		Status: req.Status,
		Remark: req.Remark,
	}

	if err := l.db.Create(dictType).Error; err != nil {
		return nil, err
	}

	return dictType, nil
}

// UpdateDictType 更新字典类型
func (l *DictLogic) UpdateDictType(req *types.UpdateDictTypeRequest) error {
	updates := map[string]any{
		"name":   req.Name,
		"status": req.Status,
		"remark": req.Remark,
	}

	return l.db.Model(&model.DictType{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteDictType 删除字典类型
func (l *DictLogic) DeleteDictType(id uint) error {
	var dictType model.DictType
	if err := l.db.First(&dictType, id).Error; err != nil {
		return err
	}

	// 删除字典数据
	l.db.Where("type_code = ?", dictType.Code).Delete(&model.DictData{})
	// 删除字典类型
	return l.db.Delete(&dictType).Error
}

// GetDictType 获取字典类型详情
func (l *DictLogic) GetDictType(id uint) (*model.DictType, error) {
	var dictType model.DictType
	if err := l.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort ASC")
	}).First(&dictType, id).Error; err != nil {
		return nil, err
	}
	return &dictType, nil
}

// ListDictTypes 获取字典类型列表
func (l *DictLogic) ListDictTypes(req *types.ListDictTypesRequest) ([]model.DictType, int64, error) {
	var dictTypes []model.DictType
	var total int64

	query := l.db.Model(&model.DictType{})

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

	if err := query.Find(&dictTypes).Error; err != nil {
		return nil, 0, err
	}

	return dictTypes, total, nil
}

// CreateDictData 创建字典数据
func (l *DictLogic) CreateDictData(req *types.CreateDictDataRequest) (*model.DictData, error) {
	dictData := &model.DictData{
		TypeCode:  req.TypeCode,
		Label:     req.Label,
		Value:     req.Value,
		Sort:      req.Sort,
		Status:    req.Status,
		IsDefault: req.IsDefault,
		CssClass:  req.CssClass,
		ListClass: req.ListClass,
		Remark:    req.Remark,
	}

	if err := l.db.Create(dictData).Error; err != nil {
		return nil, err
	}

	return dictData, nil
}

// UpdateDictData 更新字典数据
func (l *DictLogic) UpdateDictData(req *types.UpdateDictDataRequest) error {
	updates := map[string]any{
		"label":      req.Label,
		"value":      req.Value,
		"sort":       req.Sort,
		"status":     req.Status,
		"is_default": req.IsDefault,
		"css_class":  req.CssClass,
		"list_class": req.ListClass,
		"remark":     req.Remark,
	}

	return l.db.Model(&model.DictData{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteDictData 删除字典数据
func (l *DictLogic) DeleteDictData(id uint) error {
	return l.db.Delete(&model.DictData{}, id).Error
}

// GetDictDataByTypeCode 根据类型编码获取字典数据
func (l *DictLogic) GetDictDataByTypeCode(typeCode string) ([]model.DictData, error) {
	var dictData []model.DictData
	if err := l.db.Where("type_code = ? AND status = 1", typeCode).
		Order("sort ASC").
		Find(&dictData).Error; err != nil {
		return nil, err
	}
	return dictData, nil
}

// ListDictData 获取字典数据列表
func (l *DictLogic) ListDictData(req *types.ListDictDataRequest) ([]model.DictData, int64, error) {
	var dictData []model.DictData
	var total int64

	query := l.db.Model(&model.DictData{})

	if req.TypeCode != "" {
		query = query.Where("type_code = ?", req.TypeCode)
	}
	if req.Label != "" {
		query = query.Where("label LIKE ?", "%"+req.Label+"%")
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}

	query.Count(&total)

	if req.Page > 0 && req.PageSize > 0 {
		offset := (req.Page - 1) * req.PageSize
		query = query.Offset(offset).Limit(req.PageSize)
	}

	if err := query.Order("sort ASC").Find(&dictData).Error; err != nil {
		return nil, 0, err
	}

	return dictData, total, nil
}
