package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"
)

// ConfigDefinitionList 获取配置定义列表
func ConfigDefinitionList(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("projectId"), 10, 64)
	if err != nil {
		return response.Error(c, "项目ID无效")
	}

	configType := c.Query("type") // 可选参数，过滤配置类型

	definitions, err := logic.GetConfigDefinitionsByProject(c.Context(), projectID, configType)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, definitions)
}

// ConfigDefinitionCreate 创建配置定义
func ConfigDefinitionCreate(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("projectId"), 10, 64)
	if err != nil {
		return response.Error(c, "项目ID无效")
	}

	var req logic.CreateConfigDefinitionReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "请求参数无效")
	}

	req.ProjectID = projectID

	// 参数验证
	if req.Type == "" {
		return response.Error(c, "配置类型不能为空")
	}
	if req.Name == "" {
		return response.Error(c, "配置名称不能为空")
	}
	// key 可以为空，后端会自动生成

	// 默认状态为启用
	if req.Status == 0 {
		req.Status = 1
	}

	definition, err := logic.CreateConfigDefinition(c.Context(), &req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, definition)
}

// ConfigDefinitionUpdate 更新配置定义
func ConfigDefinitionUpdate(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return response.Error(c, "配置code无效")
	}

	var req logic.UpdateConfigDefinitionReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "请求参数无效")
	}

	definition, err := logic.UpdateConfigDefinition(c.Context(), code, &req)
	if err != nil {
		if err == logic.ErrConfigDefinitionNotFound {
			return response.NotFound(c, err.Error())
		}
		return response.Error(c, err.Error())
	}

	return response.Success(c, definition)
}

// ConfigDefinitionDelete 删除配置定义
func ConfigDefinitionDelete(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return response.Error(c, "配置code无效")
	}

	if err := logic.DeleteConfigDefinition(c.Context(), code); err != nil {
		if err == logic.ErrConfigDefinitionNotFound {
			return response.NotFound(c, err.Error())
		}
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ConfigDefinitionSort 更新配置定义排序
func ConfigDefinitionSort(c *fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("projectId"), 10, 64)
	if err != nil {
		return response.Error(c, "项目ID无效")
	}

	var req logic.UpdateConfigDefinitionSortReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "请求参数无效")
	}

	// 参数验证
	if req.Code == "" {
		return response.Error(c, "被移动项的code不能为空")
	}
	if req.TargetCode == "" {
		return response.Error(c, "目标位置的code不能为空")
	}
	if req.Position != "before" && req.Position != "after" {
		return response.Error(c, "position必须是before或after")
	}

	configType := c.Query("type")
	if configType == "" {
		return response.Error(c, "配置类型不能为空")
	}

	if err := logic.UpdateConfigDefinitionSort(c.Context(), projectID, configType, &req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
