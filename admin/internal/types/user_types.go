package types

import "yqhp/admin/internal/model"

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username        string `json:"username" validate:"required"`
	Password        string `json:"password" validate:"required"`
	ConfirmPassword string `json:"confirmPassword" validate:"required"`
	Nickname        string `json:"nickname"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token    string         `json:"token"`
	UserInfo *model.SysUser `json:"userInfo"`
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Gender   int8   `json:"gender"`
	DeptID   uint   `json:"deptId"`
	RoleIDs  []uint `json:"roleIds"`
	Remark   string `json:"remark"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	ID       uint   `json:"id" validate:"required"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Gender   int8   `json:"gender"`
	DeptID   uint   `json:"deptId"`
	Status   int8   `json:"status"`
	RoleIDs  []uint `json:"roleIds"`
	Remark   string `json:"remark"`
}

// ListUsersRequest 用户列表请求
type ListUsersRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Phone    string `json:"phone"`
	Status   *int8  `json:"status"`
	DeptID   uint   `json:"deptId"`
}
