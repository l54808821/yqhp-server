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
	// 文本嵌入模型
	EmbeddingModelID    *int64  `gorm:"column:embedding_model_id" json:"embedding_model_id"`
	EmbeddingModelName  *string `gorm:"column:embedding_model_name;type:varchar(100)" json:"embedding_model_name"`
	EmbeddingDimension  *int32  `gorm:"column:embedding_dimension;default:1536" json:"embedding_dimension"`
	// 多模态嵌入模型（Phase 2）
	MultimodalEnabled   *bool   `gorm:"column:multimodal_enabled;default:0" json:"multimodal_enabled"`
	MultimodalModelID   *int64  `gorm:"column:multimodal_model_id" json:"multimodal_model_id"`
	MultimodalModelName *string `gorm:"column:multimodal_model_name;type:varchar(100)" json:"multimodal_model_name"`
	MultimodalDimension *int32  `gorm:"column:multimodal_dimension" json:"multimodal_dimension"`
	// 分块配置
	ChunkSize           *int32   `gorm:"column:chunk_size;default:500" json:"chunk_size"`
	ChunkOverlap        *int32   `gorm:"column:chunk_overlap;default:50" json:"chunk_overlap"`
	// 检索配置
	SimilarityThreshold *float64 `gorm:"column:similarity_threshold;default:0.7" json:"similarity_threshold"`
	TopK                *int32   `gorm:"column:top_k;default:5" json:"top_k"`
	RetrievalMode       *string  `gorm:"column:retrieval_mode;type:varchar(20);default:vector" json:"retrieval_mode"`
	RerankModelID       *int64   `gorm:"column:rerank_model_id" json:"rerank_model_id"`
	RerankEnabled       *bool    `gorm:"column:rerank_enabled;default:0" json:"rerank_enabled"`
	// 向量库
	QdrantCollection    *string `gorm:"column:qdrant_collection;type:varchar(100)" json:"qdrant_collection"`
	// 图数据库（Phase 3）
	Neo4jDatabase       *string `gorm:"column:neo4j_database;type:varchar(100)" json:"neo4j_database"`
	GraphExtractModelID *int64  `gorm:"column:graph_extract_model_id" json:"graph_extract_model_id"`
	// 统计
	DocumentCount       *int32  `gorm:"column:document_count;default:0" json:"document_count"`
	ChunkCount          *int32  `gorm:"column:chunk_count;default:0" json:"chunk_count"`
	EntityCount         *int32  `gorm:"column:entity_count;default:0" json:"entity_count"`
	RelationCount       *int32  `gorm:"column:relation_count;default:0" json:"relation_count"`
	Metadata            *string `gorm:"column:metadata;type:json" json:"metadata"`
}

func (*TKnowledgeBase) TableName() string {
	return TableNameTKnowledgeBase
}
