package types

// CreateDictTypeRequest 创建字典类型请求
type CreateDictTypeRequest struct {
	Name   string `json:"name" validate:"required"`
	Code   string `json:"code" validate:"required"`
	Status int8   `json:"status"`
	Remark string `json:"remark"`
}

// UpdateDictTypeRequest 更新字典类型请求
type UpdateDictTypeRequest struct {
	ID     uint   `json:"id" validate:"required"`
	Name   string `json:"name"`
	Status int8   `json:"status"`
	Remark string `json:"remark"`
}

// ListDictTypesRequest 字典类型列表请求
type ListDictTypesRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Status   *int8  `json:"status"`
}

// CreateDictDataRequest 创建字典数据请求
type CreateDictDataRequest struct {
	TypeCode  string `json:"typeCode" validate:"required"`
	Label     string `json:"label" validate:"required"`
	Value     string `json:"value" validate:"required"`
	Sort      int    `json:"sort"`
	Status    int8   `json:"status"`
	IsDefault bool   `json:"isDefault"`
	CssClass  string `json:"cssClass"`
	ListClass string `json:"listClass"`
	Remark    string `json:"remark"`
}

// UpdateDictDataRequest 更新字典数据请求
type UpdateDictDataRequest struct {
	ID        uint   `json:"id" validate:"required"`
	Label     string `json:"label"`
	Value     string `json:"value"`
	Sort      int    `json:"sort"`
	Status    int8   `json:"status"`
	IsDefault bool   `json:"isDefault"`
	CssClass  string `json:"cssClass"`
	ListClass string `json:"listClass"`
	Remark    string `json:"remark"`
}

// ListDictDataRequest 字典数据列表请求
type ListDictDataRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	TypeCode string `json:"typeCode"`
	Label    string `json:"label"`
	Status   *int8  `json:"status"`
}
