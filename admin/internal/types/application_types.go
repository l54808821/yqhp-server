package types

// CreateApplicationRequest 创建应用请求
type CreateApplicationRequest struct {
	Name        string `json:"name" validate:"required"`
	Code        string `json:"code" validate:"required"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Sort        int    `json:"sort"`
	Status      int8   `json:"status"`
}

// UpdateApplicationRequest 更新应用请求
type UpdateApplicationRequest struct {
	ID          uint   `json:"id" validate:"required"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Sort        int    `json:"sort"`
	Status      int8   `json:"status"`
}

// ListApplicationsRequest 应用列表请求
type ListApplicationsRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Status   *int8  `json:"status"`
}
