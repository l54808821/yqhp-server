package model

import (
	"encoding/json"
	"time"
)

const TableNameTKnowledgeBase = "t_knowledge_base"

// KBConfig 知识库配置（存入 config JSON 列）
// 包含分块参数、检索参数、以及自动检测到的向量维度缓存
type KBConfig struct {
	// 分块参数
	ChunkSize    int    `json:"chunk_size"`
	ChunkOverlap int    `json:"chunk_overlap"`
	// 检索参数
	SimilarityThreshold float64 `json:"similarity_threshold"`
	TopK                int     `json:"top_k"`
	RetrievalMode       string  `json:"retrieval_mode"`
	RerankEnabled       bool    `json:"rerank_enabled"`
	RerankModelID       *int64  `json:"rerank_model_id,omitempty"`
	// 向量维度（自动检测后写回的缓存，不需要用户配置）
	EmbeddingDimension  int `json:"embedding_dimension,omitempty"`
	MultimodalDimension int `json:"multimodal_dimension,omitempty"`
}

// DefaultKBConfig 返回知识库默认配置
func DefaultKBConfig() KBConfig {
	return KBConfig{
		ChunkSize:           500,
		ChunkOverlap:        50,
		SimilarityThreshold: 0.3,
		TopK:                5,
		RetrievalMode:       "vector",
	}
}

// TKnowledgeBase 知识库主表（精简版：配置信息合并到 config JSON）
type TKnowledgeBase struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CreatedAt   *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   *time.Time `gorm:"column:updated_at" json:"updated_at"`
	IsDelete    *bool      `gorm:"column:is_delete;default:0;index:idx_kb_is_delete" json:"is_delete"`
	CreatedBy   *int64     `gorm:"column:created_by" json:"created_by"`
	ProjectID   int64      `gorm:"column:project_id;not null;index:idx_kb_project_id" json:"project_id"`
	Name        string     `gorm:"column:name;type:varchar(100);not null" json:"name"`
	Description *string    `gorm:"column:description;type:varchar(1024)" json:"description"`
	Type        string     `gorm:"column:type;type:varchar(20);not null;default:normal;index:idx_kb_type" json:"type"`
	Status      *int32     `gorm:"column:status;default:1;index:idx_kb_status" json:"status"`

	// 模型（只存 ID）
	EmbeddingModelID    *int64 `gorm:"column:embedding_model_id" json:"embedding_model_id"`
	MultimodalEnabled   bool   `gorm:"column:multimodal_enabled;default:0" json:"multimodal_enabled"`
	MultimodalModelID   *int64 `gorm:"column:multimodal_model_id" json:"multimodal_model_id"`
	GraphExtractModelID *int64 `gorm:"column:graph_extract_model_id" json:"graph_extract_model_id"`

	// 向量库 / 图库
	QdrantCollection *string `gorm:"column:qdrant_collection;type:varchar(100)" json:"qdrant_collection"`
	Neo4jDatabase    *string `gorm:"column:neo4j_database;type:varchar(100)" json:"neo4j_database"`

	// 配置 JSON（分块 + 检索参数 + 维度缓存）
	ConfigJSON *string `gorm:"column:config;type:json" json:"-"`
}

func (*TKnowledgeBase) TableName() string {
	return TableNameTKnowledgeBase
}

// GetConfig 获取配置（如果 JSON 为空则返回默认值）
func (m *TKnowledgeBase) GetConfig() KBConfig {
	if m.ConfigJSON == nil || *m.ConfigJSON == "" {
		return DefaultKBConfig()
	}
	var cfg KBConfig
	if err := json.Unmarshal([]byte(*m.ConfigJSON), &cfg); err != nil {
		return DefaultKBConfig()
	}
	// 确保必要字段有默认值
	defaults := DefaultKBConfig()
	if cfg.ChunkSize == 0 {
		cfg.ChunkSize = defaults.ChunkSize
	}
	if cfg.ChunkOverlap == 0 {
		cfg.ChunkOverlap = defaults.ChunkOverlap
	}
	if cfg.SimilarityThreshold == 0 {
		cfg.SimilarityThreshold = defaults.SimilarityThreshold
	}
	if cfg.TopK == 0 {
		cfg.TopK = defaults.TopK
	}
	if cfg.RetrievalMode == "" {
		cfg.RetrievalMode = defaults.RetrievalMode
	}
	return cfg
}

// SetConfig 将配置序列化后存入 ConfigJSON
func (m *TKnowledgeBase) SetConfig(cfg KBConfig) {
	b, _ := json.Marshal(cfg)
	s := string(b)
	m.ConfigJSON = &s
}
