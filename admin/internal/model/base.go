package model

import (
	"time"

	"yqhp/common/types"

	"gorm.io/gorm"
)

// BaseModel 基础模型
type BaseModel struct {
	ID        uint            `gorm:"primarykey" json:"id"`
	CreatedAt types.DateTime  `json:"createdAt"`
	UpdatedAt types.DateTime  `json:"updatedAt"`
	DeletedAt gorm.DeletedAt  `gorm:"index" json:"-"`
}

// BeforeCreate GORM创建前钩子
func (m *BaseModel) BeforeCreate(tx *gorm.DB) error {
	now := types.NewDateTime(time.Now())
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	return nil
}

// BeforeUpdate GORM更新前钩子
func (m *BaseModel) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = types.NewDateTime(time.Now())
	return nil
}

// BaseModelWithUser 带用户信息的基础模型
type BaseModelWithUser struct {
	BaseModel
	CreatedBy uint `json:"createdBy"`
	UpdatedBy uint `json:"updatedBy"`
}
