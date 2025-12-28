package service

import (
	"time"

	"yqhp/admin/internal/auth"
	"yqhp/admin/internal/model"

	"gorm.io/gorm"
)

// TokenService 令牌服务
type TokenService struct {
	db *gorm.DB
}

// NewTokenService 创建令牌服务
func NewTokenService(db *gorm.DB) *TokenService {
	return &TokenService{db: db}
}

// ListTokensRequest 令牌列表请求
type ListTokensRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
}

// ListTokens 获取令牌列表
func (s *TokenService) ListTokens(req *ListTokensRequest) ([]model.UserToken, int64, error) {
	var tokens []model.UserToken
	var total int64

	query := s.db.Model(&model.UserToken{})

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
func (s *TokenService) KickOut(userID uint) error {
	// 先查询该用户的所有Token
	var tokens []model.UserToken
	s.db.Where("user_id = ?", userID).Find(&tokens)
	
	// 逐个使token失效
	for _, t := range tokens {
		_ = auth.LogoutByToken(t.Token)
	}
	
	// 从数据库删除该用户的所有Token记录
	s.db.Where("user_id = ?", userID).Delete(&model.UserToken{})
	
	// 调用sa-token踢人下线
	return auth.KickOut(userID)
}

// KickOutByTokenID 根据Token记录ID踢人下线
func (s *TokenService) KickOutByTokenID(tokenID uint) error {
	// 先查询Token记录
	var userToken model.UserToken
	if err := s.db.First(&userToken, tokenID).Error; err != nil {
		return err
	}
	
	// 调用sa-token使token失效
	if err := auth.LogoutByToken(userToken.Token); err != nil {
		// 即使失败也继续删除数据库记录
		_ = err
	}
	
	// 从数据库删除Token记录
	return s.db.Delete(&userToken).Error
}

// KickOutByToken 根据Token踢人下线
func (s *TokenService) KickOutByToken(token string) error {
	// 从数据库删除Token记录
	s.db.Where("token = ?", token).Delete(&model.UserToken{})
	// 调用sa-token登出
	return auth.LogoutByToken(token)
}

// DisableUser 禁用用户
func (s *TokenService) DisableUser(userID uint, disableTime int64) error {
	return auth.Disable(userID, time.Duration(disableTime)*time.Second)
}

// EnableUser 解禁用户
func (s *TokenService) EnableUser(userID uint) error {
	return auth.Untie(userID)
}

// IsUserDisabled 判断用户是否被禁用
func (s *TokenService) IsUserDisabled(userID uint) bool {
	return auth.IsDisable(userID)
}

// GetLoginLogs 获取登录日志
func (s *TokenService) GetLoginLogs(req *ListLoginLogsRequest) ([]model.LoginLog, int64, error) {
	var logs []model.LoginLog
	var total int64

	query := s.db.Model(&model.LoginLog{})

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

// ListLoginLogsRequest 登录日志列表请求
type ListLoginLogsRequest struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	Username  string `json:"username"`
	Status    *int8  `json:"status"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// CreateLoginLog 创建登录日志
func (s *TokenService) CreateLoginLog(log *model.LoginLog) error {
	return s.db.Create(log).Error
}

// GetOperationLogs 获取操作日志
func (s *TokenService) GetOperationLogs(req *ListOperationLogsRequest) ([]model.OperationLog, int64, error) {
	var logs []model.OperationLog
	var total int64

	query := s.db.Model(&model.OperationLog{})

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

// ListOperationLogsRequest 操作日志列表请求
type ListOperationLogsRequest struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	Username  string `json:"username"`
	Module    string `json:"module"`
	Status    *int8  `json:"status"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// ClearLoginLogs 清空登录日志
func (s *TokenService) ClearLoginLogs() error {
	return s.db.Exec("TRUNCATE TABLE sys_login_log").Error
}

// ClearOperationLogs 清空操作日志
func (s *TokenService) ClearOperationLogs() error {
	return s.db.Exec("TRUNCATE TABLE sys_operation_log").Error
}

