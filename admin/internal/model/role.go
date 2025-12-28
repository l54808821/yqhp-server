package model

// Role 角色模型
type Role struct {
	BaseModel
	AppID     uint       `gorm:"index;not null;default:0" json:"appId"` // 所属应用ID
	Name      string     `gorm:"size:50;not null" json:"name"`
	Code      string     `gorm:"size:50;not null;index" json:"code"`
	Sort      int        `gorm:"default:0" json:"sort"`
	Status    int8       `gorm:"default:1" json:"status"` // 0:禁用 1:启用
	Remark    string     `gorm:"size:500" json:"remark"`
	Resources []Resource `gorm:"many2many:sys_role_resource;" json:"resources"`
}

// TableName 表名
func (Role) TableName() string {
	return "sys_role"
}

// RoleResource 角色资源关联表
type RoleResource struct {
	RoleID     uint `gorm:"primaryKey"`
	ResourceID uint `gorm:"primaryKey"`
	IsDelete   bool `gorm:"default:false;index"`
}

// TableName 表名
func (RoleResource) TableName() string {
	return "sys_role_resource"
}
