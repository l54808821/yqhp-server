package service

import (
	"errors"

	"yqhp/admin/internal/model"

	"gorm.io/gorm"
)

// DeptService 部门服务
type DeptService struct {
	db *gorm.DB
}

// NewDeptService 创建部门服务
func NewDeptService(db *gorm.DB) *DeptService {
	return &DeptService{db: db}
}

// CreateDeptRequest 创建部门请求
type CreateDeptRequest struct {
	ParentID uint   `json:"parentId"`
	Name     string `json:"name" validate:"required"`
	Code     string `json:"code"`
	Leader   string `json:"leader"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Sort     int    `json:"sort"`
	Status   int8   `json:"status"`
	Remark   string `json:"remark"`
}

// CreateDept 创建部门
func (s *DeptService) CreateDept(req *CreateDeptRequest) (*model.Dept, error) {
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

	if err := s.db.Create(dept).Error; err != nil {
		return nil, err
	}

	return dept, nil
}

// UpdateDeptRequest 更新部门请求
type UpdateDeptRequest struct {
	ID       uint   `json:"id" validate:"required"`
	ParentID uint   `json:"parentId"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Leader   string `json:"leader"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Sort     int    `json:"sort"`
	Status   int8   `json:"status"`
	Remark   string `json:"remark"`
}

// UpdateDept 更新部门
func (s *DeptService) UpdateDept(req *UpdateDeptRequest) error {
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

	return s.db.Model(&model.Dept{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteDept 删除部门
func (s *DeptService) DeleteDept(id uint) error {
	// 检查是否有子部门
	var count int64
	s.db.Model(&model.Dept{}).Where("parent_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("该部门下有子部门，无法删除")
	}

	// 检查是否有用户
	s.db.Model(&model.User{}).Where("dept_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("该部门下有用户，无法删除")
	}

	return s.db.Delete(&model.Dept{}, id).Error
}

// GetDept 获取部门详情
func (s *DeptService) GetDept(id uint) (*model.Dept, error) {
	var dept model.Dept
	if err := s.db.First(&dept, id).Error; err != nil {
		return nil, err
	}
	return &dept, nil
}

// GetDeptTree 获取部门树
func (s *DeptService) GetDeptTree() ([]model.Dept, error) {
	var depts []model.Dept
	if err := s.db.Order("sort ASC").Find(&depts).Error; err != nil {
		return nil, err
	}

	return buildDeptTree(depts, 0), nil
}

// GetAllDepts 获取所有部门(平铺)
func (s *DeptService) GetAllDepts() ([]model.Dept, error) {
	var depts []model.Dept
	if err := s.db.Order("sort ASC").Find(&depts).Error; err != nil {
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

