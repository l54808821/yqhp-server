package logic

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

// DocumentProcessor 文档处理器
// 负责文档解析、分块、Embedding 生成和向量写入
type DocumentProcessor struct{}

// NewDocumentProcessor 创建文档处理器
func NewDocumentProcessor() *DocumentProcessor {
	return &DocumentProcessor{}
}

// Process 处理文档（异步调用）
func (p *DocumentProcessor) Process(kb *model.TKnowledgeBase, doc *model.TKnowledgeDocument) error {
	ctx := context.Background()
	q := query.Use(svc.Ctx.DB)
	docQuery := q.TKnowledgeDocument

	// 更新状态为处理中
	status := "processing"
	now := time.Now()
	docQuery.WithContext(ctx).Where(docQuery.ID.Eq(doc.ID)).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": now,
	})

	// 1. 提取文本内容
	text, err := p.extractText(doc)
	if err != nil {
		p.markFailed(ctx, doc.ID, fmt.Sprintf("文本提取失败: %v", err))
		return err
	}

	if text == "" {
		p.markFailed(ctx, doc.ID, "文档内容为空")
		return fmt.Errorf("文档内容为空")
	}

	// 2. 文本分块
	chunkSize := 500
	chunkOverlap := 50
	if kb.ChunkSize != nil {
		chunkSize = int(*kb.ChunkSize)
	}
	if kb.ChunkOverlap != nil {
		chunkOverlap = int(*kb.ChunkOverlap)
	}

	chunks := p.splitText(text, chunkSize, chunkOverlap)
	if len(chunks) == 0 {
		p.markFailed(ctx, doc.ID, "分块结果为空")
		return fmt.Errorf("分块结果为空")
	}

	// 3. 生成 Embedding 向量
	vectors, err := p.generateEmbeddings(kb, chunks)
	if err != nil {
		p.markFailed(ctx, doc.ID, fmt.Sprintf("Embedding 生成失败: %v", err))
		return err
	}

	// 4. 构建向量点并写入 Qdrant
	collectionName := ""
	if kb.QdrantCollection != nil {
		collectionName = *kb.QdrantCollection
	}
	if collectionName == "" {
		p.markFailed(ctx, doc.ID, "Qdrant Collection 未配置")
		return fmt.Errorf("qdrant collection 未配置")
	}

	points := make([]VectorPoint, len(chunks))
	for i, chunk := range chunks {
		points[i] = VectorPoint{
			ID:         fmt.Sprintf("%d_%d", doc.ID, i),
			Vector:     vectors[i],
			DocumentID: doc.ID,
			ChunkIndex: i,
			Content:    chunk,
			Metadata: map[string]interface{}{
				"document_name": doc.Name,
				"chunk_index":   i,
				"total_chunks":  len(chunks),
			},
		}
	}

	if err := UpsertVectors(collectionName, points); err != nil {
		p.markFailed(ctx, doc.ID, fmt.Sprintf("向量写入失败: %v", err))
		return err
	}

	// 5. 更新文档状态为就绪
	chunkCount := int32(len(chunks))
	tokenCount := int32(utf8.RuneCountInString(text))
	readyStatus := "ready"
	docQuery.WithContext(ctx).Where(docQuery.ID.Eq(doc.ID)).Updates(map[string]interface{}{
		"status":      readyStatus,
		"chunk_count": chunkCount,
		"token_count": tokenCount,
		"updated_at":  time.Now(),
	})

	// 更新知识库的分块计数
	p.updateKBChunkCount(ctx, doc.KnowledgeBaseID)

	fmt.Printf("[INFO] 文档处理完成: docID=%d, chunks=%d, tokens=%d\n", doc.ID, chunkCount, tokenCount)
	return nil
}

// extractText 从文档中提取文本
func (p *DocumentProcessor) extractText(doc *model.TKnowledgeDocument) (string, error) {
	// 如果有直接存储的内容，优先使用
	if doc.Content != nil && *doc.Content != "" {
		return *doc.Content, nil
	}

	fileType := ""
	if doc.FileType != nil {
		fileType = *doc.FileType
	}

	switch fileType {
	case "txt", "md", "csv", "json", "html":
		// 纯文本类型，内容应该已经存储在 Content 中
		return "", fmt.Errorf("文本类型文档但内容为空")
	case "pdf":
		// TODO: 接入 PDF 解析库 (如 pdftotext, unipdf)
		return "", fmt.Errorf("PDF 解析功能待实现")
	case "docx":
		// TODO: 接入 DOCX 解析库
		return "", fmt.Errorf("DOCX 解析功能待实现")
	case "image":
		// TODO: 接入 OCR 服务
		return "", fmt.Errorf("图片 OCR 功能待实现")
	case "audio":
		// TODO: 接入语音转文字 (Whisper)
		return "", fmt.Errorf("音频转写功能待实现")
	case "video":
		// TODO: 提取音轨后转文字
		return "", fmt.Errorf("视频转写功能待实现")
	default:
		return "", fmt.Errorf("不支持的文件类型: %s", fileType)
	}
}

// splitText 文本分块（滑动窗口）
func (p *DocumentProcessor) splitText(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 5
	}

	runes := []rune(text)
	totalLen := len(runes)
	if totalLen == 0 {
		return nil
	}

	// 如果文本长度小于分块大小，直接返回
	if totalLen <= chunkSize {
		return []string{strings.TrimSpace(text)}
	}

	var chunks []string
	step := chunkSize - overlap
	if step <= 0 {
		step = chunkSize
	}

	for start := 0; start < totalLen; start += step {
		end := start + chunkSize
		if end > totalLen {
			end = totalLen
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == totalLen {
			break
		}
	}

	return chunks
}

// generateEmbeddings 生成 Embedding 向量
func (p *DocumentProcessor) generateEmbeddings(kb *model.TKnowledgeBase, chunks []string) ([][]float32, error) {
	// TODO: 调用嵌入模型 API 生成向量
	// 1. 从 kb.EmbeddingModelID 获取模型配置
	// 2. 调用 OpenAI-compatible /v1/embeddings 接口
	// 3. 返回向量列表

	// 当前返回占位零向量
	dimension := 1536
	if kb.EmbeddingDimension != nil {
		dimension = int(*kb.EmbeddingDimension)
	}

	vectors := make([][]float32, len(chunks))
	for i := range chunks {
		vectors[i] = make([]float32, dimension)
	}

	fmt.Printf("[INFO] Embedding: 生成 %d 个向量 (维度: %d) - 待接入实际Embedding模型\n", len(chunks), dimension)
	return vectors, nil
}

// markFailed 标记文档处理失败
func (p *DocumentProcessor) markFailed(ctx context.Context, docID int64, errMsg string) {
	q := query.Use(svc.Ctx.DB)
	m := q.TKnowledgeDocument
	failedStatus := "failed"
	m.WithContext(ctx).Where(m.ID.Eq(docID)).Updates(map[string]interface{}{
		"status":        failedStatus,
		"error_message": errMsg,
		"updated_at":    time.Now(),
	})
}

// updateKBChunkCount 更新知识库的分块总数
func (p *DocumentProcessor) updateKBChunkCount(ctx context.Context, kbID int64) {
	q := query.Use(svc.Ctx.DB)

	// 统计所有 ready 文档的分块总数
	var totalChunks int64
	docs, err := q.TKnowledgeDocument.WithContext(ctx).Where(
		q.TKnowledgeDocument.KnowledgeBaseID.Eq(kbID),
	).Find()
	if err != nil {
		return
	}
	for _, doc := range docs {
		if doc.ChunkCount != nil {
			totalChunks += int64(*doc.ChunkCount)
		}
	}

	cnt := int32(totalChunks)
	q.TKnowledgeBase.WithContext(ctx).Where(q.TKnowledgeBase.ID.Eq(kbID)).Update(q.TKnowledgeBase.ChunkCount, cnt)
}
