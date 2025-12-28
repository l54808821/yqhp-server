package types

// CreateDeptRequest 创建部门请求
type CreateDeptRequest struct {
	ParentID uint   `json:"parentId"`
	Name     string `json:"name" validate:"required"`
	Code     string `json:"code"`
	Leader   string `json:"leader"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Sort     int    `json:"sort"`
	Status   int8   `json:"status"`
	Remark   string `json:"remark"`
}

// UpdateDeptRequest 更新部门请求
type UpdateDeptRequest struct {
	ID       uint   `json:"id" validate:"required"`
	ParentID uint   `json:"parentId"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Leader   string `json:"leader"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Sort     int    `json:"sort"`
	Status   int8   `json:"status"`
	Remark   string `json:"remark"`
}
