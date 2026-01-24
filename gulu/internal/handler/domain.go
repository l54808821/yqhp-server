package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// DomainCreate 创建域名
// POST /api/domains
func DomainCreate(c *fiber.Ctx) error {
	var req logic.CreateDomainReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ProjectID <= 0 {
		return response.Error(c, "项目ID不能为空")
	}
	if req.EnvID <= 0 {
		return response.Error(c, "环境ID不能为空")
	}
	if req.Name == "" {
		return response.Error(c, "域名名称不能为空")
	}
	if req.Code == "" {
		return response.Error(c, "域名代码不能为空")
	}
	if req.BaseURL == "" {
		return response.Error(c, "基础URL不能为空")
	}

	domainLogic := logic.NewDomainLogic(c.UserContext())

	domain, err := domainLogic.Create(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, domain)
}

// DomainUpdate 更新域名
// PUT /api/domains/:id
func DomainUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的域名ID")
	}

	var req logic.UpdateDomainReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	domainLogic := logic.NewDomainLogic(c.UserContext())

	if err := domainLogic.Update(id, &req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// DomainDelete 删除域名
// DELETE /api/domains/:id
func DomainDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的域名ID")
	}

	domainLogic := logic.NewDomainLogic(c.UserContext())

	if err := domainLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// DomainGetByID 获取域名详情
// GET /api/domains/:id
func DomainGetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的域名ID")
	}

	domainLogic := logic.NewDomainLogic(c.UserContext())

	domain, err := domainLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "域名不存在")
	}

	return response.Success(c, domain)
}

// DomainList 获取域名列表
// GET /api/domains
func DomainList(c *fiber.Ctx) error {
	var req logic.DomainListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	domainLogic := logic.NewDomainLogic(c.UserContext())

	list, total, err := domainLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// DomainGetByEnvID 获取环境下所有域名
// GET /api/domains/env/:envId
func DomainGetByEnvID(c *fiber.Ctx) error {
	envID, err := strconv.ParseInt(c.Params("envId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的环境ID")
	}

	domainLogic := logic.NewDomainLogic(c.UserContext())

	list, err := domainLogic.GetDomainsByEnvID(envID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, list)
}
