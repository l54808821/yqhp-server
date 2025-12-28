package logic

import (
	"yqhp/admin/internal/auth"
	"yqhp/admin/internal/model"
	"yqhp/admin/internal/types"

	"gorm.io/gorm"
)

// TokenLogic 令牌逻辑
type TokenLogic struct {
	db *gorm.DB
}

// NewTokenLogic 创建令牌逻辑
func NewTokenLogic(db *gorm.DB) *TokenLogic {
	return &TokenLogic{db: db}
}

// ListTokens 获取令牌列表
func (l *TokenLogic) ListTokens(req *types.ListTokensRequest) ([]model.SysUserToken, int64, error) {
	var tokens []model.SysUserToken
	var total int64

	query := l.db.Model(&model.SysUserToken{})

	if req.UserID > 0 {
		query = query.Where("user_id = ?", req.UserID)
	}

	query.Count(&total)

	if req.Page > 0 && req.PageSize > 0 {
		offset := (req.Page - 1) * req.PageSize
		query = query.Offset(offset).Limit(req.PageSize)
	}

	if err := query.Order("created_at DESC").Find(&tokens).Error; err != nil {
		return nil, 0, err
	}

	return tokens, total, nil
}

// KickOutByUserID 踢人下线（按用户ID）
func (l *TokenLogic) KickOutByUserID(userID int64) error {
	// 先查询该用户的所有Token
	var tokens []model.SysUserToken
	l.db.Where("user_id = ?", userID).Find(&tokens)

	// 逐个使token失效
	for _, token := range tokens {
		if token.Token != nil {
			auth.LogoutByToken(*token.Token)
		}
	}

	// 从数据库删除该用户的所有Token记录（硬删除）
	l.db.Where("user_id = ?", userID).Delete(&model.SysUserToken{})

	// 调用sa-token踢人下线
	return auth.KickOut(userID)
}

// KickOutByTokenID 踢人下线（按Token ID）
func (l *TokenLogic) KickOutByTokenID(tokenID int64) error {
	var userToken model.SysUserToken
	if err := l.db.First(&userToken, tokenID).Error; err != nil {
		return err
	}

	// 调用sa-token登出
	if userToken.Token != nil {
		auth.LogoutByToken(*userToken.Token)
	}

	// 从数据库删除Token记录（硬删除）
	return l.db.Delete(&userToken).Error
}

// KickOutByToken 踢人下线（按Token字符串）
func (l *TokenLogic) KickOutByToken(token string) error {
	// 从数据库删除Token记录（硬删除）
	l.db.Where("token = ?", token).Delete(&model.SysUserToken{})
	// 调用sa-token登出
	return auth.LogoutByToken(token)
}

// DisableUser 禁用用户
func (l *TokenLogic) DisableUser(userID int64, duration int64) error {
	// 先踢人下线
	l.KickOutByUserID(userID)
	// 禁用用户
	return auth.Disable(userID, duration)
}

// EnableUser 解禁用户
func (l *TokenLogic) EnableUser(userID int64) error {
	return auth.Untie(userID)
}

// IsDisabled 检查用户是否被禁用
func (l *TokenLogic) IsDisabled(userID int64) bool {
	return auth.IsDisable(userID)
}

// GetDisableTime 获取用户禁用剩余时间
func (l *TokenLogic) GetDisableTime(userID int64) int64 {
	return auth.GetDisableTime(userID)
}

// ListLoginLogs 获取登录日志列表
func (l *TokenLogic) ListLoginLogs(req *types.ListLoginLogsRequest) ([]model.SysLoginLog, int64, error) {
	var logs []model.SysLoginLog
	var total int64

	query := l.db.Model(&model.SysLoginLog{})

	if req.Username != "" {
		query = query.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}

	query.Count(&total)

	if req.Page > 0 && req.PageSize > 0 {
		offset := (req.Page - 1) * req.PageSize
		query = query.Offset(offset).Limit(req.PageSize)
	}

	if err := query.Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// ListOperationLogs 获取操作日志列表
func (l *TokenLogic) ListOperationLogs(req *types.ListOperationLogsRequest) ([]model.SysOperationLog, int64, error) {
	var logs []model.SysOperationLog
	var total int64

	query := l.db.Model(&model.SysOperationLog{})

	if req.Username != "" {
		query = query.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.Module != "" {
		query = query.Where("module = ?", req.Module)
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}

	query.Count(&total)

	if req.Page > 0 && req.PageSize > 0 {
		offset := (req.Page - 1) * req.PageSize
		query = query.Offset(offset).Limit(req.PageSize)
	}

	if err := query.Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// ClearLoginLogs 清空登录日志（硬删除）
func (l *TokenLogic) ClearLoginLogs() error {
	return l.db.Where("1 = 1").Delete(&model.SysLoginLog{}).Error
}

// ClearOperationLogs 清空操作日志（硬删除）
func (l *TokenLogic) ClearOperationLogs() error {
	return l.db.Where("1 = 1").Delete(&model.SysOperationLog{}).Error
}
