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

// getQdrantClient 获取 Qdrant 客户端单例
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
		// 重置 once 以允许重试
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
// Collection 操作
// -----------------------------------------------

// CreateQdrantCollection 创建 Qdrant Collection
func CreateQdrantCollection(collectionName string, dimension int) error {
	client, err := getQdrantClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// 检查是否已存在
	exists, err := client.CollectionExists(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("检查 Collection 是否存在失败: %w", err)
	}
	if exists {
		// 验证维度是否匹配，不匹配则删除重建
		info, err := client.GetCollectionInfo(ctx, collectionName)
		if err == nil && info != nil {
			if params := info.GetConfig().GetParams(); params != nil {
				if vc := params.GetVectorsConfig(); vc != nil {
					if paramsMap := vc.GetParamsMap(); paramsMap != nil {
						if textParams, ok := paramsMap.GetMap()["text"]; ok {
							existingDim := int(textParams.GetSize())
							if existingDim != dimension {
								log.Printf("[WARN] Qdrant Collection %s 维度不匹配 (现有: %d, 需要: %d), 重建 Collection",
									collectionName, existingDim, dimension)
								_ = client.DeleteCollection(ctx, collectionName)
								exists = false
							}
						}
					}
				}
			}
		}
		if exists {
			log.Printf("[INFO] Qdrant Collection %s 已存在且维度匹配, 跳过创建", collectionName)
			return nil
		}
	}

	err = client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: collectionName,
		VectorsConfig: qdrant.NewVectorsConfigMap(
			map[string]*qdrant.VectorParams{
				"text": {
					Size:     uint64(dimension),
					Distance: qdrant.Distance_Cosine,
				},
			},
		),
	})
	if err != nil {
		return fmt.Errorf("创建 Collection 失败: %w", err)
	}

	log.Printf("[INFO] Qdrant Collection %s 创建成功 (维度: %d)", collectionName, dimension)
	return nil
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
// 向量写入
// -----------------------------------------------

// UpsertVectors 批量写入向量到 Qdrant
func UpsertVectors(collectionName string, points []VectorPoint) error {
	client, err := getQdrantClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// 转换为 Qdrant 点格式
	qdrantPoints := make([]*qdrant.PointStruct, 0, len(points))
	for _, p := range points {
		payload := map[string]*qdrant.Value{
			"content":     qdrant.NewValueString(p.Content),
			"document_id": qdrant.NewValueInt(p.DocumentID),
			"chunk_index": qdrant.NewValueInt(int64(p.ChunkIndex)),
		}
		// 添加额外的 metadata
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

		// 使用数字 ID: documentID * 100000 + chunkIndex
		pointID := uint64(p.DocumentID)*100000 + uint64(p.ChunkIndex)
		qdrantPoints = append(qdrantPoints, &qdrant.PointStruct{
			Id: qdrant.NewIDNum(pointID),
			Vectors: qdrant.NewVectorsMap(map[string]*qdrant.Vector{
				"text": qdrant.NewVectorDense(p.Vector),
			}),
			Payload: payload,
		})
	}

	// 分批写入，每批 100 个
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

	log.Printf("[INFO] Qdrant: 成功写入 %d 个向量到 %s", len(points), collectionName)
	return nil
}

// -----------------------------------------------
// 向量搜索
// -----------------------------------------------

// SearchVectors 向量相似度搜索
func SearchVectors(collectionName string, queryVector []float32, topK int, scoreThreshold float32) ([]SearchHit, error) {
	client, err := getQdrantClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	results, err := client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: collectionName,
		Query:          qdrant.NewQueryDense(queryVector),
		Using:          qdrant.PtrOf("text"),
		Limit:          qdrant.PtrOf(uint64(topK)),
		ScoreThreshold: qdrant.PtrOf(scoreThreshold),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("向量搜索失败: %w", err)
	}

	hits := make([]SearchHit, 0, len(results))
	for _, r := range results {
		hit := SearchHit{
			ID:    fmt.Sprintf("%d", r.GetId().GetNum()),
			Score: float64(r.GetScore()),
		}

		// 提取 payload
		payload := r.GetPayload()
		if payload != nil {
			if v, ok := payload["content"]; ok {
				hit.Content = v.GetStringValue()
			}
			if v, ok := payload["document_id"]; ok {
				hit.DocumentID = v.GetIntegerValue()
			}
			if v, ok := payload["chunk_index"]; ok {
				hit.ChunkIndex = int(v.GetIntegerValue())
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

	return hits, nil
}

// -----------------------------------------------
// 向量浏览（Scroll）
// -----------------------------------------------

// ScrollDocumentVectors 获取指定文档的所有向量点（不做相似度搜索，遍历读取）
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
			if v, ok := payload["document_id"]; ok {
				hit.DocumentID = v.GetIntegerValue()
			}
			if v, ok := payload["chunk_index"]; ok {
				hit.ChunkIndex = int(v.GetIntegerValue())
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

	// 使用 filter 条件删除
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

// VectorPoint 向量数据点
type VectorPoint struct {
	ID         string                 `json:"id"`
	Vector     []float32              `json:"vector"`
	DocumentID int64                  `json:"document_id"`
	ChunkIndex int                    `json:"chunk_index"`
	Content    string                 `json:"content"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// SearchHit 搜索命中结果
type SearchHit struct {
	ID         string                 `json:"id"`
	Score      float64                `json:"score"`
	Content    string                 `json:"content"`
	DocumentID int64                  `json:"document_id"`
	ChunkIndex int                    `json:"chunk_index"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
