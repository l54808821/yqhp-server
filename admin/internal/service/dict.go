package service

import (
	"errors"

	"yqhp/admin/internal/model"

	"gorm.io/gorm"
)

// DictService 字典服务
type DictService struct {
	db *gorm.DB
}

// NewDictService 创建字典服务
func NewDictService(db *gorm.DB) *DictService {
	return &DictService{db: db}
}

// CreateDictTypeRequest 创建字典类型请求
type CreateDictTypeRequest struct {
	Name   string `json:"name" validate:"required"`
	Code   string `json:"code" validate:"required"`
	Status int8   `json:"status"`
	Remark string `json:"remark"`
}

// CreateDictType 创建字典类型
func (s *DictService) CreateDictType(req *CreateDictTypeRequest) (*model.DictType, error) {
	// 检查编码是否存在
	var count int64
	s.db.Model(&model.DictType{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		return nil, errors.New("字典类型编码已存在")
	}

	dictType := &model.DictType{
		Name:   req.Name,
		Code:   req.Code,
		Status: req.Status,
		Remark: req.Remark,
	}

	if err := s.db.Create(dictType).Error; err != nil {
		return nil, err
	}

	return dictType, nil
}

// UpdateDictTypeRequest 更新字典类型请求
type UpdateDictTypeRequest struct {
	ID     uint   `json:"id" validate:"required"`
	Name   string `json:"name"`
	Status int8   `json:"status"`
	Remark string `json:"remark"`
}

// UpdateDictType 更新字典类型
func (s *DictService) UpdateDictType(req *UpdateDictTypeRequest) error {
	updates := map[string]any{
		"name":   req.Name,
		"status": req.Status,
		"remark": req.Remark,
	}

	return s.db.Model(&model.DictType{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteDictType 删除字典类型
func (s *DictService) DeleteDictType(id uint) error {
	var dictType model.DictType
	if err := s.db.First(&dictType, id).Error; err != nil {
		return err
	}

	// 删除字典数据
	s.db.Where("type_code = ?", dictType.Code).Delete(&model.DictData{})
	// 删除字典类型
	return s.db.Delete(&dictType).Error
}

// GetDictType 获取字典类型详情
func (s *DictService) GetDictType(id uint) (*model.DictType, error) {
	var dictType model.DictType
	if err := s.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort ASC")
	}).First(&dictType, id).Error; err != nil {
		return nil, err
	}
	return &dictType, nil
}

// ListDictTypesRequest 字典类型列表请求
type ListDictTypesRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Status   *int8  `json:"status"`
}

// ListDictTypes 获取字典类型列表
func (s *DictService) ListDictTypes(req *ListDictTypesRequest) ([]model.DictType, int64, error) {
	var dictTypes []model.DictType
	var total int64

	query := s.db.Model(&model.DictType{})

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

// CreateDictDataRequest 创建字典数据请求
type CreateDictDataRequest struct {
	TypeCode  string `json:"typeCode" validate:"required"`
	Label     string `json:"label" validate:"required"`
	Value     string `json:"value" validate:"required"`
	Sort      int    `json:"sort"`
	Status    int8   `json:"status"`
	IsDefault bool   `json:"isDefault"`
	CssClass  string `json:"cssClass"`
	ListClass string `json:"listClass"`
	Remark    string `json:"remark"`
}

// CreateDictData 创建字典数据
func (s *DictService) CreateDictData(req *CreateDictDataRequest) (*model.DictData, error) {
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

	if err := s.db.Create(dictData).Error; err != nil {
		return nil, err
	}

	return dictData, nil
}

// UpdateDictDataRequest 更新字典数据请求
type UpdateDictDataRequest struct {
	ID        uint   `json:"id" validate:"required"`
	Label     string `json:"label"`
	Value     string `json:"value"`
	Sort      int    `json:"sort"`
	Status    int8   `json:"status"`
	IsDefault bool   `json:"isDefault"`
	CssClass  string `json:"cssClass"`
	ListClass string `json:"listClass"`
	Remark    string `json:"remark"`
}

// UpdateDictData 更新字典数据
func (s *DictService) UpdateDictData(req *UpdateDictDataRequest) error {
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

	return s.db.Model(&model.DictData{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteDictData 删除字典数据
func (s *DictService) DeleteDictData(id uint) error {
	return s.db.Delete(&model.DictData{}, id).Error
}

// GetDictDataByTypeCode 根据类型编码获取字典数据
func (s *DictService) GetDictDataByTypeCode(typeCode string) ([]model.DictData, error) {
	var dictData []model.DictData
	if err := s.db.Where("type_code = ? AND status = 1", typeCode).
		Order("sort ASC").
		Find(&dictData).Error; err != nil {
		return nil, err
	}
	return dictData, nil
}

// ListDictDataRequest 字典数据列表请求
type ListDictDataRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	TypeCode string `json:"typeCode"`
	Label    string `json:"label"`
	Status   *int8  `json:"status"`
}

// ListDictData 获取字典数据列表
func (s *DictService) ListDictData(req *ListDictDataRequest) ([]model.DictData, int64, error) {
	var dictData []model.DictData
	var total int64

	query := s.db.Model(&model.DictData{})

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

