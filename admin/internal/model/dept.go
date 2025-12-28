package model

// Dept 部门模型
type Dept struct {
	BaseModel
	ParentID uint   `json:"parentId"`
	Name     string `gorm:"size:50;not null" json:"name"`
	Code     string `gorm:"size:50" json:"code"`
	Leader   string `gorm:"size:50" json:"leader"`   // 负责人
	Phone    string `gorm:"size:20" json:"phone"`    // 联系电话
	Email    string `gorm:"size:100" json:"email"`   // 邮箱
	Sort     int    `gorm:"default:0" json:"sort"`   // 排序
	Status   int8   `gorm:"default:1" json:"status"` // 0:禁用 1:启用
	Remark   string `gorm:"size:500" json:"remark"`
	Children []Dept `gorm:"-" json:"children,omitempty"`
}

// TableName 表名
func (Dept) TableName() string {
	return "sys_dept"
}
