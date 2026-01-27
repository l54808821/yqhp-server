package workflow

import (
	"encoding/json"
	"errors"
	"strings"

	"yqhp/gulu/internal/model"
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
	env       *model.TEnv              // 环境配置（包含 domains_json 和 vars_json）
	databases []*model.TDatabaseConfig // 数据库配置（仍然从独立表获取）
	mqs       []*model.TMqConfig       // MQ配置（仍然从独立表获取）
}

// NewConfigMerger 创建配置合并器
func NewConfigMerger() *ConfigMerger {
	return &ConfigMerger{}
}

// SetEnv 设置环境配置（新方法，推荐使用）
func (m *ConfigMerger) SetEnv(env *model.TEnv) *ConfigMerger {
	m.env = env
	return m
}

// SetDatabases 设置数据库配置
func (m *ConfigMerger) SetDatabases(databases []*model.TDatabaseConfig) *ConfigMerger {
	m.databases = databases
	return m
}

// SetMQs 设置 MQ 配置
func (m *ConfigMerger) SetMQs(mqs []*model.TMqConfig) *ConfigMerger {
	m.mqs = mqs
	return m
}

// Merge 合并所有配置
func (m *ConfigMerger) Merge() (*MergedConfig, error) {
	config := &MergedConfig{
		Domains:   make(map[string]*DomainConfig),
		Variables: make(map[string]interface{}),
		Databases: make(map[string]*DatabaseConfig),
		MQs:       make(map[string]*MQConfig),
	}

	// 从 TEnv 的 JSON 字段合并域名配置
	if m.env != nil && m.env.Domains != nil && *m.env.Domains != "" {
		var domains []model.DomainItem
		if err := json.Unmarshal([]byte(*m.env.Domains), &domains); err == nil {
			for _, domain := range domains {
				if domain.Status != 1 {
					continue
				}
				dc := &DomainConfig{
					Code:    domain.Code,
					BaseURL: domain.BaseURL,
				}
				// 转换 Headers
				if len(domain.Headers) > 0 {
					dc.Headers = make(map[string]string)
					for _, h := range domain.Headers {
						dc.Headers[h.Key] = h.Value
					}
				}
				config.Domains[domain.Code] = dc
			}
		}
	}

	// 从 TEnv 的 JSON 字段合并变量配置
	if m.env != nil && m.env.Vars != nil && *m.env.Vars != "" {
		var vars []model.VarItem
		if err := json.Unmarshal([]byte(*m.env.Vars), &vars); err == nil {
			for _, v := range vars {
				value := v.Value
				// 敏感数据需要解密
				if v.IsSensitive && value != "" {
					decrypted, err := utils.Decrypt(value)
					if err == nil {
						value = decrypted
					}
				}
				// 根据类型转换值
				switch v.Type {
				case "number":
					var num float64
					if err := json.Unmarshal([]byte(value), &num); err == nil {
						config.Variables[v.Key] = num
					} else {
						config.Variables[v.Key] = value
					}
				case "boolean":
					config.Variables[v.Key] = strings.ToLower(value) == "true"
				case "json":
					var jsonVal interface{}
					if err := json.Unmarshal([]byte(value), &jsonVal); err == nil {
						config.Variables[v.Key] = jsonVal
					} else {
						config.Variables[v.Key] = value
					}
				default:
					config.Variables[v.Key] = value
				}
			}
		}
	}

	// 合并数据库配置（仍然从独立表获取）
	for _, db := range m.databases {
		if db.Status != nil && *db.Status != 1 {
			continue
		}
		dbc := &DatabaseConfig{
			Code: db.Code,
			Type: db.Type,
			Host: db.Host,
			Port: int(db.Port),
		}
		if db.Database != nil {
			dbc.Database = *db.Database
		}
		if db.Username != nil {
			dbc.Username = *db.Username
		}
		if db.Password != nil {
			dbc.Password = *db.Password
		}
		if db.Options != nil {
			dbc.Options = *db.Options
		}
		config.Databases[db.Code] = dbc
	}

	// 合并 MQ 配置（仍然从独立表获取）
	for _, mq := range m.mqs {
		if mq.Status != nil && *mq.Status != 1 {
			continue
		}
		mqc := &MQConfig{
			Code: mq.Code,
			Type: mq.Type,
			Host: mq.Host,
			Port: int(mq.Port),
		}
		if mq.Username != nil {
			mqc.Username = *mq.Username
		}
		if mq.Password != nil {
			mqc.Password = *mq.Password
		}
		if mq.Vhost != nil {
			mqc.VHost = *mq.Vhost
		}
		if mq.Options != nil {
			mqc.Options = *mq.Options
		}
		config.MQs[mq.Code] = mqc
	}

	return config, nil
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
