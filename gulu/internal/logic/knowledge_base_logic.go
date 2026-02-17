package logic

import (
	"context"
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
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	DocumentID int64   `json:"document_id"`
	ChunkIndex int     `json:"chunk_index"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
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

	// 异步创建 Qdrant Collection（不阻塞接口返回）
	if req.Type == "normal" {
		go func() {
			if err := CreateQdrantCollection(collectionName, int(embeddingDimension)); err != nil {
				fmt.Printf("[WARN] 创建 Qdrant Collection 失败: %v\n", err)
			}
		}()
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
	kb, err := q.TKnowledgeBase.WithContext(l.ctx).Where(
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

	// 异步处理文档（分块 + Embedding + 写入 Qdrant）
	go func() {
		processor := NewDocumentProcessor()
		if err := processor.Process(kb, doc); err != nil {
			fmt.Printf("[ERROR] 文档处理失败: docID=%d, err=%v\n", doc.ID, err)
		}
	}()

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

// Search 知识库检索
func (l *KnowledgeBaseLogic) Search(kbID int64, req *KnowledgeSearchReq) ([]*KnowledgeSearchResult, error) {
	kb, err := l.GetByID(kbID)
	if err != nil {
		return nil, errors.New("知识库不存在")
	}

	if kb.QdrantCollection == "" {
		return nil, errors.New("知识库尚未初始化向量存储")
	}

	topK := req.TopK
	if topK <= 0 {
		topK = int(kb.TopK)
	}
	score := req.Score
	if score <= 0 {
		score = kb.SimilarityThreshold
	}

	// TODO: 调用 Embedding 模型生成查询向量，然后搜索 Qdrant
	// 当前返回占位结果，Phase 1 后续接入 Qdrant 检索
	return []*KnowledgeSearchResult{}, nil
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
