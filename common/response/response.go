package response

import (
	"github.com/gofiber/fiber/v2"
)

// Response 统一响应结构
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// PageData 分页数据结构
type PageData struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// 响应码定义
const (
	CodeSuccess      = 0
	CodeError        = -1
	CodeUnauthorized = 401
	CodeForbidden    = 403
	CodeNotFound     = 404
	CodeServerError  = 500
)

// 响应消息定义
const (
	MsgSuccess      = "success"
	MsgError        = "error"
	MsgUnauthorized = "unauthorized"
	MsgForbidden    = "forbidden"
	MsgNotFound     = "not found"
	MsgServerError  = "server error"
)

// Success 成功响应
func Success(c *fiber.Ctx, data any) error {
	return c.JSON(Response{
		Code:    CodeSuccess,
		Message: MsgSuccess,
		Data:    data,
	})
}

// SuccessWithMessage 成功响应带消息
func SuccessWithMessage(c *fiber.Ctx, message string, data any) error {
	return c.JSON(Response{
		Code:    CodeSuccess,
		Message: message,
		Data:    data,
	})
}

// Error 错误响应
func Error(c *fiber.Ctx, message string) error {
	return c.JSON(Response{
		Code:    CodeError,
		Message: message,
	})
}

// ErrorWithCode 错误响应带错误码
func ErrorWithCode(c *fiber.Ctx, code int, message string) error {
	return c.JSON(Response{
		Code:    code,
		Message: message,
	})
}

// Unauthorized 未授权响应
func Unauthorized(c *fiber.Ctx, message string) error {
	if message == "" {
		message = MsgUnauthorized
	}
	return c.Status(fiber.StatusUnauthorized).JSON(Response{
		Code:    CodeUnauthorized,
		Message: message,
	})
}

// Forbidden 禁止访问响应
func Forbidden(c *fiber.Ctx, message string) error {
	if message == "" {
		message = MsgForbidden
	}
	return c.Status(fiber.StatusForbidden).JSON(Response{
		Code:    CodeForbidden,
		Message: message,
	})
}

// NotFound 未找到响应
func NotFound(c *fiber.Ctx, message string) error {
	if message == "" {
		message = MsgNotFound
	}
	return c.Status(fiber.StatusNotFound).JSON(Response{
		Code:    CodeNotFound,
		Message: message,
	})
}

// ServerError 服务器错误响应
func ServerError(c *fiber.Ctx, message string) error {
	if message == "" {
		message = MsgServerError
	}
	return c.Status(fiber.StatusInternalServerError).JSON(Response{
		Code:    CodeServerError,
		Message: message,
	})
}

// Page 分页响应
func Page(c *fiber.Ctx, list any, total int64, page, pageSize int) error {
	return Success(c, PageData{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

