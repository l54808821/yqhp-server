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

// DeptLogic 部门逻辑
type DeptLogic struct {
	ctx context.Context
}

// NewDeptLogic 创建部门逻辑
func NewDeptLogic(c *fiber.Ctx) *DeptLogic {
	return &DeptLogic{ctx: c.Context()}
}

func (l *DeptLogic) db() *query.Query {
	return svc.Ctx.Query
}

// CreateDept 创建部门
func (l *DeptLogic) CreateDept(req *types.CreateDeptRequest) (*types.DeptInfo, error) {
	dept := &model.SysDept{
		ParentID: model.Int64Ptr(int64(req.ParentID)),
		Name:     req.Name,
		Code:     model.StringPtr(req.Code),
		Leader:   model.StringPtr(req.Leader),
		Phone:    model.StringPtr(req.Phone),
		Email:    model.StringPtr(req.Email),
		Sort:     model.Int64Ptr(int64(req.Sort)),
		Status:   model.Int32Ptr(int32(req.Status)),
		Remark:   model.StringPtr(req.Remark),
		IsDelete: model.BoolPtr(false),
	}

	if err := l.db().SysDept.WithContext(l.ctx).Create(dept); err != nil {
		return nil, err
	}

	return types.ToDeptInfo(dept), nil
}

// UpdateDept 更新部门
func (l *DeptLogic) UpdateDept(req *types.UpdateDeptRequest) error {
	if req.ParentID == req.ID {
		return errors.New("不能将自己设为父部门")
	}

	d := l.db().SysDept
	_, err := d.WithContext(l.ctx).Where(d.ID.Eq(int64(req.ID))).Updates(map[string]any{
		"parent_id": req.ParentID,
		"name":      req.Name,
		"code":      req.Code,
		"leader":    req.Leader,
		"phone":     req.Phone,
		"email":     req.Email,
		"sort":      req.Sort,
		"status":    req.Status,
		"remark":    req.Remark,
	})
	return err
}

// DeleteDept 删除部门
func (l *DeptLogic) DeleteDept(id int64) error {
	d := l.db().SysDept
	count, _ := d.WithContext(l.ctx).Where(d.ParentID.Eq(id), d.IsDelete.Is(false)).Count()
	if count > 0 {
		return errors.New("该部门下有子部门，无法删除")
	}

	u := l.db().SysUser
	count, _ = u.WithContext(l.ctx).Where(u.DeptID.Eq(id), u.IsDelete.Is(false)).Count()
	if count > 0 {
		return errors.New("该部门下有用户，无法删除")
	}

	_, err := d.WithContext(l.ctx).Where(d.ID.Eq(id)).Update(d.IsDelete, true)
	return err
}

// GetDept 获取部门详情
func (l *DeptLogic) GetDept(id int64) (*types.DeptInfo, error) {
	d := l.db().SysDept
	dept, err := d.WithContext(l.ctx).Where(d.ID.Eq(id), d.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}
	return types.ToDeptInfo(dept), nil
}

// GetDeptTree 获取部门树
func (l *DeptLogic) GetDeptTree() ([]types.DeptTreeInfo, error) {
	d := l.db().SysDept
	depts, err := d.WithContext(l.ctx).Where(d.IsDelete.Is(false)).Order(d.Sort).Find()
	if err != nil {
		return nil, err
	}

	return types.BuildDeptTree(depts, 0), nil
}

// GetAllDepts 获取所有部门
func (l *DeptLogic) GetAllDepts() ([]*types.DeptInfo, error) {
	d := l.db().SysDept
	depts, err := d.WithContext(l.ctx).Where(d.IsDelete.Is(false)).Order(d.Sort).Find()
	if err != nil {
		return nil, err
	}
	return types.ToDeptInfoList(depts), nil
}

// buildDeptTree 构建部门树
func buildDeptTree(depts []*model.SysDept, parentID int64) []model.DeptWithChildren {
	var tree []model.DeptWithChildren
	for _, dept := range depts {
		deptParentID := model.GetInt64(dept.ParentID)
		if deptParentID == parentID {
			node := model.DeptWithChildren{
				SysDept: *dept,
			}
			children := buildDeptTree(depts, dept.ID)
			if len(children) > 0 {
				node.Children = children
			}
			tree = append(tree, node)
		}
	}
	return tree
}
