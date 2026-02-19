package model

import "time"

const TableNameTKnowledgeSegment = "t_knowledge_segment"

// TKnowledgeSegment 知识库分块表
type TKnowledgeSegment struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CreatedAt       *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       *time.Time `gorm:"column:updated_at" json:"updated_at"`
	KnowledgeBaseID int64      `gorm:"column:knowledge_base_id;not null;index:idx_seg_kb_id" json:"knowledge_base_id"`
	DocumentID      int64      `gorm:"column:document_id;not null;index:idx_seg_doc_id" json:"document_id"`
	Content         string     `gorm:"column:content;type:text;not null" json:"content"`
	ContentType     string     `gorm:"column:content_type;type:varchar(20);not null;default:text;index:idx_seg_content_type" json:"content_type"`
	ImagePath       *string    `gorm:"column:image_path;type:varchar(512)" json:"image_path"`
	Position        int        `gorm:"column:position;not null;default:0" json:"position"`
	WordCount       int        `gorm:"column:word_count;default:0" json:"word_count"`
	Tokens          int        `gorm:"column:tokens;default:0" json:"tokens"`
	IndexNodeID     *string    `gorm:"column:index_node_id;type:varchar(64)" json:"index_node_id"`
	VectorField     string     `gorm:"column:vector_field;type:varchar(20);default:text" json:"vector_field"`
	Status          string     `gorm:"column:status;type:varchar(20);default:active;index:idx_seg_status" json:"status"`
	Enabled         bool       `gorm:"column:enabled;default:1" json:"enabled"`
	HitCount        int        `gorm:"column:hit_count;default:0" json:"hit_count"`
	Metadata        *string    `gorm:"column:metadata;type:json" json:"metadata"`
}

func (*TKnowledgeSegment) TableName() string {
	return TableNameTKnowledgeSegment
}
