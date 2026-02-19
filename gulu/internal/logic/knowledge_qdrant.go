package logic

import (
	"context"
	"fmt"
	"log"
	"sync"

	"yqhp/gulu/internal/config"
	"yqhp/gulu/internal/svc"

	"github.com/qdrant/go-client/qdrant"
)

// -----------------------------------------------
// Qdrant 客户端管理器（单例）
// -----------------------------------------------

var (
	qdrantClient *qdrant.Client
	qdrantOnce   sync.Once
	qdrantMu     sync.Mutex
)

func getQdrantClient() (*qdrant.Client, error) {
	var initErr error
	qdrantOnce.Do(func() {
		cfg := getQdrantConfig()
		client, err := qdrant.NewClient(&qdrant.Config{
			Host:   cfg.Host,
			Port:   cfg.Port,
			APIKey: cfg.APIKey,
			UseTLS: cfg.UseTLS,
		})
		if err != nil {
			initErr = fmt.Errorf("Qdrant 连接失败: %w", err)
			return
		}
		qdrantClient = client
		log.Printf("[INFO] Qdrant 已连接: %s:%d", cfg.Host, cfg.Port)
	})

	if initErr != nil {
		qdrantMu.Lock()
		qdrantOnce = sync.Once{}
		qdrantMu.Unlock()
		return nil, initErr
	}

	if qdrantClient == nil {
		return nil, fmt.Errorf("Qdrant 客户端未初始化")
	}

	return qdrantClient, nil
}

func getQdrantConfig() config.QdrantConfig {
	if svc.Ctx != nil && svc.Ctx.Config != nil {
		return svc.Ctx.Config.Qdrant
	}
	return config.QdrantConfig{
		Host: "127.0.0.1",
		Port: 6334,
	}
}

// -----------------------------------------------
// Collection 操作（支持 Named Vectors: text + image）
// -----------------------------------------------

// CollectionVectorConfig 向量字段配置
type CollectionVectorConfig struct {
	TextDimension  int // 文本向量维度（必填）
	ImageDimension int // 图片向量维度（0 表示不启用多模态）
}

// CreateQdrantCollection 创建 Qdrant Collection（仅 text 向量）
func CreateQdrantCollection(collectionName string, dimension int) error {
	return CreateQdrantCollectionMultiVector(collectionName, CollectionVectorConfig{
		TextDimension: dimension,
	})
}

// CreateQdrantCollectionMultiVector 创建支持多向量字段的 Qdrant Collection
func CreateQdrantCollectionMultiVector(collectionName string, cfg CollectionVectorConfig) error {
	client, err := getQdrantClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	exists, err := client.CollectionExists(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("检查 Collection 是否存在失败: %w", err)
	}

	if exists {
		needRecreate := false
		info, err := client.GetCollectionInfo(ctx, collectionName)
		if err == nil && info != nil {
			if params := info.GetConfig().GetParams(); params != nil {
				if vc := params.GetVectorsConfig(); vc != nil {
					if paramsMap := vc.GetParamsMap(); paramsMap != nil {
						m := paramsMap.GetMap()
						if textParams, ok := m["text"]; ok {
							if int(textParams.GetSize()) != cfg.TextDimension {
								needRecreate = true
							}
						}
						// 如果需要 image 向量但不存在，也需要重建
						if cfg.ImageDimension > 0 {
							if _, ok := m["image"]; !ok {
								needRecreate = true
							}
						}
					}
				}
			}
		}
		if needRecreate {
			log.Printf("[WARN] Qdrant Collection %s 配置不匹配, 重建 Collection", collectionName)
			_ = client.DeleteCollection(ctx, collectionName)
		} else {
			log.Printf("[INFO] Qdrant Collection %s 已存在且配置匹配, 跳过创建", collectionName)
			return nil
		}
	}

	vectorsMap := map[string]*qdrant.VectorParams{
		"text": {
			Size:     uint64(cfg.TextDimension),
			Distance: qdrant.Distance_Cosine,
		},
	}

	if cfg.ImageDimension > 0 {
		vectorsMap["image"] = &qdrant.VectorParams{
			Size:     uint64(cfg.ImageDimension),
			Distance: qdrant.Distance_Cosine,
		}
	}

	err = client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: collectionName,
		VectorsConfig:  qdrant.NewVectorsConfigMap(vectorsMap),
	})
	if err != nil {
		return fmt.Errorf("创建 Collection 失败: %w", err)
	}

	fields := "text"
	if cfg.ImageDimension > 0 {
		fields = fmt.Sprintf("text(%d) + image(%d)", cfg.TextDimension, cfg.ImageDimension)
	}
	log.Printf("[INFO] Qdrant Collection %s 创建成功 (向量: %s)", collectionName, fields)
	return nil
}

// AddImageVectorField 为已有 Collection 添加 image 向量字段
func AddImageVectorField(collectionName string, imageDimension int) error {
	client, err := getQdrantClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	exists, err := client.CollectionExists(ctx, collectionName)
	if err != nil || !exists {
		return fmt.Errorf("Collection %s 不存在", collectionName)
	}

	info, err := client.GetCollectionInfo(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("获取 Collection 信息失败: %w", err)
	}

	if params := info.GetConfig().GetParams(); params != nil {
		if vc := params.GetVectorsConfig(); vc != nil {
			if paramsMap := vc.GetParamsMap(); paramsMap != nil {
				if _, ok := paramsMap.GetMap()["image"]; ok {
					log.Printf("[INFO] Qdrant Collection %s 已有 image 字段, 跳过", collectionName)
					return nil
				}
			}
		}
	}

	// Qdrant 不支持直接添加 named vector，需要删除重建
	// 先获取 text 维度
	var textDim int
	if params := info.GetConfig().GetParams(); params != nil {
		if vc := params.GetVectorsConfig(); vc != nil {
			if paramsMap := vc.GetParamsMap(); paramsMap != nil {
				if textParams, ok := paramsMap.GetMap()["text"]; ok {
					textDim = int(textParams.GetSize())
				}
			}
		}
	}
	if textDim == 0 {
		return fmt.Errorf("无法获取 text 向量维度")
	}

	log.Printf("[WARN] 需要重建 Collection %s 以添加 image 向量字段，现有数据会丢失并需重新索引", collectionName)
	_ = client.DeleteCollection(ctx, collectionName)
	return CreateQdrantCollectionMultiVector(collectionName, CollectionVectorConfig{
		TextDimension:  textDim,
		ImageDimension: imageDimension,
	})
}

// DeleteQdrantCollection 删除 Qdrant Collection
func DeleteQdrantCollection(collectionName string) error {
	client, err := getQdrantClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	err = client.DeleteCollection(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("删除 Collection 失败: %w", err)
	}

	log.Printf("[INFO] Qdrant Collection %s 已删除", collectionName)
	return nil
}

// -----------------------------------------------
// 向量写入（支持多向量字段）
// -----------------------------------------------

// UpsertVectors 批量写入向量到 Qdrant（默认写入 text 字段）
func UpsertVectors(collectionName string, points []VectorPoint) error {
	return UpsertVectorsToField(collectionName, "text", points)
}

// UpsertVectorsToField 批量写入向量到指定字段
func UpsertVectorsToField(collectionName string, vectorField string, points []VectorPoint) error {
	client, err := getQdrantClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	qdrantPoints := make([]*qdrant.PointStruct, 0, len(points))
	for _, p := range points {
		payload := map[string]*qdrant.Value{
			"content":      qdrant.NewValueString(p.Content),
			"content_type": qdrant.NewValueString(p.ContentType),
			"document_id":  qdrant.NewValueInt(p.DocumentID),
			"chunk_index":  qdrant.NewValueInt(int64(p.ChunkIndex)),
			"vector_field": qdrant.NewValueString(vectorField),
		}
		if p.ImagePath != "" {
			payload["image_path"] = qdrant.NewValueString(p.ImagePath)
		}
		for k, v := range p.Metadata {
			switch val := v.(type) {
			case string:
				payload[k] = qdrant.NewValueString(val)
			case int:
				payload[k] = qdrant.NewValueInt(int64(val))
			case int64:
				payload[k] = qdrant.NewValueInt(val)
			case float64:
				payload[k] = qdrant.NewValueDouble(val)
			case bool:
				payload[k] = qdrant.NewValueBool(val)
			}
		}

		pointID := uint64(p.DocumentID)*100000 + uint64(p.ChunkIndex)
		qdrantPoints = append(qdrantPoints, &qdrant.PointStruct{
			Id: qdrant.NewIDNum(pointID),
			Vectors: qdrant.NewVectorsMap(map[string]*qdrant.Vector{
				vectorField: qdrant.NewVectorDense(p.Vector),
			}),
			Payload: payload,
		})
	}

	batchSize := 100
	for i := 0; i < len(qdrantPoints); i += batchSize {
		end := i + batchSize
		if end > len(qdrantPoints) {
			end = len(qdrantPoints)
		}
		batch := qdrantPoints[i:end]

		_, err = client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: collectionName,
			Points:         batch,
		})
		if err != nil {
			return fmt.Errorf("向量写入失败 (batch %d-%d): %w", i, end, err)
		}
	}

	log.Printf("[INFO] Qdrant: 成功写入 %d 个向量到 %s.%s", len(points), collectionName, vectorField)
	return nil
}

// -----------------------------------------------
// 向量搜索（支持指定搜索字段）
// -----------------------------------------------

// SearchVectors 向量相似度搜索（默认搜索 text 字段）
func SearchVectors(collectionName string, queryVector []float32, topK int, scoreThreshold float32) ([]SearchHit, error) {
	return SearchVectorsInField(collectionName, "text", queryVector, topK, scoreThreshold, nil)
}

// SearchVectorsInField 在指定向量字段中搜索
// filter 可选，传 nil 表示不过滤
func SearchVectorsInField(collectionName string, vectorField string, queryVector []float32, topK int, scoreThreshold float32, filter *qdrant.Filter) ([]SearchHit, error) {
	client, err := getQdrantClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	queryParams := &qdrant.QueryPoints{
		CollectionName: collectionName,
		Query:          qdrant.NewQueryDense(queryVector),
		Using:          qdrant.PtrOf(vectorField),
		Limit:          qdrant.PtrOf(uint64(topK)),
		ScoreThreshold: qdrant.PtrOf(scoreThreshold),
		WithPayload:    qdrant.NewWithPayload(true),
	}
	if filter != nil {
		queryParams.Filter = filter
	}

	results, err := client.Query(ctx, queryParams)
	if err != nil {
		return nil, fmt.Errorf("向量搜索失败: %w", err)
	}

	return parseSearchResults(results), nil
}

// SearchVectorsMultiField 在多个向量字段中搜索并合并结果
func SearchVectorsMultiField(collectionName string, fields map[string][]float32, topK int, scoreThreshold float32) ([]SearchHit, error) {
	var allHits []SearchHit
	seen := make(map[string]bool)

	for field, queryVector := range fields {
		hits, err := SearchVectorsInField(collectionName, field, queryVector, topK, scoreThreshold, nil)
		if err != nil {
			log.Printf("[WARN] 搜索字段 %s 失败: %v", field, err)
			continue
		}
		for _, hit := range hits {
			if !seen[hit.ID] {
				seen[hit.ID] = true
				allHits = append(allHits, hit)
			}
		}
	}

	// 按分数排序并截取 topK
	sortSearchHits(allHits)
	if len(allHits) > topK {
		allHits = allHits[:topK]
	}

	return allHits, nil
}

func parseSearchResults(results []*qdrant.ScoredPoint) []SearchHit {
	hits := make([]SearchHit, 0, len(results))
	for _, r := range results {
		hit := SearchHit{
			ID:    fmt.Sprintf("%d", r.GetId().GetNum()),
			Score: float64(r.GetScore()),
		}

		payload := r.GetPayload()
		if payload != nil {
			if v, ok := payload["content"]; ok {
				hit.Content = v.GetStringValue()
			}
			if v, ok := payload["content_type"]; ok {
				hit.ContentType = v.GetStringValue()
			}
			if v, ok := payload["document_id"]; ok {
				hit.DocumentID = v.GetIntegerValue()
			}
			if v, ok := payload["chunk_index"]; ok {
				hit.ChunkIndex = int(v.GetIntegerValue())
			}
			if v, ok := payload["image_path"]; ok {
				hit.ImagePath = v.GetStringValue()
			}
			if v, ok := payload["vector_field"]; ok {
				hit.VectorField = v.GetStringValue()
			}
			if v, ok := payload["document_name"]; ok {
				if hit.Metadata == nil {
					hit.Metadata = make(map[string]interface{})
				}
				hit.Metadata["document_name"] = v.GetStringValue()
			}
		}

		hits = append(hits, hit)
	}
	return hits
}

func sortSearchHits(hits []SearchHit) {
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].Score > hits[j-1].Score; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
}

// -----------------------------------------------
// 向量浏览（Scroll）
// -----------------------------------------------

// ScrollDocumentVectors 获取指定文档的所有向量点
func ScrollDocumentVectors(collectionName string, documentID int64) ([]SearchHit, error) {
	client, err := getQdrantClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	results, err := client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: collectionName,
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatchInt("document_id", documentID),
			},
		},
		Limit:       qdrant.PtrOf(uint32(1000)),
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("Qdrant Scroll 失败: %w", err)
	}

	var hits []SearchHit
	for _, r := range results {
		hit := SearchHit{
			ID: fmt.Sprintf("%d", r.GetId().GetNum()),
		}
		payload := r.GetPayload()
		if payload != nil {
			if v, ok := payload["content"]; ok {
				hit.Content = v.GetStringValue()
			}
			if v, ok := payload["content_type"]; ok {
				hit.ContentType = v.GetStringValue()
			}
			if v, ok := payload["document_id"]; ok {
				hit.DocumentID = v.GetIntegerValue()
			}
			if v, ok := payload["chunk_index"]; ok {
				hit.ChunkIndex = int(v.GetIntegerValue())
			}
			if v, ok := payload["image_path"]; ok {
				hit.ImagePath = v.GetStringValue()
			}
		}
		hits = append(hits, hit)
	}

	return hits, nil
}

// -----------------------------------------------
// 向量删除
// -----------------------------------------------

// DeleteDocumentVectors 删除指定文档的所有向量
func DeleteDocumentVectors(collectionName string, documentID int64) error {
	client, err := getQdrantClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	_, err = client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collectionName,
		Points: qdrant.NewPointsSelectorFilter(
			&qdrant.Filter{
				Must: []*qdrant.Condition{
					qdrant.NewMatchInt("document_id", documentID),
				},
			},
		),
	})
	if err != nil {
		return fmt.Errorf("删除文档向量失败: %w", err)
	}

	log.Printf("[INFO] Qdrant: 已删除文档 %d 的向量 (Collection: %s)", documentID, collectionName)
	return nil
}

// -----------------------------------------------
// 数据结构
// -----------------------------------------------

// VectorPoint 向量数据点（支持多模态）
type VectorPoint struct {
	ID          string                 `json:"id"`
	Vector      []float32              `json:"vector"`
	DocumentID  int64                  `json:"document_id"`
	ChunkIndex  int                    `json:"chunk_index"`
	Content     string                 `json:"content"`
	ContentType string                 `json:"content_type"` // text / image
	ImagePath   string                 `json:"image_path,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// SearchHit 搜索命中结果（支持多模态）
type SearchHit struct {
	ID          string                 `json:"id"`
	Score       float64                `json:"score"`
	Content     string                 `json:"content"`
	ContentType string                 `json:"content_type"` // text / image
	ImagePath   string                 `json:"image_path,omitempty"`
	VectorField string                 `json:"vector_field,omitempty"` // text / image
	DocumentID  int64                  `json:"document_id"`
	ChunkIndex  int                    `json:"chunk_index"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
