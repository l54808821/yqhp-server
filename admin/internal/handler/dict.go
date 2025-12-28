package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// DictListTypes 获取字典类型列表
func DictListTypes(c *fiber.Ctx) error {
	var req types.ListDictTypesRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	dictTypes, total, err := logic.NewDictLogic(c).ListDictTypes(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, dictTypes, total, req.Page, req.PageSize)
}

// DictGetType 获取字典类型详情
func DictGetType(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	dictType, err := logic.NewDictLogic(c).GetDictType(id)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, dictType)
}

// DictCreateType 创建字典类型
func DictCreateType(c *fiber.Ctx) error {
	var req types.CreateDictTypeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" || req.Code == "" {
		return response.Error(c, "名称和编码不能为空")
	}

	dictType, err := logic.NewDictLogic(c).CreateDictType(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, dictType)
}

// DictUpdateType 更新字典类型
func DictUpdateType(c *fiber.Ctx) error {
	var req types.UpdateDictTypeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "ID不能为空")
	}

	if err := logic.NewDictLogic(c).UpdateDictType(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// DictDeleteType 删除字典类型
func DictDeleteType(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewDictLogic(c).DeleteDictType(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// DictListData 获取字典数据列表
func DictListData(c *fiber.Ctx) error {
	var req types.ListDictDataRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	data, total, err := logic.NewDictLogic(c).ListDictData(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, data, total, req.Page, req.PageSize)
}

// DictGetDataByTypeCode 根据类型编码获取字典数据
func DictGetDataByTypeCode(c *fiber.Ctx) error {
	typeCode := c.Params("typeCode")
	if typeCode == "" {
		return response.Error(c, "类型编码不能为空")
	}

	data, err := logic.NewDictLogic(c).GetDictDataByTypeCode(typeCode)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, data)
}

// DictCreateData 创建字典数据
func DictCreateData(c *fiber.Ctx) error {
	var req types.CreateDictDataRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.TypeCode == "" || req.Label == "" || req.Value == "" {
		return response.Error(c, "参数不完整")
	}

	data, err := logic.NewDictLogic(c).CreateDictData(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, data)
}

// DictUpdateData 更新字典数据
func DictUpdateData(c *fiber.Ctx) error {
	var req types.UpdateDictDataRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "ID不能为空")
	}

	if err := logic.NewDictLogic(c).UpdateDictData(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// DictDeleteData 删除字典数据
func DictDeleteData(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := logic.NewDictLogic(c).DeleteDictData(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
