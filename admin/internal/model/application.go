package model

// Application 应用模型
type Application struct {
	BaseModel
	Name        string `gorm:"size:100;not null" json:"name"`
	Code        string `gorm:"size:50;uniqueIndex;not null" json:"code"`
	Description string `gorm:"size:500" json:"description"`
	Icon        string `gorm:"size:255" json:"icon"`
	Sort        int    `gorm:"default:0" json:"sort"`
	Status      int8   `gorm:"default:1" json:"status"` // 0:禁用 1:启用
}

// TableName 表名
func (Application) TableName() string {
	return "sys_application"
}

// 内置应用编码常量
const (
	AppCodeAdmin = "admin" // 后台管理系统
)
