package types

// UserAppInfo 用户-应用关联信息
type UserAppInfo struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"userId"`
	AppID        int64     `json:"appId"`
	AppName      string    `json:"appName"`      // 应用名称（关联查询）
	AppCode      string    `json:"appCode"`      // 应用代码（关联查询）
	Source       string    `json:"source"`       // 注册来源
	FirstLoginAt *DateTime `json:"firstLoginAt"` // 首次登录时间
	LastLoginAt  *DateTime `json:"lastLoginAt"`  // 最后登录时间
	LoginCount   int       `json:"loginCount"`   // 登录次数
	Status       int32     `json:"status"`       // 状态
	Remark       string    `json:"remark"`       // 备注
	CreatedAt    *DateTime `json:"createdAt"`
	UpdatedAt    *DateTime `json:"updatedAt"`
}

// CreateUserAppRequest 创建用户-应用关联请求
type CreateUserAppRequest struct {
	UserID int64  `json:"userId" validate:"required"`
	AppID  int64  `json:"appId" validate:"required"`
	Source string `json:"source"` // system, oauth, register, invite
	Remark string `json:"remark"`
}

// ListUserAppsRequest 用户应用列表请求
type ListUserAppsRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	UserID   int64  `json:"userId"`
	AppID    int64  `json:"appId"`
	Source   string `json:"source"`
	Status   *int8  `json:"status"`
}

// UserAppSource 用户应用来源常量
const (
	UserAppSourceSystem   = "system"   // 系统创建
	UserAppSourceOAuth    = "oauth"    // 第三方登录
	UserAppSourceRegister = "register" // 自主注册
	UserAppSourceInvite   = "invite"   // 邀请注册
)
