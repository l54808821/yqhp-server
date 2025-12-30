package types

// ColumnFixedConfig 列固定配置
type ColumnFixedConfig struct {
	Key   string `json:"key"`
	Fixed string `json:"fixed,omitempty"` // left | right
}

// TableViewInfo 表格视图信息
type TableViewInfo struct {
	ID           int64               `json:"id"`
	Name         string              `json:"name"`
	IsSystem     bool                `json:"isSystem"`
	IsDefault    bool                `json:"isDefault"`
	Columns      []string            `json:"columns"`
	ColumnFixed  []ColumnFixedConfig `json:"columnFixed,omitempty"`
	SearchParams map[string]any      `json:"searchParams,omitempty"`
	Sort         int32               `json:"sort"`
	CreatedBy    int64               `json:"createdBy,omitempty"`
}

// TableViewListResponse 表格视图列表响应
type TableViewListResponse struct {
	Views []TableViewInfo `json:"views"`
}

// SaveTableViewRequest 保存表格视图请求
type SaveTableViewRequest struct {
	ID           int64               `json:"id"` // 0表示新建，>0表示更新
	TableKey     string              `json:"tableKey" validate:"required"`
	Name         string              `json:"name" validate:"required"`
	IsSystem     bool                `json:"isSystem"`
	IsDefault    bool                `json:"isDefault"`
	Columns      []string            `json:"columns" validate:"required"`
	ColumnFixed  []ColumnFixedConfig `json:"columnFixed,omitempty"`
	SearchParams map[string]any      `json:"searchParams,omitempty"`
}
