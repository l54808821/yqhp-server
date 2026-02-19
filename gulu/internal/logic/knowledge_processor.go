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
	"regexp"
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
		dotIdx := strings.LastIndex(f.Name, ".")
		if dotIdx < 0 {
			continue
		}
		ext := strings.ToLower(f.Name[dotIdx:])
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

// docxRelationship 表示 DOCX 关系文件中的一条关系记录
type docxRelationship struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
}

type docxRelationships struct {
	Relationships []docxRelationship `xml:"Relationship"`
}

// parseDocxRels 解析 word/_rels/document.xml.rels，返回 rId -> 文件名 的映射（仅图片关系）
func parseDocxRels(reader *zip.Reader) map[string]string {
	imageRels := make(map[string]string)
	for _, f := range reader.File {
		if f.Name != "word/_rels/document.xml.rels" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return imageRels
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return imageRels
		}
		var rels docxRelationships
		if err := xml.Unmarshal(data, &rels); err != nil {
			return imageRels
		}
		for _, rel := range rels.Relationships {
			if strings.Contains(rel.Type, "/image") {
				imageRels[rel.ID] = rel.Target
			}
		}
	}
	return imageRels
}

// readZipFile 读取 ZIP 中指定路径的文件内容
func readZipFile(reader *zip.Reader, name string) ([]byte, error) {
	for _, f := range reader.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("文件 %s 不存在", name)
}

func extractTextFromDOCX(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("无法打开 DOCX 压缩包: %v", err)
	}

	xmlData, err := readZipFile(reader, "word/document.xml")
	if err != nil {
		return "", fmt.Errorf("DOCX 中未找到 word/document.xml")
	}

	// 解析图片关系映射：rId -> word/media/xxx.png
	imageRels := parseDocxRels(reader)

	// 构建图片数据映射：word/media/xxx.png -> 文件内容（用于保存到存储）
	// 此处我们将图片以 Markdown 占位符形式嵌入文本，格式：![image](media:word/media/xxx.png)
	// 调用方可根据需要替换为实际 URL
	text := parseWordXML(xmlData, imageRels)
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("DOCX 文档内容为空")
	}
	return text, nil
}

// parseWordXML 解析 Word XML，将段落文本和内联图片组合输出。
// imageRels：rId -> 图片相对路径（word/media/...）
// 段落分隔规则：非空段落之间以 \n 分隔，空段落输出为 \n，从而在内容段之间形成 \n\n。
func parseWordXML(data []byte, imageRels map[string]string) string {
	const (
		wNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
		aNS = "http://schemas.openxmlformats.org/drawingml/2006/main"
		rNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	)

	decoder := xml.NewDecoder(bytes.NewReader(data))

	// 收集每个段落/表格行的内容条目，最终用 \n 拼接
	var items []string

	// 当前段落状态
	var paraContent strings.Builder
	inText := false
	inPara := false
	// 表格行内容
	var rowCells []string
	var cellContent strings.Builder
	inCell := false

	getAttr := func(attrs []xml.Attr, ns, local string) string {
		for _, a := range attrs {
			if a.Name.Local == local && (ns == "" || a.Name.Space == ns || strings.HasSuffix(a.Name.Space, ns)) {
				return a.Value
			}
		}
		return ""
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Local == "tbl" && t.Name.Space == wNS:
				// 进入表格
			case t.Name.Local == "tr" && t.Name.Space == wNS:
				rowCells = nil
			case t.Name.Local == "tc" && t.Name.Space == wNS:
				inCell = true
				cellContent.Reset()
			case t.Name.Local == "p" && t.Name.Space == wNS:
				if !inCell {
					inPara = true
					paraContent.Reset()
				}
			case t.Name.Local == "t" && t.Name.Space == wNS:
				inText = true
			case t.Name.Local == "br" && t.Name.Space == wNS:
				if inCell {
					cellContent.WriteString("\n")
				} else if inPara {
					paraContent.WriteString("\n")
				}
			// 处理内联图片（DrawingML）：<a:blip r:embed="rIdN"/>
			case t.Name.Local == "blip" && t.Name.Space == aNS:
				embedID := getAttr(t.Attr, rNS, "embed")
				if embedID == "" {
					// 有些序列化时 namespace 写在属性名里
					embedID = getAttr(t.Attr, "", "embed")
				}
				if embedID != "" {
					if imgPath, ok := imageRels[embedID]; ok != false {
						imgPath = strings.TrimPrefix(imgPath, "../")
						if !strings.HasPrefix(imgPath, "word/") {
							imgPath = "word/" + imgPath
						}
						imgMd := fmt.Sprintf("![image](media:%s)", imgPath)
						if inCell {
							cellContent.WriteString(imgMd)
						} else if inPara {
							paraContent.WriteString(imgMd)
						}
					}
				}
			}

		case xml.EndElement:
			switch {
			case t.Name.Local == "t" && t.Name.Space == wNS:
				inText = false
			case t.Name.Local == "p" && t.Name.Space == wNS:
				if inCell {
					// 单元格内段落：内容追加到 cellContent，不触发 items
					break
				}
				inPara = false
				content := strings.TrimSpace(paraContent.String())
				if content != "" {
					items = append(items, content)
				} else {
					// 空段落作为分隔符
					items = append(items, "")
				}
				paraContent.Reset()
			case t.Name.Local == "tc" && t.Name.Space == wNS:
				inCell = false
				rowCells = append(rowCells, strings.TrimSpace(cellContent.String()))
			case t.Name.Local == "tr" && t.Name.Space == wNS:
				if len(rowCells) > 0 {
					items = append(items, strings.Join(rowCells, " | "))
				}
				rowCells = nil
			case t.Name.Local == "tbl" && t.Name.Space == wNS:
				// 离开表格
			}

		case xml.CharData:
			if inText {
				if inCell {
					cellContent.Write(t)
				} else if inPara {
					paraContent.Write(t)
				}
			}
		}
	}

	// 将 items 拼成最终文本：空 item 表示空段落，产生 \n\n 分隔
	var sb strings.Builder
	for i, item := range items {
		if item == "" {
			// 空段落：如果前面已有内容则写入换行
			if i > 0 {
				sb.WriteString("\n")
			}
		} else {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(item)
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

// splitBySeparator 按固定分隔符切分文本，对齐 Dify 的 FixedRecursiveCharacterTextSplitter 行为：
// 每个分隔符切出的段直接作为独立 chunk，不合并小段；只对超过 chunkSize 的段递归拆分。
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

	var result []string
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if runeLen(seg) > chunkSize {
			result = append(result, recursiveSplit(seg, fallbackSeparators, chunkSize, chunkOverlap)...)
		} else {
			result = append(result, seg)
		}
	}
	return result
}

// recursiveSplit 递归地将超大文本段拆分为不超过 chunkSize 的块。
// 当 chunkOverlap > 0 时，在最终字符级拆分阶段应用重叠。
func recursiveSplit(text string, separators []string, chunkSize, chunkOverlap int) []string {
	if runeLen(text) <= chunkSize {
		t := strings.TrimSpace(text)
		if t == "" {
			return nil
		}
		return []string{t}
	}
	if len(separators) == 0 {
		return charSplitWithOverlap(text, chunkSize, chunkOverlap)
	}

	sep := separators[0]
	remaining := separators[1:]
	if sep == "" {
		return charSplitWithOverlap(text, chunkSize, chunkOverlap)
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
			result = append(result, recursiveSplit(part, remaining, chunkSize, chunkOverlap)...)
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
	return charSplitWithOverlap(text, chunkSize, 0)
}

func charSplitWithOverlap(text string, chunkSize, overlap int) []string {
	runes := []rune(text)
	total := len(runes)
	if total == 0 {
		return nil
	}
	if overlap < 0 || overlap >= chunkSize {
		overlap = 0
	}
	step := chunkSize - overlap
	if step <= 0 {
		step = chunkSize
	}
	var chunks []string
	for start := 0; start < total; start += step {
		end := start + chunkSize
		if end > total {
			end = total
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == total {
			break
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
		"updated_at":      time.Now(),
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

var (
	// 3 个及以上连续换行 → \n\n
	reMultiNewline = regexp.MustCompile(`\n{3,}`)
	// Unicode 水平空白（含全角空格、不间断空格等），2 个及以上 → 单空格
	// 使用反引号 raw string，\x{HHHH} 由 RE2 引擎解析
	reMultiSpace = regexp.MustCompile(`[\t\f\r \x{00a0}\x{1680}\x{180e}\x{2000}-\x{200a}\x{202f}\x{205f}\x{3000}]{2,}`)
	// 控制字符（排除 \t \n \r）
	reControlChars = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]`)
	// Markdown 图片/链接占位符保护
	reMarkdownImage = regexp.MustCompile(`!\[[^\]]*\]\((https?://[^)]+)\)`)
	reMarkdownLink  = regexp.MustCompile(`\[([^\]]*)\]\((https?://[^)]+)\)`)
	// 普通 URL
	reURL = regexp.MustCompile(`https?://\S+`)
	// 电子邮件
	reEmail = regexp.MustCompile(`[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+`)
)

func cleanWhitespace(text string) string {
	// 去除控制字符
	text = reControlChars.ReplaceAllString(text, "")
	// 多换行压缩
	text = reMultiNewline.ReplaceAllString(text, "\n\n")
	// 多空白压缩（不影响换行）
	text = reMultiSpace.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func removeURLsAndEmails(text string) string {
	// 先保护 Markdown 图片和链接，避免其中的 URL 被误删
	type placeholder struct{ orig, key string }
	var placeholders []placeholder
	idx := 0

	protect := func(s string) string {
		key := fmt.Sprintf("__MD_PLACEHOLDER_%d__", idx)
		idx++
		placeholders = append(placeholders, placeholder{s, key})
		return key
	}

	text = reMarkdownImage.ReplaceAllStringFunc(text, protect)
	text = reMarkdownLink.ReplaceAllStringFunc(text, protect)

	// 删除裸 URL 和邮件
	text = reURL.ReplaceAllString(text, "")
	text = reEmail.ReplaceAllString(text, "")

	// 还原占位符
	for _, p := range placeholders {
		text = strings.ReplaceAll(text, p.key, p.orig)
	}
	return text
}
