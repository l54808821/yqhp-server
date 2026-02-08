package handler

import (
	"bufio"
	"fmt"
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// AiChatStream AI对话流式接口
// POST /api/ai-models/:id/chat
func AiChatStream(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的模型ID")
	}

	var req logic.ChatRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if len(req.Messages) == 0 {
		return response.Error(c, "消息不能为空")
	}

	// 获取模型信息（含API Key）
	aiModelLogic := logic.NewAiModelLogic(c.UserContext())
	aiModel, err := aiModelLogic.GetByIDWithKey(id)
	if err != nil {
		return response.NotFound(c, "AI模型不存在")
	}

	// 检查模型状态
	if aiModel.Status != nil && *aiModel.Status == 0 {
		return response.Error(c, "该模型已禁用")
	}

	// 设置 SSE 响应头
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	// 使用 fasthttp 的 StreamWriter 进行流式写入
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		chatLogic := logic.NewAiChatLogic(c.UserContext())

		err := chatLogic.ChatStream(aiModel.APIBaseURL, aiModel.APIKey, aiModel.ModelID, &req, func(data string) error {
			_, writeErr := fmt.Fprint(w, data)
			if writeErr != nil {
				return writeErr
			}
			return w.Flush()
		})

		if err != nil {
			// 发送错误事件
			errData := fmt.Sprintf("data: {\"error\": \"%s\"}\n\n", err.Error())
			fmt.Fprint(w, errData)
			w.Flush()
		}
	})

	return nil
}

