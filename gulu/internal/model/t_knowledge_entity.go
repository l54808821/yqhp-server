package model

import "time"

const TableNameTKnowledgeEntity = "t_knowledge_entity"

// TKnowledgeEntity 知识图谱实体表
type TKnowledgeEntity struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CreatedAt       *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       *time.Time `gorm:"column:updated_at" json:"updated_at"`
	KnowledgeBaseID int64      `gorm:"column:knowledge_base_id;not null;index:idx_entity_kb_id" json:"knowledge_base_id"`
	DocumentID      int64      `gorm:"column:document_id;not null;index:idx_entity_doc_id" json:"document_id"`
	Name            string     `gorm:"column:name;type:varchar(255);not null;index:idx_entity_name" json:"name"`
	EntityType      string     `gorm:"column:entity_type;type:varchar(100);not null;index:idx_entity_type" json:"entity_type"`
	Description     *string    `gorm:"column:description;type:text" json:"description"`
	Properties      *string    `gorm:"column:properties;type:json" json:"properties"`
	Neo4jNodeID     *string    `gorm:"column:neo4j_node_id;type:varchar(64)" json:"neo4j_node_id"`
	MentionCount    int        `gorm:"column:mention_count;default:1" json:"mention_count"`
}

func (*TKnowledgeEntity) TableName() string {
	return TableNameTKnowledgeEntity
}
