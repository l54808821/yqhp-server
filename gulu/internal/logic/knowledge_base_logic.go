package logic

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/svc"
)

// safeGo 启动一个带 panic 恢复的后台 goroutine，防止 panic 导致整个服务崩溃。
func safeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC] goroutine recovered: %v\n%s", r, debug.Stack())
			}
		}()
		fn()
	}()
}

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
	// 多模态配置（Phase 2）
	MultimodalEnabled   bool    `json:"multimodal_enabled"`
	MultimodalModelID   *int64  `json:"multimodal_model_id"`
	MultimodalModelName string  `json:"multimodal_model_name"`
	MultimodalDimension int32   `json:"multimodal_dimension"`
	// 分块配置
	ChunkSize           int32   `json:"chunk_size"`
	ChunkOverlap        int32   `json:"chunk_overlap"`
	SimilarityThreshold float64 `json:"similarity_threshold"`
	TopK                int32   `json:"top_k"`
	RetrievalMode       string  `json:"retrieval_mode"`
	// 图知识库配置（Phase 3）
	GraphExtractModelID *int64  `json:"graph_extract_model_id"`
}

type UpdateKnowledgeBaseReq struct {
	Name                string  `json:"name"`
	Description         string  `json:"description"`
	EmbeddingModelID    *int64  `json:"embedding_model_id"`
	EmbeddingModelName  string  `json:"embedding_model_name"`
	EmbeddingDimension  int32   `json:"embedding_dimension"`
	MultimodalEnabled   *bool   `json:"multimodal_enabled"`
	MultimodalModelID   *int64  `json:"multimodal_model_id"`
	MultimodalModelName string  `json:"multimodal_model_name"`
	MultimodalDimension int32   `json:"multimodal_dimension"`
	ChunkSize           int32   `json:"chunk_size"`
	ChunkOverlap        int32   `json:"chunk_overlap"`
	SimilarityThreshold float64 `json:"similarity_threshold"`
	TopK                int32   `json:"top_k"`
	RetrievalMode       string  `json:"retrieval_mode"`
	RerankModelID       *int64  `json:"rerank_model_id"`
	RerankEnabled       *bool   `json:"rerank_enabled"`
	GraphExtractModelID *int64  `json:"graph_extract_model_id"`
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
	MultimodalEnabled   bool       `json:"multimodal_enabled"`
	MultimodalModelID   *int64     `json:"multimodal_model_id"`
	MultimodalModelName string     `json:"multimodal_model_name"`
	MultimodalDimension int32      `json:"multimodal_dimension"`
	ChunkSize           int32      `json:"chunk_size"`
	ChunkOverlap        int32      `json:"chunk_overlap"`
	SimilarityThreshold float64    `json:"similarity_threshold"`
	TopK                int32      `json:"top_k"`
	RetrievalMode       string     `json:"retrieval_mode"`
	RerankModelID       *int64     `json:"rerank_model_id"`
	RerankEnabled       bool       `json:"rerank_enabled"`
	QdrantCollection    string     `json:"qdrant_collection"`
	GraphExtractModelID *int64     `json:"graph_extract_model_id"`
	DocumentCount       int32      `json:"document_count"`
	ChunkCount          int32      `json:"chunk_count"`
	EntityCount         int32      `json:"entity_count"`
	RelationCount       int32      `json:"relation_count"`
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
	ImageCount          int32      `json:"image_count"`
	IndexingStatus      string     `json:"indexing_status"`
	ErrorMessage        string     `json:"error_message"`
	ChunkCount          int32      `json:"chunk_count"`
	TokenCount          int32      `json:"token_count"`
	ParsingCompletedAt  *time.Time `json:"parsing_completed_at"`
	IndexingCompletedAt *time.Time `json:"indexing_completed_at"`
}

type KnowledgeSearchReq struct {
	Query         string  `json:"query"`
	QueryType     string  `json:"query_type"`     // text / image (default: text)
	TopK          int     `json:"top_k"`
	Score         float64 `json:"score"`
	RetrievalMode string  `json:"retrieval_mode"`
	SearchFields  string  `json:"search_fields"`  // text / image / all (default: all)
}

type KnowledgeSearchResult struct {
	SegmentID    int64                  `json:"segment_id"`
	Content      string                 `json:"content"`
	ContentType  string                 `json:"content_type"` // text / image
	ImagePath    string                 `json:"image_path,omitempty"`
	Score        float64                `json:"score"`
	DocumentID   int64                  `json:"document_id"`
	DocumentName string                 `json:"document_name"`
	ChunkIndex   int                    `json:"chunk_index"`
	WordCount    int                    `json:"word_count"`
	HitCount     int                    `json:"hit_count"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type SegmentInfo struct {
	ID           int64      `json:"id"`
	DocumentID   int64      `json:"document_id"`
	DocumentName string     `json:"document_name"`
	Content      string     `json:"content"`
	ContentType  string     `json:"content_type"` // text / image
	ImagePath    string     `json:"image_path,omitempty"`
	Position     int        `json:"position"`
	WordCount    int        `json:"word_count"`
	Enabled      bool       `json:"enabled"`
	HitCount     int        `json:"hit_count"`
	Status       string     `json:"status"`
	CreatedAt    *time.Time `json:"created_at"`
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

	multimodalEnabled := req.MultimodalEnabled
	var multimodalDimension *int32
	if req.MultimodalDimension > 0 {
		multimodalDimension = &req.MultimodalDimension
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
		MultimodalEnabled:   &multimodalEnabled,
		MultimodalModelID:   req.MultimodalModelID,
		MultimodalModelName: strPtr(req.MultimodalModelName),
		MultimodalDimension: multimodalDimension,
		ChunkSize:           &chunkSize,
		ChunkOverlap:        &chunkOverlap,
		SimilarityThreshold: &similarityThreshold,
		TopK:                &topK,
		RetrievalMode:       &retrievalMode,
		GraphExtractModelID: req.GraphExtractModelID,
	}

	if err := db.Create(kb).Error; err != nil {
		return nil, err
	}

	collectionName := fmt.Sprintf("kb_%d", kb.ID)
	db.Model(kb).Update("qdrant_collection", collectionName)
	kb.QdrantCollection = &collectionName

	if req.Type == "normal" || req.Type == "graph" {
		imageDim := 0
		if multimodalEnabled && multimodalDimension != nil {
			imageDim = int(*multimodalDimension)
		}
		if err := CreateQdrantCollectionMultiVector(collectionName, CollectionVectorConfig{
			TextDimension:  int(embeddingDimension),
			ImageDimension: imageDim,
		}); err != nil {
			log.Printf("[WARN] 创建 Qdrant Collection 失败: %v", err)
		}
	}

	// 图知识库设置 Neo4j database 名
	if req.Type == "graph" {
		neo4jDB := fmt.Sprintf("kb_%d", kb.ID)
		db.Model(kb).Update("neo4j_database", neo4jDB)
		kb.Neo4jDatabase = &neo4jDB
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
	if req.MultimodalEnabled != nil {
		updates["multimodal_enabled"] = *req.MultimodalEnabled
	}
	if req.MultimodalModelID != nil {
		updates["multimodal_model_id"] = *req.MultimodalModelID
	}
	if req.MultimodalModelName != "" {
		updates["multimodal_model_name"] = req.MultimodalModelName
	}
	if req.MultimodalDimension > 0 {
		updates["multimodal_dimension"] = req.MultimodalDimension
	}
	if req.GraphExtractModelID != nil {
		updates["graph_extract_model_id"] = *req.GraphExtractModelID
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

	safeGo(func() {
		if kb.QdrantCollection != nil && *kb.QdrantCollection != "" {
			if err := DeleteQdrantCollection(*kb.QdrantCollection); err != nil {
				log.Printf("[WARN] 删除 Qdrant Collection 失败: %v", err)
			}
		}
		// 清理图谱数据
		if kb.Type == "graph" && IsNeo4jEnabled() {
			if err := DeleteKnowledgeBaseGraph(context.Background(), id); err != nil {
				log.Printf("[WARN] 删除图谱数据失败: %v", err)
			}
		}
		db.Where("knowledge_base_id = ?", id).Delete(&model.TKnowledgeSegment{})
		db.Where("knowledge_base_id = ?", id).Delete(&model.TKnowledgeDocument{})
		db.Where("knowledge_base_id = ?", id).Delete(&model.TKnowledgeQuery{})
		db.Where("knowledge_base_id = ?", id).Delete(&model.TKnowledgeEntity{})
		db.Where("knowledge_base_id = ?", id).Delete(&model.TKnowledgeRelation{})
		GetFileStorage().DeleteDir(id)
	})

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

	safeGo(func() { l.updateDocumentCount(kbID) })
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

	safeGo(func() {
		l.updateDocumentCount(kbID)
		if kb.QdrantCollection != nil && *kb.QdrantCollection != "" {
			if err := DeleteDocumentVectors(*kb.QdrantCollection, docID); err != nil {
				log.Printf("[WARN] 清理文档向量失败: docID=%d, err=%v", docID, err)
			}
		}
		if doc.FilePath != nil && *doc.FilePath != "" {
			GetFileStorage().Delete(*doc.FilePath)
		}
	})

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

	safeGo(func() {
		l.updateDocumentCount(kbID)
		for _, doc := range docs {
			if kb.QdrantCollection != nil && *kb.QdrantCollection != "" {
				DeleteDocumentVectors(*kb.QdrantCollection, doc.ID)
			}
			if doc.FilePath != nil && *doc.FilePath != "" {
				GetFileStorage().Delete(*doc.FilePath)
			}
		}
	})

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

	safeGo(func() {
		processor := NewDocumentProcessor()
		if err := processor.Process(&kb, &doc); err != nil {
			log.Printf("[ERROR] 文档重处理失败: docID=%d, err=%v", doc.ID, err)
		}
	})

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

	safeGo(func() {
		processor := NewDocumentProcessor()
		for _, doc := range docs {
			d := doc
			if err := processor.Process(&kb, &d); err != nil {
				log.Printf("[ERROR] 批量重处理失败: docID=%d, err=%v", d.ID, err)
			}
		}
	})

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
		si := &SegmentInfo{
			ID:           seg.ID,
			DocumentID:   seg.DocumentID,
			DocumentName: doc.Name,
			Content:      seg.Content,
			ContentType:  seg.ContentType,
			Position:     seg.Position,
			WordCount:    seg.WordCount,
			Enabled:      seg.Enabled,
			HitCount:     seg.HitCount,
			Status:       seg.Status,
			CreatedAt:    seg.CreatedAt,
		}
		if seg.ImagePath != nil {
			si.ImagePath = *seg.ImagePath
		}
		result = append(result, si)
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

	safeGo(func() {
		processor := NewDocumentProcessor()
		if err := processor.Process(&kb, &doc); err != nil {
			log.Printf("[ERROR] 文档处理失败: docID=%d, err=%v", doc.ID, err)
		}
	})

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
	// score < 0 表示"使用知识库默认阈值"；score == 0 表示不过滤（Dify 默认行为）
	if score < 0 && kb.SimilarityThreshold != nil {
		score = *kb.SimilarityThreshold
	}
	if score < 0 {
		score = 0
	}

	retrievalMode := req.RetrievalMode
	if retrievalMode == "" && kb.RetrievalMode != nil {
		retrievalMode = *kb.RetrievalMode
	}
	if retrievalMode == "" {
		retrievalMode = "vector"
	}

	searchFields := req.SearchFields
	if searchFields == "" {
		searchFields = "all"
	}

	var results []*KnowledgeSearchResult

	switch retrievalMode {
	case "keyword":
		results = l.keywordSearch(kbID, req.Query, topK)
	case "hybrid":
		vectorResults := l.vectorSearch(&kb, req.Query, topK, score, searchFields)
		keywordResults := l.keywordSearch(kbID, req.Query, topK)
		results = l.mergeResults(vectorResults, keywordResults, topK)
	case "graph":
		results = l.graphSearch(&kb, req.Query, topK)
	case "hybrid_graph":
		vectorResults := l.vectorSearch(&kb, req.Query, topK, score, searchFields)
		graphResults := l.graphSearch(&kb, req.Query, topK)
		results = l.mergeResults(vectorResults, graphResults, topK)
	default:
		results = l.vectorSearch(&kb, req.Query, topK, score, searchFields)
	}

	go l.saveQueryHistory(kbID, req.Query, retrievalMode, topK, score, len(results))
	go l.updateHitCounts(results)

	return results, nil
}

func (l *KnowledgeBaseLogic) vectorSearch(kb *model.TKnowledgeBase, query string, topK int, score float64, searchFields string) []*KnowledgeSearchResult {
	if kb.EmbeddingModelID == nil || *kb.EmbeddingModelID == 0 {
		log.Printf("[ERROR] vectorSearch: 知识库 %d 未配置嵌入模型", kb.ID)
		return nil
	}

	aiModelLogic := NewAiModelLogic(l.ctx)

	var allHits []SearchHit

	// 文本向量搜索
	if searchFields == "all" || searchFields == "text" || searchFields == "" {
		aiModel, err := aiModelLogic.GetByIDWithKey(*kb.EmbeddingModelID)
		if err != nil {
			log.Printf("[ERROR] vectorSearch: 获取嵌入模型失败 (modelID=%d): %v", *kb.EmbeddingModelID, err)
			return nil
		}
		log.Printf("[DEBUG] vectorSearch: 使用模型 %s (base=%s), collection=%s, score=%.3f",
			aiModel.ModelID, aiModel.APIBaseURL, *kb.QdrantCollection, score)

		embClient := NewEmbeddingClient(aiModel.APIBaseURL, aiModel.APIKey, aiModel.ModelID)
		queryVector, err := embClient.EmbedTextAsQuery(l.ctx, query)
		if err != nil {
			log.Printf("[ERROR] vectorSearch: 查询向量化失败 (query=%q): %v", query, err)
			return nil
		}
		log.Printf("[DEBUG] vectorSearch: 查询向量维度=%d", len(queryVector))

		hits, err := SearchVectors(*kb.QdrantCollection, queryVector, topK, float32(score))
		if err != nil {
			log.Printf("[ERROR] vectorSearch: Qdrant 搜索失败 (collection=%s): %v", *kb.QdrantCollection, err)
		} else {
			log.Printf("[DEBUG] vectorSearch: Qdrant 返回 %d 条命中", len(hits))
			allHits = append(allHits, hits...)
		}
	}

	// 多模态向量搜索（用文本查询搜索图片向量空间）
	multimodalEnabled := kb.MultimodalEnabled != nil && *kb.MultimodalEnabled
	if multimodalEnabled && (searchFields == "all" || searchFields == "image") {
		if kb.MultimodalModelID != nil && *kb.MultimodalModelID != 0 {
			mmModel, err := aiModelLogic.GetByIDWithKey(*kb.MultimodalModelID)
			if err == nil {
				mmClient := NewEmbeddingClient(mmModel.APIBaseURL, mmModel.APIKey, mmModel.ModelID)
				mmInputs := []MultimodalInput{{Type: EmbeddingInputText, Text: query}}
				mmVectors, err := mmClient.EmbedMultimodal(l.ctx, mmInputs)
				if err == nil && len(mmVectors) > 0 {
					imgHits, err := SearchVectorsInField(*kb.QdrantCollection, "image", mmVectors[0], topK, float32(score), nil)
					if err != nil {
						log.Printf("[WARN] 多模态搜索失败: %v", err)
					} else {
						allHits = append(allHits, imgHits...)
					}
				}
			}
		}
	}

	// 去重并排序
	seen := make(map[string]bool)
	docNameCache := make(map[int64]string)
	results := make([]*KnowledgeSearchResult, 0, len(allHits))
	for _, hit := range allHits {
		if seen[hit.ID] {
			continue
		}
		seen[hit.ID] = true
		docName := getDocNameCached(hit.DocumentID, docNameCache)
		contentType := hit.ContentType
		if contentType == "" {
			contentType = "text"
		}
		results = append(results, &KnowledgeSearchResult{
			Content:      hit.Content,
			ContentType:  contentType,
			ImagePath:    imagePathToURL(hit.ImagePath, kb.ID),
			Score:        hit.Score,
			DocumentID:   hit.DocumentID,
			DocumentName: docName,
			ChunkIndex:   hit.ChunkIndex,
			WordCount:    utf8.RuneCountInString(hit.Content),
			Metadata:     hit.Metadata,
		})
	}

	if len(results) > topK {
		results = results[:topK]
	}
	return results
}

// graphSearch 图谱检索（Phase 3）
func (l *KnowledgeBaseLogic) graphSearch(kb *model.TKnowledgeBase, query string, topK int) []*KnowledgeSearchResult {
	if kb.Type != "graph" || !IsNeo4jEnabled() {
		return nil
	}

	// 用简单的分词作为关键词搜索图谱
	keywords := strings.Fields(query)
	if len(keywords) == 0 {
		return nil
	}

	graphResult, err := SearchGraphByKeywords(l.ctx, kb.ID, keywords, 2)
	if err != nil {
		log.Printf("[ERROR] 图谱搜索失败: %v", err)
		return nil
	}

	contextText := FormatGraphAsContext(graphResult)
	if contextText == "" {
		return nil
	}

	return []*KnowledgeSearchResult{{
		Content:      contextText,
		ContentType:  "text",
		Score:        0.9,
		DocumentName: "知识图谱",
		Metadata: map[string]interface{}{
			"source":         "graph",
			"entity_count":   len(graphResult.Entities),
			"relation_count": len(graphResult.Relations),
		},
	}}
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

// KnowledgeDiagResult 诊断结果
type KnowledgeDiagResult struct {
	KBID           int64                  `json:"kb_id"`
	KBName         string                 `json:"kb_name"`
	EmbeddingModel string                 `json:"embedding_model"`
	MySQLSegments  int64                  `json:"mysql_segments"`
	Qdrant         *QdrantCollectionDiag  `json:"qdrant"`
	EmbeddingTest  *EmbeddingTestResult   `json:"embedding_test"`
}

type EmbeddingTestResult struct {
	Success   bool   `json:"success"`
	Dimension int    `json:"dimension,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Diagnose 诊断知识库向量数据状态
func (l *KnowledgeBaseLogic) Diagnose(kbID int64) (*KnowledgeDiagResult, error) {
	db := svc.Ctx.DB

	var kb model.TKnowledgeBase
	if err := db.Where("id = ? AND is_delete = 0", kbID).First(&kb).Error; err != nil {
		return nil, fmt.Errorf("知识库不存在")
	}

	result := &KnowledgeDiagResult{
		KBID:   kb.ID,
		KBName: kb.Name,
	}

	// MySQL segment 数量
	db.Model(&model.TKnowledgeSegment{}).Where("knowledge_base_id = ?", kbID).Count(&result.MySQLSegments)

	// Qdrant 集合信息
	if kb.QdrantCollection != nil && *kb.QdrantCollection != "" {
		result.Qdrant = DiagnoseQdrantCollection(*kb.QdrantCollection)
	} else {
		result.Qdrant = &QdrantCollectionDiag{Error: "未配置 Qdrant Collection"}
	}

	// 测试 Embedding API
	if kb.EmbeddingModelID != nil && *kb.EmbeddingModelID != 0 {
		aiModelLogic := NewAiModelLogic(l.ctx)
		aiModel, err := aiModelLogic.GetByIDWithKey(*kb.EmbeddingModelID)
		if err != nil {
			result.EmbeddingTest = &EmbeddingTestResult{Error: fmt.Sprintf("获取嵌入模型失败: %v", err)}
		} else {
			result.EmbeddingModel = aiModel.ModelID
			embClient := NewEmbeddingClient(aiModel.APIBaseURL, aiModel.APIKey, aiModel.ModelID)
			vec, err := embClient.EmbedTextAsQuery(l.ctx, "测试")
			if err != nil {
				result.EmbeddingTest = &EmbeddingTestResult{Error: fmt.Sprintf("Embedding API 调用失败: %v", err)}
			} else {
				result.EmbeddingTest = &EmbeddingTestResult{Success: true, Dimension: len(vec)}
			}
		}
	} else {
		result.EmbeddingTest = &EmbeddingTestResult{Error: "未配置嵌入模型"}
	}

	return result, nil
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
		ID:                  m.ID,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
		CreatedBy:           m.CreatedBy,
		Name:                m.Name,
		Type:                m.Type,
		EmbeddingModelID:    m.EmbeddingModelID,
		MultimodalModelID:   m.MultimodalModelID,
		RerankModelID:       m.RerankModelID,
		GraphExtractModelID: m.GraphExtractModelID,
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
	if m.MultimodalEnabled != nil {
		info.MultimodalEnabled = *m.MultimodalEnabled
	}
	if m.MultimodalModelName != nil {
		info.MultimodalModelName = *m.MultimodalModelName
	}
	if m.MultimodalDimension != nil {
		info.MultimodalDimension = *m.MultimodalDimension
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
	if m.EntityCount != nil {
		info.EntityCount = *m.EntityCount
	}
	if m.RelationCount != nil {
		info.RelationCount = *m.RelationCount
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
	if m.ImageCount != nil {
		info.ImageCount = *m.ImageCount
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

// -----------------------------------------------
// 图知识库查询（Phase 3）
// -----------------------------------------------

type GraphEntityInfo struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	EntityType   string `json:"entity_type"`
	Description  string `json:"description"`
	MentionCount int    `json:"mention_count"`
}

type GraphRelationInfo struct {
	ID           int64   `json:"id"`
	SourceName   string  `json:"source_name"`
	TargetName   string  `json:"target_name"`
	RelationType string  `json:"relation_type"`
	Description  string  `json:"description"`
	Weight       float64 `json:"weight"`
}

func (l *KnowledgeBaseLogic) ListGraphEntities(kbID int64) ([]*GraphEntityInfo, error) {
	db := svc.Ctx.DB
	var entities []model.TKnowledgeEntity
	if err := db.Where("knowledge_base_id = ?", kbID).
		Order("mention_count DESC").Limit(200).
		Find(&entities).Error; err != nil {
		return nil, err
	}

	result := make([]*GraphEntityInfo, 0, len(entities))
	for _, e := range entities {
		info := &GraphEntityInfo{
			ID:           e.ID,
			Name:         e.Name,
			EntityType:   e.EntityType,
			MentionCount: e.MentionCount,
		}
		if e.Description != nil {
			info.Description = *e.Description
		}
		result = append(result, info)
	}
	return result, nil
}

func (l *KnowledgeBaseLogic) ListGraphRelations(kbID int64) ([]*GraphRelationInfo, error) {
	db := svc.Ctx.DB

	type relWithNames struct {
		model.TKnowledgeRelation
		SourceName string `gorm:"column:source_name"`
		TargetName string `gorm:"column:target_name"`
	}

	var relations []relWithNames
	if err := db.Table("t_knowledge_relation r").
		Select("r.*, se.name as source_name, te.name as target_name").
		Joins("LEFT JOIN t_knowledge_entity se ON se.id = r.source_entity_id").
		Joins("LEFT JOIN t_knowledge_entity te ON te.id = r.target_entity_id").
		Where("r.knowledge_base_id = ?", kbID).
		Limit(500).
		Find(&relations).Error; err != nil {
		return nil, err
	}

	result := make([]*GraphRelationInfo, 0, len(relations))
	for _, r := range relations {
		info := &GraphRelationInfo{
			ID:           r.ID,
			SourceName:   r.SourceName,
			TargetName:   r.TargetName,
			RelationType: r.RelationType,
			Weight:       r.Weight,
		}
		if r.Description != nil {
			info.Description = *r.Description
		}
		result = append(result, info)
	}
	return result, nil
}

// imagePathToURL 将服务端相对路径转换为前端可访问的 HTTP URL
// 存储路径格式：kb_{kbID}/images/{docID}_{i}.png
// 输出 URL：/api/knowledge-bases/{kbID}/images/{filename}
func imagePathToURL(imagePath string, kbID int64) string {
	if imagePath == "" {
		return ""
	}
	// 已经是 URL 格式，直接返回
	if strings.HasPrefix(imagePath, "/") || strings.HasPrefix(imagePath, "http") {
		return imagePath
	}
	prefix := fmt.Sprintf("kb_%d/images/", kbID)
	if strings.HasPrefix(imagePath, prefix) {
		filename := strings.TrimPrefix(imagePath, prefix)
		return fmt.Sprintf("/api/knowledge-bases/%d/images/%s", kbID, filename)
	}
	return imagePath
}
