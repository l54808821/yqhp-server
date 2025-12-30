package logic

import (
	"context"
	"encoding/json"
	"errors"

	"yqhp/admin/internal/ctxutil"
	"yqhp/admin/internal/model"
	"yqhp/admin/internal/query"
	"yqhp/admin/internal/svc"
	"yqhp/admin/internal/types"

	"github.com/gofiber/fiber/v2"
)

// TableViewLogic 表格视图逻辑
type TableViewLogic struct {
	ctx   context.Context
	fiber *fiber.Ctx
}

// NewTableViewLogic 创建表格视图逻辑
func NewTableViewLogic(c *fiber.Ctx) *TableViewLogic {
	return &TableViewLogic{ctx: c.UserContext(), fiber: c}
}

// db 获取数据库实例
func (l *TableViewLogic) db() *query.Query {
	return svc.Ctx.Query
}

// GetViews 获取用户的表格视图列表（包含系统视图和用户视图）
func (l *TableViewLogic) GetViews(tableKey string) (*types.TableViewListResponse, error) {
	userID := ctxutil.GetUserID(l.ctx)
	if userID == 0 {
		return nil, errors.New("用户未登录")
	}

	tv := l.db().SysTableView

	// 查询系统视图(user_id=0)和当前用户的视图
	views, err := tv.WithContext(l.ctx).
		Where(tv.TableKey.Eq(tableKey), tv.IsDelete.Is(false)).
		Where(tv.WithContext(l.ctx).Or(tv.UserID.Eq(0)).Or(tv.UserID.Eq(userID))).
		Order(tv.UserID, tv.Sort, tv.ID).
		Find()
	if err != nil {
		return nil, err
	}

	result := &types.TableViewListResponse{
		Views: make([]types.TableViewInfo, 0, len(views)),
	}

	for _, v := range views {
		info := l.modelToInfo(v)
		result.Views = append(result.Views, info)
	}

	return result, nil
}

// SaveView 保存表格视图
func (l *TableViewLogic) SaveView(req *types.SaveTableViewRequest) (*types.TableViewInfo, error) {
	userID := ctxutil.GetUserID(l.ctx)
	if userID == 0 {
		return nil, errors.New("用户未登录")
	}

	tv := l.db().SysTableView

	// 序列化JSON字段
	columnsJSON, _ := json.Marshal(req.Columns)
	columnFixedJSON, _ := json.Marshal(req.ColumnFixed)
	searchParamsJSON, _ := json.Marshal(req.SearchParams)

	// 确定存储的 user_id：系统视图为0，用户视图为当前用户ID
	var storeUserID int64 = userID
	if req.IsSystem {
		storeUserID = 0
	}

	// 更新已有视图
	if req.ID > 0 {
		existing, err := tv.WithContext(l.ctx).
			Where(tv.ID.Eq(req.ID), tv.IsDelete.Is(false)).
			First()
		if err != nil {
			return nil, errors.New("视图不存在")
		}

		// 检查权限：只能修改自己的视图或系统视图（需要权限）
		if existing.UserID != 0 && existing.UserID != userID {
			return nil, errors.New("无权修改此视图")
		}

		updates := map[string]any{
			"name":          req.Name,
			"user_id":       storeUserID,
			"is_default":    req.IsDefault,
			"column_keys":   string(columnsJSON),
			"column_fixed":  string(columnFixedJSON),
			"search_params": string(searchParamsJSON),
			"updated_by":    userID,
		}
		_, err = tv.WithContext(l.ctx).Where(tv.ID.Eq(req.ID)).Updates(updates)
		if err != nil {
			return nil, err
		}

		existing.Name = req.Name
		existing.UserID = storeUserID
		existing.IsDefault = model.BoolPtr(req.IsDefault)
		existing.ColumnKeys = model.StringPtr(string(columnsJSON))
		existing.ColumnFixed = model.StringPtr(string(columnFixedJSON))
		existing.SearchParams = model.StringPtr(string(searchParamsJSON))
		info := l.modelToInfo(existing)
		return &info, nil
	}

	// 获取最大排序值
	var maxSort int32 = 0
	lastView, _ := tv.WithContext(l.ctx).
		Where(tv.UserID.Eq(storeUserID), tv.TableKey.Eq(req.TableKey), tv.IsDelete.Is(false)).
		Order(tv.Sort.Desc()).
		First()
	if lastView != nil && lastView.Sort != nil {
		maxSort = *lastView.Sort + 1
	}

	// 创建新视图
	newView := &model.SysTableView{
		UserID:       storeUserID,
		TableKey:     req.TableKey,
		Name:         req.Name,
		IsDefault:    model.BoolPtr(req.IsDefault),
		ColumnKeys:   model.StringPtr(string(columnsJSON)),
		ColumnFixed:  model.StringPtr(string(columnFixedJSON)),
		SearchParams: model.StringPtr(string(searchParamsJSON)),
		Sort:         model.Int32Ptr(maxSort),
		IsDelete:     model.BoolPtr(false),
		CreatedBy:    model.Int64Ptr(userID),
		UpdatedBy:    model.Int64Ptr(userID),
	}

	if err := tv.WithContext(l.ctx).Create(newView); err != nil {
		return nil, err
	}

	info := l.modelToInfo(newView)
	return &info, nil
}

// DeleteView 删除表格视图
func (l *TableViewLogic) DeleteView(id int64) error {
	userID := ctxutil.GetUserID(l.ctx)
	if userID == 0 {
		return errors.New("用户未登录")
	}

	tv := l.db().SysTableView

	view, err := tv.WithContext(l.ctx).
		Where(tv.ID.Eq(id), tv.IsDelete.Is(false)).
		First()
	if err != nil {
		return errors.New("视图不存在")
	}

	// 默认视图不能删除
	if model.GetBool(view.IsDefault) {
		return errors.New("默认视图不能删除")
	}

	// 检查权限：只能删除自己的视图或系统视图（需要权限）
	if view.UserID != 0 && view.UserID != userID {
		return errors.New("无权删除此视图")
	}

	_, err = tv.WithContext(l.ctx).
		Where(tv.ID.Eq(id)).
		Update(tv.IsDelete, true)

	return err
}

// SetDefaultView 设置默认视图
func (l *TableViewLogic) SetDefaultView(tableKey string, id int64) error {
	userID := ctxutil.GetUserID(l.ctx)
	if userID == 0 {
		return errors.New("用户未登录")
	}

	tv := l.db().SysTableView

	// 先取消该表格下当前用户的所有默认视图
	_, err := tv.WithContext(l.ctx).
		Where(tv.TableKey.Eq(tableKey), tv.IsDelete.Is(false)).
		Where(tv.WithContext(l.ctx).Or(tv.UserID.Eq(0)).Or(tv.UserID.Eq(userID))).
		Update(tv.IsDefault, false)
	if err != nil {
		return err
	}

	// 如果 id 为 0，表示清除默认视图，直接返回
	if id == 0 {
		return nil
	}

	// 检查视图是否存在
	view, err := tv.WithContext(l.ctx).
		Where(tv.ID.Eq(id), tv.TableKey.Eq(tableKey), tv.IsDelete.Is(false)).
		First()
	if err != nil {
		return errors.New("视图不存在")
	}

	// 检查权限：只能设置自己的视图或系统视图为默认
	if view.UserID != 0 && view.UserID != userID {
		return errors.New("无权操作此视图")
	}

	// 设置新的默认视图
	_, err = tv.WithContext(l.ctx).
		Where(tv.ID.Eq(id)).
		Update(tv.IsDefault, true)

	return err
}

// UpdateViewSort 更新视图排序
func (l *TableViewLogic) UpdateViewSort(tableKey string, viewIds []int64) error {
	userID := ctxutil.GetUserID(l.ctx)
	if userID == 0 {
		return errors.New("用户未登录")
	}

	tv := l.db().SysTableView

	// 按顺序更新排序值
	for i, id := range viewIds {
		_, err := tv.WithContext(l.ctx).
			Where(tv.ID.Eq(id), tv.TableKey.Eq(tableKey), tv.IsDelete.Is(false)).
			Update(tv.Sort, int32(i))
		if err != nil {
			return err
		}
	}

	return nil
}

// modelToInfo 模型转换为响应信息
func (l *TableViewLogic) modelToInfo(v *model.SysTableView) types.TableViewInfo {
	info := types.TableViewInfo{
		ID:        v.ID,
		Name:      v.Name,
		IsSystem:  v.UserID == 0,
		IsDefault: model.GetBool(v.IsDefault),
		Sort:      model.GetInt32(v.Sort),
		CreatedBy: model.GetInt64(v.CreatedBy),
	}

	if v.ColumnKeys != nil && *v.ColumnKeys != "" {
		json.Unmarshal([]byte(*v.ColumnKeys), &info.Columns)
	}
	if v.ColumnFixed != nil && *v.ColumnFixed != "" {
		json.Unmarshal([]byte(*v.ColumnFixed), &info.ColumnFixed)
	}
	if v.SearchParams != nil && *v.SearchParams != "" {
		json.Unmarshal([]byte(*v.SearchParams), &info.SearchParams)
	}

	return info
}
