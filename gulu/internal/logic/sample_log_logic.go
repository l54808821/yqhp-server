package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/svc"
	"yqhp/gulu/internal/workflow"
)

// SampleLogLogic 采样日志逻辑
type SampleLogLogic struct {
	ctx context.Context
}

// NewSampleLogLogic 创建采样日志逻辑
func NewSampleLogLogic(ctx context.Context) *SampleLogLogic {
	return &SampleLogLogic{ctx: ctx}
}

// SampleLogQuery 采样日志查询参数
type SampleLogQuery struct {
	Page     int    `query:"page" json:"page"`
	PageSize int    `query:"pageSize" json:"pageSize"`
	StepID   string `query:"step_id" json:"step_id"`
	Status   string `query:"status" json:"status"`
	Keyword  string `query:"keyword" json:"keyword"`
}

// SampleLogResponse 采样日志响应
type SampleLogResponse struct {
	ID              int64             `json:"id"`
	ExecutionID     string            `json:"execution_id"`
	StepID          string            `json:"step_id"`
	StepName        string            `json:"step_name"`
	Timestamp       string            `json:"timestamp"`
	Status          string            `json:"status"`
	DurationMs      int64             `json:"duration_ms"`
	RequestMethod   string            `json:"request_method"`
	RequestURL      string            `json:"request_url"`
	RequestHeaders  map[string]string `json:"request_headers,omitempty"`
	RequestBody     string            `json:"request_body,omitempty"`
	ResponseStatus  int               `json:"response_status"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseBody    string            `json:"response_body,omitempty"`
	ErrorMessage    string            `json:"error_message,omitempty"`
}

// GetSampleLogs 获取采样日志（分页+搜索）
// 优先从数据库查询持久化的日志，如果没有则尝试从引擎内存获取
func (l *SampleLogLogic) GetSampleLogs(executionDBID int64, query *SampleLogQuery) ([]*SampleLogResponse, int64, error) {
	executionLogic := NewExecutionLogic(l.ctx)
	execution, err := executionLogic.GetByID(executionDBID)
	if err != nil {
		return nil, 0, errors.New("执行记录不存在")
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 15
	}

	// 尝试从数据库查询
	list, total, dbErr := l.queryFromDB(execution.ExecutionID, query)
	if dbErr == nil && total > 0 {
		return list, total, nil
	}

	// 回退到引擎内存查询
	return l.queryFromEngine(execution.ExecutionID, query)
}

// queryFromDB 从数据库查询采样日志
func (l *SampleLogLogic) queryFromDB(executionID string, q *SampleLogQuery) ([]*SampleLogResponse, int64, error) {
	db := svc.Ctx.DB
	qb := db.Model(&model.TSampleLog{}).Where("execution_id = ?", executionID)

	if q.StepID != "" {
		qb = qb.Where("step_id = ?", q.StepID)
	}
	if q.Status != "" {
		qb = qb.Where("status = ?", q.Status)
	}
	if q.Keyword != "" {
		keyword := "%" + q.Keyword + "%"
		qb = qb.Where("(request_url LIKE ? OR error_message LIKE ?)", keyword, keyword)
	}

	var total int64
	if err := qb.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return nil, 0, nil
	}

	var logs []model.TSampleLog
	offset := (q.Page - 1) * q.PageSize
	if err := qb.Order("id DESC").Offset(offset).Limit(q.PageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*SampleLogResponse, len(logs))
	for i, log := range logs {
		result[i] = l.convertToResponse(&log)
	}

	return result, total, nil
}

// queryFromEngine 从引擎内存查询采样日志
func (l *SampleLogLogic) queryFromEngine(executionID string, q *SampleLogQuery) ([]*SampleLogResponse, int64, error) {
	executionLogic := NewExecutionLogic(l.ctx)
	engineExecID := executionLogic.resolveEngineID(executionID)

	engine := workflow.GetEngine()
	if engine == nil {
		return nil, 0, nil
	}

	rawLogs, err := engine.GetSampleLogs(context.Background(), engineExecID)
	if err != nil || rawLogs == nil {
		return nil, 0, nil
	}

	logsJSON, err := json.Marshal(rawLogs)
	if err != nil {
		return nil, 0, nil
	}

	var entries []struct {
		ExecutionID     string            `json:"execution_id"`
		StepID          string            `json:"step_id"`
		StepName        string            `json:"step_name"`
		Timestamp       string            `json:"timestamp"`
		Status          string            `json:"status"`
		DurationMs      int64             `json:"duration_ms"`
		RequestMethod   string            `json:"request_method"`
		RequestURL      string            `json:"request_url"`
		RequestHeaders  map[string]string `json:"request_headers,omitempty"`
		RequestBody     string            `json:"request_body,omitempty"`
		ResponseStatus  int               `json:"response_status"`
		ResponseHeaders map[string]string `json:"response_headers,omitempty"`
		ResponseBody    string            `json:"response_body,omitempty"`
		ErrorMessage    string            `json:"error_message,omitempty"`
	}
	if err := json.Unmarshal(logsJSON, &entries); err != nil {
		return nil, 0, nil
	}

	// 客户端过滤
	var filtered []*SampleLogResponse
	for i, e := range entries {
		if q.StepID != "" && e.StepID != q.StepID {
			continue
		}
		if q.Status != "" && e.Status != q.Status {
			continue
		}
		if q.Keyword != "" {
			if !containsIgnoreCase(e.RequestURL, q.Keyword) && !containsIgnoreCase(e.ErrorMessage, q.Keyword) {
				continue
			}
		}
		filtered = append(filtered, &SampleLogResponse{
			ID:              int64(i + 1),
			ExecutionID:     e.ExecutionID,
			StepID:          e.StepID,
			StepName:        e.StepName,
			Timestamp:       e.Timestamp,
			Status:          e.Status,
			DurationMs:      e.DurationMs,
			RequestMethod:   e.RequestMethod,
			RequestURL:      e.RequestURL,
			RequestHeaders:  e.RequestHeaders,
			RequestBody:     e.RequestBody,
			ResponseStatus:  e.ResponseStatus,
			ResponseHeaders: e.ResponseHeaders,
			ResponseBody:    e.ResponseBody,
			ErrorMessage:    e.ErrorMessage,
		})
	}

	total := int64(len(filtered))

	// 分页
	offset := (q.Page - 1) * q.PageSize
	end := offset + q.PageSize
	if offset >= int(total) {
		return nil, total, nil
	}
	if end > int(total) {
		end = int(total)
	}

	return filtered[offset:end], total, nil
}

// SaveSampleLogs 批量保存采样日志到数据库
func (l *SampleLogLogic) SaveSampleLogs(logs []*model.TSampleLog) error {
	if len(logs) == 0 {
		return nil
	}

	db := svc.Ctx.DB
	return db.CreateInBatches(logs, 100).Error
}

// DeleteByExecutionID 删除指定执行的采样日志
func (l *SampleLogLogic) DeleteByExecutionID(executionID string) error {
	db := svc.Ctx.DB
	return db.Where("execution_id = ?", executionID).Delete(&model.TSampleLog{}).Error
}

func (l *SampleLogLogic) convertToResponse(log *model.TSampleLog) *SampleLogResponse {
	resp := &SampleLogResponse{
		ID:             log.ID,
		ExecutionID:    log.ExecutionID,
		StepID:         log.StepID,
		StepName:       log.StepName,
		Status:         log.Status,
		DurationMs:     log.DurationMs,
		RequestMethod:  log.RequestMethod,
		RequestURL:     log.RequestURL,
		ResponseStatus: log.ResponseStatus,
	}

	if log.Timestamp != nil {
		resp.Timestamp = log.Timestamp.Format("2006-01-02T15:04:05.000Z07:00")
	}
	if log.ErrorMessage != nil {
		resp.ErrorMessage = *log.ErrorMessage
	}
	if log.RequestBody != nil {
		resp.RequestBody = *log.RequestBody
	}
	if log.ResponseBody != nil {
		resp.ResponseBody = *log.ResponseBody
	}
	if log.RequestHeaders != nil {
		_ = json.Unmarshal([]byte(*log.RequestHeaders), &resp.RequestHeaders)
	}
	if log.ResponseHeaders != nil {
		_ = json.Unmarshal([]byte(*log.ResponseHeaders), &resp.ResponseHeaders)
	}

	return resp
}

func containsIgnoreCase(s, substr string) bool {
	if s == "" || substr == "" {
		return false
	}
	ls := strings.ToLower(s)
	lsub := strings.ToLower(substr)
	return strings.Contains(ls, lsub)
}

