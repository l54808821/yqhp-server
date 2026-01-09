package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

// TeamHandler 团队处理器
type TeamHandler struct{}

// NewTeamHandler 创建团队处理器
func NewTeamHandler() *TeamHandler {
	return &TeamHandler{}
}

// Create 创建团队
// POST /api/teams
func (h *TeamHandler) Create(c *fiber.Ctx) error {
	var req logic.CreateTeamReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "团队名称不能为空")
	}

	userID := middleware.GetCurrentUserID(c)
	teamLogic := logic.NewTeamLogic(c.UserContext())

	team, err := teamLogic.Create(&req, userID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, team)
}

// Update 更新团队
// PUT /api/teams/:id
func (h *TeamHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的团队ID")
	}

	var req logic.UpdateTeamReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	userID := middleware.GetCurrentUserID(c)
	teamLogic := logic.NewTeamLogic(c.UserContext())

	if err := teamLogic.Update(id, &req, userID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Delete 删除团队
// DELETE /api/teams/:id
func (h *TeamHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的团队ID")
	}

	teamLogic := logic.NewTeamLogic(c.UserContext())

	if err := teamLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// GetByID 获取团队详情
// GET /api/teams/:id
func (h *TeamHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的团队ID")
	}

	teamLogic := logic.NewTeamLogic(c.UserContext())

	team, err := teamLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "团队不存在")
	}

	return response.Success(c, team)
}

// List 获取团队列表（管理员用）
// GET /api/teams
func (h *TeamHandler) List(c *fiber.Ctx) error {
	var req logic.TeamListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	teamLogic := logic.NewTeamLogic(c.UserContext())

	list, total, err := teamLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// GetUserTeams 获取当前用户的团队列表
// GET /api/teams/my
func (h *TeamHandler) GetUserTeams(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	teamLogic := logic.NewTeamLogic(c.UserContext())

	list, err := teamLogic.GetUserTeams(userID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, list)
}

// AddMember 添加团队成员
// POST /api/teams/:id/members
func (h *TeamHandler) AddMember(c *fiber.Ctx) error {
	teamID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的团队ID")
	}

	var req logic.AddMemberReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.UserID == 0 {
		return response.Error(c, "用户ID不能为空")
	}
	if req.Role == "" {
		req.Role = "member"
	}

	memberLogic := logic.NewTeamMemberLogic(c.UserContext())

	member, err := memberLogic.AddMember(teamID, req.UserID, req.Role)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, member)
}

// RemoveMember 移除团队成员
// DELETE /api/teams/:id/members/:userId
func (h *TeamHandler) RemoveMember(c *fiber.Ctx) error {
	teamID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的团队ID")
	}

	userID, err := strconv.ParseInt(c.Params("userId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的用户ID")
	}

	memberLogic := logic.NewTeamMemberLogic(c.UserContext())

	if err := memberLogic.RemoveMember(teamID, userID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// UpdateMemberRole 更新成员角色
// PUT /api/teams/:id/members/:userId/role
func (h *TeamHandler) UpdateMemberRole(c *fiber.Ctx) error {
	teamID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的团队ID")
	}

	userID, err := strconv.ParseInt(c.Params("userId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的用户ID")
	}

	var req logic.UpdateRoleReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Role == "" {
		return response.Error(c, "角色不能为空")
	}

	memberLogic := logic.NewTeamMemberLogic(c.UserContext())

	if err := memberLogic.UpdateRole(teamID, userID, req.Role); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// GetMembers 获取团队成员列表
// GET /api/teams/:id/members
func (h *TeamHandler) GetMembers(c *fiber.Ctx) error {
	teamID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的团队ID")
	}

	memberLogic := logic.NewTeamMemberLogic(c.UserContext())

	members, err := memberLogic.GetTeamMembers(teamID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, members)
}
