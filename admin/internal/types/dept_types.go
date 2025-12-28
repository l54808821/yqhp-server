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

// DeptInfo 部门信息响应
type DeptInfo struct {
	ID        int64     `json:"id"`
	ParentID  int64     `json:"parentId"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Leader    string    `json:"leader"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	Sort      int64     `json:"sort"`
	Status    int32     `json:"status"`
	Remark    string    `json:"remark"`
	CreatedAt *DateTime `json:"createdAt"`
	UpdatedAt *DateTime `json:"updatedAt"`
}

// DeptTreeInfo 部门树响应
type DeptTreeInfo struct {
	DeptInfo
	Children []DeptTreeInfo `json:"children,omitempty"`
}
