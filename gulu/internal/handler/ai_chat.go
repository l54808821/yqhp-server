package handler

import (
	"bufio"
	"context"
	"fmt"
	"log"
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

	// 在进入 stream goroutine 之前，提前捕获所有需要的值
	// SetBodyStreamWriter 的回调在独立 goroutine 中运行，Fiber ctx 届时已失效
	apiBaseURL := aiModel.APIBaseURL
	apiKey := aiModel.APIKey
	modelID := aiModel.ModelID
	chatReq := req // 值拷贝

	// 设置 SSE 响应头
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	// 使用 fasthttp 的 StreamWriter 进行流式写入
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// recover 防止 panic 导致整个服务器崩溃
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[AiChatStream] recovered from panic: %v", r)
				errData := fmt.Sprintf("data: {\"error\": \"internal server error\"}\n\n")
				fmt.Fprint(w, errData)
				w.Flush()
			}
		}()

		// 使用独立的 background context，不依赖已失效的 Fiber ctx
		chatLogic := logic.NewAiChatLogic(context.Background())

		err := chatLogic.ChatStream(apiBaseURL, apiKey, modelID, &chatReq, func(data string) error {
			_, writeErr := fmt.Fprint(w, data)
			if writeErr != nil {
				return writeErr
			}
			return w.Flush()
		})

		if err != nil {
			errData := fmt.Sprintf("data: {\"error\": \"%s\"}\n\n", err.Error())
			fmt.Fprint(w, errData)
			w.Flush()
		}
	})

	return nil
}
