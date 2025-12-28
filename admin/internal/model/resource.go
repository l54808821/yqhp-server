package model

// Resource 资源/菜单模型
type Resource struct {
	BaseModel
	AppID     uint       `gorm:"index;not null;default:0" json:"appId"` // 所属应用ID
	ParentID  uint       `json:"parentId"`
	Name      string     `gorm:"size:50;not null" json:"name"`
	Code      string     `gorm:"size:100;index" json:"code"`    // 权限标识
	Type      int8       `gorm:"not null" json:"type"`          // 1:目录 2:菜单 3:按钮
	Path      string     `gorm:"size:255" json:"path"`          // 路由路径
	Component string     `gorm:"size:255" json:"component"`     // 组件路径
	Redirect  string     `gorm:"size:255" json:"redirect"`      // 重定向路径
	Icon      string     `gorm:"size:100" json:"icon"`          // 图标
	Sort      int        `gorm:"default:0" json:"sort"`         // 排序
	IsHidden  bool       `gorm:"default:false" json:"isHidden"` // 是否隐藏
	IsCache   bool       `gorm:"default:true" json:"isCache"`   // 是否缓存
	IsFrame   bool       `gorm:"default:false" json:"isFrame"`  // 是否外链
	Status    int8       `gorm:"default:1" json:"status"`       // 0:禁用 1:启用
	Remark    string     `gorm:"size:500" json:"remark"`
	Children  []Resource `gorm:"-" json:"children,omitempty"`
}

// TableName 表名
func (Resource) TableName() string {
	return "sys_resource"
}
