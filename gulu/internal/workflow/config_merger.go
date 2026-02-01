package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/utils"
)

// MergedConfig 合并后的配置
type MergedConfig struct {
	Domains   map[string]*DomainConfig   `json:"domains"`
	Variables map[string]interface{}     `json:"variables"`
	Databases map[string]*DatabaseConfig `json:"databases"`
	MQs       map[string]*MQConfig       `json:"mqs"`
}

// DomainConfig 域名配置
type DomainConfig struct {
	Code    string            `json:"code"`
	BaseURL string            `json:"base_url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Code     string `json:"code"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Options  string `json:"options,omitempty"`
}

// MQConfig MQ 配置
type MQConfig struct {
	Code     string `json:"code"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	VHost    string `json:"vhost,omitempty"`
	Options  string `json:"options,omitempty"`
}

// ConfigMerger 配置合并器
type ConfigMerger struct {
	ctx   context.Context
	envID int64
}

// NewConfigMerger 创建配置合并器
func NewConfigMerger(ctx context.Context, envID int64) *ConfigMerger {
	return &ConfigMerger{
		ctx:   ctx,
		envID: envID,
	}
}

// Merge 合并所有配置
func (m *ConfigMerger) Merge() (*MergedConfig, error) {
	config := &MergedConfig{
		Domains:   make(map[string]*DomainConfig),
		Variables: make(map[string]interface{}),
		Databases: make(map[string]*DatabaseConfig),
		MQs:       make(map[string]*MQConfig),
	}

	q := query.Q

	// 获取环境信息
	env, err := q.TEnv.WithContext(m.ctx).
		Where(q.TEnv.ID.Eq(m.envID)).
		Where(q.TEnv.IsDelete.Is(false)).
		First()
	if err != nil {
		return nil, errors.New("环境不存在")
	}

	// 获取项目的所有启用的配置定义
	definitions, err := q.TConfigDefinition.WithContext(m.ctx).
		Where(q.TConfigDefinition.ProjectID.Eq(env.ProjectID)).
		Where(q.TConfigDefinition.IsDelete.Is(false)).
		Where(q.TConfigDefinition.Status.Eq(1)).
		Find()
	if err != nil {
		return nil, err
	}

	if len(definitions) == 0 {
		return config, nil
	}

	// 获取环境的配置值
	codes := make([]string, len(definitions))
	for i, def := range definitions {
		codes[i] = def.Code
	}

	configs, err := q.TConfig.WithContext(m.ctx).
		Where(q.TConfig.EnvID.Eq(m.envID)).
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

	// 处理各类型配置
	for _, def := range definitions {
		cfg := configMap[def.Code]
		if cfg == nil {
			continue
		}

		switch def.Type {
		case model.ConfigTypeDomain:
			m.processDomain(config, def, cfg)
		case model.ConfigTypeVariable:
			m.processVariable(config, def, cfg)
		case model.ConfigTypeDatabase:
			m.processDatabase(config, def, cfg)
		case model.ConfigTypeMQ:
			m.processMQ(config, def, cfg)
		}
	}

	return config, nil
}

// processDomain 处理域名配置
func (m *ConfigMerger) processDomain(config *MergedConfig, def *model.TConfigDefinition, cfg *model.TConfig) {
	dc := &DomainConfig{
		Code: def.Key,
	}

	var value map[string]interface{}
	if err := json.Unmarshal([]byte(cfg.Value), &value); err == nil {
		if baseURL, ok := value["base_url"].(string); ok {
			dc.BaseURL = baseURL
		}
		if headers, ok := value["headers"].([]interface{}); ok && len(headers) > 0 {
			dc.Headers = make(map[string]string)
			for _, h := range headers {
				if hMap, ok := h.(map[string]interface{}); ok {
					key, _ := hMap["key"].(string)
					val, _ := hMap["value"].(string)
					if key != "" {
						dc.Headers[key] = val
					}
				}
			}
		}
	}

	config.Domains[def.Key] = dc
}

// processVariable 处理变量配置
func (m *ConfigMerger) processVariable(config *MergedConfig, def *model.TConfigDefinition, cfg *model.TConfig) {
	var varType string
	var isSensitive bool

	// 从 extra 获取变量类型和敏感标记
	if def.Extra != nil && *def.Extra != "" {
		var extra map[string]interface{}
		if err := json.Unmarshal([]byte(*def.Extra), &extra); err == nil {
			if vt, ok := extra["var_type"].(string); ok {
				varType = vt
			}
			if is, ok := extra["is_sensitive"].(bool); ok {
				isSensitive = is
			}
		}
	}

	// 从配置值获取值
	var value string
	var valueObj map[string]interface{}
	if err := json.Unmarshal([]byte(cfg.Value), &valueObj); err == nil {
		if v, ok := valueObj["value"].(string); ok {
			value = v
		}
	}

	// 敏感数据需要解密
	if isSensitive && value != "" && value != "******" {
		decrypted, err := utils.Decrypt(value)
		if err == nil {
			value = decrypted
		}
	}

	// 使用 Name 作为变量名（Key 是唯一 ID，Name 是用户定义的变量名）
	varName := def.Name

	// 根据类型转换值
	switch varType {
	case "number":
		var num float64
		if err := json.Unmarshal([]byte(value), &num); err == nil {
			config.Variables[varName] = num
		} else {
			config.Variables[varName] = value
		}
	case "boolean":
		config.Variables[varName] = strings.ToLower(value) == "true"
	case "json":
		var jsonVal interface{}
		if err := json.Unmarshal([]byte(value), &jsonVal); err == nil {
			config.Variables[varName] = jsonVal
		} else {
			config.Variables[varName] = value
		}
	default:
		config.Variables[varName] = value
	}
}

// processDatabase 处理数据库配置
func (m *ConfigMerger) processDatabase(config *MergedConfig, def *model.TConfigDefinition, cfg *model.TConfig) {
	dbc := &DatabaseConfig{
		Code: def.Key,
	}

	// 从 extra 获取数据库类型
	if def.Extra != nil && *def.Extra != "" {
		var extra map[string]interface{}
		if err := json.Unmarshal([]byte(*def.Extra), &extra); err == nil {
			if dbType, ok := extra["db_type"].(string); ok {
				dbc.Type = dbType
			}
		}
	}

	// 从配置值获取连接信息
	var value map[string]interface{}
	if err := json.Unmarshal([]byte(cfg.Value), &value); err == nil {
		if host, ok := value["host"].(string); ok {
			dbc.Host = host
		}
		if port, ok := value["port"].(float64); ok {
			dbc.Port = int(port)
		}
		if database, ok := value["database"].(string); ok {
			dbc.Database = database
		}
		if username, ok := value["username"].(string); ok {
			dbc.Username = username
		}
		if password, ok := value["password"].(string); ok {
			// 解密密码
			decrypted, err := utils.Decrypt(password)
			if err == nil {
				dbc.Password = decrypted
			} else {
				dbc.Password = password
			}
		}
		if options, ok := value["options"].(string); ok {
			dbc.Options = options
		}
	}

	config.Databases[def.Key] = dbc
}

// processMQ 处理 MQ 配置
func (m *ConfigMerger) processMQ(config *MergedConfig, def *model.TConfigDefinition, cfg *model.TConfig) {
	mqc := &MQConfig{
		Code: def.Key,
	}

	// 从 extra 获取 MQ 类型
	if def.Extra != nil && *def.Extra != "" {
		var extra map[string]interface{}
		if err := json.Unmarshal([]byte(*def.Extra), &extra); err == nil {
			if mqType, ok := extra["mq_type"].(string); ok {
				mqc.Type = mqType
			}
		}
	}

	// 从配置值获取连接信息
	var value map[string]interface{}
	if err := json.Unmarshal([]byte(cfg.Value), &value); err == nil {
		if host, ok := value["host"].(string); ok {
			mqc.Host = host
		}
		if port, ok := value["port"].(float64); ok {
			mqc.Port = int(port)
		}
		if username, ok := value["username"].(string); ok {
			mqc.Username = username
		}
		if password, ok := value["password"].(string); ok {
			// 解密密码
			decrypted, err := utils.Decrypt(password)
			if err == nil {
				mqc.Password = decrypted
			} else {
				mqc.Password = password
			}
		}
		if vhost, ok := value["vhost"].(string); ok {
			mqc.VHost = vhost
		}
		if options, ok := value["options"].(string); ok {
			mqc.Options = options
		}
	}

	config.MQs[def.Key] = mqc
}

// MergeToWorkflow 将配置合并到工作流定义
func (m *ConfigMerger) MergeToWorkflow(def *WorkflowDefinition) (*WorkflowDefinition, error) {
	if def == nil {
		return nil, errors.New("工作流定义不能为空")
	}

	config, err := m.Merge()
	if err != nil {
		return nil, err
	}

	// 创建新的工作流定义（不修改原始定义）
	newDef := &WorkflowDefinition{
		Name:        def.Name,
		Description: def.Description,
		Version:     def.Version,
		Variables:   make(map[string]interface{}),
		Steps:       def.Steps,
	}

	// 合并变量（环境变量优先）
	for k, v := range def.Variables {
		newDef.Variables[k] = v
	}
	for k, v := range config.Variables {
		newDef.Variables[k] = v
	}

	// 添加域名、数据库、MQ 配置到变量
	newDef.Variables["__domains__"] = config.Domains
	newDef.Variables["__databases__"] = config.Databases
	newDef.Variables["__mqs__"] = config.MQs

	return newDef, nil
}

// ToJSON 将合并配置转换为 JSON
func (c *MergedConfig) ToJSON() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
