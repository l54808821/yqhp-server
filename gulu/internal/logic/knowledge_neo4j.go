package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"yqhp/gulu/internal/config"
	"yqhp/gulu/internal/svc"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// -----------------------------------------------
// Neo4j 客户端管理器（单例）
// -----------------------------------------------

var (
	neo4jDriver neo4j.DriverWithContext
	neo4jOnce   sync.Once
	neo4jMu     sync.Mutex
)

func getNeo4jDriver() (neo4j.DriverWithContext, error) {
	cfg := getNeo4jConfig()
	if !cfg.Enabled {
		return nil, fmt.Errorf("Neo4j 未启用")
	}

	var initErr error
	neo4jOnce.Do(func() {
		driver, err := neo4j.NewDriverWithContext(cfg.URI, neo4j.BasicAuth(cfg.Username, cfg.Password, ""))
		if err != nil {
			initErr = fmt.Errorf("Neo4j 连接失败: %w", err)
			return
		}
		ctx := context.Background()
		if err := driver.VerifyConnectivity(ctx); err != nil {
			initErr = fmt.Errorf("Neo4j 连接验证失败: %w", err)
			driver.Close(ctx)
			return
		}
		neo4jDriver = driver
		log.Printf("[INFO] Neo4j 已连接: %s", cfg.URI)
	})

	if initErr != nil {
		neo4jMu.Lock()
		neo4jOnce = sync.Once{}
		neo4jMu.Unlock()
		return nil, initErr
	}

	if neo4jDriver == nil {
		return nil, fmt.Errorf("Neo4j 驱动未初始化")
	}

	return neo4jDriver, nil
}

func getNeo4jConfig() config.Neo4jConfig {
	if svc.Ctx != nil && svc.Ctx.Config != nil {
		return svc.Ctx.Config.Neo4j
	}
	return config.Neo4jConfig{}
}

func getNeo4jDatabase() string {
	cfg := getNeo4jConfig()
	if cfg.Database != "" {
		return cfg.Database
	}
	return "neo4j"
}

// IsNeo4jEnabled 检查 Neo4j 是否启用
func IsNeo4jEnabled() bool {
	return getNeo4jConfig().Enabled
}

// -----------------------------------------------
// 实体操作
// -----------------------------------------------

// GraphEntity 图谱实体
type GraphEntity struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`
}

// GraphRelation 图谱关系
type GraphRelation struct {
	SourceName   string            `json:"source_name"`
	SourceType   string            `json:"source_type"`
	TargetName   string            `json:"target_name"`
	TargetType   string            `json:"target_type"`
	RelationType string            `json:"relation_type"`
	Properties   map[string]string `json:"properties,omitempty"`
}

// CreateOrMergeEntity 创建或合并实体节点（使用 MERGE 避免重复）
func CreateOrMergeEntity(ctx context.Context, kbID int64, entity GraphEntity) (string, error) {
	driver, err := getNeo4jDriver()
	if err != nil {
		return "", err
	}

	database := getNeo4jDatabase()
	session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: database})
	defer session.Close(ctx)

	propsJSON, _ := json.Marshal(entity.Properties)

	result, err := session.Run(ctx,
		`MERGE (n:Entity {name: $name, type: $type, kb_id: $kbID})
		 ON CREATE SET n.properties = $properties, n.created_at = datetime()
		 ON MATCH SET n.properties = $properties, n.updated_at = datetime()
		 RETURN elementId(n) AS nodeId`,
		map[string]interface{}{
			"name":       entity.Name,
			"type":       entity.Type,
			"kbID":       kbID,
			"properties": string(propsJSON),
		},
	)
	if err != nil {
		return "", fmt.Errorf("创建实体失败: %w", err)
	}

	if result.Next(ctx) {
		nodeID, _ := result.Record().Get("nodeId")
		return fmt.Sprintf("%v", nodeID), nil
	}

	return "", fmt.Errorf("创建实体失败: 无返回结果")
}

// CreateRelation 创建关系
func CreateRelation(ctx context.Context, kbID int64, rel GraphRelation) (string, error) {
	driver, err := getNeo4jDriver()
	if err != nil {
		return "", err
	}

	database := getNeo4jDatabase()
	session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: database})
	defer session.Close(ctx)

	propsJSON, _ := json.Marshal(rel.Properties)

	result, err := session.Run(ctx,
		`MATCH (s:Entity {name: $sourceName, type: $sourceType, kb_id: $kbID})
		 MATCH (t:Entity {name: $targetName, type: $targetType, kb_id: $kbID})
		 MERGE (s)-[r:RELATES_TO {type: $relType}]->(t)
		 ON CREATE SET r.properties = $properties, r.created_at = datetime()
		 ON MATCH SET r.properties = $properties, r.updated_at = datetime()
		 RETURN elementId(r) AS relId`,
		map[string]interface{}{
			"sourceName": rel.SourceName,
			"sourceType": rel.SourceType,
			"targetName": rel.TargetName,
			"targetType": rel.TargetType,
			"kbID":       kbID,
			"relType":    rel.RelationType,
			"properties": string(propsJSON),
		},
	)
	if err != nil {
		return "", fmt.Errorf("创建关系失败: %w", err)
	}

	if result.Next(ctx) {
		relID, _ := result.Record().Get("relId")
		return fmt.Sprintf("%v", relID), nil
	}

	return "", fmt.Errorf("创建关系失败: 无返回结果")
}

// -----------------------------------------------
// 图谱查询
// -----------------------------------------------

// GraphSearchResult 图谱搜索结果
type GraphSearchResult struct {
	Entities  []GraphEntityResult  `json:"entities"`
	Relations []GraphRelationResult `json:"relations"`
	Subgraph  string               `json:"subgraph,omitempty"`
}

// GraphEntityResult 实体查询结果
type GraphEntityResult struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`
}

// GraphRelationResult 关系查询结果
type GraphRelationResult struct {
	Source       string `json:"source"`
	SourceType   string `json:"source_type"`
	Target       string `json:"target"`
	TargetType   string `json:"target_type"`
	RelationType string `json:"relation_type"`
}

// SearchGraphByKeywords 根据关键词搜索图谱中的相关实体和关系
func SearchGraphByKeywords(ctx context.Context, kbID int64, keywords []string, maxHops int) (*GraphSearchResult, error) {
	driver, err := getNeo4jDriver()
	if err != nil {
		return nil, err
	}

	if maxHops <= 0 {
		maxHops = 2
	}

	database := getNeo4jDatabase()
	session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: database})
	defer session.Close(ctx)

	// 先找到匹配关键词的实体
	entityResult, err := session.Run(ctx,
		`MATCH (n:Entity {kb_id: $kbID})
		 WHERE ANY(kw IN $keywords WHERE toLower(n.name) CONTAINS toLower(kw))
		 RETURN n.name AS name, n.type AS type, n.properties AS properties
		 LIMIT 20`,
		map[string]interface{}{
			"kbID":     kbID,
			"keywords": keywords,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("搜索实体失败: %w", err)
	}

	result := &GraphSearchResult{}
	var entityNames []string

	for entityResult.Next(ctx) {
		record := entityResult.Record()
		name, _ := record.Get("name")
		typ, _ := record.Get("type")
		propsStr, _ := record.Get("properties")

		entity := GraphEntityResult{
			Name: fmt.Sprintf("%v", name),
			Type: fmt.Sprintf("%v", typ),
		}
		if ps, ok := propsStr.(string); ok && ps != "" {
			_ = json.Unmarshal([]byte(ps), &entity.Properties)
		}
		result.Entities = append(result.Entities, entity)
		entityNames = append(entityNames, entity.Name)
	}

	if len(entityNames) == 0 {
		return result, nil
	}

	// 查找相关关系（N 跳以内）
	relQuery := fmt.Sprintf(
		`MATCH (s:Entity {kb_id: $kbID})-[r:RELATES_TO*1..%d]-(t:Entity {kb_id: $kbID})
		 WHERE s.name IN $names
		 UNWIND r AS rel
		 WITH startNode(rel) AS src, endNode(rel) AS tgt, rel
		 RETURN DISTINCT src.name AS source, src.type AS sourceType,
		        tgt.name AS target, tgt.type AS targetType, rel.type AS relType
		 LIMIT 50`, maxHops)

	relResult, err := session.Run(ctx, relQuery,
		map[string]interface{}{
			"kbID":  kbID,
			"names": entityNames,
		},
	)
	if err != nil {
		log.Printf("[WARN] 搜索关系失败: %v", err)
		return result, nil
	}

	for relResult.Next(ctx) {
		record := relResult.Record()
		source, _ := record.Get("source")
		sourceType, _ := record.Get("sourceType")
		target, _ := record.Get("target")
		targetType, _ := record.Get("targetType")
		relType, _ := record.Get("relType")

		result.Relations = append(result.Relations, GraphRelationResult{
			Source:       fmt.Sprintf("%v", source),
			SourceType:   fmt.Sprintf("%v", sourceType),
			Target:       fmt.Sprintf("%v", target),
			TargetType:   fmt.Sprintf("%v", targetType),
			RelationType: fmt.Sprintf("%v", relType),
		})
	}

	return result, nil
}

// DeleteKnowledgeBaseGraph 删除知识库的所有图谱数据
func DeleteKnowledgeBaseGraph(ctx context.Context, kbID int64) error {
	driver, err := getNeo4jDriver()
	if err != nil {
		return err
	}

	database := getNeo4jDatabase()
	session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: database})
	defer session.Close(ctx)

	_, err = session.Run(ctx,
		`MATCH (n:Entity {kb_id: $kbID}) DETACH DELETE n`,
		map[string]interface{}{"kbID": kbID},
	)
	if err != nil {
		return fmt.Errorf("删除图谱数据失败: %w", err)
	}

	log.Printf("[INFO] Neo4j: 已删除知识库 %d 的图谱数据", kbID)
	return nil
}

// DeleteDocumentGraph 删除文档相关的图谱数据
func DeleteDocumentGraph(ctx context.Context, kbID int64, docID int64) error {
	driver, err := getNeo4jDriver()
	if err != nil {
		return err
	}

	database := getNeo4jDatabase()
	session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: database})
	defer session.Close(ctx)

	_, err = session.Run(ctx,
		`MATCH (n:Entity {kb_id: $kbID, doc_id: $docID}) DETACH DELETE n`,
		map[string]interface{}{"kbID": kbID, "docID": docID},
	)
	if err != nil {
		return fmt.Errorf("删除文档图谱数据失败: %w", err)
	}

	return nil
}

// FormatGraphAsContext 将图谱搜索结果格式化为 LLM 可读的上下文文本
func FormatGraphAsContext(result *GraphSearchResult) string {
	if result == nil || (len(result.Entities) == 0 && len(result.Relations) == 0) {
		return ""
	}

	var text string

	if len(result.Entities) > 0 {
		text += "[实体]\n"
		for _, e := range result.Entities {
			text += fmt.Sprintf("- %s (%s)", e.Name, e.Type)
			if len(e.Properties) > 0 {
				for k, v := range e.Properties {
					text += fmt.Sprintf(", %s: %s", k, v)
				}
			}
			text += "\n"
		}
	}

	if len(result.Relations) > 0 {
		text += "\n[关系]\n"
		for _, r := range result.Relations {
			text += fmt.Sprintf("- %s(%s) -[%s]-> %s(%s)\n",
				r.Source, r.SourceType, r.RelationType, r.Target, r.TargetType)
		}
	}

	return text
}
