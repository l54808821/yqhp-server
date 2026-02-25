package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/svc"
	"yqhp/workflow-engine/pkg/types"
)

// PerfDetailLogic 性能测试执行详情逻辑
type PerfDetailLogic struct {
	ctx context.Context
}

func NewPerfDetailLogic(ctx context.Context) *PerfDetailLogic {
	return &PerfDetailLogic{ctx: ctx}
}

// SaveFromReport 从 PerformanceTestReport 拆解并保存到 detail 表
func (l *PerfDetailLogic) SaveFromReport(executionID string, report *types.PerformanceTestReport) error {
	now := time.Now()
	detail := &model.TExecutionPerfDetail{
		ExecutionID:        executionID,
		TotalRequests:      report.Summary.TotalRequests,
		SuccessRequests:    report.Summary.SuccessRequests,
		FailedRequests:     report.Summary.FailedRequests,
		ErrorRate:          report.Summary.ErrorRate,
		AvgQPS:             report.Summary.AvgQPS,
		PeakQPS:            report.Summary.PeakQPS,
		AvgRtMs:            report.Summary.AvgResponseTimeMs,
		MinRtMs:            report.Summary.MinResponseTimeMs,
		MaxRtMs:            report.Summary.MaxResponseTimeMs,
		P50RtMs:            report.Summary.P50ResponseTimeMs,
		P90RtMs:            report.Summary.P90ResponseTimeMs,
		P95RtMs:            report.Summary.P95ResponseTimeMs,
		P99RtMs:            report.Summary.P99ResponseTimeMs,
		MaxVUs:             report.Summary.MaxVUs,
		TotalIterations:    report.Summary.TotalIterations,
		ThroughputBPS:      report.Summary.ThroughputBytesPerSec,
		TotalDataSent:      report.Summary.TotalDataSent,
		TotalDataReceived:  report.Summary.TotalDataReceived,
		ThresholdsPassRate: report.Summary.ThresholdsPassRate,
		WorkflowName:       report.WorkflowName,
		CreatedAt:          &now,
		UpdatedAt:          &now,
	}

	if report.TimeSeries != nil {
		if j, err := json.Marshal(report.TimeSeries); err == nil {
			s := string(j)
			detail.TimeSeries = &s
		}
	}
	if report.StepDetails != nil {
		if j, err := json.Marshal(report.StepDetails); err == nil {
			s := string(j)
			detail.StepDetails = &s
		}
	}
	if report.Thresholds != nil {
		if j, err := json.Marshal(report.Thresholds); err == nil {
			s := string(j)
			detail.Thresholds = &s
		}
	}
	if report.ErrorAnalysis != nil {
		if j, err := json.Marshal(report.ErrorAnalysis); err == nil {
			s := string(j)
			detail.ErrorAnalysis = &s
		}
	}
	if report.VUTimeline != nil {
		if j, err := json.Marshal(report.VUTimeline); err == nil {
			s := string(j)
			detail.VUTimeline = &s
		}
	}
	if report.Config != nil {
		if j, err := json.Marshal(report.Config); err == nil {
			s := string(j)
			detail.Config = &s
		}
	}

	return svc.Ctx.DB.WithContext(l.ctx).Create(detail).Error
}

// GetByExecutionID 根据 execution_id 获取性能测试详情
func (l *PerfDetailLogic) GetByExecutionID(executionID string) (*model.TExecutionPerfDetail, error) {
	var detail model.TExecutionPerfDetail
	err := svc.Ctx.DB.WithContext(l.ctx).
		Where("execution_id = ?", executionID).
		First(&detail).Error
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

// AssembleReport 从主表执行记录 + detail 表组装完整的 PerformanceTestReport
func (l *PerfDetailLogic) AssembleReport(exec *model.TExecution, detail *model.TExecutionPerfDetail) *types.PerformanceTestReport {
	report := &types.PerformanceTestReport{
		ExecutionID:  exec.ExecutionID,
		WorkflowID:   formatInt64(exec.SourceID),
		WorkflowName: detail.WorkflowName,
		Status:       exec.Status,
		Summary: types.ReportSummary{
			TotalRequests:         detail.TotalRequests,
			SuccessRequests:       detail.SuccessRequests,
			FailedRequests:        detail.FailedRequests,
			ErrorRate:             detail.ErrorRate,
			AvgQPS:                detail.AvgQPS,
			PeakQPS:               detail.PeakQPS,
			AvgResponseTimeMs:     detail.AvgRtMs,
			MinResponseTimeMs:     detail.MinRtMs,
			MaxResponseTimeMs:     detail.MaxRtMs,
			P50ResponseTimeMs:     detail.P50RtMs,
			P90ResponseTimeMs:     detail.P90RtMs,
			P95ResponseTimeMs:     detail.P95RtMs,
			P99ResponseTimeMs:     detail.P99RtMs,
			MaxVUs:                detail.MaxVUs,
			TotalIterations:       detail.TotalIterations,
			ThroughputBytesPerSec: detail.ThroughputBPS,
			TotalDataSent:         detail.TotalDataSent,
			TotalDataReceived:     detail.TotalDataReceived,
			ThresholdsPassRate:    detail.ThresholdsPassRate,
		},
	}

	if exec.StartTime != nil {
		report.StartTime = *exec.StartTime
	}
	if exec.EndTime != nil {
		report.EndTime = *exec.EndTime
	}
	if exec.Duration != nil {
		report.Summary.TotalDurationMs = *exec.Duration
	}

	if detail.TimeSeries != nil {
		_ = json.Unmarshal([]byte(*detail.TimeSeries), &report.TimeSeries)
	}
	if detail.StepDetails != nil {
		_ = json.Unmarshal([]byte(*detail.StepDetails), &report.StepDetails)
	}
	if detail.Thresholds != nil {
		_ = json.Unmarshal([]byte(*detail.Thresholds), &report.Thresholds)
	}
	if detail.ErrorAnalysis != nil {
		_ = json.Unmarshal([]byte(*detail.ErrorAnalysis), &report.ErrorAnalysis)
	}
	if detail.VUTimeline != nil {
		_ = json.Unmarshal([]byte(*detail.VUTimeline), &report.VUTimeline)
	}
	if detail.Config != nil {
		_ = json.Unmarshal([]byte(*detail.Config), &report.Config)
	}

	return report
}

func formatInt64(v int64) string {
	return fmt.Sprintf("%d", v)
}
