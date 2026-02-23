package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"yqhp/gulu/internal/model"
)

// GraphProcessor 图知识库处理器
// 使用 LLM 从文本中抽取实体和关系，构建知识图谱
type GraphProcessor struct{}

func NewGraphProcessor() *GraphProcessor {
	return &GraphProcessor{}
}

// ExtractedGraph LLM 抽取的图谱结构
type ExtractedGraph struct {
	Entities  []ExtractedEntity   `json:"entities"`
	Relations []ExtractedRelation `json:"relations"`
}

// ExtractedEntity LLM 抽取的实体
type ExtractedEntity struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ExtractedRelation LLM 抽取的关系
type ExtractedRelation struct {
	Source       string `json:"source"`
	Target       string `json:"target"`
	RelationType string `json:"relation_type"`
	Description  string `json:"description"`
}

// ProcessDocument 处理图知识库的文档
// 1. LLM 抽取实体和关系 2. 写入 Neo4j（唯一数据源）
func (g *GraphProcessor) ProcessDocument(kb *model.TKnowledgeBase, doc *model.TKnowledgeDocument, text string, chunks []string) error {
	ctx := context.Background()

	if kb.GraphExtractModelID == nil || *kb.GraphExtractModelID == 0 {
		return fmt.Errorf("图知识库未配置实体抽取模型")
	}

	if !IsNeo4jEnabled() {
		return fmt.Errorf("Neo4j 未启用，请先配置 Neo4j 连接")
	}

	aiModelLogic := NewAiModelLogic(ctx)
	aiModel, err := aiModelLogic.GetByIDWithKey(*kb.GraphExtractModelID)
	if err != nil {
		return fmt.Errorf("实体抽取模型不存在: %w", err)
	}

	var allEntities []ExtractedEntity
	var allRelations []ExtractedRelation

	for i, chunk := range chunks {
		log.Printf("[INFO] 图谱抽取: 处理第 %d/%d 个分块", i+1, len(chunks))

		graph, err := g.extractEntitiesAndRelations(ctx, aiModel.APIBaseURL, aiModel.APIKey, aiModel.ModelID, chunk)
		if err != nil {
			log.Printf("[WARN] 分块 %d 实体抽取失败: %v", i, err)
			continue
		}

		allEntities = append(allEntities, graph.Entities...)
		allRelations = append(allRelations, graph.Relations...)
	}

	// 实体去重（保留描述更长的版本）
	entityMap := make(map[string]*ExtractedEntity)
	for i := range allEntities {
		key := allEntities[i].Name + "|" + allEntities[i].Type
		if existing, ok := entityMap[key]; ok {
			if len(allEntities[i].Description) > len(existing.Description) {
				entityMap[key] = &allEntities[i]
			}
		} else {
			entityMap[key] = &allEntities[i]
		}
	}

	// 写入 Neo4j
	entityCount := 0
	for _, entity := range entityMap {
		_, err := CreateOrMergeEntity(ctx, kb.ID, doc.ID, GraphEntity{
			Name: entity.Name,
			Type: entity.Type,
		}, entity.Description)
		if err != nil {
			log.Printf("[WARN] 写入实体失败: %v", err)
			continue
		}
		entityCount++
	}

	relationCount := 0
	for _, rel := range allRelations {
		_, err := CreateRelation(ctx, kb.ID, GraphRelation{
			SourceName:   rel.Source,
			SourceType:   g.findEntityType(allEntities, rel.Source),
			TargetName:   rel.Target,
			TargetType:   g.findEntityType(allEntities, rel.Target),
			RelationType: rel.RelationType,
		}, rel.Description)
		if err != nil {
			log.Printf("[WARN] 写入关系失败: %v", err)
			continue
		}
		relationCount++
	}

	log.Printf("[INFO] 图谱构建完成: docID=%d, entities=%d, relations=%d", doc.ID, entityCount, relationCount)
	return nil
}

func (g *GraphProcessor) findEntityType(entities []ExtractedEntity, name string) string {
	for _, e := range entities {
		if e.Name == name {
			return e.Type
		}
	}
	return "Unknown"
}

const entityExtractionPrompt = `你是一个知识图谱实体关系抽取专家。请从以下文本中提取所有有意义的实体和它们之间的关系。

要求：
1. 实体类型包括但不限于：人物、组织、地点、事件、概念、技术、产品、时间等
2. 关系应当简洁明了，如"属于"、"创建"、"位于"、"参与"、"使用"等
3. 只提取文本中明确存在的信息，不要推测
4. 实体名称保持原文，不要翻译或改写

请以严格的 JSON 格式返回，不要包含其他文字：
{
  "entities": [
    {"name": "实体名称", "type": "实体类型", "description": "简短描述"}
  ],
  "relations": [
    {"source": "源实体名称", "target": "目标实体名称", "relation_type": "关系类型", "description": "关系描述"}
  ]
}

文本内容：
%s`

// extractEntitiesAndRelations 调用 LLM 抽取实体和关系
func (g *GraphProcessor) extractEntitiesAndRelations(ctx context.Context, baseURL, apiKey, modelID, text string) (*ExtractedGraph, error) {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	prompt := fmt.Sprintf(entityExtractionPrompt, text)

	reqBody := map[string]interface{}{
		"model": modelID,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature":  0.1,
		"max_tokens":   4096,
		"response_format": map[string]string{"type": "json_object"},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("请求序列化失败: %w", err)
	}

	url := baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	httpClient := &http.Client{Timeout: 120 * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("LLM HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM API 返回错误 (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}
	if chatResp.Error != nil {
		return nil, fmt.Errorf("LLM API 错误: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM 返回空结果")
	}

	content := chatResp.Choices[0].Message.Content
	var graph ExtractedGraph
	if err := json.Unmarshal([]byte(content), &graph); err != nil {
		return nil, fmt.Errorf("解析实体关系 JSON 失败: %w (content: %s)", err, content)
	}

	return &graph, nil
}
