package logic

import (
	"context"

	"yqhp/admin/internal/auth"
	"yqhp/admin/internal/model"
	"yqhp/admin/internal/query"
	"yqhp/admin/internal/svc"
	"yqhp/admin/internal/types"

	"github.com/gofiber/fiber/v2"
)

// TokenLogic 令牌逻辑
type TokenLogic struct {
	ctx context.Context
}

// NewTokenLogic 创建令牌逻辑
func NewTokenLogic(c *fiber.Ctx) *TokenLogic {
	return &TokenLogic{ctx: c.Context()}
}

func (l *TokenLogic) db() *query.Query {
	return svc.Ctx.Query
}

// ListTokens 获取令牌列表
func (l *TokenLogic) ListTokens(req *types.ListTokensRequest) ([]*model.SysUserToken, int64, error) {
	t := l.db().SysUserToken
	q := t.WithContext(l.ctx)

	if req.UserID > 0 {
		q = q.Where(t.UserID.Eq(int64(req.UserID)))
	}

	total, _ := q.Count()

	if req.Page > 0 && req.PageSize > 0 {
		q = q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	tokens, err := q.Order(t.CreatedAt.Desc()).Find()
	return tokens, total, err
}

// KickOutByUserID 踢人下线（按用户ID）
func (l *TokenLogic) KickOutByUserID(userID int64) error {
	t := l.db().SysUserToken
	tokens, _ := t.WithContext(l.ctx).Where(t.UserID.Eq(userID)).Find()

	for _, token := range tokens {
		if token.Token != nil {
			auth.LogoutByToken(*token.Token)
		}
	}

	t.WithContext(l.ctx).Where(t.UserID.Eq(userID)).Delete()
	return auth.KickOut(userID)
}

// KickOutByTokenID 踢人下线（按Token ID）
func (l *TokenLogic) KickOutByTokenID(tokenID int64) error {
	t := l.db().SysUserToken
	userToken, err := t.WithContext(l.ctx).Where(t.ID.Eq(tokenID)).First()
	if err != nil {
		return err
	}

	if userToken.Token != nil {
		auth.LogoutByToken(*userToken.Token)
	}

	_, err = t.WithContext(l.ctx).Where(t.ID.Eq(tokenID)).Delete()
	return err
}

// KickOutByToken 踢人下线（按Token字符串）
func (l *TokenLogic) KickOutByToken(token string) error {
	t := l.db().SysUserToken
	t.WithContext(l.ctx).Where(t.Token.Eq(token)).Delete()
	return auth.LogoutByToken(token)
}

// DisableUser 禁用用户
func (l *TokenLogic) DisableUser(userID int64, duration int64) error {
	l.KickOutByUserID(userID)
	return auth.Disable(userID, duration)
}

// EnableUser 解禁用户
func (l *TokenLogic) EnableUser(userID int64) error {
	return auth.Untie(userID)
}

// ListLoginLogs 获取登录日志列表
func (l *TokenLogic) ListLoginLogs(req *types.ListLoginLogsRequest) ([]*model.SysLoginLog, int64, error) {
	log := l.db().SysLoginLog
	q := log.WithContext(l.ctx)

	if req.Username != "" {
		q = q.Where(log.Username.Like("%" + req.Username + "%"))
	}
	if req.Status != nil {
		q = q.Where(log.Status.Eq(int32(*req.Status)))
	}

	total, _ := q.Count()

	if req.Page > 0 && req.PageSize > 0 {
		q = q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	logs, err := q.Order(log.CreatedAt.Desc()).Find()
	return logs, total, err
}

// ListOperationLogs 获取操作日志列表
func (l *TokenLogic) ListOperationLogs(req *types.ListOperationLogsRequest) ([]*model.SysOperationLog, int64, error) {
	log := l.db().SysOperationLog
	q := log.WithContext(l.ctx)

	if req.Username != "" {
		q = q.Where(log.Username.Like("%" + req.Username + "%"))
	}
	if req.Module != "" {
		q = q.Where(log.Module.Eq(req.Module))
	}
	if req.Status != nil {
		q = q.Where(log.Status.Eq(int32(*req.Status)))
	}

	total, _ := q.Count()

	if req.Page > 0 && req.PageSize > 0 {
		q = q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	logs, err := q.Order(log.CreatedAt.Desc()).Find()
	return logs, total, err
}

// ClearLoginLogs 清空登录日志
func (l *TokenLogic) ClearLoginLogs() error {
	log := l.db().SysLoginLog
	_, err := log.WithContext(l.ctx).Where(log.ID.Gt(0)).Delete()
	return err
}

// ClearOperationLogs 清空操作日志
func (l *TokenLogic) ClearOperationLogs() error {
	log := l.db().SysOperationLog
	_, err := log.WithContext(l.ctx).Where(log.ID.Gt(0)).Delete()
	return err
}
