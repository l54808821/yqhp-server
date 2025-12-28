package model

// SysConfig 系统参数配置模型
type SysConfig struct {
	BaseModel
	Name    string `gorm:"size:100;not null" json:"name"`
	Key     string `gorm:"size:100;uniqueIndex;not null" json:"key"`
	Value   string `gorm:"type:text" json:"value"`
	Type    string `gorm:"size:50" json:"type"` // 参数类型: string, number, boolean, json
	IsBuilt bool   `gorm:"default:false" json:"isBuilt"` // 是否内置
	Remark  string `gorm:"size:500" json:"remark"`
}

// TableName 表名
func (SysConfig) TableName() string {
	return "sys_config"
}

