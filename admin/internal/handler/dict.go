package handler

import (
	"strconv"

	"yqhp/admin/internal/logic"
	"yqhp/admin/internal/types"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// DictHandler 字典处理器
type DictHandler struct {
	dictLogic *logic.DictLogic
}

// NewDictHandler 创建字典处理器
func NewDictHandler(dictLogic *logic.DictLogic) *DictHandler {
	return &DictHandler{dictLogic: dictLogic}
}

// ListTypes 获取字典类型列表
func (h *DictHandler) ListTypes(c *fiber.Ctx) error {
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

	dictTypes, total, err := h.dictLogic.ListDictTypes(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, dictTypes, total, req.Page, req.PageSize)
}

// GetType 获取字典类型详情
func (h *DictHandler) GetType(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	dictType, err := h.dictLogic.GetDictType(uint(id))
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, dictType)
}

// CreateType 创建字典类型
func (h *DictHandler) CreateType(c *fiber.Ctx) error {
	var req types.CreateDictTypeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" || req.Code == "" {
		return response.Error(c, "名称和编码不能为空")
	}

	dictType, err := h.dictLogic.CreateDictType(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, dictType)
}

// UpdateType 更新字典类型
func (h *DictHandler) UpdateType(c *fiber.Ctx) error {
	var req types.UpdateDictTypeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "ID不能为空")
	}

	if err := h.dictLogic.UpdateDictType(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// DeleteType 删除字典类型
func (h *DictHandler) DeleteType(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := h.dictLogic.DeleteDictType(uint(id)); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// ListData 获取字典数据列表
func (h *DictHandler) ListData(c *fiber.Ctx) error {
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

	data, total, err := h.dictLogic.ListDictData(&req)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Page(c, data, total, req.Page, req.PageSize)
}

// GetDataByTypeCode 根据类型编码获取字典数据
func (h *DictHandler) GetDataByTypeCode(c *fiber.Ctx) error {
	typeCode := c.Params("typeCode")
	if typeCode == "" {
		return response.Error(c, "类型编码不能为空")
	}

	data, err := h.dictLogic.GetDictDataByTypeCode(typeCode)
	if err != nil {
		return response.Error(c, "获取失败")
	}

	return response.Success(c, data)
}

// CreateData 创建字典数据
func (h *DictHandler) CreateData(c *fiber.Ctx) error {
	var req types.CreateDictDataRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.TypeCode == "" || req.Label == "" || req.Value == "" {
		return response.Error(c, "参数不完整")
	}

	data, err := h.dictLogic.CreateDictData(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, data)
}

// UpdateData 更新字典数据
func (h *DictHandler) UpdateData(c *fiber.Ctx) error {
	var req types.UpdateDictDataRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.ID == 0 {
		return response.Error(c, "ID不能为空")
	}

	if err := h.dictLogic.UpdateDictData(&req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// DeleteData 删除字典数据
func (h *DictHandler) DeleteData(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "参数错误")
	}

	if err := h.dictLogic.DeleteDictData(uint(id)); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}
