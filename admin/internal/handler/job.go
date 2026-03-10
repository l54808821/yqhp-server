package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// JobList 获取定时任务列表
func JobList(c *fiber.Ctx) error {
	var req types.ListJobsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	jobs, total, err := logic.NewJobLogic(c).ListJobs(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, jobs, total, req.Page, req.PageSize)
}

// JobGet 获取定时任务详情
func JobGet(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	job, err := logic.NewJobLogic(c).GetJob(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, job)
}

// JobCreate 创建定时任务
func JobCreate(c *fiber.Ctx) error {
	var req types.CreateJobRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" || req.HandlerName == "" || req.CronExpression == "" {
		return response.Error(c, "任务名称、处理器名称和Cron表达式不能为空")
	}

	job, err := logic.NewJobLogic(c).CreateJob(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, job)
}

// JobUpdate 更新定时任务
func JobUpdate(c *fiber.Ctx) error {
	var req types.UpdateJobRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "ID不能为空")
	}

	if err := logic.NewJobLogic(c).UpdateJob(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// JobDelete 删除定时任务
func JobDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewJobLogic(c).DeleteJob(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// JobChangeStatus 变更任务状态
func JobChangeStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	var req types.ChangeJobStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if err := logic.NewJobLogic(c).ChangeJobStatus(id, req.Status); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// JobRunOnce 立即执行一次任务
func JobRunOnce(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	job, err := logic.NewJobLogic(c).GetJob(id)
	if err != nil {
		return response.Error(c, "任务不存在")
	}

	if svc := logic.GetScheduler(); svc != nil {
		if triggerErr := svc.TriggerOnce(job.HandlerName); triggerErr != nil {
			return response.Error(c, "触发执行失败: "+triggerErr.Error())
		}
	}

	return response.Success(c, nil)
}

// JobLogList 获取任务执行日志列表
func JobLogList(c *fiber.Ctx) error {
	var req types.ListJobLogsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	logs, total, err := logic.NewJobLogic(c).ListJobLogs(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, logs, total, req.Page, req.PageSize)
}

// JobLogGet 获取任务日志详情
func JobLogGet(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	log, err := logic.NewJobLogic(c).GetJobLog(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, log)
}

// JobLogClean 清空任务日志
func JobLogClean(c *fiber.Ctx) error {
	jobID, _ := strconv.ParseInt(c.Query("jobId"), 10, 64)

	if err := logic.NewJobLogic(c).CleanJobLogs(jobID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
