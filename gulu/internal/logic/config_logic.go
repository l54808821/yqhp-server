package logic

import (
	"context"
	"encoding/json"
	"errors"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/utils"
)

var (
	ErrConfigNotFound = errors.New("配置不存在")
)

// ConfigItem 配置项（包含定义和值）
type ConfigItem struct {
	// 来自配置定义
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Extra       any    `json:"extra"`
	Sort        int32  `json:"sort"`
	Status      int32  `json:"status"`

	// 来自配置值
	Value any `json:"value"`
}

// GetConfigsByEnv 获取环境的配置列表
// 返回配置定义和值的组合
func GetConfigsByEnv(ctx context.Context, envID int64, configType string) ([]*ConfigItem, error) {
	q := query.Q

	// 获取环境信息
	env, err := q.TEnv.WithContext(ctx).
		Where(q.TEnv.ID.Eq(envID)).
		Where(q.TEnv.IsDelete.Is(false)).
		First()
	if err != nil {
		return nil, errors.New("环境不存在")
	}

	// 获取项目的配置定义
	defQuery := q.TConfigDefinition.WithContext(ctx).
		Where(q.TConfigDefinition.ProjectID.Eq(env.ProjectID)).
		Where(q.TConfigDefinition.IsDelete.Is(false))
	if configType != "" {
		defQuery = defQuery.Where(q.TConfigDefinition.Type.Eq(configType))
	}
	definitions, err := defQuery.Order(q.TConfigDefinition.Sort, q.TConfigDefinition.ID).Find()
	if err != nil {
		return nil, err
	}

	if len(definitions) == 0 {
		return []*ConfigItem{}, nil
	}

	// 获取环境的配置值
	codes := make([]string, len(definitions))
	for i, def := range definitions {
		codes[i] = def.Code
	}

	configs, err := q.TConfig.WithContext(ctx).
		Where(q.TConfig.EnvID.Eq(envID)).
		Where(q.TConfig.Code.In(codes...)).
		Find()
	if err != nil {
		return nil, err
	}

	// 构建 code -> config 的映射
	configMap := make(map[string]*model.TConfig)
	for _, cfg := range configs {
		configMap[cfg.Code] = cfg
	}

	// 组合结果
	items := make([]*ConfigItem, 0, len(definitions))
	for _, def := range definitions {
		item := &ConfigItem{
			Code:        def.Code,
			Name:        def.Name,
			Description: "",
			Type:        def.Type,
			Sort:        0,
			Status:      1,
		}

		if def.Description != nil {
			item.Description = *def.Description
		}
		if def.Sort != nil {
			item.Sort = *def.Sort
		}
		if def.Status != nil {
			item.Status = *def.Status
		}

		// 解析 extra
		if def.Extra != nil && *def.Extra != "" {
			var extra any
			if err := json.Unmarshal([]byte(*def.Extra), &extra); err == nil {
				item.Extra = extra
			}
		}

		// 解析配置值
		if cfg, ok := configMap[def.Code]; ok {
			var value any
			if err := json.Unmarshal([]byte(cfg.Value), &value); err == nil {
				// 处理敏感变量的脱敏显示
				if def.Type == model.ConfigTypeVariable {
					value = maskSensitiveValue(def, value)
				}
				item.Value = value
			}
		} else {
			// 如果没有配置值，使用默认值
			var defaultValue any
			json.Unmarshal([]byte(getDefaultValue(def.Type)), &defaultValue)
			item.Value = defaultValue
		}

		items = append(items, item)
	}

	return items, nil
}

// maskSensitiveValue 对敏感变量值进行脱敏
func maskSensitiveValue(def *model.TConfigDefinition, value any) any {
	if def.Extra == nil {
		return value
	}

	var extra model.VariableExtra
	if err := json.Unmarshal([]byte(*def.Extra), &extra); err != nil {
		return value
	}

	if !extra.IsSensitive {
		return value
	}

	// 脱敏处理
	if valueMap, ok := value.(map[string]interface{}); ok {
		valueMap["value"] = "******"
		return valueMap
	}

	return value
}

// UpdateConfigValue 更新配置值
type UpdateConfigValueReq struct {
	Value any `json:"value"`
}

// UpdateConfigValue 更新单个配置的值
func UpdateConfigValue(ctx context.Context, envID int64, code string, req *UpdateConfigValueReq) error {
	q := query.Q

	// 获取环境信息
	env, err := q.TEnv.WithContext(ctx).
		Where(q.TEnv.ID.Eq(envID)).
		Where(q.TEnv.IsDelete.Is(false)).
		First()
	if err != nil {
		return errors.New("环境不存在")
	}

	// 获取配置定义
	definition, err := GetConfigDefinitionByCode(ctx, code)
	if err != nil {
		return err
	}

	// 序列化值
	valueBytes, err := json.Marshal(req.Value)
	if err != nil {
		return err
	}
	valueStr := string(valueBytes)

	// 对敏感变量进行加密
	if definition.Type == model.ConfigTypeVariable {
		valueStr, err = encryptSensitiveValue(definition, valueStr)
		if err != nil {
			return err
		}
	}

	// 检查配置是否存在
	existing, _ := q.TConfig.WithContext(ctx).
		Where(q.TConfig.EnvID.Eq(envID)).
		Where(q.TConfig.Code.Eq(code)).
		First()

	if existing != nil {
		// 更新
		_, err = q.TConfig.WithContext(ctx).
			Where(q.TConfig.ID.Eq(existing.ID)).
			Update(q.TConfig.Value, valueStr)
	} else {
		// 创建
		err = q.TConfig.WithContext(ctx).Create(&model.TConfig{
			ProjectID: env.ProjectID,
			EnvID:     envID,
			Type:      definition.Type,
			Code:      code,
			Value:     valueStr,
		})
	}

	return err
}

// encryptSensitiveValue 加密敏感变量值
func encryptSensitiveValue(def *model.TConfigDefinition, valueStr string) (string, error) {
	if def.Extra == nil {
		return valueStr, nil
	}

	var extra model.VariableExtra
	if err := json.Unmarshal([]byte(*def.Extra), &extra); err != nil {
		return valueStr, nil
	}

	if !extra.IsSensitive {
		return valueStr, nil
	}

	// 解析值
	var value model.VariableValue
	if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
		return valueStr, nil
	}

	// 如果值是脱敏占位符，不更新
	if value.Value == "******" {
		return valueStr, nil
	}

	// 加密
	encrypted, err := utils.Encrypt(value.Value)
	if err != nil {
		return valueStr, err
	}

	value.Value = encrypted
	newValueBytes, err := json.Marshal(value)
	if err != nil {
		return valueStr, err
	}

	return string(newValueBytes), nil
}

// BatchUpdateConfigValues 批量更新配置值
type BatchUpdateConfigValuesReq struct {
	Items []struct {
		Code  string `json:"code"`
		Value any    `json:"value"`
	} `json:"items"`
}

// BatchUpdateConfigValues 批量更新配置值
func BatchUpdateConfigValues(ctx context.Context, envID int64, req *BatchUpdateConfigValuesReq) error {
	for _, item := range req.Items {
		if err := UpdateConfigValue(ctx, envID, item.Code, &UpdateConfigValueReq{Value: item.Value}); err != nil {
			return err
		}
	}
	return nil
}

// GetConfigsForExecution 获取环境的配置用于执行
// 返回解密后的完整配置
func GetConfigsForExecution(ctx context.Context, envID int64) (map[string][]*ConfigItem, error) {
	result := make(map[string][]*ConfigItem)

	// 获取所有类型的配置
	for _, configType := range []string{model.ConfigTypeDomain, model.ConfigTypeVariable, model.ConfigTypeDatabase, model.ConfigTypeMQ} {
		items, err := getConfigsForExecutionByType(ctx, envID, configType)
		if err != nil {
			return nil, err
		}
		result[configType] = items
	}

	return result, nil
}

// getConfigsForExecutionByType 获取指定类型的配置用于执行
func getConfigsForExecutionByType(ctx context.Context, envID int64, configType string) ([]*ConfigItem, error) {
	q := query.Q

	// 获取环境信息
	env, err := q.TEnv.WithContext(ctx).
		Where(q.TEnv.ID.Eq(envID)).
		Where(q.TEnv.IsDelete.Is(false)).
		First()
	if err != nil {
		return nil, errors.New("环境不存在")
	}

	// 获取配置定义
	definitions, err := q.TConfigDefinition.WithContext(ctx).
		Where(q.TConfigDefinition.ProjectID.Eq(env.ProjectID)).
		Where(q.TConfigDefinition.Type.Eq(configType)).
		Where(q.TConfigDefinition.IsDelete.Is(false)).
		Where(q.TConfigDefinition.Status.Eq(1)).
		Order(q.TConfigDefinition.Sort, q.TConfigDefinition.ID).
		Find()
	if err != nil {
		return nil, err
	}

	if len(definitions) == 0 {
		return []*ConfigItem{}, nil
	}

	// 获取配置值
	codes := make([]string, len(definitions))
	for i, def := range definitions {
		codes[i] = def.Code
	}

	configs, err := q.TConfig.WithContext(ctx).
		Where(q.TConfig.EnvID.Eq(envID)).
		Where(q.TConfig.Code.In(codes...)).
		Find()
	if err != nil {
		return nil, err
	}

	configMap := make(map[string]*model.TConfig)
	for _, cfg := range configs {
		configMap[cfg.Code] = cfg
	}

	// 组合结果（解密敏感值）
	items := make([]*ConfigItem, 0, len(definitions))
	for _, def := range definitions {
		item := &ConfigItem{
			Code: def.Code,
			Name: def.Name,
			Type: def.Type,
		}

		if def.Description != nil {
			item.Description = *def.Description
		}
		if def.Sort != nil {
			item.Sort = *def.Sort
		}
		if def.Status != nil {
			item.Status = *def.Status
		}

		// 解析 extra
		if def.Extra != nil && *def.Extra != "" {
			var extra any
			if err := json.Unmarshal([]byte(*def.Extra), &extra); err == nil {
				item.Extra = extra
			}
		}

		// 解析配置值并解密
		if cfg, ok := configMap[def.Code]; ok {
			var value any
			if err := json.Unmarshal([]byte(cfg.Value), &value); err == nil {
				// 解密敏感变量
				if def.Type == model.ConfigTypeVariable {
					value = decryptSensitiveValue(def, value)
				}
				item.Value = value
			}
		}

		items = append(items, item)
	}

	return items, nil
}

// decryptSensitiveValue 解密敏感变量值
func decryptSensitiveValue(def *model.TConfigDefinition, value any) any {
	if def.Extra == nil {
		return value
	}

	var extra model.VariableExtra
	if err := json.Unmarshal([]byte(*def.Extra), &extra); err != nil {
		return value
	}

	if !extra.IsSensitive {
		return value
	}

	// 解密
	if valueMap, ok := value.(map[string]interface{}); ok {
		if encryptedValue, ok := valueMap["value"].(string); ok {
			decrypted, err := utils.Decrypt(encryptedValue)
			if err == nil {
				valueMap["value"] = decrypted
			}
		}
		return valueMap
	}

	return value
}
