package logic

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
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
	if doc.Content == nil || *doc.Content == "" {
		return "", fmt.Errorf("文档内容为空")
	}

	fileType := ""
	if doc.FileType != nil {
		fileType = *doc.FileType
	}

	// 纯文本类型，内容直接以 UTF-8 字符串形式存储
	if IsTextFileType(fileType) {
		return *doc.Content, nil
	}

	// 二进制文件：Content 字段存储的是 Base64 编码后的原始数据
	rawBytes, err := Base64Decode(*doc.Content)
	if err != nil {
		return "", fmt.Errorf("Base64 解码失败: %v", err)
	}

	switch fileType {
	case "pdf":
		text, err := extractTextFromPDF(rawBytes)
		if err != nil {
			return "", fmt.Errorf("PDF 文本提取失败: %v", err)
		}
		return text, nil
	case "docx":
		text, err := extractTextFromDOCX(rawBytes)
		if err != nil {
			return "", fmt.Errorf("DOCX 文本提取失败: %v", err)
		}
		return text, nil
	case "image":
		// TODO: 接入 OCR 服务
		return "", fmt.Errorf("图片 OCR 功能待实现（已保存原始数据 %d 字节）", len(rawBytes))
	case "audio":
		// TODO: 接入语音转文字 (Whisper)
		return "", fmt.Errorf("音频转写功能待实现（已保存原始数据 %d 字节）", len(rawBytes))
	case "video":
		// TODO: 提取音轨后转文字
		return "", fmt.Errorf("视频转写功能待实现（已保存原始数据 %d 字节）", len(rawBytes))
	default:
		return "", fmt.Errorf("不支持的文件类型: %s", fileType)
	}
}

// -----------------------------------------------
// DOCX 解析（纯标准库实现，无第三方依赖）
// DOCX 是一个 ZIP 包，核心文本在 word/document.xml 中
// -----------------------------------------------

// extractTextFromDOCX 从 DOCX 二进制数据中提取纯文本
func extractTextFromDOCX(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("无法打开 DOCX 压缩包: %v", err)
	}

	var docXML *zip.File
	for _, f := range reader.File {
		if f.Name == "word/document.xml" {
			docXML = f
			break
		}
	}

	if docXML == nil {
		return "", fmt.Errorf("DOCX 中未找到 word/document.xml")
	}

	rc, err := docXML.Open()
	if err != nil {
		return "", fmt.Errorf("无法读取 document.xml: %v", err)
	}
	defer rc.Close()

	xmlData, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("读取 document.xml 失败: %v", err)
	}

	text := parseWordXML(xmlData)
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("DOCX 文档内容为空")
	}

	return text, nil
}

// parseWordXML 解析 word/document.xml 提取文本内容
// 遍历 XML token，遇到 <w:t> 标签提取文本，遇到 </w:p> 标签插入换行
func parseWordXML(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var sb strings.Builder
	inText := false

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			// <w:t> 或 <w:t xml:space="preserve"> 包含文本内容
			if t.Name.Local == "t" && t.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				inText = true
			}
		case xml.EndElement:
			if t.Name.Local == "t" && t.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				inText = false
			}
			// 段落结束，添加换行
			if t.Name.Local == "p" && t.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				sb.WriteString("\n")
			}
			// 表格行结束，添加换行
			if t.Name.Local == "tr" && t.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				sb.WriteString("\n")
			}
		case xml.CharData:
			if inText {
				sb.Write(t)
			}
		}
	}

	return sb.String()
}

// -----------------------------------------------
// PDF 简易文本提取（从二进制流中提取可见文本）
// 注：这是一个轻量级实现，适用于包含文本层的 PDF。
// 扫描件 PDF（纯图片）需要 OCR，此处无法处理。
// 后续可替换为 pdftotext 或 unipdf 等专业库。
// -----------------------------------------------

// extractTextFromPDF 从 PDF 二进制数据中提取文本
func extractTextFromPDF(data []byte) (string, error) {
	// 策略 1：查找 PDF 文本流中的文本对象
	// PDF 文本通常在 BT ... ET 块中，用 Tj/TJ/' /" 操作符渲染
	text := extractPDFTextStreams(data)
	if text != "" {
		return text, nil
	}

	// 策略 2：提取所有可打印 ASCII 字符（降级方案）
	text = extractVisibleText(data)
	if text != "" {
		return text, nil
	}

	return "", fmt.Errorf("无法从 PDF 中提取文本，可能是扫描件（纯图片），需要 OCR 支持")
}

// extractPDFTextStreams 从 PDF 二进制中提取 BT...ET 文本块中的括号内文本
func extractPDFTextStreams(data []byte) string {
	var sb strings.Builder
	dataStr := string(data)

	// 查找所有 BT...ET 文本块
	for {
		btIdx := strings.Index(dataStr, "BT")
		if btIdx < 0 {
			break
		}
		etIdx := strings.Index(dataStr[btIdx:], "ET")
		if etIdx < 0 {
			break
		}

		block := dataStr[btIdx : btIdx+etIdx+2]
		// 提取括号中的文本: (text) Tj 或 [(text)] TJ
		texts := extractParenText(block)
		for _, t := range texts {
			sb.WriteString(t)
		}
		sb.WriteString(" ")

		dataStr = dataStr[btIdx+etIdx+2:]
	}

	result := strings.TrimSpace(sb.String())
	// 清理不可见字符和多余空格
	result = strings.Join(strings.Fields(result), " ")

	if len(result) < 20 {
		return ""
	}
	return result
}

// extractParenText 从 PDF 文本块中提取括号内的字符串
func extractParenText(block string) []string {
	var results []string
	depth := 0
	var current strings.Builder

	for i := 0; i < len(block); i++ {
		ch := block[i]
		if ch == '(' && (i == 0 || block[i-1] != '\\') {
			depth++
			if depth == 1 {
				current.Reset()
				continue
			}
		}
		if ch == ')' && (i == 0 || block[i-1] != '\\') {
			depth--
			if depth == 0 {
				text := current.String()
				// 过滤掉纯控制字符序列
				if len(text) > 0 && isPrintableText(text) {
					results = append(results, text)
				}
				continue
			}
		}
		if depth > 0 {
			current.WriteByte(ch)
		}
	}

	return results
}

// isPrintableText 检查字符串是否包含足够比例的可打印字符
func isPrintableText(s string) bool {
	printable := 0
	total := len(s)
	if total == 0 {
		return false
	}
	for _, r := range s {
		if r >= 32 && r < 127 || r >= 0x4e00 && r <= 0x9fff || r == '\n' || r == '\r' {
			printable++
		}
	}
	return float64(printable)/float64(total) > 0.5
}

// extractVisibleText 从二进制数据中提取可打印的 UTF-8 文本片段（降级方案）
func extractVisibleText(data []byte) string {
	var sb strings.Builder
	for _, b := range data {
		if b >= 32 && b < 127 || b == '\n' || b == '\r' || b == '\t' {
			sb.WriteByte(b)
		}
	}
	text := strings.TrimSpace(sb.String())
	if len(text) < 50 {
		return ""
	}
	return text
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
