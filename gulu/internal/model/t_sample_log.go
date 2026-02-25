package model

import "time"

const TableNameTSampleLog = "t_sample_log"

// TSampleLog 采样日志表
type TSampleLog struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ExecutionID     string     `gorm:"column:execution_id;type:varchar(64);not null;index:idx_sample_log_execution_id" json:"execution_id"`
	StepID          string     `gorm:"column:step_id;type:varchar(128);not null" json:"step_id"`
	StepName        string     `gorm:"column:step_name;type:varchar(256);default:''" json:"step_name"`
	Timestamp       *time.Time `gorm:"column:timestamp;type:datetime(3);not null" json:"timestamp"`
	Status          string     `gorm:"column:status;type:varchar(16);not null" json:"status"`
	DurationMs      int64      `gorm:"column:duration_ms;not null;default:0" json:"duration_ms"`
	RequestMethod   string     `gorm:"column:request_method;type:varchar(16);default:''" json:"request_method"`
	RequestURL      string     `gorm:"column:request_url;type:text" json:"request_url"`
	RequestHeaders  *string    `gorm:"column:request_headers;type:json" json:"request_headers"`
	RequestBody     *string    `gorm:"column:request_body;type:text" json:"request_body"`
	ResponseStatus  int        `gorm:"column:response_status;default:0" json:"response_status"`
	ResponseHeaders *string    `gorm:"column:response_headers;type:json" json:"response_headers"`
	ResponseBody    *string    `gorm:"column:response_body;type:text" json:"response_body"`
	ErrorMessage    *string    `gorm:"column:error_message;type:text" json:"error_message"`
}

func (*TSampleLog) TableName() string {
	return TableNameTSampleLog
}
