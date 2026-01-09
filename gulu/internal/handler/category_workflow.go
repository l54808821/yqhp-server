package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// CategoryWorkflowHandler 工作流分类处理器
type CategoryWorkflowHandler struct{}

// NewCategoryWorkflowHandler 创建工作流分类处理器
func NewCategoryWorkflowHandler() *CategoryWorkflowHandler {
	return &CategoryWorkflowHandler{}
}

// Create 创建分类
// POST /api/projects/:projectId/categories
func (h *CategoryWorkflowHandler) Create(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("projectId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	var req logic.CreateCategoryReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "分类名称不能为空")
	}
	if req.Type == "" {
		return response.Error(c, "分类类型不能为空")
	}
	req.ProjectID = projectID

	categoryLogic := logic.NewCategoryWorkflowLogic(c.UserContext())

	category, err := categoryLogic.Create(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, category)
}

// Update 更新分类
// PUT /api/categories/:id
func (h *CategoryWorkflowHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的分类ID")
	}

	var req logic.UpdateCategoryReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	categoryLogic := logic.NewCategoryWorkflowLogic(c.UserContext())

	if err := categoryLogic.Update(id, &req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Delete 删除分类
// DELETE /api/categories/:id
func (h *CategoryWorkflowHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的分类ID")
	}

	categoryLogic := logic.NewCategoryWorkflowLogic(c.UserContext())

	if err := categoryLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// GetByID 获取分类详情
// GET /api/categories/:id
func (h *CategoryWorkflowHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的分类ID")
	}

	categoryLogic := logic.NewCategoryWorkflowLogic(c.UserContext())

	category, err := categoryLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "分类不存在")
	}

	return response.Success(c, category)
}

// GetTree 获取分类树
// GET /api/projects/:projectId/categories
func (h *CategoryWorkflowHandler) GetTree(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("projectId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	categoryLogic := logic.NewCategoryWorkflowLogic(c.UserContext())

	tree, err := categoryLogic.GetTree(projectID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, tree)
}

// Move 移动分类
// PUT /api/categories/:id/move
func (h *CategoryWorkflowHandler) Move(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的分类ID")
	}

	var req logic.MoveCategoryReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.TargetID <= 0 {
		return response.Error(c, "目标节点ID不能为空")
	}
	if req.Position != "before" && req.Position != "after" && req.Position != "inside" {
		return response.Error(c, "位置参数无效，必须是 before、after 或 inside")
	}

	categoryLogic := logic.NewCategoryWorkflowLogic(c.UserContext())

	if err := categoryLogic.Move(id, &req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// Search 搜索工作流
// GET /api/projects/:projectId/categories/search
func (h *CategoryWorkflowHandler) Search(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("projectId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的项目ID")
	}

	keyword := c.Query("keyword")
	if keyword == "" {
		return response.Error(c, "搜索关键词不能为空")
	}

	categoryLogic := logic.NewCategoryWorkflowLogic(c.UserContext())

	results, err := categoryLogic.Search(projectID, keyword)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, results)
}

// UpdateSort 批量更新排序
// PUT /api/categories/sort
func (h *CategoryWorkflowHandler) UpdateSort(c *fiber.Ctx) error {
	var req []struct {
		ID   int64 `json:"id"`
		Sort int32 `json:"sort"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	categoryLogic := logic.NewCategoryWorkflowLogic(c.UserContext())

	if err := categoryLogic.UpdateSort(req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
