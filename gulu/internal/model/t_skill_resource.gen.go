package model

import (
	"time"
)

const TableNameTSkillResource = "t_skill_resource"

// TSkillResource Skill 文件表
type TSkillResource struct {
	ID          int64      `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement:true" json:"id"`
	CreatedAt   *time.Time `gorm:"column:created_at;type:datetime" json:"created_at"`
	UpdatedAt   *time.Time `gorm:"column:updated_at;type:datetime" json:"updated_at"`
	SkillID     int64      `gorm:"column:skill_id;type:bigint unsigned;not null;index:idx_skill_resource_skill_id,priority:1;comment:所属Skill ID" json:"skill_id"`
	Path        string     `gorm:"column:path;type:varchar(500);not null;comment:相对路径" json:"path"`
	Content     *string    `gorm:"column:content;type:longtext;comment:文件内容" json:"content"`
	ContentType *string    `gorm:"column:content_type;type:varchar(100);default:text/plain;comment:MIME类型" json:"content_type"`
	Size        *int32     `gorm:"column:size;type:int;default:0;comment:文件大小" json:"size"`
}

func (*TSkillResource) TableName() string {
	return TableNameTSkillResource
}
