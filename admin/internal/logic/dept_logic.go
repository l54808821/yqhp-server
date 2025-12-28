package logic

import (
	"errors"

	"yqhp/admin/internal/model"
	"yqhp/admin/internal/types"

	"gorm.io/gorm"
)

// DeptLogic 部门逻辑
type DeptLogic struct {
	db *gorm.DB
}

// NewDeptLogic 创建部门逻辑
func NewDeptLogic(db *gorm.DB) *DeptLogic {
	return &DeptLogic{db: db}
}

// CreateDept 创建部门
func (l *DeptLogic) CreateDept(req *types.CreateDeptRequest) (*model.SysDept, error) {
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

	if err := l.db.Create(dept).Error; err != nil {
		return nil, err
	}

	return dept, nil
}

// UpdateDept 更新部门
func (l *DeptLogic) UpdateDept(req *types.UpdateDeptRequest) error {
	// 不能将自己设为父部门
	if req.ParentID == req.ID {
		return errors.New("不能将自己设为父部门")
	}

	updates := map[string]any{
		"parent_id": req.ParentID,
		"name":      req.Name,
		"code":      req.Code,
		"leader":    req.Leader,
		"phone":     req.Phone,
		"email":     req.Email,
		"sort":      req.Sort,
		"status":    req.Status,
		"remark":    req.Remark,
	}

	return l.db.Model(&model.SysDept{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteDept 删除部门（软删除）
func (l *DeptLogic) DeleteDept(id int64) error {
	// 检查是否有子部门
	var count int64
	l.db.Model(&model.SysDept{}).Where("parent_id = ? AND is_delete = ?", id, false).Count(&count)
	if count > 0 {
		return errors.New("该部门下有子部门，无法删除")
	}

	// 检查是否有用户
	l.db.Model(&model.SysUser{}).Where("dept_id = ? AND is_delete = ?", id, false).Count(&count)
	if count > 0 {
		return errors.New("该部门下有用户，无法删除")
	}

	return l.db.Model(&model.SysDept{}).Where("id = ?", id).Update("is_delete", true).Error
}

// GetDept 获取部门详情
func (l *DeptLogic) GetDept(id int64) (*model.SysDept, error) {
	var dept model.SysDept
	if err := l.db.Where("is_delete = ?", false).First(&dept, id).Error; err != nil {
		return nil, err
	}
	return &dept, nil
}

// GetDeptTree 获取部门树
func (l *DeptLogic) GetDeptTree() ([]model.DeptWithChildren, error) {
	var depts []model.SysDept
	if err := l.db.Where("is_delete = ?", false).Order("sort ASC").Find(&depts).Error; err != nil {
		return nil, err
	}

	return buildDeptTree(depts, 0), nil
}

// GetAllDepts 获取所有部门(平铺)
func (l *DeptLogic) GetAllDepts() ([]model.SysDept, error) {
	var depts []model.SysDept
	if err := l.db.Where("is_delete = ?", false).Order("sort ASC").Find(&depts).Error; err != nil {
		return nil, err
	}
	return depts, nil
}

// buildDeptTree 构建部门树
func buildDeptTree(depts []model.SysDept, parentID int64) []model.DeptWithChildren {
	var tree []model.DeptWithChildren
	for _, dept := range depts {
		deptParentID := model.GetInt64(dept.ParentID)
		if deptParentID == parentID {
			node := model.DeptWithChildren{
				SysDept: dept,
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
