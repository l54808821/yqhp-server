package logic

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

// KnowledgeBaseLogic 知识库逻辑
type KnowledgeBaseLogic struct {
	ctx context.Context
}

// NewKnowledgeBaseLogic 创建知识库逻辑
func NewKnowledgeBaseLogic(ctx context.Context) *KnowledgeBaseLogic {
	return &KnowledgeBaseLogic{ctx: ctx}
}

// -----------------------------------------------
// Request / Response 结构体
// -----------------------------------------------

// CreateKnowledgeBaseReq 创建知识库请求
type CreateKnowledgeBaseReq struct {
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	Type               string  `json:"type"`
	EmbeddingModelID   *int64  `json:"embedding_model_id"`
	EmbeddingModelName string  `json:"embedding_model_name"`
	EmbeddingDimension int32   `json:"embedding_dimension"`
	ChunkSize          int32   `json:"chunk_size"`
	ChunkOverlap       int32   `json:"chunk_overlap"`
	SimilarityThreshold float64 `json:"similarity_threshold"`
	TopK               int32   `json:"top_k"`
}

// UpdateKnowledgeBaseReq 更新知识库请求
type UpdateKnowledgeBaseReq struct {
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	EmbeddingModelID   *int64  `json:"embedding_model_id"`
	EmbeddingModelName string  `json:"embedding_model_name"`
	EmbeddingDimension int32   `json:"embedding_dimension"`
	ChunkSize          int32   `json:"chunk_size"`
	ChunkOverlap       int32   `json:"chunk_overlap"`
	SimilarityThreshold float64 `json:"similarity_threshold"`
	TopK               int32   `json:"top_k"`
}

// KnowledgeBaseListReq 知识库列表请求
type KnowledgeBaseListReq struct {
	Page     int    `query:"page"`
	PageSize int    `query:"pageSize"`
	Name     string `query:"name"`
	Type     string `query:"type"`
	Status   *int32 `query:"status"`
}

// KnowledgeBaseInfo 知识库返回信息
type KnowledgeBaseInfo struct {
	ID                 int64      `json:"id"`
	CreatedAt          *time.Time `json:"created_at"`
	UpdatedAt          *time.Time `json:"updated_at"`
	CreatedBy          *int64     `json:"created_by"`
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	Type               string     `json:"type"`
	Status             int32      `json:"status"`
	EmbeddingModelID   *int64     `json:"embedding_model_id"`
	EmbeddingModelName string     `json:"embedding_model_name"`
	EmbeddingDimension int32      `json:"embedding_dimension"`
	ChunkSize          int32      `json:"chunk_size"`
	ChunkOverlap       int32      `json:"chunk_overlap"`
	SimilarityThreshold float64   `json:"similarity_threshold"`
	TopK               int32      `json:"top_k"`
	QdrantCollection   string     `json:"qdrant_collection"`
	DocumentCount      int32      `json:"document_count"`
	ChunkCount         int32      `json:"chunk_count"`
}

// CreateKnowledgeDocumentReq 创建知识库文档请求
type CreateKnowledgeDocumentReq struct {
	KnowledgeBaseID int64   `json:"knowledge_base_id"`
	Name            string  `json:"name"`
	FileType        string  `json:"file_type"`
	Content         *string `json:"content"`
	ContentEncoding string  `json:"content_encoding"` // "base64" 表示内容已 Base64 编码
	FileSize        int64   `json:"file_size"`
}

// KnowledgeDocumentInfo 知识库文档返回信息
type KnowledgeDocumentInfo struct {
	ID              int64      `json:"id"`
	CreatedAt       *time.Time `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at"`
	KnowledgeBaseID int64      `json:"knowledge_base_id"`
	Name            string     `json:"name"`
	FileType        string     `json:"file_type"`
	FileSize        int64      `json:"file_size"`
	Status          string     `json:"status"`
	ErrorMessage    string     `json:"error_message"`
	ChunkCount      int32      `json:"chunk_count"`
	TokenCount      int32      `json:"token_count"`
}

// KnowledgeSearchReq 知识库检索请求
type KnowledgeSearchReq struct {
	Query  string  `json:"query"`
	TopK   int     `json:"top_k"`
	Score  float64 `json:"score"`
}

// KnowledgeSearchResult 知识库检索结果
type KnowledgeSearchResult struct {
	Content      string                 `json:"content"`
	Score        float64                `json:"score"`
	DocumentID   int64                  `json:"document_id"`
	DocumentName string                 `json:"document_name"`
	ChunkIndex   int                    `json:"chunk_index"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentChunkInfo 文档分块信息
type DocumentChunkInfo struct {
	ChunkIndex int    `json:"chunk_index"`
	Content    string `json:"content"`
	CharCount  int    `json:"char_count"`
	Enabled    bool   `json:"enabled"`
}

// -----------------------------------------------
// CRUD 方法
// -----------------------------------------------

// Create 创建知识库
func (l *KnowledgeBaseLogic) Create(req *CreateKnowledgeBaseReq) (*KnowledgeBaseInfo, error) {
	now := time.Now()
	isDelete := false
	status := int32(1)

	// 设置默认值
	embeddingDimension := req.EmbeddingDimension
	if embeddingDimension == 0 {
		embeddingDimension = 1536
	}
	chunkSize := req.ChunkSize
	if chunkSize == 0 {
		chunkSize = 500
	}
	chunkOverlap := req.ChunkOverlap
	if chunkOverlap == 0 {
		chunkOverlap = 50
	}
	similarityThreshold := req.SimilarityThreshold
	if similarityThreshold == 0 {
		similarityThreshold = 0.7
	}
	topK := req.TopK
	if topK == 0 {
		topK = 5
	}

	kb := &model.TKnowledgeBase{
		CreatedAt:          &now,
		UpdatedAt:          &now,
		IsDelete:           &isDelete,
		Name:               req.Name,
		Description:        strPtr(req.Description),
		Type:               req.Type,
		Status:             &status,
		EmbeddingModelID:   req.EmbeddingModelID,
		EmbeddingModelName: strPtr(req.EmbeddingModelName),
		EmbeddingDimension: &embeddingDimension,
		ChunkSize:          &chunkSize,
		ChunkOverlap:       &chunkOverlap,
		SimilarityThreshold: &similarityThreshold,
		TopK:               &topK,
	}

	// 创建数据库记录
	q := query.Use(svc.Ctx.DB)
	err := q.TKnowledgeBase.WithContext(l.ctx).Create(kb)
	if err != nil {
		return nil, err
	}

	// 生成 Qdrant Collection 名称
	collectionName := fmt.Sprintf("kb_%d", kb.ID)
	kb.QdrantCollection = &collectionName
	_, err = q.TKnowledgeBase.WithContext(l.ctx).Where(q.TKnowledgeBase.ID.Eq(kb.ID)).Update(q.TKnowledgeBase.QdrantCollection, collectionName)
	if err != nil {
		return nil, fmt.Errorf("更新 Collection 名称失败: %w", err)
	}

	// 同步创建 Qdrant Collection（确保写入前 Collection 已就绪）
	if req.Type == "normal" {
		if err := CreateQdrantCollection(collectionName, int(embeddingDimension)); err != nil {
			fmt.Printf("[WARN] 创建 Qdrant Collection 失败: %v\n", err)
		}
	}

	return l.toKnowledgeBaseInfo(kb), nil
}

// Update 更新知识库
func (l *KnowledgeBaseLogic) Update(id int64, req *UpdateKnowledgeBaseReq) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TKnowledgeBase

	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("知识库不存在")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	updates["description"] = req.Description
	if req.EmbeddingModelID != nil {
		updates["embedding_model_id"] = *req.EmbeddingModelID
	}
	if req.EmbeddingModelName != "" {
		updates["embedding_model_name"] = req.EmbeddingModelName
	}
	if req.EmbeddingDimension > 0 {
		updates["embedding_dimension"] = req.EmbeddingDimension
	}
	if req.ChunkSize > 0 {
		updates["chunk_size"] = req.ChunkSize
	}
	if req.ChunkOverlap >= 0 {
		updates["chunk_overlap"] = req.ChunkOverlap
	}
	if req.SimilarityThreshold > 0 {
		updates["similarity_threshold"] = req.SimilarityThreshold
	}
	if req.TopK > 0 {
		updates["top_k"] = req.TopK
	}

	_, err = m.WithContext(l.ctx).Where(m.ID.Eq(id)).Updates(updates)
	return err
}

// Delete 删除知识库（软删除）
func (l *KnowledgeBaseLogic) Delete(id int64) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TKnowledgeBase

	kb, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return errors.New("知识库不存在")
	}

	isDelete := true
	_, err = m.WithContext(l.ctx).Where(m.ID.Eq(id)).Update(m.IsDelete, isDelete)
	if err != nil {
		return err
	}

	// 异步清理 Qdrant Collection
	if kb.QdrantCollection != nil && *kb.QdrantCollection != "" {
		go func() {
			if err := DeleteQdrantCollection(*kb.QdrantCollection); err != nil {
				fmt.Printf("[WARN] 删除 Qdrant Collection 失败: %v\n", err)
			}
		}()
	}

	return nil
}

// GetByID 根据 ID 获取知识库
func (l *KnowledgeBaseLogic) GetByID(id int64) (*KnowledgeBaseInfo, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TKnowledgeBase

	kb, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}

	return l.toKnowledgeBaseInfo(kb), nil
}

// List 获取知识库列表
func (l *KnowledgeBaseLogic) List(req *KnowledgeBaseListReq) ([]*KnowledgeBaseInfo, int64, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TKnowledgeBase

	qb := m.WithContext(l.ctx).Where(m.IsDelete.Is(false))

	if req.Name != "" {
		qb = qb.Where(m.Name.Like("%" + req.Name + "%"))
	}
	if req.Type != "" {
		qb = qb.Where(m.Type.Eq(req.Type))
	}
	if req.Status != nil {
		qb = qb.Where(m.Status.Eq(*req.Status))
	}

	total, err := qb.Count()
	if err != nil {
		return nil, 0, err
	}

	offset := (req.Page - 1) * req.PageSize
	list, err := qb.Order(m.ID.Desc()).Offset(offset).Limit(req.PageSize).Find()
	if err != nil {
		return nil, 0, err
	}

	result := make([]*KnowledgeBaseInfo, 0, len(list))
	for _, item := range list {
		result = append(result, l.toKnowledgeBaseInfo(item))
	}

	return result, total, nil
}

// UpdateStatus 更新知识库状态
func (l *KnowledgeBaseLogic) UpdateStatus(id int64, status int32) error {
	q := query.Use(svc.Ctx.DB)
	m := q.TKnowledgeBase

	now := time.Now()
	_, err := m.WithContext(l.ctx).Where(m.ID.Eq(id), m.IsDelete.Is(false)).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": now,
	})
	return err
}

// -----------------------------------------------
// 文档管理方法
// -----------------------------------------------

// CreateDocument 创建文档
func (l *KnowledgeBaseLogic) CreateDocument(req *CreateKnowledgeDocumentReq) (*KnowledgeDocumentInfo, error) {
	// 验证知识库存在
	q := query.Use(svc.Ctx.DB)
	_, err := q.TKnowledgeBase.WithContext(l.ctx).Where(
		q.TKnowledgeBase.ID.Eq(req.KnowledgeBaseID),
		q.TKnowledgeBase.IsDelete.Is(false),
	).First()
	if err != nil {
		return nil, errors.New("知识库不存在")
	}

	now := time.Now()
	status := "pending"

	doc := &model.TKnowledgeDocument{
		CreatedAt:       &now,
		UpdatedAt:       &now,
		KnowledgeBaseID: req.KnowledgeBaseID,
		Name:            req.Name,
		FileType:        strPtr(req.FileType),
		FileSize:        &req.FileSize,
		Content:         req.Content,
		Status:          &status,
	}

	err = q.TKnowledgeDocument.WithContext(l.ctx).Create(doc)
	if err != nil {
		return nil, err
	}

	// 更新知识库文档计数
	go func() {
		l.updateDocumentCount(req.KnowledgeBaseID)
	}()

	// 不再自动处理，等待用户在 Step 2 配置分段参数后手动触发
	return l.toDocumentInfo(doc), nil
}

// ListDocuments 获取文档列表
func (l *KnowledgeBaseLogic) ListDocuments(kbID int64) ([]*KnowledgeDocumentInfo, error) {
	q := query.Use(svc.Ctx.DB)
	m := q.TKnowledgeDocument

	list, err := m.WithContext(l.ctx).Where(m.KnowledgeBaseID.Eq(kbID)).Order(m.ID.Desc()).Find()
	if err != nil {
		return nil, err
	}

	result := make([]*KnowledgeDocumentInfo, 0, len(list))
	for _, item := range list {
		result = append(result, l.toDocumentInfo(item))
	}
	return result, nil
}

// DeleteDocument 删除文档
func (l *KnowledgeBaseLogic) DeleteDocument(kbID, docID int64) error {
	q := query.Use(svc.Ctx.DB)

	// 获取知识库信息（用于清理 Qdrant 数据）
	kb, err := q.TKnowledgeBase.WithContext(l.ctx).Where(
		q.TKnowledgeBase.ID.Eq(kbID),
		q.TKnowledgeBase.IsDelete.Is(false),
	).First()
	if err != nil {
		return errors.New("知识库不存在")
	}

	// 删除数据库记录
	_, err = q.TKnowledgeDocument.WithContext(l.ctx).Where(
		q.TKnowledgeDocument.ID.Eq(docID),
		q.TKnowledgeDocument.KnowledgeBaseID.Eq(kbID),
	).Delete()
	if err != nil {
		return err
	}

	// 更新文档计数
	go func() {
		l.updateDocumentCount(kbID)
	}()

	// 异步清理 Qdrant 中该文档的向量
	if kb.QdrantCollection != nil && *kb.QdrantCollection != "" {
		go func() {
			if err := DeleteDocumentVectors(*kb.QdrantCollection, docID); err != nil {
				fmt.Printf("[WARN] 清理文档向量失败: docID=%d, err=%v\n", docID, err)
			}
		}()
	}

	return nil
}

// ReprocessDocument 重新处理文档
func (l *KnowledgeBaseLogic) ReprocessDocument(kbID, docID int64) error {
	q := query.Use(svc.Ctx.DB)

	kb, err := q.TKnowledgeBase.WithContext(l.ctx).Where(
		q.TKnowledgeBase.ID.Eq(kbID),
		q.TKnowledgeBase.IsDelete.Is(false),
	).First()
	if err != nil {
		return errors.New("知识库不存在")
	}

	doc, err := q.TKnowledgeDocument.WithContext(l.ctx).Where(
		q.TKnowledgeDocument.ID.Eq(docID),
		q.TKnowledgeDocument.KnowledgeBaseID.Eq(kbID),
	).First()
	if err != nil {
		return errors.New("文档不存在")
	}

	// 更新状态为处理中
	status := "processing"
	_, _ = q.TKnowledgeDocument.WithContext(l.ctx).Where(q.TKnowledgeDocument.ID.Eq(docID)).Update(q.TKnowledgeDocument.Status, status)

	// 异步重新处理
	go func() {
		processor := NewDocumentProcessor()
		if err := processor.Process(kb, doc); err != nil {
			fmt.Printf("[ERROR] 文档重处理失败: docID=%d, err=%v\n", doc.ID, err)
		}
	}()

	return nil
}

// GetDocumentChunks 获取文档的分块列表（从 Qdrant 读取）
func (l *KnowledgeBaseLogic) GetDocumentChunks(kbID, docID int64) ([]*DocumentChunkInfo, error) {
	q := query.Use(svc.Ctx.DB)

	// 获取知识库信息
	kb, err := q.TKnowledgeBase.WithContext(l.ctx).Where(
		q.TKnowledgeBase.ID.Eq(kbID),
		q.TKnowledgeBase.IsDelete.Is(false),
	).First()
	if err != nil {
		return nil, errors.New("知识库不存在")
	}

	if kb.QdrantCollection == nil || *kb.QdrantCollection == "" {
		return nil, errors.New("知识库尚未初始化向量存储")
	}

	// 验证文档存在
	_, err = q.TKnowledgeDocument.WithContext(l.ctx).Where(
		q.TKnowledgeDocument.ID.Eq(docID),
		q.TKnowledgeDocument.KnowledgeBaseID.Eq(kbID),
	).First()
	if err != nil {
		return nil, errors.New("文档不存在")
	}

	// 从 Qdrant 获取该文档的所有分块
	hits, err := ScrollDocumentVectors(*kb.QdrantCollection, docID)
	if err != nil {
		return nil, fmt.Errorf("获取分块失败: %w", err)
	}

	// 按 chunk_index 排序
	chunks := make([]*DocumentChunkInfo, 0, len(hits))
	for _, hit := range hits {
		content := hit.Content
		chunks = append(chunks, &DocumentChunkInfo{
			ChunkIndex: hit.ChunkIndex,
			Content:    content,
			CharCount:  len([]rune(content)),
			Enabled:    true,
		})
	}

	// 按 chunk_index 排序
	for i := 0; i < len(chunks); i++ {
		for j := i + 1; j < len(chunks); j++ {
			if chunks[i].ChunkIndex > chunks[j].ChunkIndex {
				chunks[i], chunks[j] = chunks[j], chunks[i]
			}
		}
	}

	return chunks, nil
}

// -----------------------------------------------
// 分块预览 + 文档处理
// -----------------------------------------------

// PreviewChunksReq 预览分块请求
type PreviewChunksReq struct {
	DocumentID   int64               `json:"document_id"`
	Content      string              `json:"content"`
	ChunkSetting *model.ChunkSetting `json:"chunk_setting"`
}

// PreviewChunkItem 预览分块项
type PreviewChunkItem struct {
	Index     int    `json:"index"`
	Content   string `json:"content"`
	CharCount int    `json:"char_count"`
}

// ProcessDocumentReq 处理文档请求（Step 2 确认分段参数后触发）
type ProcessDocumentReq struct {
	ChunkSetting *model.ChunkSetting `json:"chunk_setting"`
}

// PreviewChunks 预览文档分块（不写入 Qdrant，仅用于前端展示）
func (l *KnowledgeBaseLogic) PreviewChunks(kbID int64, req *PreviewChunksReq) ([]*PreviewChunkItem, error) {
	text := req.Content

	// 如果传了 document_id，从数据库读取文档并提取文本
	if req.DocumentID > 0 {
		q := query.Use(svc.Ctx.DB)
		doc, err := q.TKnowledgeDocument.WithContext(l.ctx).Where(
			q.TKnowledgeDocument.ID.Eq(req.DocumentID),
			q.TKnowledgeDocument.KnowledgeBaseID.Eq(kbID),
		).First()
		if err != nil {
			return nil, errors.New("文档不存在")
		}
		processor := NewDocumentProcessor()
		extracted, err := processor.extractText(doc)
		if err != nil {
			return nil, fmt.Errorf("文本提取失败: %w", err)
		}
		text = extracted
		// DEBUG: 临时日志，查看提取的文本内容
		fmt.Printf("\n=== DEBUG extractText ===\nlen=%d\nfirst500=%q\n=== END DEBUG ===\n", len(text), func() string {
			r := []rune(text)
			if len(r) > 500 {
				return string(r[:500])
			}
			return text
		}())
	}

	if text == "" {
		return nil, errors.New("文档内容为空")
	}

	// 合并默认值
	cs := model.DefaultChunkSetting()
	if req.ChunkSetting != nil {
		if req.ChunkSetting.Separator != "" {
			cs.Separator = req.ChunkSetting.Separator
		}
		if req.ChunkSetting.ChunkSize > 0 {
			cs.ChunkSize = req.ChunkSetting.ChunkSize
		}
		if req.ChunkSetting.ChunkOverlap >= 0 {
			cs.ChunkOverlap = req.ChunkSetting.ChunkOverlap
		}
		cs.CleanWhitespace = req.ChunkSetting.CleanWhitespace
		cs.RemoveURLs = req.ChunkSetting.RemoveURLs
	}

	// 文本预处理
	if cs.CleanWhitespace {
		text = cleanWhitespace(text)
	}
	if cs.RemoveURLs {
		text = removeURLsAndEmails(text)
	}

	processor := NewDocumentProcessor()

	// 使用分段标识符切分
	var chunks []string
	fmt.Printf("\n=== DEBUG split ===\nseparator=%q chunkSize=%d overlap=%d cleanWS=%v\n", cs.Separator, cs.ChunkSize, cs.ChunkOverlap, cs.CleanWhitespace)
	if cs.Separator != "" {
		chunks = splitBySeparator(text, cs.Separator, cs.ChunkSize, cs.ChunkOverlap)
	} else {
		chunks = processor.splitText(text, cs.ChunkSize, cs.ChunkOverlap)
	}
	fmt.Printf("chunks count=%d\n", len(chunks))
	for i, c := range chunks {
		fmt.Printf("  chunk[%d] len=%d preview=%q\n", i, len([]rune(c)), func() string {
			r := []rune(c)
			if len(r) > 80 {
				return string(r[:80]) + "..."
			}
			return c
		}())
	}
	fmt.Println("=== END DEBUG split ===")

	result := make([]*PreviewChunkItem, 0, len(chunks))
	for i, chunk := range chunks {
		result = append(result, &PreviewChunkItem{
			Index:     i,
			Content:   chunk,
			CharCount: len([]rune(chunk)),
		})
	}

	return result, nil
}

// ProcessDocument 确认分段参数并开始处理文档
func (l *KnowledgeBaseLogic) ProcessDocument(kbID, docID int64, req *ProcessDocumentReq) error {
	q := query.Use(svc.Ctx.DB)

	kb, err := q.TKnowledgeBase.WithContext(l.ctx).Where(
		q.TKnowledgeBase.ID.Eq(kbID),
		q.TKnowledgeBase.IsDelete.Is(false),
	).First()
	if err != nil {
		return errors.New("知识库不存在")
	}

	doc, err := q.TKnowledgeDocument.WithContext(l.ctx).Where(
		q.TKnowledgeDocument.ID.Eq(docID),
		q.TKnowledgeDocument.KnowledgeBaseID.Eq(kbID),
	).First()
	if err != nil {
		return errors.New("文档不存在")
	}

	// 保存文档级别的分段参数（整体写入 chunk_setting JSON）
	cs := model.DefaultChunkSetting()
	if req.ChunkSetting != nil {
		if req.ChunkSetting.Separator != "" {
			cs.Separator = req.ChunkSetting.Separator
		}
		if req.ChunkSetting.ChunkSize > 0 {
			cs.ChunkSize = req.ChunkSetting.ChunkSize
		}
		if req.ChunkSetting.ChunkOverlap >= 0 {
			cs.ChunkOverlap = req.ChunkSetting.ChunkOverlap
		}
		cs.CleanWhitespace = req.ChunkSetting.CleanWhitespace
		cs.RemoveURLs = req.ChunkSetting.RemoveURLs
	}

	updates := map[string]interface{}{
		"chunk_setting": cs,
		"status":        "processing",
	}
	q.TKnowledgeDocument.WithContext(l.ctx).Where(q.TKnowledgeDocument.ID.Eq(docID)).Updates(updates)

	// 重新读取更新后的文档
	doc, _ = q.TKnowledgeDocument.WithContext(l.ctx).Where(q.TKnowledgeDocument.ID.Eq(docID)).First()

	// 异步处理
	go func() {
		processor := NewDocumentProcessor()
		if err := processor.Process(kb, doc); err != nil {
			fmt.Printf("[ERROR] 文档处理失败: docID=%d, err=%v\n", doc.ID, err)
		}
	}()

	return nil
}

// cleanWhitespace 替换连续空格、换行符、制表符
func cleanWhitespace(text string) string {
	// 将制表符替换为空格
	text = strings.ReplaceAll(text, "\t", " ")
	// 合并连续空格
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	// 合并连续换行
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(text)
}

// removeURLsAndEmails 删除 URL 和邮件地址
func removeURLsAndEmails(text string) string {
	// 简单的 URL 移除（http/https）
	lines := strings.Split(text, "\n")
	var result []string
	for _, line := range lines {
		words := strings.Fields(line)
		var cleaned []string
		for _, w := range words {
			if strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") {
				continue
			}
			if strings.Contains(w, "@") && strings.Contains(w, ".") {
				continue
			}
			cleaned = append(cleaned, w)
		}
		result = append(result, strings.Join(cleaned, " "))
	}
	return strings.Join(result, "\n")
}

// Search 知识库检索
func (l *KnowledgeBaseLogic) Search(kbID int64, req *KnowledgeSearchReq) ([]*KnowledgeSearchResult, error) {
	// 获取知识库信息
	q := query.Use(svc.Ctx.DB)
	kbModel, err := q.TKnowledgeBase.WithContext(l.ctx).Where(
		q.TKnowledgeBase.ID.Eq(kbID),
		q.TKnowledgeBase.IsDelete.Is(false),
	).First()
	if err != nil {
		return nil, errors.New("知识库不存在")
	}

	if kbModel.QdrantCollection == nil || *kbModel.QdrantCollection == "" {
		return nil, errors.New("知识库尚未初始化向量存储")
	}

	topK := req.TopK
	if topK <= 0 && kbModel.TopK != nil {
		topK = int(*kbModel.TopK)
	}
	if topK <= 0 {
		topK = 5
	}
	score := req.Score
	if score <= 0 && kbModel.SimilarityThreshold != nil {
		score = *kbModel.SimilarityThreshold
	}

	// 1. 获取 Embedding 客户端
	if kbModel.EmbeddingModelID == nil || *kbModel.EmbeddingModelID == 0 {
		return nil, errors.New("知识库未配置嵌入模型")
	}

	aiModelLogic := NewAiModelLogic(l.ctx)
	aiModel, err := aiModelLogic.GetByIDWithKey(*kbModel.EmbeddingModelID)
	if err != nil {
		return nil, fmt.Errorf("获取嵌入模型配置失败: %w", err)
	}

	embClient := NewEmbeddingClient(aiModel.APIBaseURL, aiModel.APIKey, aiModel.ModelID)

	// 2. 将查询文本转为向量
	queryVector, err := embClient.EmbedText(l.ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("查询文本向量化失败: %w", err)
	}

	// 3. 在 Qdrant 中搜索
	hits, err := SearchVectors(*kbModel.QdrantCollection, queryVector, topK, float32(score))
	if err != nil {
		return nil, fmt.Errorf("向量搜索失败: %w", err)
	}

	// 4. 补充文档名称
	docNameCache := make(map[int64]string)
	results := make([]*KnowledgeSearchResult, 0, len(hits))
	for _, hit := range hits {
		docName := ""
		if hit.DocumentID > 0 {
			if name, ok := docNameCache[hit.DocumentID]; ok {
				docName = name
			} else {
				doc, err := q.TKnowledgeDocument.WithContext(l.ctx).Where(
					q.TKnowledgeDocument.ID.Eq(hit.DocumentID),
				).First()
				if err == nil {
					docName = doc.Name
				}
				docNameCache[hit.DocumentID] = docName
			}
		}

		results = append(results, &KnowledgeSearchResult{
			Content:      hit.Content,
			Score:        hit.Score,
			DocumentID:   hit.DocumentID,
			DocumentName: docName,
			ChunkIndex:   hit.ChunkIndex,
			Metadata:     hit.Metadata,
		})
	}

	return results, nil
}

// -----------------------------------------------
// 内部辅助方法
// -----------------------------------------------

func (l *KnowledgeBaseLogic) updateDocumentCount(kbID int64) {
	q := query.Use(svc.Ctx.DB)
	count, err := q.TKnowledgeDocument.WithContext(l.ctx).Where(q.TKnowledgeDocument.KnowledgeBaseID.Eq(kbID)).Count()
	if err != nil {
		return
	}
	cnt := int32(count)
	q.TKnowledgeBase.WithContext(l.ctx).Where(q.TKnowledgeBase.ID.Eq(kbID)).Update(q.TKnowledgeBase.DocumentCount, cnt)
}

func (l *KnowledgeBaseLogic) toKnowledgeBaseInfo(m *model.TKnowledgeBase) *KnowledgeBaseInfo {
	info := &KnowledgeBaseInfo{
		ID:        m.ID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
		CreatedBy: m.CreatedBy,
		Name:      m.Name,
		Type:      m.Type,
	}

	if m.Description != nil {
		info.Description = *m.Description
	}
	if m.Status != nil {
		info.Status = *m.Status
	}
	info.EmbeddingModelID = m.EmbeddingModelID
	if m.EmbeddingModelName != nil {
		info.EmbeddingModelName = *m.EmbeddingModelName
	}
	if m.EmbeddingDimension != nil {
		info.EmbeddingDimension = *m.EmbeddingDimension
	}
	if m.ChunkSize != nil {
		info.ChunkSize = *m.ChunkSize
	}
	if m.ChunkOverlap != nil {
		info.ChunkOverlap = *m.ChunkOverlap
	}
	if m.SimilarityThreshold != nil {
		info.SimilarityThreshold = *m.SimilarityThreshold
	}
	if m.TopK != nil {
		info.TopK = *m.TopK
	}
	if m.QdrantCollection != nil {
		info.QdrantCollection = *m.QdrantCollection
	}
	if m.DocumentCount != nil {
		info.DocumentCount = *m.DocumentCount
	}
	if m.ChunkCount != nil {
		info.ChunkCount = *m.ChunkCount
	}

	return info
}

func (l *KnowledgeBaseLogic) toDocumentInfo(m *model.TKnowledgeDocument) *KnowledgeDocumentInfo {
	info := &KnowledgeDocumentInfo{
		ID:              m.ID,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
		KnowledgeBaseID: m.KnowledgeBaseID,
		Name:            m.Name,
	}

	if m.FileType != nil {
		info.FileType = *m.FileType
	}
	if m.FileSize != nil {
		info.FileSize = *m.FileSize
	}
	if m.Status != nil {
		info.Status = *m.Status
	}
	if m.ErrorMessage != nil {
		info.ErrorMessage = *m.ErrorMessage
	}
	if m.ChunkCount != nil {
		info.ChunkCount = *m.ChunkCount
	}
	if m.TokenCount != nil {
		info.TokenCount = *m.TokenCount
	}

	return info
}

// InferFileType 根据文件名推断文件类型
func InferFileType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return "pdf"
	case ".txt":
		return "txt"
	case ".md", ".markdown":
		return "md"
	case ".doc", ".docx":
		return "docx"
	case ".html", ".htm":
		return "html"
	case ".csv":
		return "csv"
	case ".json":
		return "json"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return "image"
	case ".mp3", ".wav", ".m4a", ".ogg", ".flac":
		return "audio"
	case ".mp4", ".avi", ".mov", ".webm", ".mkv":
		return "video"
	default:
		return "txt"
	}
}

// IsTextFileType 判断文件类型是否为纯文本类
func IsTextFileType(fileType string) bool {
	switch fileType {
	case "txt", "md", "csv", "json", "html":
		return true
	default:
		return false
	}
}

// Base64Encode 将字节数据编码为 Base64 字符串
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode 将 Base64 字符串解码为字节数据
func Base64Decode(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}
