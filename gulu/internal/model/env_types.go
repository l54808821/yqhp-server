package model

// DomainItem 域名配置项（存储在 t_env.domains 字段中）
type DomainItem struct {
	Code        string       `json:"code"`                  // 域名代码，用于工作流中引用
	Name        string       `json:"name"`                  // 域名名称
	BaseURL     string       `json:"base_url"`              // 基础URL
	Headers     []HeaderItem `json:"headers,omitempty"`     // 公共请求头
	Description string       `json:"description,omitempty"` // 描述
	Sort        int          `json:"sort"`                  // 排序
	Status      int          `json:"status"`                // 状态: 1-启用 0-禁用
}

// HeaderItem 请求头项
type HeaderItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// VarItem 变量配置项（存储在 t_env.vars 字段中）
type VarItem struct {
	Key         string `json:"key"`                   // 变量键
	Name        string `json:"name"`                  // 变量名称
	Value       string `json:"value"`                 // 变量值
	Type        string `json:"type"`                  // 变量类型: string, number, boolean, json
	IsSensitive bool   `json:"is_sensitive"`          // 是否敏感数据
	Description string `json:"description,omitempty"` // 描述
}
