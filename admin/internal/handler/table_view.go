package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// TableViewGet 获取表格视图列表
func TableViewGet(c *fiber.Ctx) error {
	tableKey := c.Params("tableKey")
	if tableKey == "" {
		return response.Error(c, "表格标识不能为空")
	}

	result, err := logic.NewTableViewLogic(c).GetViews(tableKey)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// TableViewSave 保存表格视图
func TableViewSave(c *fiber.Ctx) error {
	var req types.SaveTableViewRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.TableKey == "" || req.Name == "" {
		return response.Error(c, "参数不完整")
	}

	result, err := logic.NewTableViewLogic(c).SaveView(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// TableViewDelete 删除表格视图
func TableViewDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewTableViewLogic(c).DeleteView(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
