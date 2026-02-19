package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const TableNameTKnowledgeDocument = "t_knowledge_document"

// ChunkSetting 文档分段设置
type ChunkSetting struct {
	Separator       string `json:"separator"`
	ChunkSize       int    `json:"chunk_size"`
	ChunkOverlap    int    `json:"chunk_overlap"`
	CleanWhitespace bool   `json:"clean_whitespace"`
	RemoveURLs      bool   `json:"remove_urls"`
}

// Value 实现 driver.Valuer 接口
func (c ChunkSetting) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan 实现 sql.Scanner 接口
func (c *ChunkSetting) Scan(val interface{}) error {
	if val == nil {
		return nil
	}
	b, ok := val.([]byte)
	if !ok {
		return fmt.Errorf("ChunkSetting.Scan: unsupported type %T", val)
	}
	return json.Unmarshal(b, c)
}

// DefaultChunkSetting 返回默认分段设置
func DefaultChunkSetting() *ChunkSetting {
	return &ChunkSetting{
		Separator:       "\\n\\n",
		ChunkSize:       500,
		ChunkOverlap:    50,
		CleanWhitespace: true,
		RemoveURLs:      false,
	}
}

// TKnowledgeDocument 知识库文档表
type TKnowledgeDocument struct {
	ID                  int64         `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CreatedAt           *time.Time    `gorm:"column:created_at" json:"created_at"`
	UpdatedAt           *time.Time    `gorm:"column:updated_at" json:"updated_at"`
	KnowledgeBaseID     int64         `gorm:"column:knowledge_base_id;not null;index:idx_doc_kb_id" json:"knowledge_base_id"`
	Name                string        `gorm:"column:name;type:varchar(255);not null" json:"name"`
	FileType            *string       `gorm:"column:file_type;type:varchar(20)" json:"file_type"`
	FilePath            *string       `gorm:"column:file_path;type:varchar(512)" json:"file_path"`
	FileSize            *int64        `gorm:"column:file_size;default:0" json:"file_size"`
	WordCount           *int32        `gorm:"column:word_count;default:0" json:"word_count"`
	ImageCount          *int32        `gorm:"column:image_count;default:0" json:"image_count"`
	ChunkSetting        *ChunkSetting `gorm:"column:chunk_setting;type:json" json:"chunk_setting"`
	IndexingStatus      *string       `gorm:"column:indexing_status;type:varchar(32);default:waiting;index:idx_doc_indexing_status" json:"indexing_status"`
	ErrorMessage        *string       `gorm:"column:error_message;type:text" json:"error_message"`
	ChunkCount          *int32        `gorm:"column:chunk_count;default:0" json:"chunk_count"`
	TokenCount          *int32        `gorm:"column:token_count;default:0" json:"token_count"`
	ParsingCompletedAt  *time.Time    `gorm:"column:parsing_completed_at" json:"parsing_completed_at"`
	IndexingCompletedAt *time.Time    `gorm:"column:indexing_completed_at" json:"indexing_completed_at"`
}

func (*TKnowledgeDocument) TableName() string {
	return TableNameTKnowledgeDocument
}
