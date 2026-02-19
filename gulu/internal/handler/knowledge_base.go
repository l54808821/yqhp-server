package handler

import (
	"fmt"
	"mime"
	"path/filepath"
	"strconv"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// parseKBID 从路由参数 ":id" 解析知识库 ID（消除各 handler 中的重复代码）
func parseKBID(c *fiber.Ctx) (int64, error) {
	return strconv.ParseInt(c.Params("id"), 10, 64)
}

// -----------------------------------------------
// 知识库 CRUD
// -----------------------------------------------

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

func KnowledgeBaseGetByID(c *fiber.Ctx) error {
	id, err := parseKBID(c)
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

func KnowledgeBaseUpdate(c *fiber.Ctx) error {
	id, err := parseKBID(c)
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

func KnowledgeBaseDelete(c *fiber.Ctx) error {
	id, err := parseKBID(c)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	if err := kbLogic.Delete(id); err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, nil)
}

func KnowledgeBaseUpdateStatus(c *fiber.Ctx) error {
	id, err := parseKBID(c)
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

// -----------------------------------------------
// 文档管理
// -----------------------------------------------

func KnowledgeDocumentUpload(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	file, fileErr := c.FormFile("file")
	if fileErr != nil || file == nil {
		return response.Error(c, "请上传文件")
	}

	fileType := logic.InferFileType(file.Filename)

	f, err := file.Open()
	if err != nil {
		return response.Error(c, "读取文件失败")
	}
	defer f.Close()

	storage := logic.GetFileStorage()
	relPath, err := storage.Save(kbID, file.Filename, f)
	if err != nil {
		return response.Error(c, "保存文件失败: "+err.Error())
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	result, err := kbLogic.CreateDocument(kbID, file.Filename, fileType, relPath, file.Size)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, result)
}

// KnowledgeFileUpload 仅上传文件到磁盘，不创建数据库记录。
// 用于向导式上传流程：先上传文件预览，确认后再建记录并处理。
func KnowledgeFileUpload(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	file, fileErr := c.FormFile("file")
	if fileErr != nil || file == nil {
		return response.Error(c, "请上传文件")
	}

	f, err := file.Open()
	if err != nil {
		return response.Error(c, "读取文件失败")
	}
	defer f.Close()

	storage := logic.GetFileStorage()
	relPath, err := storage.Save(kbID, file.Filename, f)
	if err != nil {
		return response.Error(c, "保存文件失败: "+err.Error())
	}

	return response.Success(c, logic.UploadFileResult{
		FilePath: relPath,
		FileName: file.Filename,
		FileType: logic.InferFileType(file.Filename),
		FileSize: file.Size,
	})
}

// KnowledgeDocumentCreateAndProcess 一次性创建文档记录并启动异步处理。
// 配合 KnowledgeFileUpload 使用：文件已在磁盘上，此接口创建 DB 记录后立即处理。
func KnowledgeDocumentCreateAndProcess(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	var req logic.CreateAndProcessReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if req.FilePath == "" || req.FileName == "" {
		return response.Error(c, "文件信息不完整")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	result, err := kbLogic.CreateAndProcessDocument(kbID, &req)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, result)
}

func KnowledgeDocumentList(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	var req logic.KnowledgeDocumentListReq
	if err := c.QueryParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	list, err := kbLogic.ListDocuments(kbID, &req)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, list)
}

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

func KnowledgeDocumentPreviewChunks(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	var req logic.PreviewChunksReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if req.DocumentID == 0 && req.FilePath == "" && req.Content == "" {
		return response.Error(c, "请提供文档ID、文件路径或文档内容")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	chunks, err := kbLogic.PreviewChunks(kbID, &req)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, chunks)
}

func KnowledgeDocumentProcess(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}
	docID, err := strconv.ParseInt(c.Params("docId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的文档ID")
	}

	var req logic.ProcessDocumentReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	if err := kbLogic.ProcessDocument(kbID, docID, &req); err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, nil)
}

// -----------------------------------------------
// 批量操作
// -----------------------------------------------

func KnowledgeDocumentBatchDelete(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	var req logic.BatchDocIDsReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if len(req.DocumentIDs) == 0 {
		return response.Error(c, "请选择要删除的文档")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	if err := kbLogic.BatchDeleteDocuments(kbID, req.DocumentIDs); err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, nil)
}

func KnowledgeDocumentBatchReprocess(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	var req logic.BatchDocIDsReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if len(req.DocumentIDs) == 0 {
		return response.Error(c, "请选择要重处理的文档")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	if err := kbLogic.BatchReprocessDocuments(kbID, req.DocumentIDs); err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, nil)
}

func KnowledgeIndexingStatus(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	status, err := kbLogic.GetIndexingStatus(kbID)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, status)
}

// -----------------------------------------------
// 分块管理
// -----------------------------------------------

func KnowledgeDocumentSegments(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}
	docID, err := strconv.ParseInt(c.Params("docId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的文档ID")
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "20"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	segments, total, err := kbLogic.GetDocumentSegments(kbID, docID, page, pageSize)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Page(c, segments, total, page, pageSize)
}

func KnowledgeSegmentUpdate(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}
	segID, err := strconv.ParseInt(c.Params("segId"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的分块ID")
	}

	var req logic.UpdateSegmentReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	if err := kbLogic.UpdateSegment(kbID, segID, &req); err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, nil)
}

// -----------------------------------------------
// 检索与查询历史
// -----------------------------------------------

func KnowledgeBaseSearch(c *fiber.Ctx) error {
	id, err := parseKBID(c)
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

func KnowledgeQueryHistory(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	history, err := kbLogic.GetQueryHistory(kbID, limit)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, history)
}

// -----------------------------------------------
// 诊断接口
// -----------------------------------------------

// KnowledgeBaseDiagnose 诊断知识库向量数据状态
// GET /api/knowledge-bases/:id/diagnose
func KnowledgeBaseDiagnose(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	diag, err := kbLogic.Diagnose(kbID)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, diag)
}

// -----------------------------------------------
// 图片资源访问
// -----------------------------------------------

// KnowledgeImageServe 提供知识库图片的访问服务
// GET /api/knowledge-bases/:id/images/:filename
func KnowledgeImageServe(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("无效的知识库ID")
	}

	filename := c.Params("filename")
	if filename == "" {
		return c.Status(fiber.StatusBadRequest).SendString("文件名不能为空")
	}

	relPath := fmt.Sprintf("kb_%d/images/%s", kbID, filename)
	storage := logic.GetFileStorage()
	data, err := storage.Read(relPath)
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("图片不存在")
	}

	ext := filepath.Ext(filename)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "image/png"
	}
	c.Set("Content-Type", mimeType)
	c.Set("Cache-Control", "public, max-age=86400")
	return c.Send(data)
}

// -----------------------------------------------
// 图知识库（Phase 3）
// -----------------------------------------------

func KnowledgeGraphSearch(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	var req struct {
		Keywords []string `json:"keywords"`
		MaxHops  int      `json:"max_hops"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if len(req.Keywords) == 0 {
		return response.Error(c, "关键词不能为空")
	}

	if !logic.IsNeo4jEnabled() {
		return response.Error(c, "Neo4j 未启用")
	}

	result, err := logic.SearchGraphByKeywords(c.UserContext(), kbID, req.Keywords, req.MaxHops)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, result)
}

func KnowledgeGraphEntities(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	entities, err := kbLogic.ListGraphEntities(kbID)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, entities)
}

func KnowledgeGraphRelations(c *fiber.Ctx) error {
	kbID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的知识库ID")
	}

	kbLogic := logic.NewKnowledgeBaseLogic(c.UserContext())
	relations, err := kbLogic.ListGraphRelations(kbID)
	if err != nil {
		return response.Error(c, err.Error())
	}
	return response.Success(c, relations)
}
