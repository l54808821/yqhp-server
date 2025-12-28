package types

// CreateRoleRequest 创建角色请求
type CreateRoleRequest struct {
	AppID       uint   `json:"appId" validate:"required"`
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
	AppID    uint   `json:"appId"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Status   *int8  `json:"status"`
}

// RoleInfo 角色信息响应
type RoleInfo struct {
	ID        int64     `json:"id"`
	AppID     int64     `json:"appId"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Sort      int64     `json:"sort"`
	Status    int32     `json:"status"`
	Remark    string    `json:"remark"`
	CreatedBy int64     `json:"createdBy"`
	UpdatedBy int64     `json:"updatedBy"`
	CreatedAt *DateTime `json:"createdAt"`
	UpdatedAt *DateTime `json:"updatedAt"`
}
