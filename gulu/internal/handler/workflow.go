package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

// WorkflowCreate 创建工作流
// POST /api/workflows
func WorkflowCreate(c *fiber.Ctx) error {
	var req logic.CreateWorkflowReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败: "+err.Error())
	}

	if req.ProjectID <= 0 {
		return response.Error(c, "项目ID不能为空")
	}
	if req.Name == "" {
		return response.Error(c, "工作流名称不能为空")
	}
	if req.Definition == "" {
		return response.Error(c, "工作流定义不能为空")
	}

	userID := middleware.GetCurrentUserID(c)
	workflowLogic := logic.NewWorkflowLogic(c.UserContext())

	wf, err := workflowLogic.Create(&req, userID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, wf)
}

// WorkflowUpdate 更新工作流
// PUT /api/workflows/:id
func WorkflowUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的工作流ID")
	}

	var req logic.UpdateWorkflowReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	userID := middleware.GetCurrentUserID(c)
	workflowLogic := logic.NewWorkflowLogic(c.UserContext())

	if err := workflowLogic.Update(id, &req, userID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// WorkflowDelete 删除工作流
// DELETE /api/workflows/:id
func WorkflowDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的工作流ID")
	}

	workflowLogic := logic.NewWorkflowLogic(c.UserContext())

	if err := workflowLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// WorkflowGetByID 获取工作流详情
// GET /api/workflows/:id
func WorkflowGetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的工作流ID")
	}

	workflowLogic := logic.NewWorkflowLogic(c.UserContext())

	wf, err := workflowLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "工作流不存在")
	}

	return response.Success(c, wf)
}

// WorkflowList 获取工作流列表
// GET /api/workflows
func WorkflowList(c *fiber.Ctx) error {
	var req logic.WorkflowListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	workflowLogic := logic.NewWorkflowLogic(c.UserContext())

	list, total, err := workflowLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// WorkflowGetByProjectID 根据项目ID获取工作流列表
// GET /api/workflows/project/:projectId
func WorkflowGetByProjectID(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("projectId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	workflowLogic := logic.NewWorkflowLogic(c.UserContext())

	list, err := workflowLogic.GetByProjectID(projectID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, list)
}

// WorkflowCopy 复制工作流
// POST /api/workflows/:id/copy
func WorkflowCopy(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的工作流ID")
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "新工作流名称不能为空")
	}

	userID := middleware.GetCurrentUserID(c)
	workflowLogic := logic.NewWorkflowLogic(c.UserContext())

	wf, err := workflowLogic.Copy(id, req.Name, userID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, wf)
}

// WorkflowExportYAML 导出工作流为 YAML
// GET /api/workflows/:id/yaml
func WorkflowExportYAML(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的工作流ID")
	}

	workflowLogic := logic.NewWorkflowLogic(c.UserContext())

	yamlContent, err := workflowLogic.ExportYAML(id)
	if err != nil {
		return response.Error(c, err.Error())
	}

	c.Set("Content-Type", "application/x-yaml")
	c.Set("Content-Disposition", "attachment; filename=workflow.yaml")
	return c.SendString(yamlContent)
}

// WorkflowImportYAML 从 YAML 导入工作流
// POST /api/workflows/import
func WorkflowImportYAML(c *fiber.Ctx) error {
	var req struct {
		ProjectID   int64  `json:"project_id"`
		YAMLContent string `json:"yaml_content"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ProjectID <= 0 {
		return response.Error(c, "项目ID不能为空")
	}
	if req.YAMLContent == "" {
		return response.Error(c, "YAML 内容不能为空")
	}

	userID := middleware.GetCurrentUserID(c)
	workflowLogic := logic.NewWorkflowLogic(c.UserContext())

	wf, err := workflowLogic.ImportYAML(req.ProjectID, req.YAMLContent, userID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, wf)
}

// WorkflowValidate 验证工作流
// POST /api/workflows/:id/validate
func WorkflowValidate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的工作流ID")
	}

	workflowLogic := logic.NewWorkflowLogic(c.UserContext())

	result, err := workflowLogic.Validate(id)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// WorkflowValidateDefinition 验证工作流定义
// POST /api/workflows/validate
func WorkflowValidateDefinition(c *fiber.Ctx) error {
	var req struct {
		Definition string `json:"definition"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Definition == "" {
		return response.Error(c, "工作流定义不能为空")
	}

	workflowLogic := logic.NewWorkflowLogic(c.UserContext())

	result, err := workflowLogic.ValidateDefinition(req.Definition)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// WorkflowUpdateStatus 更新工作流状态
// PUT /api/workflows/:id/status
func WorkflowUpdateStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的工作流ID")
	}

	var req struct {
		Status int32 `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	userID := middleware.GetCurrentUserID(c)
	workflowLogic := logic.NewWorkflowLogic(c.UserContext())

	if err := workflowLogic.UpdateStatus(id, req.Status, userID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
