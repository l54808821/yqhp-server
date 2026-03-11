package handler

import (
	"io"
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// SkillCreate 创建Skill
func SkillCreate(c *fiber.Ctx) error {
	var req logic.CreateSkillReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if req.Name == "" {
		return response.Error(c, "Skill名称不能为空")
	}

	skillLogic := logic.NewSkillLogic(c.UserContext())
	result, err := skillLogic.Create(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, result)
}

// SkillUpdate 更新Skill
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

// SkillSearch 搜索 Skill（供 workflow-engine find_skills 工具内部调用）
// GET /api/skills/search?q=代码审查&category=编程&limit=10
func SkillSearch(c *fiber.Ctx) error {
	var req logic.SkillSearchReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	skillLogic := logic.NewSkillLogic(c.UserContext())
	list, err := skillLogic.Search(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, list)
}

// SkillGetBody 获取 Skill 摘要 + SKILL.md body（供 workflow-engine use_skill 工具调用）
// GET /api/skills/:id/body
func SkillGetBody(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的Skill ID")
	}
	skillLogic := logic.NewSkillLogic(c.UserContext())
	info, body, err := skillLogic.GetSkillBody(id)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, map[string]interface{}{
		"skill": info,
		"body":  body,
	})
}

// SkillUpdateStatus 更新Skill状态
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
func SkillGetCategories(c *fiber.Ctx) error {
	skillLogic := logic.NewSkillLogic(c.UserContext())
	categories, err := skillLogic.GetCategories()
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, categories)
}

// SkillImport 导入 Skill（Agent Skills 标准 zip 格式）
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

// SkillResourceList 获取 Skill 文件列表
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

// SkillResourceCreate 创建/更新 Skill 文件
func SkillResourceCreate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的Skill ID")
	}
	var req logic.CreateResourceReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if req.Path == "" {
		return response.Error(c, "文件路径不能为空")
	}
	skillLogic := logic.NewSkillLogic(c.UserContext())
	result, err := skillLogic.CreateResource(id, &req)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, result)
}

// SkillResourceGetContent 获取 Skill 文件内容
func SkillResourceGetContent(c *fiber.Ctx) error {
	resourceID, err := strconv.ParseInt(c.Params("resourceId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的资源ID")
	}
	skillLogic := logic.NewSkillLogic(c.UserContext())
	content, err := skillLogic.GetResourceContent(resourceID)
	if err != nil {
		return response.Error(c, "资源不存在")
	}
	return response.Success(c, map[string]string{"content": content})
}

// SkillResourceUpdate 更新 Skill 文件内容
func SkillResourceUpdate(c *fiber.Ctx) error {
	resourceID, err := strconv.ParseInt(c.Params("resourceId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的资源ID")
	}
	var req logic.UpdateResourceReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	skillLogic := logic.NewSkillLogic(c.UserContext())
	if err := skillLogic.UpdateResource(resourceID, &req); err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, nil)
}

// SkillResourceDelete 删除 Skill 文件
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
