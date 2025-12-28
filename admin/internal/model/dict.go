package model

// DictType 字典类型模型
type DictType struct {
	BaseModel
	Name   string     `gorm:"size:100;not null" json:"name"`
	Code   string     `gorm:"size:100;uniqueIndex;not null" json:"code"`
	Status int8       `gorm:"default:1" json:"status"` // 0:禁用 1:启用
	Remark string     `gorm:"size:500" json:"remark"`
	Items  []DictData `gorm:"foreignKey:TypeCode;references:Code" json:"items,omitempty"`
}

// TableName 表名
func (DictType) TableName() string {
	return "sys_dict_type"
}

// DictData 字典数据模型
type DictData struct {
	BaseModel
	TypeCode  string `gorm:"size:100;index;not null" json:"typeCode"`
	Label     string `gorm:"size:100;not null" json:"label"`
	Value     string `gorm:"size:100;not null" json:"value"`
	Sort      int    `gorm:"default:0" json:"sort"`
	Status    int8   `gorm:"default:1" json:"status"` // 0:禁用 1:启用
	IsDefault bool   `gorm:"default:false" json:"isDefault"`
	CssClass  string `gorm:"size:100" json:"cssClass"`  // 样式类
	ListClass string `gorm:"size:100" json:"listClass"` // 列表样式
	Remark    string `gorm:"size:500" json:"remark"`
}

// TableName 表名
func (DictData) TableName() string {
	return "sys_dict_data"
}
