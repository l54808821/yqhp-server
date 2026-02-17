package handler

import (
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// KnowledgeBaseCreate 创建知识库
// POST /api/knowledge-bases
func KnowledgeBaseCreate(c *fiber.Ctx) error {
	var req logic.CreateKnowledgeBaseReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Name == "" {
		return response.Error(c, "知识库名称不能为空")
	}
	if req.Type == "" {
		req.Type = "normal"
	}
	if req.Type != "normal" && req.Type != "graph" {
		return response.Error(c, "知识库类型必须为 normal 或 graph")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	result, err := kbLogic.Create(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// KnowledgeBaseList 获取知识库列表
// GET /api/knowledge-bases
func KnowledgeBaseList(c *fiber.Ctx) error {
	var req logic.KnowledgeBaseListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	list, total, err := kbLogic.List(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Page(c, list, total, req.Page, req.PageSize)
}

// KnowledgeBaseGetByID 获取知识库详情
// GET /api/knowledge-bases/:id
func KnowledgeBaseGetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	result, err := kbLogic.GetByID(id)
	if err != nil {
		return response.NotFound(c, "知识库不存在")
	}

	return response.Success(c, result)
}

// KnowledgeBaseUpdate 更新知识库
// PUT /api/knowledge-bases/:id
func KnowledgeBaseUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	var req logic.UpdateKnowledgeBaseReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	if err := kbLogic.Update(id, &req); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// KnowledgeBaseDelete 删除知识库
// DELETE /api/knowledge-bases/:id
func KnowledgeBaseDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	if err := kbLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// KnowledgeBaseUpdateStatus 更新知识库状态
// PUT /api/knowledge-bases/:id/status
func KnowledgeBaseUpdateStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	var req struct {
		Status int32 `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	if err := kbLogic.UpdateStatus(id, req.Status); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// KnowledgeDocumentUpload 上传知识库文档
// POST /api/knowledge-bases/:id/documents
func KnowledgeDocumentUpload(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	// 支持两种方式：文件上传或直接传文本内容
	file, fileErr := c.FormFile("file")
	var req logic.CreateKnowledgeDocumentReq
	req.KnowledgeBaseID = kbID

	if fileErr == nil && file != nil {
		// 文件上传模式
		req.Name = file.Filename
		req.FileSize = file.Size

		// 读取文件内容
		f, err := file.Open()
		if err != nil {
			return response.Error(c, "读取文件失败")
		}
		defer f.Close()

		buf := make([]byte, file.Size)
		n, err := f.Read(buf)
		if err != nil {
			return response.Error(c, "读取文件内容失败")
		}
		content := string(buf[:n])
		req.Content = &content

		// 推断文件类型
		req.FileType = logic.InferFileType(file.Filename)
	} else {
		// JSON 内容模式
		if err := c.BodyParser(&req); err != nil {
			return response.Error(c, "参数解析失败: 请上传文件或提供JSON内容")
		}
	}

	if req.Name == "" {
		return response.Error(c, "文档名称不能为空")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	result, err := kbLogic.CreateDocument(&req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// KnowledgeDocumentList 获取知识库文档列表
// GET /api/knowledge-bases/:id/documents
func KnowledgeDocumentList(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	list, err := kbLogic.ListDocuments(kbID)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, list)
}

// KnowledgeDocumentDelete 删除知识库文档
// DELETE /api/knowledge-bases/:id/documents/:docId
func KnowledgeDocumentDelete(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	docID, err := strconv.ParseInt(c.Params("docId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的文档ID")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	if err := kbLogic.DeleteDocument(kbID, docID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// KnowledgeDocumentReprocess 重新处理文档
// POST /api/knowledge-bases/:id/documents/:docId/reprocess
func KnowledgeDocumentReprocess(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	docID, err := strconv.ParseInt(c.Params("docId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的文档ID")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	if err := kbLogic.ReprocessDocument(kbID, docID); err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, nil)
}

// KnowledgeBaseSearch 知识库检索测试
// POST /api/knowledge-bases/:id/search
func KnowledgeBaseSearch(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	var req logic.KnowledgeSearchReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	if req.Query == "" {
		return response.Error(c, "检索内容不能为空")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	results, err := kbLogic.Search(id, &req)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, results)
}
