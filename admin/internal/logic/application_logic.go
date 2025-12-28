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

// ApplicationLogic 应用逻辑
type ApplicationLogic struct {
	ctx context.Context
}

// NewApplicationLogic 创建应用逻辑
func NewApplicationLogic(c *fiber.Ctx) *ApplicationLogic {
	return &ApplicationLogic{ctx: c.Context()}
}

func (l *ApplicationLogic) db() *query.Query {
	return svc.Ctx.Query
}

// CreateApplication 创建应用
func (l *ApplicationLogic) CreateApplication(req *types.CreateApplicationRequest) (*types.AppInfo, error) {
	app := l.db().SysApplication
	count, _ := app.WithContext(l.ctx).Where(app.Code.Eq(req.Code), app.IsDelete.Is(false)).Count()
	if count > 0 {
		return nil, errors.New("应用编码已存在")
	}

	application := &model.SysApplication{
		Name:        req.Name,
		Code:        req.Code,
		Description: model.StringPtr(req.Description),
		Icon:        model.StringPtr(req.Icon),
		Sort:        model.Int64Ptr(int64(req.Sort)),
		Status:      model.Int32Ptr(int32(req.Status)),
		IsDelete:    model.BoolPtr(false),
	}

	if err := app.WithContext(l.ctx).Create(application); err != nil {
		return nil, err
	}

	return types.ToAppInfo(application), nil
}

// UpdateApplication 更新应用
func (l *ApplicationLogic) UpdateApplication(req *types.UpdateApplicationRequest) error {
	app := l.db().SysApplication
	_, err := app.WithContext(l.ctx).Where(app.ID.Eq(int64(req.ID))).Updates(map[string]any{
		"name":        req.Name,
		"description": req.Description,
		"icon":        req.Icon,
		"sort":        req.Sort,
		"status":      req.Status,
	})
	return err
}

// DeleteApplication 删除应用
func (l *ApplicationLogic) DeleteApplication(id int64) error {
	r := l.db().SysRole
	roleCount, _ := r.WithContext(l.ctx).Where(r.AppID.Eq(id), r.IsDelete.Is(false)).Count()
	if roleCount > 0 {
		return errors.New("该应用下有角色，无法删除")
	}

	res := l.db().SysResource
	resourceCount, _ := res.WithContext(l.ctx).Where(res.AppID.Eq(id), res.IsDelete.Is(false)).Count()
	if resourceCount > 0 {
		return errors.New("该应用下有资源，无法删除")
	}

	app := l.db().SysApplication
	_, err := app.WithContext(l.ctx).Where(app.ID.Eq(id)).Update(app.IsDelete, true)
	return err
}

// GetApplication 获取应用详情
func (l *ApplicationLogic) GetApplication(id int64) (*types.AppInfo, error) {
	app := l.db().SysApplication
	application, err := app.WithContext(l.ctx).Where(app.ID.Eq(id), app.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}
	return types.ToAppInfo(application), nil
}

// ListApplications 获取应用列表
func (l *ApplicationLogic) ListApplications(req *types.ListApplicationsRequest) ([]*types.AppInfo, int64, error) {
	app := l.db().SysApplication
	q := app.WithContext(l.ctx).Where(app.IsDelete.Is(false))

	if req.Name != "" {
		q = q.Where(app.Name.Like("%" + req.Name + "%"))
	}
	if req.Code != "" {
		q = q.Where(app.Code.Like("%" + req.Code + "%"))
	}
	if req.Status != nil {
		q = q.Where(app.Status.Eq(int32(*req.Status)))
	}

	total, _ := q.Count()

	if req.Page > 0 && req.PageSize > 0 {
		q = q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	apps, err := q.Order(app.Sort).Find()
	if err != nil {
		return nil, 0, err
	}
	return types.ToAppInfoList(apps), total, nil
}

// GetAllApplications 获取所有应用
func (l *ApplicationLogic) GetAllApplications() ([]*types.AppInfo, error) {
	app := l.db().SysApplication
	apps, err := app.WithContext(l.ctx).Where(app.Status.Eq(1), app.IsDelete.Is(false)).Order(app.Sort).Find()
	if err != nil {
		return nil, err
	}
	return types.ToAppInfoList(apps), nil
}
