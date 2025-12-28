package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	"yqhp/admin/internal/model"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// OperationLogMiddleware 操作日志中间件
func OperationLogMiddleware(db *gorm.DB, module, action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		startTime := time.Now()

		// 读取请求体
		bodyBytes, _ := io.ReadAll(bytes.NewReader(c.Body()))

		// 执行下一个处理器
		err := c.Next()

		// 计算耗时
		duration := time.Since(startTime).Milliseconds()

		// 获取用户信息
		userID := GetCurrentUserID(c)
		username := ""
		if user, ok := c.Locals("user").(*model.User); ok {
			username = user.Username
		}

		// 获取响应状态
		status := int8(1)
		errorMsg := ""
		if err != nil {
			status = 0
			errorMsg = err.Error()
		}

		// 创建操作日志
		log := &model.OperationLog{
			UserID:    userID,
			Username:  username,
			Module:    module,
			Action:    action,
			Method:    c.Method(),
			Path:      c.Path(),
			IP:        c.IP(),
			UserAgent: c.Get("User-Agent"),
			Params:    string(bodyBytes),
			Status:    status,
			Duration:  duration,
			ErrorMsg:  errorMsg,
		}

		// 异步保存日志
		go func() {
			db.Create(log)
		}()

		return err
	}
}

// LogOperation 记录操作日志(手动调用)
func LogOperation(db *gorm.DB, c *fiber.Ctx, module, action string, params any, result any, err error) {
	userID := GetCurrentUserID(c)
	username := ""
	if user, ok := c.Locals("user").(*model.User); ok {
		username = user.Username
	}

	status := int8(1)
	errorMsg := ""
	if err != nil {
		status = 0
		errorMsg = err.Error()
	}

	paramsStr := ""
	if params != nil {
		if bytes, err := json.Marshal(params); err == nil {
			paramsStr = string(bytes)
		}
	}

	resultStr := ""
	if result != nil {
		if bytes, err := json.Marshal(result); err == nil {
			resultStr = string(bytes)
		}
	}

	log := &model.OperationLog{
		UserID:    userID,
		Username:  username,
		Module:    module,
		Action:    action,
		Method:    c.Method(),
		Path:      c.Path(),
		IP:        c.IP(),
		UserAgent: c.Get("User-Agent"),
		Params:    paramsStr,
		Result:    resultStr,
		Status:    status,
		ErrorMsg:  errorMsg,
	}

	go func() {
		db.Create(log)
	}()
}

