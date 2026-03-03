package model

import "time"

const TableNameTAiProvider = "t_ai_provider"

// TAiProvider AI供应商表
type TAiProvider struct {
	ID          int64      `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement:true" json:"id"`
	CreatedAt   *time.Time `gorm:"column:created_at;type:datetime" json:"created_at"`
	UpdatedAt   *time.Time `gorm:"column:updated_at;type:datetime" json:"updated_at"`
	IsDelete    *bool      `gorm:"column:is_delete;type:tinyint(1)" json:"is_delete"`
	CreatedBy   *int64     `gorm:"column:created_by;type:bigint unsigned;comment:创建人ID" json:"created_by"`
	Name        string     `gorm:"column:name;type:varchar(100);not null;comment:供应商名称" json:"name"`
	ProviderType string   `gorm:"column:provider_type;type:varchar(50);not null;comment:供应商类型标识" json:"provider_type"`
	APIBaseURL  string     `gorm:"column:api_base_url;type:varchar(500);not null;comment:API Base URL" json:"api_base_url"`
	APIKey      string     `gorm:"column:api_key;type:varchar(500);comment:API Key" json:"api_key"`
	Icon        *string    `gorm:"column:icon;type:varchar(200);comment:供应商图标" json:"icon"`
	Description *string    `gorm:"column:description;type:varchar(500);comment:供应商描述" json:"description"`
	Sort        *int32     `gorm:"column:sort;type:int;comment:排序" json:"sort"`
	Status      *int32     `gorm:"column:status;type:tinyint;default:1;comment:状态: 1-启用 0-禁用" json:"status"`
}

func (*TAiProvider) TableName() string {
	return TableNameTAiProvider
}
