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
	ErrConfigDefinitionNotFound  = errors.New("配置定义不存在")
	ErrConfigDefinitionDuplicate = errors.New("配置定义的key已存在")
)

// CreateConfigDefinitionReq 创建配置定义请求
type CreateConfigDefinitionReq struct {
	ProjectID   int64  `json:"project_id"`
	Type        string `json:"type"`        // domain/variable/database/mq
	Key         string `json:"key"`         // 用户定义的标识
	Name        string `json:"name"`        // 显示名称
	Description string `json:"description"` // 描述
	Extra       any    `json:"extra"`       // 类型特有属性
	Sort        int32  `json:"sort"`
	Status      int32  `json:"status"`
}

// UpdateConfigDefinitionReq 更新配置定义请求
type UpdateConfigDefinitionReq struct {
	Key         *string `json:"key"`
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

	// 检查 key 是否已存在
	existing, _ := q.TConfigDefinition.WithContext(ctx).
		Where(q.TConfigDefinition.ProjectID.Eq(req.ProjectID)).
		Where(q.TConfigDefinition.Type.Eq(req.Type)).
		Where(q.TConfigDefinition.Key.Eq(req.Key)).
		Where(q.TConfigDefinition.IsDelete.Is(false)).
		First()
	if existing != nil {
		return nil, ErrConfigDefinitionDuplicate
	}

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

	isDelete := false
	definition := &model.TConfigDefinition{
		ProjectID:   req.ProjectID,
		Type:        req.Type,
		Code:        code,
		Key:         req.Key,
		Name:        req.Name,
		Description: &req.Description,
		Extra:       extraStr,
		Sort:        &req.Sort,
		Status:      &req.Status,
		IsDelete:    &isDelete,
	}

	// 开启事务
	err := q.Transaction(func(tx *query.Query) error {
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
	definition, err := GetConfigDefinitionByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	// 如果修改了 key，检查是否重复
	if req.Key != nil && *req.Key != definition.Key {
		existing, _ := q.TConfigDefinition.WithContext(ctx).
			Where(q.TConfigDefinition.ProjectID.Eq(definition.ProjectID)).
			Where(q.TConfigDefinition.Type.Eq(definition.Type)).
			Where(q.TConfigDefinition.Key.Eq(*req.Key)).
			Where(q.TConfigDefinition.Code.Neq(code)).
			Where(q.TConfigDefinition.IsDelete.Is(false)).
			First()
		if existing != nil {
			return nil, ErrConfigDefinitionDuplicate
		}
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if req.Key != nil {
		updates["key"] = *req.Key
	}
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
