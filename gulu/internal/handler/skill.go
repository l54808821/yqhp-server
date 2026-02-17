package handler

import (
	"io"
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// SkillCreate 创建Skill
// POST /api/skills
func SkillCreate(c *fiber.Ctx) error {
	var req logic.CreateSkillReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "Skill名称不能为空")
	}
	if req.SystemPrompt == "" {
		return response.Error(c, "系统提示词不能为空")
	}

	skillLogic := logic.NewSkillLogic(c.UserContext())

	result, err := skillLogic.Create(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// SkillUpdate 更新Skill
// PUT /api/skills/:id
func SkillUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的Skill ID")
	}

	var req logic.UpdateSkillReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	skillLogic := logic.NewSkillLogic(c.UserContext())

	if err := skillLogic.Update(id, &req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// SkillDelete 删除Skill
// DELETE /api/skills/:id
func SkillDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的Skill ID")
	}

	skillLogic := logic.NewSkillLogic(c.UserContext())

	if err := skillLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// SkillGetByID 获取Skill详情
// GET /api/skills/:id
func SkillGetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的Skill ID")
	}

	skillLogic := logic.NewSkillLogic(c.UserContext())

	result, err := skillLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "Skill不存在")
	}

	return response.Success(c, result)
}

// SkillList 获取Skill列表
// GET /api/skills
func SkillList(c *fiber.Ctx) error {
	var req logic.SkillListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	skillLogic := logic.NewSkillLogic(c.UserContext())

	list, total, err := skillLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// SkillUpdateStatus 更新Skill状态
// PUT /api/skills/:id/status
func SkillUpdateStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的Skill ID")
	}

	var req struct {
		Status int32 `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	skillLogic := logic.NewSkillLogic(c.UserContext())

	if err := skillLogic.UpdateStatus(id, req.Status); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// SkillGetCategories 获取分类列表
// GET /api/skills/categories
func SkillGetCategories(c *fiber.Ctx) error {
	skillLogic := logic.NewSkillLogic(c.UserContext())

	categories, err := skillLogic.GetCategories()
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, categories)
}

// SkillImport 导入 Skill（Agent Skills 标准 zip 格式）
// POST /api/skills/import
func SkillImport(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return response.Error(c, "请上传 zip 文件")
	}

	f, err := file.Open()
	if err != nil {
		return response.Error(c, "读取文件失败")
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return response.Error(c, "读取文件内容失败")
	}

	importLogic := logic.NewSkillImportExportLogic(c.UserContext())
	result, err := importLogic.Import(data)
	if err != nil {
		return response.Error(c, "导入失败: "+err.Error())
	}

	return response.Success(c, result)
}

// SkillExport 导出 Skill（Agent Skills 标准 zip 格式）
// GET /api/skills/:id/export
func SkillExport(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的Skill ID")
	}

	exportLogic := logic.NewSkillImportExportLogic(c.UserContext())
	zipData, filename, err := exportLogic.Export(id)
	if err != nil {
		return response.Error(c, "导出失败: "+err.Error())
	}

	c.Set("Content-Disposition", "attachment; filename="+filename)
	c.Set("Content-Type", "application/zip")
	return c.Send(zipData)
}

// SkillResourceList 获取 Skill 资源文件列表
// GET /api/skills/:id/resources
func SkillResourceList(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的Skill ID")
	}

	skillLogic := logic.NewSkillLogic(c.UserContext())
	resources, err := skillLogic.ListResources(id)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, resources)
}

// SkillResourceCreate 上传 Skill 资源文件
// POST /api/skills/:id/resources
func SkillResourceCreate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的Skill ID")
	}

	var req logic.CreateResourceReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Category == "" || req.Filename == "" {
		return response.Error(c, "资源类别和文件名不能为空")
	}
	if req.Category != "scripts" && req.Category != "references" && req.Category != "assets" {
		return response.Error(c, "资源类别必须为 scripts、references 或 assets")
	}

	skillLogic := logic.NewSkillLogic(c.UserContext())
	result, err := skillLogic.CreateResource(id, &req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// SkillResourceDelete 删除 Skill 资源文件
// DELETE /api/skills/:id/resources/:resourceId
func SkillResourceDelete(c *fiber.Ctx) error {
	resourceID, err := strconv.ParseInt(c.Params("resourceId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的资源ID")
	}

	skillLogic := logic.NewSkillLogic(c.UserContext())
	if err := skillLogic.DeleteResource(resourceID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
