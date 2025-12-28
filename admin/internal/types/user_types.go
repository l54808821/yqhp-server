package types

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username        string `json:"username" validate:"required"`
	Password        string `json:"password" validate:"required"`
	ConfirmPassword string `json:"confirmPassword" validate:"required"`
	Nickname        string `json:"nickname"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token    string    `json:"token"`
	UserInfo *UserInfo `json:"userInfo"`
}

// UserInfo 用户信息响应
type UserInfo struct {
	ID              int64     `json:"id"`
	Username        string    `json:"username"`
	Nickname        string    `json:"nickname"`
	Avatar          string    `json:"avatar"`
	Email           string    `json:"email"`
	Phone           string    `json:"phone"`
	Gender          int32     `json:"gender"`
	Status          int32     `json:"status"`
	DeptID          int64     `json:"deptId"`
	Platform        string    `json:"platform"`        // 用户来源平台
	PlatformUID     string    `json:"platformUid"`     // 平台唯一标识(长码)
	PlatformShortID string    `json:"platformShortId"` // 平台唯一标识(短码)
	Remark          string    `json:"remark"`
	LastLoginAt     *DateTime `json:"lastLoginAt"`
	LastLoginIP     string    `json:"lastLoginIp"`
	CreatedBy       int64     `json:"createdBy"`
	UpdatedBy       int64     `json:"updatedBy"`
	CreatedAt       *DateTime `json:"createdAt"`
	UpdatedAt       *DateTime `json:"updatedAt"`
	Roles           []RoleRef `json:"roles"`
}

// RoleRef 角色引用（简化版）
type RoleRef struct {
	ID     int64  `json:"id"`
	AppID  int64  `json:"appId"`
	Name   string `json:"name"`
	Code   string `json:"code"`
	Status int32  `json:"status"`
}

// AppRoleConfig 应用角色配置（用于用户角色分配）
type AppRoleConfig struct {
	AppID   int64   `json:"appId"`
	RoleIDs []int64 `json:"roleIds"`
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username        string          `json:"username" validate:"required"`
	Password        string          `json:"password" validate:"required"`
	Nickname        string          `json:"nickname"`
	Email           string          `json:"email"`
	Phone           string          `json:"phone"`
	Gender          int8            `json:"gender"`
	DeptID          uint            `json:"deptId"`
	Platform        string          `json:"platform"`        // 用户来源平台: system, github, wechat, feishu, dingtalk, qq, gitee
	PlatformUID     string          `json:"platformUid"`     // 平台唯一标识(长码)
	PlatformShortID string          `json:"platformShortId"` // 平台唯一标识(短码)
	AppRoles        []AppRoleConfig `json:"appRoles"`        // 按应用配置角色
	Remark          string          `json:"remark"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	ID              uint             `json:"id" validate:"required"`
	Nickname        string           `json:"nickname"`
	Avatar          string           `json:"avatar"`
	Email           string           `json:"email"`
	Phone           string           `json:"phone"`
	Gender          int8             `json:"gender"`
	DeptID          uint             `json:"deptId"`
	Status          int8             `json:"status"`
	Platform        string           `json:"platform"`        // 用户来源平台
	PlatformUID     string           `json:"platformUid"`     // 平台唯一标识(长码)
	PlatformShortID string           `json:"platformShortId"` // 平台唯一标识(短码)
	AppRoles        *[]AppRoleConfig `json:"appRoles"`        // 使用指针类型，区分"没传"和"传了空数组"
	Remark          string           `json:"remark"`
}

// ListUsersRequest 用户列表请求
type ListUsersRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Phone    string `json:"phone"`
	Status   *int8  `json:"status"`
	DeptID   uint   `json:"deptId"`
}

// UserBasicInfo 用户基本信息（用于展示组件）
type UserBasicInfo struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}
