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
		if user, ok := c.Locals("user").(*model.SysUser); ok {
			username = user.Username
		}

		// 获取响应状态
		status := int32(1)
		errorMsg := ""
		if err != nil {
			status = 0
			errorMsg = err.Error()
		}

		method := c.Method()
		path := c.Path()
		ip := c.IP()
		userAgent := c.Get("User-Agent")
		params := string(bodyBytes)

		// 创建操作日志
		log := &model.SysOperationLog{
			UserID:    model.Int64Ptr(int64(userID)),
			Username:  model.StringPtr(username),
			Module:    model.StringPtr(module),
			Action:    model.StringPtr(action),
			Method:    model.StringPtr(method),
			Path:      model.StringPtr(path),
			IP:        model.StringPtr(ip),
			UserAgent: model.StringPtr(userAgent),
			Params:    model.StringPtr(params),
			Status:    model.Int32Ptr(status),
			Duration:  model.Int64Ptr(duration),
			ErrorMsg:  model.StringPtr(errorMsg),
			IsDelete:  model.BoolPtr(false),
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
	if user, ok := c.Locals("user").(*model.SysUser); ok {
		username = user.Username
	}

	status := int32(1)
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

	method := c.Method()
	path := c.Path()
	ip := c.IP()
	userAgent := c.Get("User-Agent")

	log := &model.SysOperationLog{
		UserID:    model.Int64Ptr(int64(userID)),
		Username:  model.StringPtr(username),
		Module:    model.StringPtr(module),
		Action:    model.StringPtr(action),
		Method:    model.StringPtr(method),
		Path:      model.StringPtr(path),
		IP:        model.StringPtr(ip),
		UserAgent: model.StringPtr(userAgent),
		Params:    model.StringPtr(paramsStr),
		Result:    model.StringPtr(resultStr),
		Status:    model.Int32Ptr(status),
		ErrorMsg:  model.StringPtr(errorMsg),
		IsDelete:  model.BoolPtr(false),
	}

	go func() {
		db.Create(log)
	}()
}
