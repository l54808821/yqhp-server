package model

import "yqhp/common/types"

// UserToken 用户令牌模型
type UserToken struct {
	BaseModel
	UserID       uint           `gorm:"index" json:"userId"`
	Token        string         `gorm:"size:500;uniqueIndex" json:"token"`
	Device       string         `gorm:"size:50" json:"device"`   // 设备类型: pc, mobile, tablet
	Platform     string         `gorm:"size:50" json:"platform"` // 平台: web, app, wechat
	IP           string         `gorm:"size:50" json:"ip"`
	UserAgent    string         `gorm:"size:500" json:"userAgent"`
	ExpireAt     types.DateTime `json:"expireAt"`
	LastActiveAt types.DateTime `json:"lastActiveAt"`
}

// TableName 表名
func (UserToken) TableName() string {
	return "sys_user_token"
}

// LoginLog 登录日志模型
type LoginLog struct {
	BaseModel
	UserID    uint   `gorm:"index" json:"userId"`
	Username  string `gorm:"size:50" json:"username"`
	IP        string `gorm:"size:50" json:"ip"`
	Location  string `gorm:"size:100" json:"location"`
	Browser   string `gorm:"size:100" json:"browser"`
	OS        string `gorm:"size:100" json:"os"`
	Status    int8   `gorm:"default:1" json:"status"` // 0:失败 1:成功
	Message   string `gorm:"size:255" json:"message"`
	LoginType string `gorm:"size:50" json:"loginType"` // password, oauth
}

// TableName 表名
func (LoginLog) TableName() string {
	return "sys_login_log"
}

// OperationLog 操作日志模型
type OperationLog struct {
	BaseModel
	UserID    uint   `gorm:"index" json:"userId"`
	Username  string `gorm:"size:50" json:"username"`
	Module    string `gorm:"size:50" json:"module"`   // 模块名
	Action    string `gorm:"size:50" json:"action"`   // 操作类型
	Method    string `gorm:"size:10" json:"method"`   // 请求方法
	Path      string `gorm:"size:255" json:"path"`    // 请求路径
	IP        string `gorm:"size:50" json:"ip"`
	UserAgent string `gorm:"size:500" json:"userAgent"`
	Params    string `gorm:"type:text" json:"params"` // 请求参数
	Result    string `gorm:"type:text" json:"result"` // 响应结果
	Status    int8   `gorm:"default:1" json:"status"` // 0:失败 1:成功
	Duration  int64  `json:"duration"`                // 耗时(ms)
	ErrorMsg  string `gorm:"size:500" json:"errorMsg"`
}

// TableName 表名
func (OperationLog) TableName() string {
	return "sys_operation_log"
}
