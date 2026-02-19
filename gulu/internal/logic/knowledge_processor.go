package logic

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/svc"
)

// DocumentProcessor 文档处理器（支持多模态和图知识库）
type DocumentProcessor struct{}

func NewDocumentProcessor() *DocumentProcessor {
	return &DocumentProcessor{}
}

// ImageChunk 从文档中提取的图片
type ImageChunk struct {
	Data        []byte
	Description string
	FilePath    string // 存储后的路径
}

// Process 处理文档（完整 ETL 流水线，支持多模态 + 图知识库）
func (p *DocumentProcessor) Process(kb *model.TKnowledgeBase, doc *model.TKnowledgeDocument) error {
	ctx := context.Background()
	db := svc.Ctx.DB

	// ── Stage 1: Parsing ──
	p.updateIndexingStatus(ctx, doc.ID, "parsing")

	text, err := p.extractText(doc)
	if err != nil {
		p.markFailed(ctx, doc.ID, fmt.Sprintf("文本提取失败: %v", err))
		return err
	}

	// 提取文档中的图片（多模态支持）
	var images []ImageChunk
	multimodalEnabled := kb.MultimodalEnabled != nil && *kb.MultimodalEnabled
	if multimodalEnabled {
		images = p.extractImages(doc)
		if len(images) > 0 {
			storage := GetFileStorage()
			for i := range images {
				imgPath := fmt.Sprintf("kb_%d/images/%d_%d.png", kb.ID, doc.ID, i)
				if err := storage.SaveBytes(imgPath, images[i].Data); err != nil {
					log.Printf("[WARN] 保存图片失败: %v", err)
					continue
				}
				images[i].FilePath = imgPath
			}
		}
	}

	if strings.TrimSpace(text) == "" && len(images) == 0 {
		p.markFailed(ctx, doc.ID, "文档内容为空")
		return fmt.Errorf("文档内容为空")
	}

	wordCount := int32(utf8.RuneCountInString(text))
	imageCount := int32(len(images))
	now := time.Now()
	db.Model(&model.TKnowledgeDocument{}).Where("id = ?", doc.ID).Updates(map[string]interface{}{
		"word_count":           wordCount,
		"image_count":          imageCount,
		"parsing_completed_at": now,
	})

	// ── Stage 2: Cleaning ──
	p.updateIndexingStatus(ctx, doc.ID, "cleaning")

	cs := doc.ChunkSetting
	if cs == nil {
		cs = model.DefaultChunkSetting()
	}
	if cs.CleanWhitespace {
		text = cleanWhitespace(text)
	}
	if cs.RemoveURLs {
		text = removeURLsAndEmails(text)
	}

	// ── Stage 3: Splitting ──
	p.updateIndexingStatus(ctx, doc.ID, "splitting")

	chunkSize := cs.ChunkSize
	chunkOverlap := cs.ChunkOverlap
	if chunkSize <= 0 && kb.ChunkSize != nil {
		chunkSize = int(*kb.ChunkSize)
	}
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if chunkOverlap < 0 && kb.ChunkOverlap != nil {
		chunkOverlap = int(*kb.ChunkOverlap)
	}
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 5
	}

	var chunks []string
	if strings.TrimSpace(text) != "" {
		separator := cs.Separator
		if separator != "" {
			chunks = splitBySeparator(text, separator, chunkSize, chunkOverlap)
		} else {
			chunks = splitText(text, chunkSize, chunkOverlap)
		}
	}

	if len(chunks) == 0 && len(images) == 0 {
		p.markFailed(ctx, doc.ID, "分块结果为空")
		return fmt.Errorf("分块结果为空")
	}

	// ── Stage 4: Indexing ──
	p.updateIndexingStatus(ctx, doc.ID, "indexing")

	collectionName := ""
	if kb.QdrantCollection != nil {
		collectionName = *kb.QdrantCollection
	}
	if collectionName == "" {
		p.markFailed(ctx, doc.ID, "Qdrant Collection 未配置")
		return fmt.Errorf("qdrant collection 未配置")
	}

	dimension := 1536
	if kb.EmbeddingDimension != nil {
		dimension = int(*kb.EmbeddingDimension)
	}

	// 根据是否启用多模态决定 Collection 配置
	imageDimension := 0
	if multimodalEnabled && kb.MultimodalDimension != nil {
		imageDimension = int(*kb.MultimodalDimension)
	}
	if err := CreateQdrantCollectionMultiVector(collectionName, CollectionVectorConfig{
		TextDimension:  dimension,
		ImageDimension: imageDimension,
	}); err != nil {
		log.Printf("[WARN] 确保 Qdrant Collection 存在失败: %v", err)
	}

	// 清理旧数据
	if err := DeleteDocumentVectors(collectionName, doc.ID); err != nil {
		log.Printf("[WARN] 删除旧向量失败: %v", err)
	}
	db.Where("document_id = ?", doc.ID).Delete(&model.TKnowledgeSegment{})

	// 4a. 文本向量化和写入
	totalChunks := 0
	if len(chunks) > 0 {
		vectors, err := p.generateEmbeddings(kb, chunks)
		if err != nil {
			p.markFailed(ctx, doc.ID, fmt.Sprintf("Embedding 生成失败: %v", err))
			return err
		}

		points := make([]VectorPoint, len(chunks))
		for i, chunk := range chunks {
			points[i] = VectorPoint{
				ID:          fmt.Sprintf("%d_%d", doc.ID, i),
				Vector:      vectors[i],
				DocumentID:  doc.ID,
				ChunkIndex:  i,
				Content:     chunk,
				ContentType: "text",
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

		// 写入 MySQL text segments
		segments := make([]*model.TKnowledgeSegment, len(chunks))
		nowT := time.Now()
		for i, chunk := range chunks {
			pointID := fmt.Sprintf("%d", uint64(doc.ID)*100000+uint64(i))
			wc := utf8.RuneCountInString(chunk)
			segments[i] = &model.TKnowledgeSegment{
				CreatedAt:       &nowT,
				UpdatedAt:       &nowT,
				KnowledgeBaseID: doc.KnowledgeBaseID,
				DocumentID:      doc.ID,
				Content:         chunk,
				ContentType:     "text",
				Position:        i,
				WordCount:       wc,
				Tokens:          wc,
				IndexNodeID:     &pointID,
				VectorField:     "text",
				Status:          "active",
				Enabled:         true,
			}
		}
		if err := db.CreateInBatches(segments, 100).Error; err != nil {
			log.Printf("[WARN] MySQL 文本分块写入失败: %v", err)
		}
		totalChunks += len(chunks)
	}

	// 4b. 图片向量化和写入（多模态）
	if multimodalEnabled && len(images) > 0 {
		imgErr := p.indexImages(kb, doc, images, collectionName, len(chunks))
		if imgErr != nil {
			log.Printf("[WARN] 图片索引失败: %v", imgErr)
		} else {
			totalChunks += len(images)
		}
	}

	// 4c. 图知识库处理（实体关系抽取）
	if kb.Type == "graph" && len(chunks) > 0 {
		graphProcessor := NewGraphProcessor()
		if err := graphProcessor.ProcessDocument(kb, doc, text, chunks); err != nil {
			log.Printf("[WARN] 图谱处理失败: %v", err)
		}
	}

	// ── Stage 5: Completed ──
	chunkCount := int32(totalChunks)
	tokenCount := int32(utf8.RuneCountInString(text))
	completedAt := time.Now()
	db.Model(&model.TKnowledgeDocument{}).Where("id = ?", doc.ID).Updates(map[string]interface{}{
		"indexing_status":       "completed",
		"chunk_count":           chunkCount,
		"token_count":           tokenCount,
		"indexing_completed_at": completedAt,
		"error_message":         nil,
		"updated_at":            completedAt,
	})

	p.updateKBCounts(ctx, doc.KnowledgeBaseID)
	log.Printf("[INFO] 文档处理完成: docID=%d, textChunks=%d, images=%d, words=%d", doc.ID, len(chunks), len(images), wordCount)
	return nil
}

// indexImages 对图片进行多模态向量化并写入 Qdrant
func (p *DocumentProcessor) indexImages(kb *model.TKnowledgeBase, doc *model.TKnowledgeDocument, images []ImageChunk, collectionName string, startIndex int) error {
	if kb.MultimodalModelID == nil || *kb.MultimodalModelID == 0 {
		return fmt.Errorf("未配置多模态嵌入模型")
	}

	ctx := context.Background()
	aiModelLogic := NewAiModelLogic(ctx)
	aiModel, err := aiModelLogic.GetByIDWithKey(*kb.MultimodalModelID)
	if err != nil {
		return fmt.Errorf("多模态嵌入模型不存在: %w", err)
	}

	embClient := NewEmbeddingClient(aiModel.APIBaseURL, aiModel.APIKey, aiModel.ModelID)

	imageDataList := make([][]byte, 0, len(images))
	validImages := make([]ImageChunk, 0, len(images))
	for _, img := range images {
		if len(img.Data) > 0 && img.FilePath != "" {
			imageDataList = append(imageDataList, img.Data)
			validImages = append(validImages, img)
		}
	}

	if len(imageDataList) == 0 {
		return nil
	}

	vectors, err := embClient.EmbedImages(ctx, imageDataList)
	if err != nil {
		return fmt.Errorf("图片 Embedding 生成失败: %w", err)
	}

	points := make([]VectorPoint, len(validImages))
	for i, img := range validImages {
		description := img.Description
		if description == "" {
			description = fmt.Sprintf("图片 %d (来自文档 %s)", i+1, doc.Name)
		}
		points[i] = VectorPoint{
			ID:          fmt.Sprintf("%d_img_%d", doc.ID, i),
			Vector:      vectors[i],
			DocumentID:  doc.ID,
			ChunkIndex:  startIndex + i,
			Content:     description,
			ContentType: "image",
			ImagePath:   img.FilePath,
			Metadata: map[string]interface{}{
				"document_name": doc.Name,
				"image_index":   i,
			},
		}
	}

	if err := UpsertVectorsToField(collectionName, "image", points); err != nil {
		return fmt.Errorf("图片向量写入失败: %w", err)
	}

	// 写入 MySQL image segments
	db := svc.Ctx.DB
	segments := make([]*model.TKnowledgeSegment, len(validImages))
	nowT := time.Now()
	for i, img := range validImages {
		pointID := fmt.Sprintf("%d", uint64(doc.ID)*100000+uint64(startIndex+i))
		description := img.Description
		if description == "" {
			description = fmt.Sprintf("图片 %d", i+1)
		}
		imgPath := img.FilePath
		segments[i] = &model.TKnowledgeSegment{
			CreatedAt:       &nowT,
			UpdatedAt:       &nowT,
			KnowledgeBaseID: doc.KnowledgeBaseID,
			DocumentID:      doc.ID,
			Content:         description,
			ContentType:     "image",
			ImagePath:       &imgPath,
			Position:        startIndex + i,
			WordCount:       0,
			Tokens:          0,
			IndexNodeID:     &pointID,
			VectorField:     "image",
			Status:          "active",
			Enabled:         true,
		}
	}
	if err := db.CreateInBatches(segments, 100).Error; err != nil {
		log.Printf("[WARN] MySQL 图片分块写入失败: %v", err)
	}

	log.Printf("[INFO] 多模态: 成功索引 %d 张图片", len(validImages))
	return nil
}

// -----------------------------------------------
// 文本提取器
// -----------------------------------------------

func (p *DocumentProcessor) extractText(doc *model.TKnowledgeDocument) (string, error) {
	fileType := ""
	if doc.FileType != nil {
		fileType = *doc.FileType
	}

	if doc.FilePath == nil || *doc.FilePath == "" {
		return "", fmt.Errorf("文件路径为空")
	}

	storage := GetFileStorage()
	data, err := storage.Read(*doc.FilePath)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}

	switch fileType {
	case "txt", "md", "csv", "json":
		return string(data), nil
	case "html":
		return extractTextFromHTML(data), nil
	case "pdf":
		return extractTextFromPDF(data, storage.FullPath(*doc.FilePath))
	case "docx":
		return extractTextFromDOCX(data)
	case "image":
		return fmt.Sprintf("[图片文件: %s]", doc.Name), nil
	default:
		return string(data), nil
	}
}

// extractImages 从文档中提取嵌入的图片
func (p *DocumentProcessor) extractImages(doc *model.TKnowledgeDocument) []ImageChunk {
	fileType := ""
	if doc.FileType != nil {
		fileType = *doc.FileType
	}

	if doc.FilePath == nil || *doc.FilePath == "" {
		return nil
	}

	storage := GetFileStorage()
	data, err := storage.Read(*doc.FilePath)
	if err != nil {
		return nil
	}

	switch fileType {
	case "image":
		return []ImageChunk{{
			Data:        data,
			Description: fmt.Sprintf("图片文件: %s", doc.Name),
		}}
	case "docx":
		return extractImagesFromDOCX(data)
	case "md":
		return extractImagesFromMarkdown(string(data), storage, doc.KnowledgeBaseID)
	default:
		return nil
	}
}

// extractImagesFromDOCX 从 DOCX 中提取嵌入的图片
func extractImagesFromDOCX(data []byte) []ImageChunk {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}

	var images []ImageChunk
	for _, f := range reader.File {
		if !strings.HasPrefix(f.Name, "word/media/") {
			continue
		}
		ext := strings.ToLower(f.Name[strings.LastIndex(f.Name, "."):])
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".webp" && ext != ".bmp" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		imgData, err := io.ReadAll(rc)
		rc.Close()
		if err != nil || len(imgData) == 0 {
			continue
		}

		// 跳过过小的图片（可能是图标等）
		if len(imgData) < 1024 {
			continue
		}

		images = append(images, ImageChunk{
			Data:        imgData,
			Description: fmt.Sprintf("文档嵌入图片: %s", f.Name),
		})
	}

	return images
}

// extractImagesFromMarkdown 从 Markdown 中提取图片引用
// 支持 ![alt](path) 格式，提取本地图片
func extractImagesFromMarkdown(content string, storage *FileStorage, kbID int64) []ImageChunk {
	var images []ImageChunk

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 匹配 ![alt](path) 格式
		imgStart := strings.Index(line, "![")
		if imgStart < 0 {
			continue
		}
		altEnd := strings.Index(line[imgStart:], "](")
		if altEnd < 0 {
			continue
		}
		pathStart := imgStart + altEnd + 2
		pathEnd := strings.Index(line[pathStart:], ")")
		if pathEnd < 0 {
			continue
		}

		imgPath := line[pathStart : pathStart+pathEnd]

		if strings.HasPrefix(imgPath, "http://") || strings.HasPrefix(imgPath, "https://") {
			continue
		}

		imgData, err := storage.Read(imgPath)
		if err != nil {
			continue
		}
		if len(imgData) < 1024 {
			continue
		}

		alt := ""
		if altEnd > 2 {
			alt = line[imgStart+2 : imgStart+altEnd]
		}
		if alt == "" {
			alt = fmt.Sprintf("Markdown 图片: %s", imgPath)
		}

		images = append(images, ImageChunk{
			Data:        imgData,
			Description: alt,
		})
	}

	return images
}

// -----------------------------------------------
// PDF 提取（优先用 pdftotext，降级到内置解析）
// -----------------------------------------------

func extractTextFromPDF(data []byte, filePath string) (string, error) {
	// 优先尝试 pdftotext（poppler-utils），解析质量最好
	text, err := extractPDFWithPdftotext(filePath)
	if err == nil && strings.TrimSpace(text) != "" {
		return text, nil
	}

	// 降级到内置 BT/ET 解析
	text = extractPDFTextStreams(data)
	if strings.TrimSpace(text) != "" {
		return text, nil
	}

	return "", fmt.Errorf("无法从 PDF 中提取文本，可能是扫描件（纯图片），需要 OCR 支持")
}

func extractPDFWithPdftotext(filePath string) (string, error) {
	cmd := exec.Command("pdftotext", "-enc", "UTF-8", "-layout", filePath, "-")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext 执行失败: %w (%s)", err, stderr.String())
	}
	return out.String(), nil
}

func extractPDFTextStreams(data []byte) string {
	var sb strings.Builder
	dataStr := string(data)

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
		texts := extractParenText(block)
		for _, t := range texts {
			sb.WriteString(t)
		}
		sb.WriteString(" ")
		dataStr = dataStr[btIdx+etIdx+2:]
	}

	result := strings.TrimSpace(sb.String())
	result = strings.Join(strings.Fields(result), " ")

	if len(result) < 20 {
		return ""
	}
	return result
}

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

func isPrintableText(s string) bool {
	printable := 0
	total := len(s)
	if total == 0 {
		return false
	}
	for _, r := range s {
		if (r >= 32 && r < 127) || (r >= 0x4e00 && r <= 0x9fff) || r == '\n' || r == '\r' {
			printable++
		}
	}
	return float64(printable)/float64(total) > 0.5
}

// -----------------------------------------------
// DOCX 提取（增强版：支持表格、列表）
// -----------------------------------------------

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

func parseWordXML(data []byte) string {
	const wNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

	decoder := xml.NewDecoder(bytes.NewReader(data))
	var sb strings.Builder
	inText := false
	inTable := false
	cellIdx := 0
	listLevel := -1

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Local == "t" && t.Name.Space == wNS:
				inText = true
			case t.Name.Local == "br" && t.Name.Space == wNS:
				sb.WriteString("\n")
			case t.Name.Local == "tbl" && t.Name.Space == wNS:
				inTable = true
			case t.Name.Local == "tc" && t.Name.Space == wNS:
				if cellIdx > 0 {
					sb.WriteString("\t| ")
				}
			case t.Name.Local == "numPr" && t.Name.Space == wNS:
				listLevel = 0
			case t.Name.Local == "ilvl" && t.Name.Space == wNS:
				for _, attr := range t.Attr {
					if attr.Name.Local == "val" {
						if v := attr.Value; v != "" {
							lvl := 0
							for _, c := range v {
								lvl = lvl*10 + int(c-'0')
							}
							listLevel = lvl
						}
					}
				}
			}

		case xml.EndElement:
			switch {
			case t.Name.Local == "t" && t.Name.Space == wNS:
				inText = false
			case t.Name.Local == "p" && t.Name.Space == wNS:
				if listLevel >= 0 {
					listLevel = -1
				}
				sb.WriteString("\n")
			case t.Name.Local == "tr" && t.Name.Space == wNS:
				sb.WriteString("\n")
				cellIdx = 0
			case t.Name.Local == "tc" && t.Name.Space == wNS:
				cellIdx++
			case t.Name.Local == "tbl" && t.Name.Space == wNS:
				inTable = false
				sb.WriteString("\n")
			}

		case xml.CharData:
			if inText {
				_ = inTable
				sb.Write(t)
			}
		}
	}

	return sb.String()
}

// -----------------------------------------------
// HTML 提取（使用 golang.org/x/net/html）
// -----------------------------------------------

func extractTextFromHTML(data []byte) string {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return string(data)
	}

	var sb strings.Builder
	var extractNode func(*html.Node)

	skipTags := map[string]bool{
		"script": true, "style": true, "noscript": true, "head": true,
	}
	blockTags := map[string]bool{
		"p": true, "div": true, "br": true, "h1": true, "h2": true,
		"h3": true, "h4": true, "h5": true, "h6": true, "li": true,
		"tr": true, "blockquote": true, "pre": true, "section": true,
		"article": true, "header": true, "footer": true,
	}

	extractNode = func(n *html.Node) {
		if n.Type == html.ElementNode && skipTags[n.Data] {
			return
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
		}
		if n.Type == html.ElementNode && blockTags[n.Data] {
			sb.WriteString("\n")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractNode(c)
		}
		if n.Type == html.ElementNode && blockTags[n.Data] {
			sb.WriteString("\n")
		}
	}
	extractNode(doc)

	text := sb.String()
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(text)
}

// -----------------------------------------------
// 文本分块
// -----------------------------------------------

var fallbackSeparators = []string{"\n\n", "。", ". ", " ", ""}

func splitText(text string, chunkSize, overlap int) []string {
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

func splitBySeparator(text, separator string, chunkSize, chunkOverlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 5
	}

	actualSep := strings.ReplaceAll(separator, `\n`, "\n")
	segments := strings.Split(text, actualSep)

	var flatChunks []string
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if runeLen(seg) > chunkSize {
			flatChunks = append(flatChunks, recursiveSplit(seg, fallbackSeparators, chunkSize)...)
		} else {
			flatChunks = append(flatChunks, seg)
		}
	}

	if chunkOverlap <= 0 || len(flatChunks) <= 1 {
		return flatChunks
	}
	return mergeSplits(flatChunks, chunkSize, chunkOverlap)
}

func recursiveSplit(text string, separators []string, chunkSize int) []string {
	if runeLen(text) <= chunkSize {
		t := strings.TrimSpace(text)
		if t == "" {
			return nil
		}
		return []string{t}
	}
	if len(separators) == 0 {
		return charSplit(text, chunkSize)
	}

	sep := separators[0]
	remaining := separators[1:]
	if sep == "" {
		return charSplit(text, chunkSize)
	}

	parts := strings.Split(text, sep)
	var goodSplits []string
	var result []string

	flushGood := func() {
		if len(goodSplits) > 0 {
			merged := mergeSplitsSimple(goodSplits, sep, chunkSize)
			result = append(result, merged...)
			goodSplits = nil
		}
	}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if runeLen(part) <= chunkSize {
			goodSplits = append(goodSplits, part)
		} else {
			flushGood()
			result = append(result, recursiveSplit(part, remaining, chunkSize)...)
		}
	}
	flushGood()
	return result
}

func mergeSplits(splits []string, chunkSize, overlap int) []string {
	if len(splits) == 0 {
		return nil
	}

	var chunks []string
	var current []string
	currentLen := 0

	for _, s := range splits {
		sLen := runeLen(s)
		joinLen := currentLen + sLen
		if len(current) > 0 {
			joinLen++
		}

		if joinLen > chunkSize && len(current) > 0 {
			chunk := strings.TrimSpace(strings.Join(current, "\n"))
			if chunk != "" {
				chunks = append(chunks, chunk)
			}
			for currentLen > overlap && len(current) > 0 {
				currentLen -= runeLen(current[0])
				if len(current) > 1 {
					currentLen--
				}
				current = current[1:]
			}
		}

		current = append(current, s)
		currentLen = 0
		for i, c := range current {
			currentLen += runeLen(c)
			if i > 0 {
				currentLen++
			}
		}
	}

	if len(current) > 0 {
		chunk := strings.TrimSpace(strings.Join(current, "\n"))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
	}
	return chunks
}

func mergeSplitsSimple(splits []string, sep string, chunkSize int) []string {
	var chunks []string
	var current strings.Builder

	for _, s := range splits {
		newLen := runeLen(current.String()) + runeLen(s)
		if current.Len() > 0 {
			newLen += runeLen(sep)
		}
		if newLen > chunkSize && current.Len() > 0 {
			chunk := strings.TrimSpace(current.String())
			if chunk != "" {
				chunks = append(chunks, chunk)
			}
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString(sep)
		}
		current.WriteString(s)
	}

	if current.Len() > 0 {
		chunk := strings.TrimSpace(current.String())
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
	}
	return chunks
}

func charSplit(text string, chunkSize int) []string {
	runes := []rune(text)
	var chunks []string
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[i:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
	}
	return chunks
}

func runeLen(s string) int {
	return len([]rune(s))
}

// -----------------------------------------------
// Embedding 生成
// -----------------------------------------------

func (p *DocumentProcessor) generateEmbeddings(kb *model.TKnowledgeBase, chunks []string) ([][]float32, error) {
	embClient, err := p.getEmbeddingClient(kb)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	vectors, err := embClient.EmbedTexts(ctx, chunks)
	if err != nil {
		return nil, fmt.Errorf("Embedding 生成失败: %w", err)
	}
	log.Printf("[INFO] Embedding: 成功生成 %d 个向量 (模型: %s)", len(vectors), embClient.Model)
	return vectors, nil
}

func (p *DocumentProcessor) getEmbeddingClient(kb *model.TKnowledgeBase) (*EmbeddingClient, error) {
	if kb.EmbeddingModelID == nil || *kb.EmbeddingModelID == 0 {
		return nil, fmt.Errorf("知识库未配置嵌入模型")
	}
	aiModelLogic := NewAiModelLogic(context.Background())
	aiModel, err := aiModelLogic.GetByIDWithKey(*kb.EmbeddingModelID)
	if err != nil {
		return nil, fmt.Errorf("嵌入模型不存在 (ID=%d): %w", *kb.EmbeddingModelID, err)
	}
	return NewEmbeddingClient(aiModel.APIBaseURL, aiModel.APIKey, aiModel.ModelID), nil
}

// -----------------------------------------------
// 状态管理
// -----------------------------------------------

func (p *DocumentProcessor) updateIndexingStatus(ctx context.Context, docID int64, status string) {
	db := svc.Ctx.DB
	db.Model(&model.TKnowledgeDocument{}).Where("id = ?", docID).Updates(map[string]interface{}{
		"indexing_status": status,
		"updated_at":     time.Now(),
	})
}

func (p *DocumentProcessor) markFailed(ctx context.Context, docID int64, errMsg string) {
	db := svc.Ctx.DB
	db.Model(&model.TKnowledgeDocument{}).Where("id = ?", docID).Updates(map[string]interface{}{
		"indexing_status": "error",
		"error_message":   errMsg,
		"updated_at":      time.Now(),
	})
}

func (p *DocumentProcessor) updateKBCounts(ctx context.Context, kbID int64) {
	db := svc.Ctx.DB
	var docCount int64
	db.Model(&model.TKnowledgeDocument{}).Where("knowledge_base_id = ?", kbID).Count(&docCount)

	var chunkCount int64
	db.Model(&model.TKnowledgeSegment{}).Where("knowledge_base_id = ? AND status = 'active'", kbID).Count(&chunkCount)

	db.Model(&model.TKnowledgeBase{}).Where("id = ?", kbID).Updates(map[string]interface{}{
		"document_count": int32(docCount),
		"chunk_count":    int32(chunkCount),
	})
}

// -----------------------------------------------
// 文本预处理工具
// -----------------------------------------------

func cleanWhitespace(text string) string {
	text = strings.ReplaceAll(text, "\t", " ")
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(text)
}

func removeURLsAndEmails(text string) string {
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
