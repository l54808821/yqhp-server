package model

import "time"

const TableNameTKnowledgeQuery = "t_knowledge_query"

// TKnowledgeQuery 知识库查询历史表
type TKnowledgeQuery struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CreatedAt       *time.Time `gorm:"column:created_at" json:"created_at"`
	KnowledgeBaseID int64      `gorm:"column:knowledge_base_id;not null;index:idx_query_kb_id" json:"knowledge_base_id"`
	QueryText       string     `gorm:"column:query_text;type:text;not null" json:"query_text"`
	RetrievalMode   string     `gorm:"column:retrieval_mode;type:varchar(20);default:vector" json:"retrieval_mode"`
	TopK            int        `gorm:"column:top_k;default:5" json:"top_k"`
	ScoreThreshold  float64    `gorm:"column:score_threshold;default:0" json:"score_threshold"`
	ResultCount     int        `gorm:"column:result_count;default:0" json:"result_count"`
	Source          string     `gorm:"column:source;type:varchar(32);default:hit_testing" json:"source"`
	CreatedBy       *int64     `gorm:"column:created_by" json:"created_by"`
}

func (*TKnowledgeQuery) TableName() string {
	return TableNameTKnowledgeQuery
}
