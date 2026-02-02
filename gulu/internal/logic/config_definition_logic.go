package logic

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
)

var (
	ErrConfigDefinitionNotFound = errors.New("配置定义不存在")
)

// CreateConfigDefinitionReq 创建配置定义请求
type CreateConfigDefinitionReq struct {
	ProjectID   int64  `json:"project_id"`
	Type        string `json:"type"`        // domain/variable/database/mq
	Name        string `json:"name"`        // 显示名称
	Description string `json:"description"` // 描述
	Extra       any    `json:"extra"`       // 类型特有属性
	Sort        int32  `json:"sort"`
	Status      int32  `json:"status"`
}

// UpdateConfigDefinitionReq 更新配置定义请求
type UpdateConfigDefinitionReq struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Extra       any     `json:"extra"`
	Sort        *int32  `json:"sort"`
	Status      *int32  `json:"status"`
}

// ConfigDefinitionWithValue 配置定义（包含当前环境的值）
type ConfigDefinitionWithValue struct {
	*model.TConfigDefinition
	Value string `json:"value"` // 当前环境的配置值
}

// CreateConfigDefinition 创建配置定义
// 创建后自动为项目下所有环境创建对应的配置记录
func CreateConfigDefinition(ctx context.Context, req *CreateConfigDefinitionReq) (*model.TConfigDefinition, error) {
	q := query.Q

	// 生成唯一 code
	code := uuid.New().String()

	// 序列化 extra
	var extraStr *string
	if req.Extra != nil {
		extraBytes, err := json.Marshal(req.Extra)
		if err != nil {
			return nil, err
		}
		s := string(extraBytes)
		extraStr = &s
	}

	// 自动获取下一个排序值
	nextSort, err := GetNextSort(ctx, req.ProjectID, req.Type)
	if err != nil {
		nextSort = SortGap
	}

	isDelete := false
	definition := &model.TConfigDefinition{
		ProjectID:   req.ProjectID,
		Type:        req.Type,
		Code:        code,
		Name:        req.Name,
		Description: &req.Description,
		Extra:       extraStr,
		Sort:        &nextSort,
		Status:      &req.Status,
		IsDelete:    &isDelete,
	}

	// 开启事务
	err = q.Transaction(func(tx *query.Query) error {
		// 创建配置定义
		if err := tx.TConfigDefinition.WithContext(ctx).Create(definition); err != nil {
			return err
		}

		// 获取项目下所有环境
		envs, err := tx.TEnv.WithContext(ctx).
			Where(tx.TEnv.ProjectID.Eq(req.ProjectID)).
			Where(tx.TEnv.IsDelete.Is(false)).
			Find()
		if err != nil {
			return err
		}

		// 为每个环境创建配置记录
		if len(envs) > 0 {
			configs := make([]*model.TConfig, 0, len(envs))
			defaultValue := getDefaultValue(req.Type)
			for _, env := range envs {
				configs = append(configs, &model.TConfig{
					ProjectID: req.ProjectID,
					EnvID:     env.ID,
					Type:      req.Type,
					Code:      code,
					Value:     defaultValue,
				})
			}
			if err := tx.TConfig.WithContext(ctx).CreateInBatches(configs, 100); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return definition, nil
}

// getDefaultValue 获取配置类型的默认值
func getDefaultValue(configType string) string {
	switch configType {
	case model.ConfigTypeDomain:
		return `{"base_url":"","headers":[]}`
	case model.ConfigTypeVariable:
		return `{"value":""}`
	case model.ConfigTypeDatabase:
		return `{"host":"","port":3306,"database":"","username":"","password":"","options":""}`
	case model.ConfigTypeMQ:
		return `{"host":"","port":5672,"username":"","password":"","vhost":"/","options":""}`
	default:
		return `{}`
	}
}

// GetConfigDefinitionsByProject 获取项目的配置定义列表
func GetConfigDefinitionsByProject(ctx context.Context, projectID int64, configType string) ([]*model.TConfigDefinition, error) {
	q := query.Q

	query := q.TConfigDefinition.WithContext(ctx).
		Where(q.TConfigDefinition.ProjectID.Eq(projectID)).
		Where(q.TConfigDefinition.IsDelete.Is(false))

	if configType != "" {
		query = query.Where(q.TConfigDefinition.Type.Eq(configType))
	}

	return query.Order(q.TConfigDefinition.Sort, q.TConfigDefinition.ID).Find()
}

// GetConfigDefinitionByCode 根据 code 获取配置定义
func GetConfigDefinitionByCode(ctx context.Context, code string) (*model.TConfigDefinition, error) {
	q := query.Q

	definition, err := q.TConfigDefinition.WithContext(ctx).
		Where(q.TConfigDefinition.Code.Eq(code)).
		Where(q.TConfigDefinition.IsDelete.Is(false)).
		First()
	if err != nil {
		return nil, ErrConfigDefinitionNotFound
	}

	return definition, nil
}

// UpdateConfigDefinition 更新配置定义
func UpdateConfigDefinition(ctx context.Context, code string, req *UpdateConfigDefinitionReq) (*model.TConfigDefinition, error) {
	q := query.Q

	// 获取现有定义
	_, err := GetConfigDefinitionByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Extra != nil {
		extraBytes, err := json.Marshal(req.Extra)
		if err != nil {
			return nil, err
		}
		updates["extra"] = string(extraBytes)
	}
	if req.Sort != nil {
		updates["sort"] = *req.Sort
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) > 0 {
		_, err = q.TConfigDefinition.WithContext(ctx).
			Where(q.TConfigDefinition.Code.Eq(code)).
			Updates(updates)
		if err != nil {
			return nil, err
		}
	}

	// 返回更新后的定义
	return GetConfigDefinitionByCode(ctx, code)
}

// DeleteConfigDefinition 删除配置定义
// 同时删除所有环境中该配置的值
func DeleteConfigDefinition(ctx context.Context, code string) error {
	q := query.Q

	// 获取现有定义
	definition, err := GetConfigDefinitionByCode(ctx, code)
	if err != nil {
		return err
	}

	// 开启事务
	return q.Transaction(func(tx *query.Query) error {
		// 软删除配置定义
		isDelete := true
		_, err := tx.TConfigDefinition.WithContext(ctx).
			Where(tx.TConfigDefinition.ID.Eq(definition.ID)).
			Update(tx.TConfigDefinition.IsDelete, isDelete)
		if err != nil {
			return err
		}

		// 删除所有环境中该配置的值
		_, err = tx.TConfig.WithContext(ctx).
			Where(tx.TConfig.Code.Eq(code)).
			Delete()
		if err != nil {
			return err
		}

		return nil
	})
}

// SyncConfigDefinitionsToEnv 同步配置定义到新环境
// 当创建新环境时调用，为新环境创建所有配置记录
func SyncConfigDefinitionsToEnv(ctx context.Context, projectID, envID int64) error {
	q := query.Q

	// 获取项目的所有配置定义
	definitions, err := q.TConfigDefinition.WithContext(ctx).
		Where(q.TConfigDefinition.ProjectID.Eq(projectID)).
		Where(q.TConfigDefinition.IsDelete.Is(false)).
		Find()
	if err != nil {
		return err
	}

	if len(definitions) == 0 {
		return nil
	}

	// 为新环境创建配置记录
	configs := make([]*model.TConfig, 0, len(definitions))
	for _, def := range definitions {
		configs = append(configs, &model.TConfig{
			ProjectID: projectID,
			EnvID:     envID,
			Type:      def.Type,
			Code:      def.Code,
			Value:     getDefaultValue(def.Type),
		})
	}

	return q.TConfig.WithContext(ctx).CreateInBatches(configs, 100)
}

// ==================== 配置定义排序 ====================

const (
	SortGap    = 1000 // 初始排序间隔
	MinSortGap = 1    // 最小间隔，触发重新平衡
)

// UpdateConfigDefinitionSortReq 更新配置定义排序请求
type UpdateConfigDefinitionSortReq struct {
	Code     string `json:"code"`      // 被移动项的 code
	TargetCode string `json:"target_code"` // 目标位置项的 code
	Position string `json:"position"`  // before 或 after
}

// UpdateConfigDefinitionSort 更新配置定义排序（稀疏整数排序）
// 大多数情况只更新 1 条记录，空间耗尽时批量重新平衡
func UpdateConfigDefinitionSort(ctx context.Context, projectID int64, configType string, req *UpdateConfigDefinitionSortReq) error {
	q := query.Q

	// 获取被移动项
	draggedDef, err := q.TConfigDefinition.WithContext(ctx).
		Where(q.TConfigDefinition.Code.Eq(req.Code)).
		Where(q.TConfigDefinition.ProjectID.Eq(projectID)).
		Where(q.TConfigDefinition.Type.Eq(configType)).
		Where(q.TConfigDefinition.IsDelete.Is(false)).
		First()
	if err != nil {
		return errors.New("被移动的配置不存在")
	}

	// 获取目标位置项
	targetDef, err := q.TConfigDefinition.WithContext(ctx).
		Where(q.TConfigDefinition.Code.Eq(req.TargetCode)).
		Where(q.TConfigDefinition.ProjectID.Eq(projectID)).
		Where(q.TConfigDefinition.Type.Eq(configType)).
		Where(q.TConfigDefinition.IsDelete.Is(false)).
		First()
	if err != nil {
		return errors.New("目标配置不存在")
	}

	// 获取同类型的所有配置定义，按 sort 排序
	definitions, err := q.TConfigDefinition.WithContext(ctx).
		Where(q.TConfigDefinition.ProjectID.Eq(projectID)).
		Where(q.TConfigDefinition.Type.Eq(configType)).
		Where(q.TConfigDefinition.IsDelete.Is(false)).
		Order(q.TConfigDefinition.Sort, q.TConfigDefinition.ID).
		Find()
	if err != nil {
		return err
	}

	// 找到目标项的索引
	var targetIdx int
	for i, def := range definitions {
		if def.Code == req.TargetCode {
			targetIdx = i
			break
		}
	}

	// 计算新的 sort 值
	var newSort int32
	var needRebalance bool

	if req.Position == "before" {
		// 插入到目标项之前
		if targetIdx == 0 {
			// 插入到最前面
			targetSort := getSort(targetDef)
			if targetSort > SortGap {
				newSort = targetSort - SortGap
			} else if targetSort > MinSortGap {
				newSort = targetSort / 2
			} else {
				needRebalance = true
			}
		} else {
			// 插入到两项之间
			prevDef := definitions[targetIdx-1]
			if prevDef.Code == req.Code {
				// 前一项就是自己，不需要移动
				return nil
			}
			prevSort := getSort(prevDef)
			targetSort := getSort(targetDef)
			gap := targetSort - prevSort
			if gap > MinSortGap {
				newSort = prevSort + gap/2
			} else {
				needRebalance = true
			}
		}
	} else {
		// 插入到目标项之后
		if targetIdx == len(definitions)-1 {
			// 插入到最后面
			targetSort := getSort(targetDef)
			newSort = targetSort + SortGap
		} else {
			// 插入到两项之间
			nextDef := definitions[targetIdx+1]
			if nextDef.Code == req.Code {
				// 后一项就是自己，不需要移动
				return nil
			}
			targetSort := getSort(targetDef)
			nextSort := getSort(nextDef)
			gap := nextSort - targetSort
			if gap > MinSortGap {
				newSort = targetSort + gap/2
			} else {
				needRebalance = true
			}
		}
	}

	if needRebalance {
		// 需要重新平衡：批量更新所有记录
		return rebalanceSort(ctx, projectID, configType, draggedDef.Code, req.TargetCode, req.Position)
	}

	// 只更新被移动项的 sort 值
	_, err = q.TConfigDefinition.WithContext(ctx).
		Where(q.TConfigDefinition.ID.Eq(draggedDef.ID)).
		Update(q.TConfigDefinition.Sort, newSort)
	return err
}

// getSort 获取配置定义的 sort 值
func getSort(def *model.TConfigDefinition) int32 {
	if def.Sort != nil {
		return *def.Sort
	}
	return 0
}

// rebalanceSort 重新平衡排序（批量更新）
func rebalanceSort(ctx context.Context, projectID int64, configType string, draggedCode string, targetCode string, position string) error {
	q := query.Q

	// 获取所有配置定义
	definitions, err := q.TConfigDefinition.WithContext(ctx).
		Where(q.TConfigDefinition.ProjectID.Eq(projectID)).
		Where(q.TConfigDefinition.Type.Eq(configType)).
		Where(q.TConfigDefinition.IsDelete.Is(false)).
		Order(q.TConfigDefinition.Sort, q.TConfigDefinition.ID).
		Find()
	if err != nil {
		return err
	}

	// 找到拖动项和目标项的索引
	var draggedIdx, targetIdx int
	var draggedDef *model.TConfigDefinition
	for i, def := range definitions {
		if def.Code == draggedCode {
			draggedIdx = i
			draggedDef = def
		}
		if def.Code == targetCode {
			targetIdx = i
		}
	}

	// 从列表中移除拖动项
	definitions = append(definitions[:draggedIdx], definitions[draggedIdx+1:]...)

	// 重新计算目标索引
	if draggedIdx < targetIdx {
		targetIdx--
	}

	// 根据 position 插入到正确位置
	var insertIdx int
	if position == "before" {
		insertIdx = targetIdx
	} else {
		insertIdx = targetIdx + 1
	}

	// 插入拖动项到新位置
	definitions = append(definitions[:insertIdx], append([]*model.TConfigDefinition{draggedDef}, definitions[insertIdx:]...)...)

	// 批量更新排序值（使用大间隔）
	for i, def := range definitions {
		newSort := int32((i + 1) * SortGap)
		_, err := q.TConfigDefinition.WithContext(ctx).
			Where(q.TConfigDefinition.ID.Eq(def.ID)).
			Update(q.TConfigDefinition.Sort, newSort)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetNextSort 获取下一个排序值（用于新建配置时）
func GetNextSort(ctx context.Context, projectID int64, configType string) (int32, error) {
	q := query.Q

	// 获取当前最大的 sort 值
	def, err := q.TConfigDefinition.WithContext(ctx).
		Where(q.TConfigDefinition.ProjectID.Eq(projectID)).
		Where(q.TConfigDefinition.Type.Eq(configType)).
		Where(q.TConfigDefinition.IsDelete.Is(false)).
		Order(q.TConfigDefinition.Sort.Desc()).
		First()
	if err != nil {
		// 没有记录，返回初始值
		return SortGap, nil
	}

	return getSort(def) + SortGap, nil
}
