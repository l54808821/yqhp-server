package types

import "time"

// ReportType 报告类型
type ReportType string

const (
	// ReportTypeDebug 调试报告
	ReportTypeDebug ReportType = "debug"
	// ReportTypePerformance 性能测试报告
	ReportTypePerformance ReportType = "performance"
	// ReportTypeDataGeneration 数据生成报告
	ReportTypeDataGeneration ReportType = "data_generation"
)

// BaseReport 基础报告结构
type BaseReport struct {
	ExecutionID string     `json:"execution_id"`
	WorkflowID  int64      `json:"workflow_id"`
	Mode        string     `json:"mode"`
	Status      string     `json:"status"`
	StartTime   *time.Time `json:"start_time"`
	EndTime     *time.Time `json:"end_time"`
	Duration    int64      `json:"duration_ms"`
}

// DebugReport 调试报告
type DebugReport struct {
	BaseReport
	TotalSteps   int           `json:"total_steps"`
	SuccessSteps int           `json:"success_steps"`
	FailedSteps  int           `json:"failed_steps"`
	StepResults  []*StepResult `json:"step_results"`
}

// StepResult 步骤执行结果
type StepResult struct {
	StepID    string                 `json:"step_id"`
	StepName  string                 `json:"step_name"`
	Status    string                 `json:"status"`
	Duration  int64                  `json:"duration_ms"`
	Output    map[string]interface{} `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Logs      []string               `json:"logs,omitempty"`
	StartTime *time.Time             `json:"start_time"`
	EndTime   *time.Time             `json:"end_time"`
}

// PerformanceReport 性能测试报告
type PerformanceReport struct {
	BaseReport
	// 性能指标
	TotalRequests   int64   `json:"total_requests"`           // 总请求数
	SuccessRequests int64   `json:"success_requests"`         // 成功请求数
	FailedRequests  int64   `json:"failed_requests"`          // 失败请求数
	QPS             float64 `json:"qps"`                      // 每秒请求数
	AvgResponseTime float64 `json:"avg_response_time_ms"`     // 平均响应时间(毫秒)
	MinResponseTime float64 `json:"min_response_time_ms"`     // 最小响应时间(毫秒)
	MaxResponseTime float64 `json:"max_response_time_ms"`     // 最大响应时间(毫秒)
	P50ResponseTime float64 `json:"p50_response_time_ms"`     // P50响应时间(毫秒)
	P90ResponseTime float64 `json:"p90_response_time_ms"`     // P90响应时间(毫秒)
	P95ResponseTime float64 `json:"p95_response_time_ms"`     // P95响应时间(毫秒)
	P99ResponseTime float64 `json:"p99_response_time_ms"`     // P99响应时间(毫秒)
	ErrorRate       float64 `json:"error_rate"`               // 错误率(百分比)
	Throughput      float64 `json:"throughput_bytes_per_sec"` // 吞吐量(字节/秒)

	// 并发配置
	Concurrency int `json:"concurrency"` // 并发数
	Iterations  int `json:"iterations"`  // 迭代次数

	// 时间序列数据（用于图表）
	TimeSeriesData []*PerformanceTimePoint `json:"time_series_data,omitempty"`

	// 错误分布
	ErrorDistribution map[string]int64 `json:"error_distribution,omitempty"`
}

// PerformanceTimePoint 性能时间点数据
type PerformanceTimePoint struct {
	Timestamp    time.Time `json:"timestamp"`
	QPS          float64   `json:"qps"`
	ResponseTime float64   `json:"response_time_ms"`
	ErrorRate    float64   `json:"error_rate"`
	ActiveUsers  int       `json:"active_users"`
}

// DataGenerationReport 数据生成报告
type DataGenerationReport struct {
	BaseReport
	// 生成统计
	TotalRecords   int64   `json:"total_records"`   // 总记录数
	SuccessRecords int64   `json:"success_records"` // 成功记录数
	FailedRecords  int64   `json:"failed_records"`  // 失败记录数
	SuccessRate    float64 `json:"success_rate"`    // 成功率(百分比)

	// 生成速度
	RecordsPerSecond float64 `json:"records_per_second"` // 每秒生成记录数

	// 数据分布
	DataDistribution map[string]*DataTypeStats `json:"data_distribution,omitempty"`

	// 错误详情
	Errors []*DataGenerationError `json:"errors,omitempty"`
}

// DataTypeStats 数据类型统计
type DataTypeStats struct {
	TypeName string `json:"type_name"` // 数据类型名称
	Count    int64  `json:"count"`     // 生成数量
	Success  int64  `json:"success"`   // 成功数量
	Failed   int64  `json:"failed"`    // 失败数量
}

// DataGenerationError 数据生成错误
type DataGenerationError struct {
	RecordIndex int64     `json:"record_index"` // 记录索引
	ErrorType   string    `json:"error_type"`   // 错误类型
	ErrorMsg    string    `json:"error_msg"`    // 错误消息
	Timestamp   time.Time `json:"timestamp"`    // 发生时间
}

// ExecutionReport 统一执行报告（根据类型包含不同内容）
type ExecutionReport struct {
	Type                 ReportType            `json:"type"`
	DebugReport          *DebugReport          `json:"debug_report,omitempty"`
	PerformanceReport    *PerformanceReport    `json:"performance_report,omitempty"`
	DataGenerationReport *DataGenerationReport `json:"data_generation_report,omitempty"`
}

// NewDebugReport 创建调试报告
func NewDebugReport(executionID string, workflowID int64) *DebugReport {
	return &DebugReport{
		BaseReport: BaseReport{
			ExecutionID: executionID,
			WorkflowID:  workflowID,
			Mode:        "debug",
		},
		StepResults: make([]*StepResult, 0),
	}
}

// NewPerformanceReport 创建性能测试报告
func NewPerformanceReport(executionID string, workflowID int64) *PerformanceReport {
	return &PerformanceReport{
		BaseReport: BaseReport{
			ExecutionID: executionID,
			WorkflowID:  workflowID,
			Mode:        "execute",
		},
		TimeSeriesData:    make([]*PerformanceTimePoint, 0),
		ErrorDistribution: make(map[string]int64),
	}
}

// NewDataGenerationReport 创建数据生成报告
func NewDataGenerationReport(executionID string, workflowID int64) *DataGenerationReport {
	return &DataGenerationReport{
		BaseReport: BaseReport{
			ExecutionID: executionID,
			WorkflowID:  workflowID,
			Mode:        "execute",
		},
		DataDistribution: make(map[string]*DataTypeStats),
		Errors:           make([]*DataGenerationError, 0),
	}
}

// CalculatePerformanceMetrics 计算性能指标
func (r *PerformanceReport) CalculatePerformanceMetrics() {
	if r.TotalRequests == 0 {
		return
	}

	// 计算错误率
	r.ErrorRate = float64(r.FailedRequests) / float64(r.TotalRequests) * 100

	// 计算 QPS
	if r.Duration > 0 {
		r.QPS = float64(r.TotalRequests) / (float64(r.Duration) / 1000)
	}
}

// CalculateDataGenerationMetrics 计算数据生成指标
func (r *DataGenerationReport) CalculateDataGenerationMetrics() {
	if r.TotalRecords == 0 {
		return
	}

	// 计算成功率
	r.SuccessRate = float64(r.SuccessRecords) / float64(r.TotalRecords) * 100

	// 计算每秒生成记录数
	if r.Duration > 0 {
		r.RecordsPerSecond = float64(r.TotalRecords) / (float64(r.Duration) / 1000)
	}
}
