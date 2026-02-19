package model

import "time"

const TableNameTKnowledgeRelation = "t_knowledge_relation"

// TKnowledgeRelation 知识图谱关系表
type TKnowledgeRelation struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CreatedAt       *time.Time `gorm:"column:created_at" json:"created_at"`
	KnowledgeBaseID int64      `gorm:"column:knowledge_base_id;not null;index:idx_rel_kb_id" json:"knowledge_base_id"`
	DocumentID      int64      `gorm:"column:document_id;not null;index:idx_rel_doc_id" json:"document_id"`
	SourceEntityID  int64      `gorm:"column:source_entity_id;not null;index:idx_rel_source" json:"source_entity_id"`
	TargetEntityID  int64      `gorm:"column:target_entity_id;not null;index:idx_rel_target" json:"target_entity_id"`
	RelationType    string     `gorm:"column:relation_type;type:varchar(100);not null;index:idx_rel_type" json:"relation_type"`
	Description     *string    `gorm:"column:description;type:text" json:"description"`
	Properties      *string    `gorm:"column:properties;type:json" json:"properties"`
	Neo4jRelID      *string    `gorm:"column:neo4j_rel_id;type:varchar(64)" json:"neo4j_rel_id"`
	Weight          float64    `gorm:"column:weight;default:1.0" json:"weight"`
}

func (*TKnowledgeRelation) TableName() string {
	return TableNameTKnowledgeRelation
}
