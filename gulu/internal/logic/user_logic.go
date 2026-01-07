package logic

import (
	"yqhp/gulu/internal/client"
	"yqhp/gulu/internal/svc"
)

// UserLogic 用户逻辑
type UserLogic struct {
	adminClient *client.AdminClient
}

// NewUserLogic 创建用户逻辑
func NewUserLogic() *UserLogic {
	return &UserLogic{
		adminClient: client.NewAdminClient(),
	}
}

// GetUserInfo 获取用户信息（通过 Admin API）
func (l *UserLogic) GetUserInfo(token string) (*client.UserInfoResp, error) {
	return l.adminClient.GetUserInfo(token)
}

// GetAppCode 获取应用编码
func (l *UserLogic) GetAppCode() string {
	return svc.Ctx.Config.Gulu.AppCode
}
