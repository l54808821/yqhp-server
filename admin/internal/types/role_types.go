package types

// CreateRoleRequest 创建角色请求
type CreateRoleRequest struct {
	Name        string `json:"name" validate:"required"`
	Code        string `json:"code" validate:"required"`
	Sort        int    `json:"sort"`
	Status      int8   `json:"status"`
	Remark      string `json:"remark"`
	ResourceIDs []uint `json:"resourceIds"`
}

// UpdateRoleRequest 更新角色请求
type UpdateRoleRequest struct {
	ID          uint   `json:"id" validate:"required"`
	Name        string `json:"name"`
	Sort        int    `json:"sort"`
	Status      int8   `json:"status"`
	Remark      string `json:"remark"`
	ResourceIDs []uint `json:"resourceIds"`
}

// ListRolesRequest 角色列表请求
type ListRolesRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Status   *int8  `json:"status"`
}
