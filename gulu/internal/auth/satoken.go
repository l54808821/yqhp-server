package auth

import (
	"fmt"
	"yqhp/gulu/internal/config"

	"github.com/click33/sa-token-go/core"
	satokenConfig "github.com/click33/sa-token-go/core/config"
	satokenRedis "github.com/click33/sa-token-go/storage/redis"
	"github.com/click33/sa-token-go/stputil"
)

var manager *core.Manager

// InitSaToken 初始化SaToken (共享 Redis 存储，用于 SSO Token 验证)
func InitSaToken(cfg *config.Config) error {
	// 构建Redis URL: redis://:password@host:port/db
	var redisURL string
	if cfg.Redis.Password != "" {
		redisURL = fmt.Sprintf("redis://:%s@%s:%d/%d", cfg.Redis.Password, cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.DB)
	} else {
		redisURL = fmt.Sprintf("redis://%s:%d/%d", cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.DB)
	}

	// 使用Redis存储（与 Admin 服务共享，实现 SSO）
	storage, err := satokenRedis.NewStorage(redisURL)
	if err != nil {
		return fmt.Errorf("Redis存储初始化失败: %v", err)
	}
	fmt.Println("[SaToken] 使用共享Redis存储 (SSO模式)")

	// 解析TokenStyle
	tokenStyle := parseTokenStyle(cfg.SaToken.TokenStyle)
	fmt.Printf("[SaToken] Token风格: %s\n", cfg.SaToken.TokenStyle)

	// 使用Builder模式创建Manager (配置需与 Admin 服务保持一致)
	builder := core.NewBuilder().
		Storage(storage).
		TokenName(cfg.SaToken.TokenName).
		TokenStyle(tokenStyle).
		Timeout(cfg.SaToken.Timeout).
		ActiveTimeout(cfg.SaToken.ActiveTimeout).
		IsConcurrent(cfg.SaToken.IsConcurrent).
		IsShare(cfg.SaToken.IsShare).
		MaxLoginCount(cfg.SaToken.MaxLoginCount).
		IsLog(cfg.SaToken.IsLog)

	// 如果使用JWT风格，设置JWT密钥
	if tokenStyle == satokenConfig.TokenStyleJWT && cfg.SaToken.JwtSecretKey != "" {
		builder = builder.JwtSecretKey(cfg.SaToken.JwtSecretKey)
		fmt.Println("[SaToken] JWT密钥已配置")
	}

	manager = builder.Build()

	// 设置全局Manager
	stputil.SetManager(manager)

	return nil
}

// parseTokenStyle 解析Token风格配置
func parseTokenStyle(style string) satokenConfig.TokenStyle {
	switch style {
	case "uuid":
		return satokenConfig.TokenStyleUUID
	case "simple-uuid":
		return satokenConfig.TokenStyleSimple
	case "random-32":
		return satokenConfig.TokenStyleRandom32
	case "random-64":
		return satokenConfig.TokenStyleRandom64
	case "random-128":
		return satokenConfig.TokenStyleRandom128
	case "jwt":
		return satokenConfig.TokenStyleJWT
	case "hash":
		return satokenConfig.TokenStyleHash
	case "timestamp":
		return satokenConfig.TokenStyleTimestamp
	case "tik":
		return satokenConfig.TokenStyleTik
	default:
		return satokenConfig.TokenStyleUUID
	}
}

// GetManager 获取Manager
func GetManager() *core.Manager {
	return manager
}

// IsLogin 判断是否登录
func IsLogin(tokenValue string) bool {
	return stputil.IsLogin(tokenValue)
}

// GetLoginId 获取登录ID
func GetLoginId(tokenValue string) (string, error) {
	return stputil.GetLoginID(tokenValue)
}

// CheckLogin 检查登录状态
func CheckLogin(tokenValue string) error {
	return stputil.CheckLogin(tokenValue)
}
