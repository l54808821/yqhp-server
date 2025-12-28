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
func (l *DeptLogic) CreateDept(req *types.CreateDeptRequest) (*model.Dept, error) {
	dept := &model.Dept{
		ParentID: req.ParentID,
		Name:     req.Name,
		Code:     req.Code,
		Leader:   req.Leader,
		Phone:    req.Phone,
		Email:    req.Email,
		Sort:     req.Sort,
		Status:   req.Status,
		Remark:   req.Remark,
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

	return l.db.Model(&model.Dept{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteDept 删除部门
func (l *DeptLogic) DeleteDept(id uint) error {
	// 检查是否有子部门
	var count int64
	l.db.Model(&model.Dept{}).Where("parent_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("该部门下有子部门，无法删除")
	}

	// 检查是否有用户
	l.db.Model(&model.User{}).Where("dept_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("该部门下有用户，无法删除")
	}

	return l.db.Delete(&model.Dept{}, id).Error
}

// GetDept 获取部门详情
func (l *DeptLogic) GetDept(id uint) (*model.Dept, error) {
	var dept model.Dept
	if err := l.db.First(&dept, id).Error; err != nil {
		return nil, err
	}
	return &dept, nil
}

// GetDeptTree 获取部门树
func (l *DeptLogic) GetDeptTree() ([]model.Dept, error) {
	var depts []model.Dept
	if err := l.db.Order("sort ASC").Find(&depts).Error; err != nil {
		return nil, err
	}

	return buildDeptTree(depts, 0), nil
}

// GetAllDepts 获取所有部门(平铺)
func (l *DeptLogic) GetAllDepts() ([]model.Dept, error) {
	var depts []model.Dept
	if err := l.db.Order("sort ASC").Find(&depts).Error; err != nil {
		return nil, err
	}
	return depts, nil
}

// buildDeptTree 构建部门树
func buildDeptTree(depts []model.Dept, parentID uint) []model.Dept {
	var tree []model.Dept
	for _, dept := range depts {
		if dept.ParentID == parentID {
			children := buildDeptTree(depts, dept.ID)
			if len(children) > 0 {
				dept.Children = children
			}
			tree = append(tree, dept)
		}
	}
	return tree
}
