package logic

import (
	"context"

	"yqhp/admin/internal/model"
	"yqhp/admin/internal/query"
	"yqhp/admin/internal/svc"
	"yqhp/admin/internal/types"

	"github.com/gofiber/fiber/v2"
)

// UserAppLogic 用户-应用关联逻辑
type UserAppLogic struct {
	ctx   context.Context
	fiber *fiber.Ctx
}

// NewUserAppLogic 创建用户-应用关联逻辑
func NewUserAppLogic(c *fiber.Ctx) *UserAppLogic {
	return &UserAppLogic{ctx: c.UserContext(), fiber: c}
}

func (l *UserAppLogic) db() *query.Query {
	return svc.Ctx.Query
}

// ListUserApps 获取用户-应用关联列表
func (l *UserAppLogic) ListUserApps(req *types.ListUserAppsRequest) ([]*types.UserAppInfo, int64, error) {
	ua := l.db().SysUserApp
	q := ua.WithContext(l.ctx).Where(ua.IsDelete.Is(false))

	if req.UserID > 0 {
		q = q.Where(ua.UserID.Eq(req.UserID))
	}
	if req.AppID > 0 {
		q = q.Where(ua.AppID.Eq(req.AppID))
	}
	if req.Source != "" {
		q = q.Where(ua.Source.Eq(req.Source))
	}
	if req.Status != nil {
		q = q.Where(ua.Status.Eq(int32(*req.Status)))
	}

	total, _ := q.Count()

	if req.Page > 0 && req.PageSize > 0 {
		q = q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	userApps, err := q.Order(ua.ID.Desc()).Find()
	if err != nil {
		return nil, 0, err
	}

	// 获取应用信息
	appMap := l.getAppMap(userApps)

	// 转换为响应类型
	list := make([]*types.UserAppInfo, len(userApps))
	for i, ua := range userApps {
		info := types.ToUserAppInfo(ua)
		if app, ok := appMap[ua.AppID]; ok {
			info.AppName = app.Name
			info.AppCode = app.Code
		}
		list[i] = info
	}

	return list, total, nil
}

// GetUserApps 获取用户的应用关联
func (l *UserAppLogic) GetUserApps(userID int64) ([]*types.UserAppInfo, error) {
	ua := l.db().SysUserApp
	userApps, err := ua.WithContext(l.ctx).Where(ua.UserID.Eq(userID), ua.IsDelete.Is(false)).Find()
	if err != nil {
		return nil, err
	}

	// 获取应用信息
	appMap := l.getAppMap(userApps)

	// 转换为响应类型
	list := make([]*types.UserAppInfo, len(userApps))
	for i, ua := range userApps {
		info := types.ToUserAppInfo(ua)
		if app, ok := appMap[ua.AppID]; ok {
			info.AppName = app.Name
			info.AppCode = app.Code
		}
		list[i] = info
	}

	return list, nil
}

// GetAppUsers 获取应用的用户关联
func (l *UserAppLogic) GetAppUsers(appID int64) ([]*types.UserAppInfo, error) {
	ua := l.db().SysUserApp
	userApps, err := ua.WithContext(l.ctx).Where(ua.AppID.Eq(appID), ua.IsDelete.Is(false)).Find()
	if err != nil {
		return nil, err
	}

	// 获取应用信息
	a := l.db().SysApplication
	app, _ := a.WithContext(l.ctx).Where(a.ID.Eq(appID)).First()

	// 转换为响应类型
	list := make([]*types.UserAppInfo, len(userApps))
	for i, ua := range userApps {
		info := types.ToUserAppInfo(ua)
		if app != nil {
			info.AppName = app.Name
			info.AppCode = app.Code
		}
		list[i] = info
	}

	return list, nil
}

// getAppMap 获取应用ID到应用的映射
func (l *UserAppLogic) getAppMap(userApps []*model.SysUserApp) map[int64]*model.SysApplication {
	appMap := make(map[int64]*model.SysApplication)
	if len(userApps) == 0 {
		return appMap
	}

	// 收集应用ID
	appIDSet := make(map[int64]struct{})
	for _, ua := range userApps {
		appIDSet[ua.AppID] = struct{}{}
	}

	if len(appIDSet) == 0 {
		return appMap
	}

	appIDs := make([]int64, 0, len(appIDSet))
	for id := range appIDSet {
		appIDs = append(appIDs, id)
	}

	// 查询应用
	a := l.db().SysApplication
	apps, _ := a.WithContext(l.ctx).Where(a.ID.In(appIDs...)).Find()
	for _, app := range apps {
		appMap[app.ID] = app
	}

	return appMap
}
