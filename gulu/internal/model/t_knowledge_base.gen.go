package model

import (
	"time"
)

const TableNameTKnowledgeBase = "t_knowledge_base"

// TKnowledgeBase 知识库主表
type TKnowledgeBase struct {
	ID                  int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CreatedAt           *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt           *time.Time `gorm:"column:updated_at" json:"updated_at"`
	IsDelete            *bool      `gorm:"column:is_delete;default:0;index:idx_kb_is_delete" json:"is_delete"`
	CreatedBy           *int64     `gorm:"column:created_by" json:"created_by"`
	UpdatedBy           *int64     `gorm:"column:updated_by" json:"updated_by"`
	ProjectID           int64      `gorm:"column:project_id;not null;index:idx_kb_project_id" json:"project_id"`
	Name                string     `gorm:"column:name;type:varchar(100);not null" json:"name"`
	Description         *string    `gorm:"column:description;type:varchar(1024)" json:"description"`
	Type                string     `gorm:"column:type;type:varchar(20);not null;default:normal;index:idx_kb_type" json:"type"`
	Status              *int32     `gorm:"column:status;default:1;index:idx_kb_status" json:"status"`
	EmbeddingModelID    *int64     `gorm:"column:embedding_model_id" json:"embedding_model_id"`
	EmbeddingModelName  *string    `gorm:"column:embedding_model_name;type:varchar(100)" json:"embedding_model_name"`
	EmbeddingDimension  *int32     `gorm:"column:embedding_dimension;default:1536" json:"embedding_dimension"`
	ChunkSize           *int32     `gorm:"column:chunk_size;default:500" json:"chunk_size"`
	ChunkOverlap        *int32     `gorm:"column:chunk_overlap;default:50" json:"chunk_overlap"`
	SimilarityThreshold *float64   `gorm:"column:similarity_threshold;default:0.7" json:"similarity_threshold"`
	TopK                *int32     `gorm:"column:top_k;default:5" json:"top_k"`
	RetrievalMode       *string    `gorm:"column:retrieval_mode;type:varchar(20);default:vector" json:"retrieval_mode"`
	RerankModelID       *int64     `gorm:"column:rerank_model_id" json:"rerank_model_id"`
	RerankEnabled       *bool      `gorm:"column:rerank_enabled;default:0" json:"rerank_enabled"`
	QdrantCollection    *string    `gorm:"column:qdrant_collection;type:varchar(100)" json:"qdrant_collection"`
	DocumentCount       *int32     `gorm:"column:document_count;default:0" json:"document_count"`
	ChunkCount          *int32     `gorm:"column:chunk_count;default:0" json:"chunk_count"`
	Metadata            *string    `gorm:"column:metadata;type:json" json:"metadata"`
}

func (*TKnowledgeBase) TableName() string {
	return TableNameTKnowledgeBase
}
