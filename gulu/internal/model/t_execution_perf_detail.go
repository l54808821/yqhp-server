package model

import "time"

const TableNameTExecutionPerfDetail = "t_execution_perf_detail"

// TExecutionPerfDetail 性能测试执行详情表
type TExecutionPerfDetail struct {
	ID                 int64      `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement:true" json:"id"`
	ExecutionID        string     `gorm:"column:execution_id;type:varchar(100);not null;uniqueIndex:uk_execution_id" json:"execution_id"`
	TotalRequests      int64      `gorm:"column:total_requests;default:0" json:"total_requests"`
	SuccessRequests    int64      `gorm:"column:success_requests;default:0" json:"success_requests"`
	FailedRequests     int64      `gorm:"column:failed_requests;default:0" json:"failed_requests"`
	ErrorRate          float64    `gorm:"column:error_rate;default:0" json:"error_rate"`
	AvgQPS             float64    `gorm:"column:avg_qps;default:0" json:"avg_qps"`
	PeakQPS            float64    `gorm:"column:peak_qps;default:0" json:"peak_qps"`
	AvgRtMs            float64    `gorm:"column:avg_rt_ms;default:0" json:"avg_rt_ms"`
	MinRtMs            float64    `gorm:"column:min_rt_ms;default:0" json:"min_rt_ms"`
	MaxRtMs            float64    `gorm:"column:max_rt_ms;default:0" json:"max_rt_ms"`
	P50RtMs            float64    `gorm:"column:p50_rt_ms;default:0" json:"p50_rt_ms"`
	P90RtMs            float64    `gorm:"column:p90_rt_ms;default:0" json:"p90_rt_ms"`
	P95RtMs            float64    `gorm:"column:p95_rt_ms;default:0" json:"p95_rt_ms"`
	P99RtMs            float64    `gorm:"column:p99_rt_ms;default:0" json:"p99_rt_ms"`
	MaxVUs             int        `gorm:"column:max_vus;default:0" json:"max_vus"`
	TotalIterations    int64      `gorm:"column:total_iterations;default:0" json:"total_iterations"`
	ThroughputBPS      float64    `gorm:"column:throughput_bps;default:0" json:"throughput_bps"`
	TotalDataSent      int64      `gorm:"column:total_data_sent;default:0" json:"total_data_sent"`
	TotalDataReceived  int64      `gorm:"column:total_data_received;default:0" json:"total_data_received"`
	ThresholdsPassRate float64    `gorm:"column:thresholds_pass_rate;default:0" json:"thresholds_pass_rate"`
	TimeSeries         *string    `gorm:"column:time_series;type:longtext" json:"time_series,omitempty"`
	StepDetails        *string    `gorm:"column:step_details;type:longtext" json:"step_details,omitempty"`
	Thresholds         *string    `gorm:"column:thresholds;type:text" json:"thresholds,omitempty"`
	ErrorAnalysis      *string    `gorm:"column:error_analysis;type:text" json:"error_analysis,omitempty"`
	VUTimeline         *string    `gorm:"column:vu_timeline;type:text" json:"vu_timeline,omitempty"`
	Config             *string    `gorm:"column:config;type:text" json:"config,omitempty"`
	WorkflowName       string     `gorm:"column:workflow_name;type:varchar(256);default:''" json:"workflow_name"`
	CreatedAt          *time.Time `gorm:"column:created_at;type:datetime" json:"created_at"`
	UpdatedAt          *time.Time `gorm:"column:updated_at;type:datetime" json:"updated_at"`
}

func (*TExecutionPerfDetail) TableName() string {
	return TableNameTExecutionPerfDetail
}
