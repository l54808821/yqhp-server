package logic

import (
	"context"
	"errors"

	"yqhp/admin/internal/model"
	"yqhp/admin/internal/query"
	"yqhp/admin/internal/svc"
	"yqhp/admin/internal/types"

	"github.com/gofiber/fiber/v2"
)

// DictLogic 字典逻辑
type DictLogic struct {
	ctx context.Context
}

// NewDictLogic 创建字典逻辑
func NewDictLogic(c *fiber.Ctx) *DictLogic {
	return &DictLogic{ctx: c.Context()}
}

func (l *DictLogic) db() *query.Query {
	return svc.Ctx.Query
}

// CreateDictType 创建字典类型
func (l *DictLogic) CreateDictType(req *types.CreateDictTypeRequest) (*types.DictTypeInfo, error) {
	dt := l.db().SysDictType
	count, _ := dt.WithContext(l.ctx).Where(dt.Code.Eq(req.Code), dt.IsDelete.Is(false)).Count()
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

	if err := dt.WithContext(l.ctx).Create(dictType); err != nil {
		return nil, err
	}

	return types.ToDictTypeInfo(dictType), nil
}

// UpdateDictType 更新字典类型
func (l *DictLogic) UpdateDictType(req *types.UpdateDictTypeRequest) error {
	dt := l.db().SysDictType
	_, err := dt.WithContext(l.ctx).Where(dt.ID.Eq(int64(req.ID))).Updates(map[string]any{
		"name":   req.Name,
		"status": req.Status,
		"remark": req.Remark,
	})
	return err
}

// DeleteDictType 删除字典类型
func (l *DictLogic) DeleteDictType(id int64) error {
	dt := l.db().SysDictType
	dictType, err := dt.WithContext(l.ctx).Where(dt.ID.Eq(id), dt.IsDelete.Is(false)).First()
	if err != nil {
		return err
	}

	dd := l.db().SysDictDatum
	dd.WithContext(l.ctx).Where(dd.TypeCode.Eq(dictType.Code), dd.IsDelete.Is(false)).Update(dd.IsDelete, true)

	_, err = dt.WithContext(l.ctx).Where(dt.ID.Eq(id)).Update(dt.IsDelete, true)
	return err
}

// GetDictType 获取字典类型详情
func (l *DictLogic) GetDictType(id int64) (*types.DictTypeInfo, error) {
	dt := l.db().SysDictType
	dictType, err := dt.WithContext(l.ctx).Where(dt.ID.Eq(id), dt.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}
	return types.ToDictTypeInfo(dictType), nil
}

// ListDictTypes 获取字典类型列表
func (l *DictLogic) ListDictTypes(req *types.ListDictTypesRequest) ([]*types.DictTypeInfo, int64, error) {
	dt := l.db().SysDictType
	q := dt.WithContext(l.ctx).Where(dt.IsDelete.Is(false))

	if req.Name != "" {
		q = q.Where(dt.Name.Like("%" + req.Name + "%"))
	}
	if req.Code != "" {
		q = q.Where(dt.Code.Like("%" + req.Code + "%"))
	}
	if req.Status != nil {
		q = q.Where(dt.Status.Eq(int32(*req.Status)))
	}

	total, _ := q.Count()

	if req.Page > 0 && req.PageSize > 0 {
		q = q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	dictTypes, err := q.Find()
	if err != nil {
		return nil, 0, err
	}
	return types.ToDictTypeInfoList(dictTypes), total, nil
}

// CreateDictData 创建字典数据
func (l *DictLogic) CreateDictData(req *types.CreateDictDataRequest) (*types.DictDataInfo, error) {
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

	if err := l.db().SysDictDatum.WithContext(l.ctx).Create(dictData); err != nil {
		return nil, err
	}

	return types.ToDictDataInfo(dictData), nil
}

// UpdateDictData 更新字典数据
func (l *DictLogic) UpdateDictData(req *types.UpdateDictDataRequest) error {
	dd := l.db().SysDictDatum
	_, err := dd.WithContext(l.ctx).Where(dd.ID.Eq(int64(req.ID))).Updates(map[string]any{
		"label":      req.Label,
		"value":      req.Value,
		"sort":       req.Sort,
		"status":     req.Status,
		"is_default": req.IsDefault,
		"css_class":  req.CssClass,
		"list_class": req.ListClass,
		"remark":     req.Remark,
	})
	return err
}

// DeleteDictData 删除字典数据
func (l *DictLogic) DeleteDictData(id int64) error {
	dd := l.db().SysDictDatum
	_, err := dd.WithContext(l.ctx).Where(dd.ID.Eq(id)).Update(dd.IsDelete, true)
	return err
}

// GetDictDataByTypeCode 根据类型编码获取字典数据
func (l *DictLogic) GetDictDataByTypeCode(typeCode string) ([]*types.DictDataInfo, error) {
	dd := l.db().SysDictDatum
	data, err := dd.WithContext(l.ctx).Where(dd.TypeCode.Eq(typeCode), dd.Status.Eq(1), dd.IsDelete.Is(false)).Order(dd.Sort).Find()
	if err != nil {
		return nil, err
	}
	return types.ToDictDataInfoList(data), nil
}

// ListDictData 获取字典数据列表
func (l *DictLogic) ListDictData(req *types.ListDictDataRequest) ([]*types.DictDataInfo, int64, error) {
	dd := l.db().SysDictDatum
	q := dd.WithContext(l.ctx).Where(dd.IsDelete.Is(false))

	if req.TypeCode != "" {
		q = q.Where(dd.TypeCode.Eq(req.TypeCode))
	}
	if req.Label != "" {
		q = q.Where(dd.Label.Like("%" + req.Label + "%"))
	}
	if req.Status != nil {
		q = q.Where(dd.Status.Eq(int32(*req.Status)))
	}

	total, _ := q.Count()

	if req.Page > 0 && req.PageSize > 0 {
		q = q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	data, err := q.Order(dd.Sort).Find()
	if err != nil {
		return nil, 0, err
	}
	return types.ToDictDataInfoList(data), total, nil
}
