package logic

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/svc"
)

// KnowledgeBaseLogic 知识库逻辑
type KnowledgeBaseLogic struct {
	ctx context.Context
}

func NewKnowledgeBaseLogic(ctx context.Context) *KnowledgeBaseLogic {
	return &KnowledgeBaseLogic{ctx: ctx}
}

// -----------------------------------------------
// Request / Response 结构体
// -----------------------------------------------

type CreateKnowledgeBaseReq struct {
	Name                string  `json:"name"`
	Description         string  `json:"description"`
	Type                string  `json:"type"`
	EmbeddingModelID    *int64  `json:"embedding_model_id"`
	EmbeddingModelName  string  `json:"embedding_model_name"`
	EmbeddingDimension  int32   `json:"embedding_dimension"`
	ChunkSize           int32   `json:"chunk_size"`
	ChunkOverlap        int32   `json:"chunk_overlap"`
	SimilarityThreshold float64 `json:"similarity_threshold"`
	TopK                int32   `json:"top_k"`
	RetrievalMode       string  `json:"retrieval_mode"`
}

type UpdateKnowledgeBaseReq struct {
	Name                string  `json:"name"`
	Description         string  `json:"description"`
	EmbeddingModelID    *int64  `json:"embedding_model_id"`
	EmbeddingModelName  string  `json:"embedding_model_name"`
	EmbeddingDimension  int32   `json:"embedding_dimension"`
	ChunkSize           int32   `json:"chunk_size"`
	ChunkOverlap        int32   `json:"chunk_overlap"`
	SimilarityThreshold float64 `json:"similarity_threshold"`
	TopK                int32   `json:"top_k"`
	RetrievalMode       string  `json:"retrieval_mode"`
	RerankModelID       *int64  `json:"rerank_model_id"`
	RerankEnabled       *bool   `json:"rerank_enabled"`
}

type KnowledgeBaseListReq struct {
	Page     int    `query:"page"`
	PageSize int    `query:"pageSize"`
	Name     string `query:"name"`
	Type     string `query:"type"`
	Status   *int32 `query:"status"`
}

type KnowledgeBaseInfo struct {
	ID                  int64      `json:"id"`
	CreatedAt           *time.Time `json:"created_at"`
	UpdatedAt           *time.Time `json:"updated_at"`
	CreatedBy           *int64     `json:"created_by"`
	Name                string     `json:"name"`
	Description         string     `json:"description"`
	Type                string     `json:"type"`
	Status              int32      `json:"status"`
	EmbeddingModelID    *int64     `json:"embedding_model_id"`
	EmbeddingModelName  string     `json:"embedding_model_name"`
	EmbeddingDimension  int32      `json:"embedding_dimension"`
	ChunkSize           int32      `json:"chunk_size"`
	ChunkOverlap        int32      `json:"chunk_overlap"`
	SimilarityThreshold float64    `json:"similarity_threshold"`
	TopK                int32      `json:"top_k"`
	RetrievalMode       string     `json:"retrieval_mode"`
	RerankModelID       *int64     `json:"rerank_model_id"`
	RerankEnabled       bool       `json:"rerank_enabled"`
	QdrantCollection    string     `json:"qdrant_collection"`
	DocumentCount       int32      `json:"document_count"`
	ChunkCount          int32      `json:"chunk_count"`
}

type KnowledgeDocumentInfo struct {
	ID                  int64      `json:"id"`
	CreatedAt           *time.Time `json:"created_at"`
	UpdatedAt           *time.Time `json:"updated_at"`
	KnowledgeBaseID     int64      `json:"knowledge_base_id"`
	Name                string     `json:"name"`
	FileType            string     `json:"file_type"`
	FileSize            int64      `json:"file_size"`
	WordCount           int32      `json:"word_count"`
	IndexingStatus      string     `json:"indexing_status"`
	ErrorMessage        string     `json:"error_message"`
	ChunkCount          int32      `json:"chunk_count"`
	TokenCount          int32      `json:"token_count"`
	ParsingCompletedAt  *time.Time `json:"parsing_completed_at"`
	IndexingCompletedAt *time.Time `json:"indexing_completed_at"`
}

type KnowledgeSearchReq struct {
	Query         string  `json:"query"`
	TopK          int     `json:"top_k"`
	Score         float64 `json:"score"`
	RetrievalMode string  `json:"retrieval_mode"`
}

type KnowledgeSearchResult struct {
	SegmentID    int64                  `json:"segment_id"`
	Content      string                 `json:"content"`
	Score        float64                `json:"score"`
	DocumentID   int64                  `json:"document_id"`
	DocumentName string                 `json:"document_name"`
	ChunkIndex   int                    `json:"chunk_index"`
	WordCount    int                    `json:"word_count"`
	HitCount     int                    `json:"hit_count"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type SegmentInfo struct {
	ID              int64      `json:"id"`
	DocumentID      int64      `json:"document_id"`
	DocumentName    string     `json:"document_name"`
	Content         string     `json:"content"`
	Position        int        `json:"position"`
	WordCount       int        `json:"word_count"`
	Enabled         bool       `json:"enabled"`
	HitCount        int        `json:"hit_count"`
	Status          string     `json:"status"`
	CreatedAt       *time.Time `json:"created_at"`
}

type PreviewChunksReq struct {
	DocumentID   int64               `json:"document_id"`
	Content      string              `json:"content"`
	ChunkSetting *model.ChunkSetting `json:"chunk_setting"`
}

type PreviewChunkItem struct {
	Index     int    `json:"index"`
	Content   string `json:"content"`
	CharCount int    `json:"char_count"`
}

type ProcessDocumentReq struct {
	ChunkSetting *model.ChunkSetting `json:"chunk_setting"`
}

type BatchDocIDsReq struct {
	DocumentIDs []int64 `json:"document_ids"`
}

type UpdateSegmentReq struct {
	Content *string `json:"content"`
	Enabled *bool   `json:"enabled"`
}

type QueryHistoryItem struct {
	ID            int64      `json:"id"`
	QueryText     string     `json:"query_text"`
	RetrievalMode string     `json:"retrieval_mode"`
	ResultCount   int        `json:"result_count"`
	CreatedAt     *time.Time `json:"created_at"`
}

// -----------------------------------------------
// 知识库 CRUD
// -----------------------------------------------

func (l *KnowledgeBaseLogic) Create(req *CreateKnowledgeBaseReq) (*KnowledgeBaseInfo, error) {
	db := svc.Ctx.DB
	now := time.Now()
	isDelete := false
	status := int32(1)

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
	retrievalMode := req.RetrievalMode
	if retrievalMode == "" {
		retrievalMode = "vector"
	}

	kb := &model.TKnowledgeBase{
		CreatedAt:           &now,
		UpdatedAt:           &now,
		IsDelete:            &isDelete,
		Name:                req.Name,
		Description:         strPtr(req.Description),
		Type:                req.Type,
		Status:              &status,
		EmbeddingModelID:    req.EmbeddingModelID,
		EmbeddingModelName:  strPtr(req.EmbeddingModelName),
		EmbeddingDimension:  &embeddingDimension,
		ChunkSize:           &chunkSize,
		ChunkOverlap:        &chunkOverlap,
		SimilarityThreshold: &similarityThreshold,
		TopK:                &topK,
		RetrievalMode:       &retrievalMode,
	}

	if err := db.Create(kb).Error; err != nil {
		return nil, err
	}

	collectionName := fmt.Sprintf("kb_%d", kb.ID)
	db.Model(kb).Update("qdrant_collection", collectionName)
	kb.QdrantCollection = &collectionName

	if req.Type == "normal" {
		if err := CreateQdrantCollection(collectionName, int(embeddingDimension)); err != nil {
			log.Printf("[WARN] 创建 Qdrant Collection 失败: %v", err)
		}
	}

	return l.toKnowledgeBaseInfo(kb), nil
}

func (l *KnowledgeBaseLogic) Update(id int64, req *UpdateKnowledgeBaseReq) error {
	db := svc.Ctx.DB

	var kb model.TKnowledgeBase
	if err := db.Where("id = ? AND is_delete = 0", id).First(&kb).Error; err != nil {
		return errors.New("知识库不存在")
	}

	updates := map[string]interface{}{
		"updated_at": time.Now(),
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
	if req.RetrievalMode != "" {
		updates["retrieval_mode"] = req.RetrievalMode
	}
	if req.RerankModelID != nil {
		updates["rerank_model_id"] = *req.RerankModelID
	}
	if req.RerankEnabled != nil {
		updates["rerank_enabled"] = *req.RerankEnabled
	}

	return db.Model(&model.TKnowledgeBase{}).Where("id = ?", id).Updates(updates).Error
}

func (l *KnowledgeBaseLogic) Delete(id int64) error {
	db := svc.Ctx.DB

	var kb model.TKnowledgeBase
	if err := db.Where("id = ? AND is_delete = 0", id).First(&kb).Error; err != nil {
		return errors.New("知识库不存在")
	}

	db.Model(&model.TKnowledgeBase{}).Where("id = ?", id).Update("is_delete", true)

	go func() {
		if kb.QdrantCollection != nil && *kb.QdrantCollection != "" {
			if err := DeleteQdrantCollection(*kb.QdrantCollection); err != nil {
				log.Printf("[WARN] 删除 Qdrant Collection 失败: %v", err)
			}
		}
		db.Where("knowledge_base_id = ?", id).Delete(&model.TKnowledgeSegment{})
		db.Where("knowledge_base_id = ?", id).Delete(&model.TKnowledgeDocument{})
		db.Where("knowledge_base_id = ?", id).Delete(&model.TKnowledgeQuery{})
		GetFileStorage().DeleteDir(id)
	}()

	return nil
}

func (l *KnowledgeBaseLogic) GetByID(id int64) (*KnowledgeBaseInfo, error) {
	db := svc.Ctx.DB
	var kb model.TKnowledgeBase
	if err := db.Where("id = ? AND is_delete = 0", id).First(&kb).Error; err != nil {
		return nil, err
	}
	return l.toKnowledgeBaseInfo(&kb), nil
}

func (l *KnowledgeBaseLogic) List(req *KnowledgeBaseListReq) ([]*KnowledgeBaseInfo, int64, error) {
	db := svc.Ctx.DB
	q := db.Model(&model.TKnowledgeBase{}).Where("is_delete = 0")

	if req.Name != "" {
		q = q.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Type != "" {
		q = q.Where("type = ?", req.Type)
	}
	if req.Status != nil {
		q = q.Where("status = ?", *req.Status)
	}

	var total int64
	q.Count(&total)

	var list []model.TKnowledgeBase
	offset := (req.Page - 1) * req.PageSize
	if err := q.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*KnowledgeBaseInfo, 0, len(list))
	for i := range list {
		result = append(result, l.toKnowledgeBaseInfo(&list[i]))
	}
	return result, total, nil
}

func (l *KnowledgeBaseLogic) UpdateStatus(id int64, status int32) error {
	db := svc.Ctx.DB
	return db.Model(&model.TKnowledgeBase{}).Where("id = ? AND is_delete = 0", id).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}).Error
}

// -----------------------------------------------
// 文档管理
// -----------------------------------------------

func (l *KnowledgeBaseLogic) CreateDocument(kbID int64, name, fileType, filePath string, fileSize int64) (*KnowledgeDocumentInfo, error) {
	db := svc.Ctx.DB

	var kb model.TKnowledgeBase
	if err := db.Where("id = ? AND is_delete = 0", kbID).First(&kb).Error; err != nil {
		return nil, errors.New("知识库不存在")
	}

	now := time.Now()
	status := "waiting"

	doc := &model.TKnowledgeDocument{
		CreatedAt:       &now,
		UpdatedAt:       &now,
		KnowledgeBaseID: kbID,
		Name:            name,
		FileType:        &fileType,
		FilePath:        &filePath,
		FileSize:        &fileSize,
		IndexingStatus:  &status,
	}

	if err := db.Create(doc).Error; err != nil {
		return nil, err
	}

	go l.updateDocumentCount(kbID)
	return l.toDocumentInfo(doc), nil
}

func (l *KnowledgeBaseLogic) ListDocuments(kbID int64) ([]*KnowledgeDocumentInfo, error) {
	db := svc.Ctx.DB
	var list []model.TKnowledgeDocument
	if err := db.Where("knowledge_base_id = ?", kbID).Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}

	result := make([]*KnowledgeDocumentInfo, 0, len(list))
	for i := range list {
		result = append(result, l.toDocumentInfo(&list[i]))
	}
	return result, nil
}

func (l *KnowledgeBaseLogic) DeleteDocument(kbID, docID int64) error {
	db := svc.Ctx.DB

	var kb model.TKnowledgeBase
	if err := db.Where("id = ? AND is_delete = 0", kbID).First(&kb).Error; err != nil {
		return errors.New("知识库不存在")
	}

	var doc model.TKnowledgeDocument
	if err := db.Where("id = ? AND knowledge_base_id = ?", docID, kbID).First(&doc).Error; err != nil {
		return errors.New("文档不存在")
	}

	db.Where("id = ?", docID).Delete(&model.TKnowledgeDocument{})
	db.Where("document_id = ?", docID).Delete(&model.TKnowledgeSegment{})

	go func() {
		l.updateDocumentCount(kbID)
		if kb.QdrantCollection != nil && *kb.QdrantCollection != "" {
			if err := DeleteDocumentVectors(*kb.QdrantCollection, docID); err != nil {
				log.Printf("[WARN] 清理文档向量失败: docID=%d, err=%v", docID, err)
			}
		}
		if doc.FilePath != nil && *doc.FilePath != "" {
			GetFileStorage().Delete(*doc.FilePath)
		}
	}()

	return nil
}

func (l *KnowledgeBaseLogic) BatchDeleteDocuments(kbID int64, docIDs []int64) error {
	db := svc.Ctx.DB

	var kb model.TKnowledgeBase
	if err := db.Where("id = ? AND is_delete = 0", kbID).First(&kb).Error; err != nil {
		return errors.New("知识库不存在")
	}

	var docs []model.TKnowledgeDocument
	db.Where("id IN ? AND knowledge_base_id = ?", docIDs, kbID).Find(&docs)

	db.Where("id IN ? AND knowledge_base_id = ?", docIDs, kbID).Delete(&model.TKnowledgeDocument{})
	db.Where("document_id IN ? AND knowledge_base_id = ?", docIDs, kbID).Delete(&model.TKnowledgeSegment{})

	go func() {
		l.updateDocumentCount(kbID)
		for _, doc := range docs {
			if kb.QdrantCollection != nil && *kb.QdrantCollection != "" {
				DeleteDocumentVectors(*kb.QdrantCollection, doc.ID)
			}
			if doc.FilePath != nil && *doc.FilePath != "" {
				GetFileStorage().Delete(*doc.FilePath)
			}
		}
	}()

	return nil
}

func (l *KnowledgeBaseLogic) ReprocessDocument(kbID, docID int64) error {
	db := svc.Ctx.DB

	var kb model.TKnowledgeBase
	if err := db.Where("id = ? AND is_delete = 0", kbID).First(&kb).Error; err != nil {
		return errors.New("知识库不存在")
	}

	var doc model.TKnowledgeDocument
	if err := db.Where("id = ? AND knowledge_base_id = ?", docID, kbID).First(&doc).Error; err != nil {
		return errors.New("文档不存在")
	}

	db.Model(&model.TKnowledgeDocument{}).Where("id = ?", docID).Updates(map[string]interface{}{
		"indexing_status": "waiting",
		"error_message":   nil,
	})

	go func() {
		processor := NewDocumentProcessor()
		if err := processor.Process(&kb, &doc); err != nil {
			log.Printf("[ERROR] 文档重处理失败: docID=%d, err=%v", doc.ID, err)
		}
	}()

	return nil
}

func (l *KnowledgeBaseLogic) BatchReprocessDocuments(kbID int64, docIDs []int64) error {
	db := svc.Ctx.DB

	var kb model.TKnowledgeBase
	if err := db.Where("id = ? AND is_delete = 0", kbID).First(&kb).Error; err != nil {
		return errors.New("知识库不存在")
	}

	var docs []model.TKnowledgeDocument
	db.Where("id IN ? AND knowledge_base_id = ?", docIDs, kbID).Find(&docs)

	for _, doc := range docs {
		db.Model(&model.TKnowledgeDocument{}).Where("id = ?", doc.ID).Updates(map[string]interface{}{
			"indexing_status": "waiting",
			"error_message":   nil,
		})
	}

	go func() {
		processor := NewDocumentProcessor()
		for _, doc := range docs {
			d := doc
			if err := processor.Process(&kb, &d); err != nil {
				log.Printf("[ERROR] 批量重处理失败: docID=%d, err=%v", d.ID, err)
			}
		}
	}()

	return nil
}

// GetIndexingStatus 获取知识库下所有文档的索引状态
func (l *KnowledgeBaseLogic) GetIndexingStatus(kbID int64) ([]*KnowledgeDocumentInfo, error) {
	db := svc.Ctx.DB
	var docs []model.TKnowledgeDocument
	if err := db.Where("knowledge_base_id = ?", kbID).
		Select("id, name, file_type, indexing_status, error_message, chunk_count, updated_at").
		Find(&docs).Error; err != nil {
		return nil, err
	}

	result := make([]*KnowledgeDocumentInfo, 0, len(docs))
	for i := range docs {
		result = append(result, l.toDocumentInfo(&docs[i]))
	}
	return result, nil
}

// -----------------------------------------------
// 分块管理
// -----------------------------------------------

func (l *KnowledgeBaseLogic) GetDocumentSegments(kbID, docID int64, page, pageSize int) ([]*SegmentInfo, int64, error) {
	db := svc.Ctx.DB

	q := db.Model(&model.TKnowledgeSegment{}).Where("knowledge_base_id = ? AND document_id = ?", kbID, docID)

	var total int64
	q.Count(&total)

	var segments []model.TKnowledgeSegment
	offset := (page - 1) * pageSize
	if err := q.Order("position ASC").Offset(offset).Limit(pageSize).Find(&segments).Error; err != nil {
		return nil, 0, err
	}

	var doc model.TKnowledgeDocument
	db.Where("id = ?", docID).First(&doc)

	result := make([]*SegmentInfo, 0, len(segments))
	for _, seg := range segments {
		result = append(result, &SegmentInfo{
			ID:           seg.ID,
			DocumentID:   seg.DocumentID,
			DocumentName: doc.Name,
			Content:      seg.Content,
			Position:     seg.Position,
			WordCount:    seg.WordCount,
			Enabled:      seg.Enabled,
			HitCount:     seg.HitCount,
			Status:       seg.Status,
			CreatedAt:    seg.CreatedAt,
		})
	}
	return result, total, nil
}

func (l *KnowledgeBaseLogic) UpdateSegment(kbID, segID int64, req *UpdateSegmentReq) error {
	db := svc.Ctx.DB

	var seg model.TKnowledgeSegment
	if err := db.Where("id = ? AND knowledge_base_id = ?", segID, kbID).First(&seg).Error; err != nil {
		return errors.New("分块不存在")
	}

	updates := map[string]interface{}{"updated_at": time.Now()}
	needReEmbed := false

	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
		if *req.Enabled {
			updates["status"] = "active"
		} else {
			updates["status"] = "disabled"
		}
	}
	if req.Content != nil && *req.Content != seg.Content {
		updates["content"] = *req.Content
		updates["word_count"] = utf8.RuneCountInString(*req.Content)
		needReEmbed = true
	}

	if err := db.Model(&model.TKnowledgeSegment{}).Where("id = ?", segID).Updates(updates).Error; err != nil {
		return err
	}

	// 内容变更需要重新生成向量
	if needReEmbed {
		go l.reEmbedSegment(kbID, segID, *req.Content)
	}

	return nil
}

func (l *KnowledgeBaseLogic) reEmbedSegment(kbID, segID int64, content string) {
	db := svc.Ctx.DB

	var kb model.TKnowledgeBase
	if err := db.Where("id = ?", kbID).First(&kb).Error; err != nil {
		return
	}

	processor := NewDocumentProcessor()
	embClient, err := processor.getEmbeddingClient(&kb)
	if err != nil {
		log.Printf("[ERROR] 重新生成向量失败: %v", err)
		return
	}

	vector, err := embClient.EmbedText(context.Background(), content)
	if err != nil {
		log.Printf("[ERROR] 重新生成向量失败: %v", err)
		return
	}

	var seg model.TKnowledgeSegment
	if err := db.Where("id = ?", segID).First(&seg).Error; err != nil {
		return
	}

	if kb.QdrantCollection != nil && seg.IndexNodeID != nil {
		point := VectorPoint{
			ID:         *seg.IndexNodeID,
			Vector:     vector,
			DocumentID: seg.DocumentID,
			ChunkIndex: seg.Position,
			Content:    content,
		}
		UpsertVectors(*kb.QdrantCollection, []VectorPoint{point})
	}
}

// -----------------------------------------------
// 分块预览 + 文档处理
// -----------------------------------------------

func (l *KnowledgeBaseLogic) PreviewChunks(kbID int64, req *PreviewChunksReq) ([]*PreviewChunkItem, error) {
	text := req.Content

	if req.DocumentID > 0 {
		db := svc.Ctx.DB
		var doc model.TKnowledgeDocument
		if err := db.Where("id = ? AND knowledge_base_id = ?", req.DocumentID, kbID).First(&doc).Error; err != nil {
			return nil, errors.New("文档不存在")
		}
		processor := NewDocumentProcessor()
		extracted, err := processor.extractText(&doc)
		if err != nil {
			return nil, fmt.Errorf("文本提取失败: %w", err)
		}
		text = extracted
	}

	if text == "" {
		return nil, errors.New("文档内容为空")
	}

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

	if cs.CleanWhitespace {
		text = cleanWhitespace(text)
	}
	if cs.RemoveURLs {
		text = removeURLsAndEmails(text)
	}

	var chunks []string
	if cs.Separator != "" {
		chunks = splitBySeparator(text, cs.Separator, cs.ChunkSize, cs.ChunkOverlap)
	} else {
		chunks = splitText(text, cs.ChunkSize, cs.ChunkOverlap)
	}

	result := make([]*PreviewChunkItem, 0, len(chunks))
	for i, chunk := range chunks {
		result = append(result, &PreviewChunkItem{
			Index:     i,
			Content:   chunk,
			CharCount: utf8.RuneCountInString(chunk),
		})
	}
	return result, nil
}

func (l *KnowledgeBaseLogic) ProcessDocument(kbID, docID int64, req *ProcessDocumentReq) error {
	db := svc.Ctx.DB

	var kb model.TKnowledgeBase
	if err := db.Where("id = ? AND is_delete = 0", kbID).First(&kb).Error; err != nil {
		return errors.New("知识库不存在")
	}

	var doc model.TKnowledgeDocument
	if err := db.Where("id = ? AND knowledge_base_id = ?", docID, kbID).First(&doc).Error; err != nil {
		return errors.New("文档不存在")
	}

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

	db.Model(&model.TKnowledgeDocument{}).Where("id = ?", docID).Updates(map[string]interface{}{
		"chunk_setting":   cs,
		"indexing_status": "waiting",
	})

	db.Where("id = ?", docID).First(&doc)

	go func() {
		processor := NewDocumentProcessor()
		if err := processor.Process(&kb, &doc); err != nil {
			log.Printf("[ERROR] 文档处理失败: docID=%d, err=%v", doc.ID, err)
		}
	}()

	return nil
}

// -----------------------------------------------
// 知识库检索
// -----------------------------------------------

func (l *KnowledgeBaseLogic) Search(kbID int64, req *KnowledgeSearchReq) ([]*KnowledgeSearchResult, error) {
	db := svc.Ctx.DB

	var kb model.TKnowledgeBase
	if err := db.Where("id = ? AND is_delete = 0", kbID).First(&kb).Error; err != nil {
		return nil, errors.New("知识库不存在")
	}

	if kb.QdrantCollection == nil || *kb.QdrantCollection == "" {
		return nil, errors.New("知识库尚未初始化向量存储")
	}

	topK := req.TopK
	if topK <= 0 && kb.TopK != nil {
		topK = int(*kb.TopK)
	}
	if topK <= 0 {
		topK = 5
	}
	score := req.Score
	if score <= 0 && kb.SimilarityThreshold != nil {
		score = *kb.SimilarityThreshold
	}

	retrievalMode := req.RetrievalMode
	if retrievalMode == "" && kb.RetrievalMode != nil {
		retrievalMode = *kb.RetrievalMode
	}
	if retrievalMode == "" {
		retrievalMode = "vector"
	}

	var results []*KnowledgeSearchResult

	switch retrievalMode {
	case "keyword":
		results = l.keywordSearch(kbID, req.Query, topK)
	case "hybrid":
		vectorResults := l.vectorSearch(&kb, req.Query, topK, score)
		keywordResults := l.keywordSearch(kbID, req.Query, topK)
		results = l.mergeResults(vectorResults, keywordResults, topK)
	default:
		results = l.vectorSearch(&kb, req.Query, topK, score)
	}

	// 保存查询历史
	go l.saveQueryHistory(kbID, req.Query, retrievalMode, topK, score, len(results))

	// 更新命中计数
	go l.updateHitCounts(results)

	return results, nil
}

func (l *KnowledgeBaseLogic) vectorSearch(kb *model.TKnowledgeBase, query string, topK int, score float64) []*KnowledgeSearchResult {
	if kb.EmbeddingModelID == nil || *kb.EmbeddingModelID == 0 {
		return nil
	}

	aiModelLogic := NewAiModelLogic(l.ctx)
	aiModel, err := aiModelLogic.GetByIDWithKey(*kb.EmbeddingModelID)
	if err != nil {
		log.Printf("[ERROR] 获取嵌入模型失败: %v", err)
		return nil
	}

	embClient := NewEmbeddingClient(aiModel.APIBaseURL, aiModel.APIKey, aiModel.ModelID)
	queryVector, err := embClient.EmbedText(l.ctx, query)
	if err != nil {
		log.Printf("[ERROR] 查询向量化失败: %v", err)
		return nil
	}

	hits, err := SearchVectors(*kb.QdrantCollection, queryVector, topK, float32(score))
	if err != nil {
		log.Printf("[ERROR] 向量搜索失败: %v", err)
		return nil
	}

	docNameCache := make(map[int64]string)
	results := make([]*KnowledgeSearchResult, 0, len(hits))
	for _, hit := range hits {
		docName := getDocNameCached(hit.DocumentID, docNameCache)
		results = append(results, &KnowledgeSearchResult{
			Content:      hit.Content,
			Score:        hit.Score,
			DocumentID:   hit.DocumentID,
			DocumentName: docName,
			ChunkIndex:   hit.ChunkIndex,
			WordCount:    utf8.RuneCountInString(hit.Content),
			Metadata:     hit.Metadata,
		})
	}
	return results
}

func (l *KnowledgeBaseLogic) keywordSearch(kbID int64, query string, topK int) []*KnowledgeSearchResult {
	db := svc.Ctx.DB
	var segments []model.TKnowledgeSegment

	err := db.Where("knowledge_base_id = ? AND enabled = 1 AND MATCH(content) AGAINST(? IN NATURAL LANGUAGE MODE)", kbID, query).
		Limit(topK).Find(&segments).Error
	if err != nil {
		log.Printf("[WARN] 全文检索失败，降级到 LIKE: %v", err)
		db.Where("knowledge_base_id = ? AND enabled = 1 AND content LIKE ?", kbID, "%"+query+"%").
			Limit(topK).Find(&segments)
	}

	docNameCache := make(map[int64]string)
	results := make([]*KnowledgeSearchResult, 0, len(segments))
	for _, seg := range segments {
		docName := getDocNameCached(seg.DocumentID, docNameCache)
		results = append(results, &KnowledgeSearchResult{
			SegmentID:    seg.ID,
			Content:      seg.Content,
			Score:        0.5,
			DocumentID:   seg.DocumentID,
			DocumentName: docName,
			ChunkIndex:   seg.Position,
			WordCount:    seg.WordCount,
			HitCount:     seg.HitCount,
		})
	}
	return results
}

func (l *KnowledgeBaseLogic) mergeResults(vectorResults, keywordResults []*KnowledgeSearchResult, topK int) []*KnowledgeSearchResult {
	seen := make(map[string]bool)
	var merged []*KnowledgeSearchResult

	for _, r := range vectorResults {
		key := fmt.Sprintf("%d_%d", r.DocumentID, r.ChunkIndex)
		if !seen[key] {
			seen[key] = true
			merged = append(merged, r)
		}
	}
	for _, r := range keywordResults {
		key := fmt.Sprintf("%d_%d", r.DocumentID, r.ChunkIndex)
		if !seen[key] {
			seen[key] = true
			r.Score = r.Score * 0.8
			merged = append(merged, r)
		}
	}

	if len(merged) > topK {
		merged = merged[:topK]
	}
	return merged
}

// -----------------------------------------------
// 查询历史
// -----------------------------------------------

func (l *KnowledgeBaseLogic) GetQueryHistory(kbID int64, limit int) ([]*QueryHistoryItem, error) {
	db := svc.Ctx.DB
	if limit <= 0 {
		limit = 20
	}

	var queries []model.TKnowledgeQuery
	if err := db.Where("knowledge_base_id = ?", kbID).
		Order("id DESC").Limit(limit).Find(&queries).Error; err != nil {
		return nil, err
	}

	result := make([]*QueryHistoryItem, 0, len(queries))
	for _, q := range queries {
		result = append(result, &QueryHistoryItem{
			ID:            q.ID,
			QueryText:     q.QueryText,
			RetrievalMode: q.RetrievalMode,
			ResultCount:   q.ResultCount,
			CreatedAt:     q.CreatedAt,
		})
	}
	return result, nil
}

func (l *KnowledgeBaseLogic) saveQueryHistory(kbID int64, query, mode string, topK int, score float64, resultCount int) {
	db := svc.Ctx.DB
	now := time.Now()
	q := &model.TKnowledgeQuery{
		CreatedAt:       &now,
		KnowledgeBaseID: kbID,
		QueryText:       query,
		RetrievalMode:   mode,
		TopK:            topK,
		ScoreThreshold:  score,
		ResultCount:     resultCount,
		Source:          "hit_testing",
	}
	db.Create(q)
}

func (l *KnowledgeBaseLogic) updateHitCounts(results []*KnowledgeSearchResult) {
	db := svc.Ctx.DB
	for _, r := range results {
		if r.SegmentID > 0 {
			db.Exec("UPDATE t_knowledge_segment SET hit_count = hit_count + 1 WHERE id = ?", r.SegmentID)
		}
	}
}

// -----------------------------------------------
// 工具方法
// -----------------------------------------------

func getDocNameCached(docID int64, cache map[int64]string) string {
	if name, ok := cache[docID]; ok {
		return name
	}
	db := svc.Ctx.DB
	var doc model.TKnowledgeDocument
	if err := db.Where("id = ?", docID).Select("name").First(&doc).Error; err == nil {
		cache[docID] = doc.Name
		return doc.Name
	}
	cache[docID] = ""
	return ""
}

func (l *KnowledgeBaseLogic) updateDocumentCount(kbID int64) {
	db := svc.Ctx.DB
	var count int64
	db.Model(&model.TKnowledgeDocument{}).Where("knowledge_base_id = ?", kbID).Count(&count)
	db.Model(&model.TKnowledgeBase{}).Where("id = ?", kbID).Update("document_count", int32(count))
}

func (l *KnowledgeBaseLogic) toKnowledgeBaseInfo(m *model.TKnowledgeBase) *KnowledgeBaseInfo {
	info := &KnowledgeBaseInfo{
		ID:               m.ID,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
		CreatedBy:        m.CreatedBy,
		Name:             m.Name,
		Type:             m.Type,
		EmbeddingModelID: m.EmbeddingModelID,
		RerankModelID:    m.RerankModelID,
	}
	if m.Description != nil {
		info.Description = *m.Description
	}
	if m.Status != nil {
		info.Status = *m.Status
	}
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
	if m.RetrievalMode != nil {
		info.RetrievalMode = *m.RetrievalMode
	}
	if m.RerankEnabled != nil {
		info.RerankEnabled = *m.RerankEnabled
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
		ID:                  m.ID,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
		KnowledgeBaseID:     m.KnowledgeBaseID,
		Name:                m.Name,
		ParsingCompletedAt:  m.ParsingCompletedAt,
		IndexingCompletedAt: m.IndexingCompletedAt,
	}
	if m.FileType != nil {
		info.FileType = *m.FileType
	}
	if m.FileSize != nil {
		info.FileSize = *m.FileSize
	}
	if m.WordCount != nil {
		info.WordCount = *m.WordCount
	}
	if m.IndexingStatus != nil {
		info.IndexingStatus = *m.IndexingStatus
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
	default:
		return "txt"
	}
}

func IsTextFileType(fileType string) bool {
	switch fileType {
	case "txt", "md", "csv", "json", "html":
		return true
	default:
		return false
	}
}
