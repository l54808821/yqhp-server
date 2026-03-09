package handler

import (
	"mime"
	"path/filepath"
	"strings"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// ReportFileUploadRequest 报告文件上传请求（JSON body，内容为文本）
type ReportFileUploadRequest struct {
	Content  string `json:"content"`
	FileName string `json:"file_name"`
	FileType string `json:"file_type"`
}

// ReportFileUpload 上传报告文本内容为文件
// POST /api/report-files
func ReportFileUpload(c *fiber.Ctx) error {
	var req ReportFileUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "请求解析失败: "+err.Error())
	}
	if req.Content == "" {
		return response.Error(c, "content 不能为空")
	}
	if req.FileName == "" {
		req.FileName = "report.html"
	}

	storage := logic.GetFileStorage()
	reader := strings.NewReader(req.Content)
	relPath, err := storage.SaveAttachment("reports", req.FileName, reader)
	if err != nil {
		return response.Error(c, "保存报告文件失败: "+err.Error())
	}

	url := "/api/attachments/files/" + relPath

	return response.Success(c, fiber.Map{
		"url":      url,
		"fileName": req.FileName,
		"fileType": req.FileType,
	})
}

// AttachmentUploadResult 附件上传返回
type AttachmentUploadResult struct {
	URL      string `json:"url"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	Type     string `json:"type"` // image / audio / video / file
}

// AttachmentUpload 通用附件上传
// POST /api/attachments/upload
func AttachmentUpload(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil || file == nil {
		return response.Error(c, "请上传文件")
	}

	category := c.FormValue("category", "chat")

	f, err := file.Open()
	if err != nil {
		return response.Error(c, "读取文件失败")
	}
	defer f.Close()

	storage := logic.GetFileStorage()
	relPath, err := storage.SaveAttachment(category, file.Filename, f)
	if err != nil {
		return response.Error(c, "保存文件失败: "+err.Error())
	}

	mimeType := detectMimeType(file.Filename)

	return response.Success(c, AttachmentUploadResult{
		URL:      "/api/attachments/files/" + relPath,
		Name:     file.Filename,
		Size:     file.Size,
		MimeType: mimeType,
		Type:     logic.InferMediaType(mimeType),
	})
}

// AttachmentServe 静态附件文件服务
// GET /api/attachments/files/*
func AttachmentServe(c *fiber.Ctx) error {
	relPath := c.Params("*")
	if relPath == "" {
		return response.Error(c, "路径不能为空")
	}

	if strings.Contains(relPath, "..") {
		return response.Error(c, "非法路径")
	}

	storage := logic.GetFileStorage()
	fullPath := storage.FullPath(relPath)

	return c.SendFile(fullPath)
}

func detectMimeType(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return "application/octet-stream"
	}
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}
