package logic

import (
	"yqhp/gulu/internal/client"
	"yqhp/gulu/internal/svc"
)

// ResourceLogic 资源逻辑
type ResourceLogic struct {
	adminClient *client.AdminClient
	appCode     string
}

// NewResourceLogic 创建资源逻辑
func NewResourceLogic() *ResourceLogic {
	return &ResourceLogic{
		adminClient: client.NewAdminClient(),
		appCode:     svc.Ctx.Config.Gulu.AppCode,
	}
}

// GetUserMenus 获取用户菜单（通过 Admin API，按 gulu 应用过滤）
func (l *ResourceLogic) GetUserMenus(token string) ([]client.MenuResp, error) {
	return l.adminClient.GetUserMenus(token, l.appCode)
}

// GetUserCodes 获取用户权限码（通过 Admin API，按 gulu 应用过滤）
func (l *ResourceLogic) GetUserCodes(token string) ([]string, error) {
	return l.adminClient.GetUserPermissionCodes(token, l.appCode)
}
