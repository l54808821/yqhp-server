package model

import (
	"yqhp/common/types"
)

// User 用户模型
type User struct {
	BaseModel
	Username    string              `gorm:"size:50;uniqueIndex;not null" json:"username"`
	Password    string              `gorm:"size:255;not null" json:"-"`
	Nickname    string              `gorm:"size:50" json:"nickname"`
	Avatar      string              `gorm:"size:255" json:"avatar"`
	Email       string              `gorm:"size:100" json:"email"`
	Phone       string              `gorm:"size:20" json:"phone"`
	Gender      int8                `gorm:"default:0" json:"gender"` // 0:未知 1:男 2:女
	Status      int8                `gorm:"default:1" json:"status"` // 0:禁用 1:启用
	DeptID      uint                `json:"deptId"`
	LastLoginAt *types.DateTime     `json:"lastLoginAt"`
	LastLoginIP string              `gorm:"size:50" json:"lastLoginIp"`
	Remark      string              `gorm:"size:500" json:"remark"`
	Roles       []Role              `gorm:"many2many:sys_user_role;" json:"roles"`
}

// TableName 表名
func (User) TableName() string {
	return "sys_user"
}

// UserRole 用户角色关联表
type UserRole struct {
	UserID uint `gorm:"primaryKey"`
	RoleID uint `gorm:"primaryKey"`
}

// TableName 表名
func (UserRole) TableName() string {
	return "sys_user_role"
}
