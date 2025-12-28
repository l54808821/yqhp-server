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
func (l *DictLogic) CreateDictType(req *types.CreateDictTypeRequest) (*model.SysDictType, error) {
	// 检查编码是否存在
	var count int64
	l.db.Model(&model.SysDictType{}).Where("code = ? AND is_delete = ?", req.Code, false).Count(&count)
	if count > 0 {
		return nil, errors.New("字典类型编码已存在")
	}

	dictType := &model.SysDictType{
		Name:     req.Name,
		Code:     req.Code,
		Status:   model.Int32Ptr(int32(req.Status)),
		Remark:   model.StringPtr(req.Remark),
		IsDelete: model.BoolPtr(false),
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

	return l.db.Model(&model.SysDictType{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteDictType 删除字典类型（软删除）
func (l *DictLogic) DeleteDictType(id int64) error {
	var dictType model.SysDictType
	if err := l.db.Where("is_delete = ?", false).First(&dictType, id).Error; err != nil {
		return err
	}

	// 软删除字典数据
	l.db.Model(&model.SysDictDatum{}).Where("type_code = ? AND is_delete = ?", dictType.Code, false).Update("is_delete", true)
	// 软删除字典类型
	return l.db.Model(&dictType).Update("is_delete", true).Error
}

// GetDictType 获取字典类型详情
func (l *DictLogic) GetDictType(id int64) (*model.SysDictType, error) {
	var dictType model.SysDictType
	if err := l.db.Where("is_delete = ?", false).First(&dictType, id).Error; err != nil {
		return nil, err
	}
	return &dictType, nil
}

// ListDictTypes 获取字典类型列表
func (l *DictLogic) ListDictTypes(req *types.ListDictTypesRequest) ([]model.SysDictType, int64, error) {
	var dictTypes []model.SysDictType
	var total int64

	query := l.db.Model(&model.SysDictType{}).Where("is_delete = ?", false)

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
func (l *DictLogic) CreateDictData(req *types.CreateDictDataRequest) (*model.SysDictDatum, error) {
	dictData := &model.SysDictDatum{
		TypeCode:  req.TypeCode,
		Label:     req.Label,
		Value:     req.Value,
		Sort:      model.Int64Ptr(int64(req.Sort)),
		Status:    model.Int32Ptr(int32(req.Status)),
		IsDefault: model.BoolPtr(req.IsDefault),
		CSSClass:  model.StringPtr(req.CssClass),
		ListClass: model.StringPtr(req.ListClass),
		Remark:    model.StringPtr(req.Remark),
		IsDelete:  model.BoolPtr(false),
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

	return l.db.Model(&model.SysDictDatum{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteDictData 删除字典数据（软删除）
func (l *DictLogic) DeleteDictData(id int64) error {
	return l.db.Model(&model.SysDictDatum{}).Where("id = ?", id).Update("is_delete", true).Error
}

// GetDictDataByTypeCode 根据类型编码获取字典数据
func (l *DictLogic) GetDictDataByTypeCode(typeCode string) ([]model.SysDictDatum, error) {
	var dictData []model.SysDictDatum
	if err := l.db.Where("type_code = ? AND status = 1 AND is_delete = ?", typeCode, false).
		Order("sort ASC").
		Find(&dictData).Error; err != nil {
		return nil, err
	}
	return dictData, nil
}

// ListDictData 获取字典数据列表
func (l *DictLogic) ListDictData(req *types.ListDictDataRequest) ([]model.SysDictDatum, int64, error) {
	var dictData []model.SysDictDatum
	var total int64

	query := l.db.Model(&model.SysDictDatum{}).Where("is_delete = ?", false)

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
