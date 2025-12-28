package logic

import (
	"time"

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
func (l *TokenLogic) ListTokens(req *types.ListTokensRequest) ([]model.UserToken, int64, error) {
	var tokens []model.UserToken
	var total int64

	query := l.db.Model(&model.UserToken{})

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

// KickOut 根据用户ID踢人下线（踢掉该用户的所有会话）
func (l *TokenLogic) KickOut(userID uint) error {
	// 先查询该用户的所有Token
	var tokens []model.UserToken
	l.db.Where("user_id = ?", userID).Find(&tokens)

	// 逐个使token失效
	for _, t := range tokens {
		_ = auth.LogoutByToken(t.Token)
	}

	// 从数据库删除该用户的所有Token记录
	l.db.Where("user_id = ?", userID).Delete(&model.UserToken{})

	// 调用sa-token踢人下线
	return auth.KickOut(userID)
}

// KickOutByTokenID 根据Token记录ID踢人下线
func (l *TokenLogic) KickOutByTokenID(tokenID uint) error {
	// 先查询Token记录
	var userToken model.UserToken
	if err := l.db.First(&userToken, tokenID).Error; err != nil {
		return err
	}

	// 调用sa-token使token失效
	if err := auth.LogoutByToken(userToken.Token); err != nil {
		// 即使失败也继续删除数据库记录
		_ = err
	}

	// 从数据库删除Token记录
	return l.db.Delete(&userToken).Error
}

// KickOutByToken 根据Token踢人下线
func (l *TokenLogic) KickOutByToken(token string) error {
	// 从数据库删除Token记录
	l.db.Where("token = ?", token).Delete(&model.UserToken{})
	// 调用sa-token登出
	return auth.LogoutByToken(token)
}

// DisableUser 禁用用户
func (l *TokenLogic) DisableUser(userID uint, disableTime int64) error {
	return auth.Disable(userID, time.Duration(disableTime)*time.Second)
}

// EnableUser 解禁用户
func (l *TokenLogic) EnableUser(userID uint) error {
	return auth.Untie(userID)
}

// IsUserDisabled 判断用户是否被禁用
func (l *TokenLogic) IsUserDisabled(userID uint) bool {
	return auth.IsDisable(userID)
}

// GetLoginLogs 获取登录日志
func (l *TokenLogic) GetLoginLogs(req *types.ListLoginLogsRequest) ([]model.LoginLog, int64, error) {
	var logs []model.LoginLog
	var total int64

	query := l.db.Model(&model.LoginLog{})

	if req.Username != "" {
		query = query.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	if req.StartTime != "" {
		query = query.Where("created_at >= ?", req.StartTime)
	}
	if req.EndTime != "" {
		query = query.Where("created_at <= ?", req.EndTime)
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

// GetOperationLogs 获取操作日志
func (l *TokenLogic) GetOperationLogs(req *types.ListOperationLogsRequest) ([]model.OperationLog, int64, error) {
	var logs []model.OperationLog
	var total int64

	query := l.db.Model(&model.OperationLog{})

	if req.Username != "" {
		query = query.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.Module != "" {
		query = query.Where("module = ?", req.Module)
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	if req.StartTime != "" {
		query = query.Where("created_at >= ?", req.StartTime)
	}
	if req.EndTime != "" {
		query = query.Where("created_at <= ?", req.EndTime)
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

// CreateLoginLog 创建登录日志
func (l *TokenLogic) CreateLoginLog(log *model.LoginLog) error {
	return l.db.Create(log).Error
}

// ClearLoginLogs 清空登录日志
func (l *TokenLogic) ClearLoginLogs() error {
	return l.db.Exec("TRUNCATE TABLE sys_login_log").Error
}

// ClearOperationLogs 清空操作日志
func (l *TokenLogic) ClearOperationLogs() error {
	return l.db.Exec("TRUNCATE TABLE sys_operation_log").Error
}
