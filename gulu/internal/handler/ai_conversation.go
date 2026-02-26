package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

// AIConversationCreate 创建会话
// POST /api/workflows/:id/conversations
func AIConversationCreate(c *fiber.Ctx) error {
	workflowID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的工作流ID")
	}

	var req logic.CreateConversationReq
	c.BodyParser(&req)

	userID := middleware.GetCurrentUserID(c)
	l := logic.NewAIConversationLogic(c.UserContext())

	conv, err := l.Create(workflowID, &req, userID)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, conv)
}

// AIConversationList 获取会话列表
// GET /api/workflows/:id/conversations
func AIConversationList(c *fiber.Ctx) error {
	workflowID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的工作流ID")
	}

	l := logic.NewAIConversationLogic(c.UserContext())
	list, err := l.List(workflowID)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, list)
}

// AIConversationGetDetail 获取会话详情（含消息）
// GET /api/conversations/:convId
func AIConversationGetDetail(c *fiber.Ctx) error {
	convID, err := strconv.ParseInt(c.Params("convId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的会话ID")
	}

	l := logic.NewAIConversationLogic(c.UserContext())
	detail, err := l.GetDetail(convID)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, detail)
}

// AIConversationDelete 删除会话
// DELETE /api/conversations/:convId
func AIConversationDelete(c *fiber.Ctx) error {
	convID, err := strconv.ParseInt(c.Params("convId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的会话ID")
	}

	l := logic.NewAIConversationLogic(c.UserContext())
	if err := l.Delete(convID); err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, nil)
}

// AIConversationUpdateTitle 修改会话标题
// PUT /api/conversations/:convId/title
func AIConversationUpdateTitle(c *fiber.Ctx) error {
	convID, err := strconv.ParseInt(c.Params("convId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的会话ID")
	}

	var req logic.UpdateTitleReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	l := logic.NewAIConversationLogic(c.UserContext())
	if err := l.UpdateTitle(convID, req.Title); err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, nil)
}

// AIConversationSaveMessage 保存消息
// POST /api/conversations/:convId/messages
func AIConversationSaveMessage(c *fiber.Ctx) error {
	convID, err := strconv.ParseInt(c.Params("convId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的会话ID")
	}

	var req logic.SaveMessageReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	l := logic.NewAIConversationLogic(c.UserContext())
	msg, err := l.SaveMessage(convID, &req)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, msg)
}
