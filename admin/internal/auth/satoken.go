package auth

import (
	"fmt"
	"time"
	"yqhp/admin/internal/config"

	"github.com/click33/sa-token-go/core"
	"github.com/click33/sa-token-go/storage/memory"
	satokenRedis "github.com/click33/sa-token-go/storage/redis"
	"github.com/click33/sa-token-go/stputil"
)

var manager *core.Manager

// InitSaToken 初始化SaToken
func InitSaToken(cfg *config.Config) error {
	var storage core.Storage
	var err error

	// 根据配置选择存储方式
	// 如果Redis配置有效，使用Redis存储；否则使用内存存储
	if cfg.Redis.Host != "" && cfg.Redis.Port > 0 {
		// 构建Redis URL: redis://:password@host:port/db
		var redisURL string
		if cfg.Redis.Password != "" {
			redisURL = fmt.Sprintf("redis://:%s@%s:%d/%d", cfg.Redis.Password, cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.DB)
		} else {
			redisURL = fmt.Sprintf("redis://%s:%d/%d", cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.DB)
		}
		// 使用Redis存储（持久化，服务重启后token仍然有效）
		storage, err = satokenRedis.NewStorage(redisURL)
		if err != nil {
			fmt.Printf("[SaToken] Redis存储初始化失败: %v，降级使用内存存储\n", err)
			storage = memory.NewStorage()
			fmt.Println("[SaToken] 使用内存存储（警告：服务重启后token会丢失）")
		} else {
			fmt.Println("[SaToken] 使用Redis存储")
		}
	} else {
		// 使用内存存储（服务重启后token会丢失）
		storage = memory.NewStorage()
		fmt.Println("[SaToken] 使用内存存储（警告：服务重启后token会丢失）")
	}

	// 使用Builder模式创建Manager
	manager = core.NewBuilder().
		Storage(storage).
		TokenName(cfg.SaToken.TokenName).
		Timeout(cfg.SaToken.Timeout).
		ActiveTimeout(cfg.SaToken.ActiveTimeout).
		IsConcurrent(cfg.SaToken.IsConcurrent).
		IsShare(cfg.SaToken.IsShare).
		MaxLoginCount(cfg.SaToken.MaxLoginCount).
		IsLog(cfg.SaToken.IsLog).
		Build()

	// 设置全局Manager
	stputil.SetManager(manager)

	return nil
}

// GetManager 获取Manager
func GetManager() *core.Manager {
	return manager
}

// Login 登录
func Login(loginId any) (string, error) {
	return stputil.Login(loginId)
}

// LoginWithDevice 指定设备登录
func LoginWithDevice(loginId any, device string) (string, error) {
	return stputil.Login(loginId, device)
}

// Logout 登出
func Logout(loginId any, device ...string) error {
	return stputil.Logout(loginId, device...)
}

// LogoutByToken 根据Token登出
func LogoutByToken(tokenValue string) error {
	err := stputil.LogoutByToken(tokenValue)
	if err != nil {
		fmt.Printf("[SaToken] LogoutByToken失败: token=%s, error=%v\n", tokenValue[:min(20, len(tokenValue))], err)
	} else {
		fmt.Printf("[SaToken] LogoutByToken成功: token=%s...\n", tokenValue[:min(20, len(tokenValue))])
	}
	return err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// IsLogin 判断是否登录
func IsLogin(tokenValue string) bool {
	result := stputil.IsLogin(tokenValue)
	if !result {
		fmt.Printf("[SaToken] IsLogin检查: token=%s..., 结果=未登录\n", tokenValue[:min(20, len(tokenValue))])
	}
	return result
}

// GetLoginId 获取登录ID
func GetLoginId(tokenValue string) (string, error) {
	return stputil.GetLoginID(tokenValue)
}

// GetLoginIdAsString 获取登录ID(字符串)
func GetLoginIdAsString(tokenValue string) (string, error) {
	return stputil.GetLoginID(tokenValue)
}

// GetTokenValue 获取登录ID对应的Token值
func GetTokenValue(loginId any, device ...string) (string, error) {
	return stputil.GetTokenValue(loginId, device...)
}

// CheckLogin 检查登录状态
func CheckLogin(tokenValue string) error {
	return stputil.CheckLogin(tokenValue)
}

// KickOut 踢人下线
func KickOut(loginId any, device ...string) error {
	return stputil.Kickout(loginId, device...)
}

// Disable 禁用账号
func Disable(loginId any, duration int64) error {
	return stputil.Disable(loginId, time.Duration(duration)*time.Second)
}

// GetDisableTime 获取禁用剩余时间
func GetDisableTime(loginId any) int64 {
	result, _ := stputil.GetDisableTime(loginId)
	return result
}

// IsDisable 判断是否被禁用
func IsDisable(loginId any) bool {
	return stputil.IsDisable(loginId)
}

// Untie 解除禁用
func Untie(loginId any) error {
	return stputil.Untie(loginId)
}

// HasRole 判断是否拥有角色
func HasRole(loginId any, role string) bool {
	return stputil.HasRole(loginId, role)
}

// HasRoleAnd 判断是否拥有所有角色
func HasRoleAnd(loginId any, roles []string) bool {
	return stputil.HasRolesAnd(loginId, roles)
}

// HasRoleOr 判断是否拥有任一角色
func HasRoleOr(loginId any, roles []string) bool {
	return stputil.HasRolesOr(loginId, roles)
}

// CheckRole 校验角色
func CheckRole(tokenValue string, role string) error {
	return stputil.CheckRole(tokenValue, role)
}

// HasPermission 判断是否拥有权限
func HasPermission(loginId any, permission string) bool {
	return stputil.HasPermission(loginId, permission)
}

// HasPermissionAnd 判断是否拥有所有权限
func HasPermissionAnd(loginId any, permissions []string) bool {
	return stputil.HasPermissionsAnd(loginId, permissions)
}

// HasPermissionOr 判断是否拥有任一权限
func HasPermissionOr(loginId any, permissions []string) bool {
	return stputil.HasPermissionsOr(loginId, permissions)
}

// CheckPermission 校验权限
func CheckPermission(tokenValue string, permission string) error {
	return stputil.CheckPermission(tokenValue, permission)
}
