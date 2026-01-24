package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

// ProjectCreate 创建项目
// POST /api/projects
func ProjectCreate(c *fiber.Ctx) error {
	var req logic.CreateProjectReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "项目名称不能为空")
	}
	if req.TeamID == 0 {
		return response.Error(c, "团队ID不能为空")
	}

	userID := middleware.GetCurrentUserID(c)
	projectLogic := logic.NewProjectLogic(c.UserContext())

	project, err := projectLogic.Create(&req, userID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, project)
}

// ProjectUpdate 更新项目
// PUT /api/projects/:id
func ProjectUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	var req logic.UpdateProjectReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	userID := middleware.GetCurrentUserID(c)
	projectLogic := logic.NewProjectLogic(c.UserContext())

	if err := projectLogic.Update(id, &req, userID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ProjectDelete 删除项目
// DELETE /api/projects/:id
func ProjectDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	projectLogic := logic.NewProjectLogic(c.UserContext())

	if err := projectLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ProjectGetByID 获取项目详情
// GET /api/projects/:id
func ProjectGetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	projectLogic := logic.NewProjectLogic(c.UserContext())

	project, err := projectLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "项目不存在")
	}

	return response.Success(c, project)
}

// ProjectList 获取项目列表
// GET /api/projects
func ProjectList(c *fiber.Ctx) error {
	var req logic.ProjectListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	projectLogic := logic.NewProjectLogic(c.UserContext())

	list, total, err := projectLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// ProjectUpdateStatus 更新项目状态
// PUT /api/projects/:id/status
func ProjectUpdateStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	var req struct {
		Status int32 `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	userID := middleware.GetCurrentUserID(c)
	projectLogic := logic.NewProjectLogic(c.UserContext())

	if err := projectLogic.Update(id, &logic.UpdateProjectReq{Status: req.Status}, userID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ProjectGetAll 获取所有启用的项目（用于下拉选择）
// GET /api/projects/all
func ProjectGetAll(c *fiber.Ctx) error {
	projectLogic := logic.NewProjectLogic(c.UserContext())

	list, err := projectLogic.GetAllProjects()
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, list)
}

// ProjectGetByTeamID 获取团队下的项目列表
// GET /api/teams/:id/projects
func ProjectGetByTeamID(c *fiber.Ctx) error {
	teamID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的团队ID")
	}

	projectLogic := logic.NewProjectLogic(c.UserContext())

	list, err := projectLogic.GetByTeamID(teamID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, list)
}

// ProjectCreateInTeam 在团队下创建项目
// POST /api/teams/:id/projects
func ProjectCreateInTeam(c *fiber.Ctx) error {
	teamID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的团队ID")
	}

	var req logic.CreateProjectReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "项目名称不能为空")
	}
	req.TeamID = teamID

	userID := middleware.GetCurrentUserID(c)
	projectLogic := logic.NewProjectLogic(c.UserContext())

	project, err := projectLogic.Create(&req, userID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, project)
}

// ProjectAddMember 添加项目成员
// POST /api/projects/:id/members
func ProjectAddMember(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	var req logic.AddProjectMemberReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.UserID == 0 {
		return response.Error(c, "用户ID不能为空")
	}

	memberLogic := logic.NewProjectMemberLogic(c.UserContext())

	member, err := memberLogic.AddMember(projectID, req.UserID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, member)
}

// ProjectRemoveMember 移除项目成员
// DELETE /api/projects/:id/members/:userId
func ProjectRemoveMember(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	userID, err := strconv.ParseInt(c.Params("userId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的用户ID")
	}

	memberLogic := logic.NewProjectMemberLogic(c.UserContext())

	if err := memberLogic.RemoveMember(projectID, userID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ProjectGetMembers 获取项目成员列表
// GET /api/projects/:id/members
func ProjectGetMembers(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	memberLogic := logic.NewProjectMemberLogic(c.UserContext())

	members, err := memberLogic.GetProjectMembers(projectID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, members)
}

// ProjectGrantPermission 授予权限
// POST /api/projects/:id/permissions
func ProjectGrantPermission(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	var req logic.GrantPermissionReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.UserID == 0 {
		return response.Error(c, "用户ID不能为空")
	}
	if req.PermissionCode == "" {
		return response.Error(c, "权限代码不能为空")
	}

	permLogic := logic.NewProjectPermissionLogic(c.UserContext())

	permission, err := permLogic.GrantPermission(projectID, req.UserID, req.PermissionCode)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, permission)
}

// ProjectRevokePermission 撤销权限
// DELETE /api/projects/:id/permissions/:userId/:code
func ProjectRevokePermission(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	userID, err := strconv.ParseInt(c.Params("userId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的用户ID")
	}

	code := c.Params("code")
	if code == "" {
		return response.Error(c, "权限代码不能为空")
	}

	permLogic := logic.NewProjectPermissionLogic(c.UserContext())

	if err := permLogic.RevokePermission(projectID, userID, code); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ProjectGetPermissions 获取项目权限列表
// GET /api/projects/:id/permissions
func ProjectGetPermissions(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	permLogic := logic.NewProjectPermissionLogic(c.UserContext())

	permissions, err := permLogic.GetProjectPermissions(projectID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, permissions)
}

// ProjectGetUserPermissions 获取用户在项目中的权限
// GET /api/projects/:id/permissions/user/:userId
func ProjectGetUserPermissions(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	userID, err := strconv.ParseInt(c.Params("userId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的用户ID")
	}

	permLogic := logic.NewProjectPermissionLogic(c.UserContext())

	permissions, err := permLogic.GetUserPermissions(projectID, userID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, permissions)
}
