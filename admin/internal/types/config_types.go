package types

// CreateConfigRequest 创建配置请求
type CreateConfigRequest struct {
	Name    string `json:"name" validate:"required"`
	Key     string `json:"key" validate:"required"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	IsBuilt bool   `json:"isBuilt"`
	Remark  string `json:"remark"`
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	ID     uint   `json:"id" validate:"required"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	Type   string `json:"type"`
	Remark string `json:"remark"`
}

// ListConfigsRequest 配置列表请求
type ListConfigsRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Key      string `json:"key"`
}

// ConfigInfo 配置响应
type ConfigInfo struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Type      string    `json:"type"`
	IsBuilt   bool      `json:"isBuilt"`
	Remark    string    `json:"remark"`
	CreatedAt *DateTime `json:"createdAt"`
	UpdatedAt *DateTime `json:"updatedAt"`
}
