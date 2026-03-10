package logic

import (
	"context"
	"errors"

	"yqhp/admin/internal/ctxutil"
	"yqhp/admin/internal/model"
	"yqhp/admin/internal/svc"
	"yqhp/admin/internal/types"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// JobLogic 定时任务逻辑
type JobLogic struct {
	ctx context.Context
}

// NewJobLogic 创建定时任务逻辑
func NewJobLogic(c *fiber.Ctx) *JobLogic {
	return &JobLogic{ctx: c.UserContext()}
}

func (l *JobLogic) db() *gorm.DB {
	return svc.Ctx.DB
}

// CreateJob 创建定时任务
func (l *JobLogic) CreateJob(req *types.CreateJobRequest) (*types.JobInfo, error) {
	var count int64
	l.db().Model(&model.SysJob{}).
		Where("handler_name = ? AND is_delete = ?", req.HandlerName, false).
		Count(&count)
	if count > 0 {
		return nil, errors.New("处理器名称已存在")
	}

	userID := ctxutil.GetUserID(l.ctx)
	job := &model.SysJob{
		Name:           req.Name,
		JobGroup:       model.StringPtr(req.JobGroup),
		HandlerName:    req.HandlerName,
		CronExpression: req.CronExpression,
		Params:         model.StringPtr(req.Params),
		Status:         model.Int32Ptr(req.Status),
		Source:         model.StringPtr("system"),
		MisfirePolicy:  model.Int32Ptr(req.MisfirePolicy),
		Concurrent:     model.Int32Ptr(req.Concurrent),
		RetryCount:     model.Int32Ptr(req.RetryCount),
		RetryInterval:  model.Int32Ptr(req.RetryInterval),
		Remark:         model.StringPtr(req.Remark),
		IsDelete:       model.BoolPtr(false),
		CreatedBy:      model.Int64Ptr(userID),
		UpdatedBy:      model.Int64Ptr(userID),
	}

	if err := l.db().Create(job).Error; err != nil {
		return nil, err
	}

	return types.ToJobInfo(job), nil
}

// UpdateJob 更新定时任务
func (l *JobLogic) UpdateJob(req *types.UpdateJobRequest) error {
	userID := ctxutil.GetUserID(l.ctx)
	return l.db().Model(&model.SysJob{}).Where("id = ?", req.ID).Updates(map[string]any{
		"name":            req.Name,
		"job_group":       req.JobGroup,
		"handler_name":    req.HandlerName,
		"cron_expression": req.CronExpression,
		"params":          req.Params,
		"misfire_policy":  req.MisfirePolicy,
		"concurrent":      req.Concurrent,
		"retry_count":     req.RetryCount,
		"retry_interval":  req.RetryInterval,
		"remark":          req.Remark,
		"updated_by":      userID,
	}).Error
}

// DeleteJob 删除定时任务
func (l *JobLogic) DeleteJob(id int64) error {
	return l.db().Model(&model.SysJob{}).Where("id = ?", id).Update("is_delete", true).Error
}

// GetJob 获取定时任务详情
func (l *JobLogic) GetJob(id int64) (*types.JobInfo, error) {
	var job model.SysJob
	if err := l.db().Where("id = ? AND is_delete = ?", id, false).First(&job).Error; err != nil {
		return nil, err
	}
	return types.ToJobInfo(&job), nil
}

// ChangeJobStatus 变更任务状态
func (l *JobLogic) ChangeJobStatus(id int64, status int32) error {
	userID := ctxutil.GetUserID(l.ctx)
	return l.db().Model(&model.SysJob{}).Where("id = ?", id).Updates(map[string]any{
		"status":     status,
		"updated_by": userID,
	}).Error
}

// ListJobs 获取定时任务列表
func (l *JobLogic) ListJobs(req *types.ListJobsRequest) ([]*types.JobInfo, int64, error) {
	q := l.db().Model(&model.SysJob{}).Where("is_delete = ?", false)

	if req.Name != "" {
		q = q.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.JobGroup != "" {
		q = q.Where("job_group = ?", req.JobGroup)
	}
	if req.Status != nil {
		q = q.Where("status = ?", *req.Status)
	}

	var total int64
	q.Count(&total)

	if req.Page > 0 && req.PageSize > 0 {
		q = q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	var jobs []*model.SysJob
	if err := q.Order("id DESC").Find(&jobs).Error; err != nil {
		return nil, 0, err
	}
	return types.ToJobInfoList(jobs), total, nil
}

// ListJobLogs 获取任务执行日志列表
func (l *JobLogic) ListJobLogs(req *types.ListJobLogsRequest) ([]*types.JobLogInfo, int64, error) {
	q := l.db().Model(&model.SysJobLog{})

	if req.JobID > 0 {
		q = q.Where("job_id = ?", req.JobID)
	}
	if req.JobName != "" {
		q = q.Where("job_name LIKE ?", "%"+req.JobName+"%")
	}
	if req.Status != nil {
		q = q.Where("status = ?", *req.Status)
	}

	var total int64
	q.Count(&total)

	if req.Page > 0 && req.PageSize > 0 {
		q = q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	var logs []*model.SysJobLog
	if err := q.Order("id DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return types.ToJobLogInfoList(logs), total, nil
}

// GetJobLog 获取任务日志详情
func (l *JobLogic) GetJobLog(id int64) (*types.JobLogInfo, error) {
	var log model.SysJobLog
	if err := l.db().Where("id = ?", id).First(&log).Error; err != nil {
		return nil, err
	}
	return types.ToJobLogInfo(&log), nil
}

// CleanJobLogs 清空任务日志
func (l *JobLogic) CleanJobLogs(jobID int64) error {
	q := l.db()
	if jobID > 0 {
		q = q.Where("job_id = ?", jobID)
	}
	return q.Delete(&model.SysJobLog{}).Error
}
