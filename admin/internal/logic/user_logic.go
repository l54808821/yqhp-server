package logic

import (
	"context"
	"errors"
	"time"

	"yqhp/admin/internal/auth"
	"yqhp/admin/internal/model"
	"yqhp/admin/internal/query"
	"yqhp/admin/internal/svc"
	"yqhp/admin/internal/types"
	"yqhp/common/utils"

	"github.com/gofiber/fiber/v2"
)

// UserLogic 用户逻辑
type UserLogic struct {
	ctx context.Context
}

// NewUserLogic 创建用户逻辑
func NewUserLogic(c *fiber.Ctx) *UserLogic {
	return &UserLogic{ctx: c.Context()}
}

// db 获取数据库实例（复杂查询用）
func (l *UserLogic) db() *query.Query {
	return svc.Ctx.Query
}

// Register 用户注册
func (l *UserLogic) Register(req *types.RegisterRequest, ip string) (*types.LoginResponse, error) {
	if req.Password != req.ConfirmPassword {
		return nil, errors.New("两次密码输入不一致")
	}
	if len(req.Username) < 4 || len(req.Username) > 20 {
		return nil, errors.New("用户名长度应为4-20个字符")
	}
	if len(req.Password) < 6 || len(req.Password) > 20 {
		return nil, errors.New("密码长度应为6-20个字符")
	}

	u := l.db().SysUser
	// 检查用户名
	count, _ := u.WithContext(l.ctx).Where(u.Username.Eq(req.Username), u.IsDelete.Is(false)).Count()
	if count > 0 {
		return nil, errors.New("用户名已存在")
	}
	// 检查邮箱
	if req.Email != "" {
		count, _ = u.WithContext(l.ctx).Where(u.Email.Eq(req.Email), u.IsDelete.Is(false)).Count()
		if count > 0 {
			return nil, errors.New("邮箱已被使用")
		}
	}
	// 检查手机号
	if req.Phone != "" {
		count, _ = u.WithContext(l.ctx).Where(u.Phone.Eq(req.Phone), u.IsDelete.Is(false)).Count()
		if count > 0 {
			return nil, errors.New("手机号已被使用")
		}
	}

	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}

	user := &model.SysUser{
		Username: req.Username,
		Password: utils.MD5(req.Password),
		Nickname: model.StringPtr(nickname),
		Email:    model.StringPtr(req.Email),
		Phone:    model.StringPtr(req.Phone),
		Status:   model.Int32Ptr(1),
		IsDelete: model.BoolPtr(false),
	}

	if err := u.WithContext(l.ctx).Create(user); err != nil {
		return nil, errors.New("注册失败，请稍后重试")
	}

	token, err := auth.Login(user.ID)
	if err != nil {
		return nil, errors.New("注册成功，但自动登录失败")
	}

	now := time.Now()
	u.WithContext(l.ctx).Where(u.ID.Eq(user.ID)).Updates(map[string]any{
		"last_login_at": now,
		"last_login_ip": ip,
	})

	l.saveUserToken(user.ID, token, ip, now)
	l.recordLoginLog(user.ID, req.Username, ip, 1, "注册并登录成功", "register")

	// 新注册用户没有角色
	return &types.LoginResponse{Token: token, UserInfo: types.ToUserInfoWithRoles(user, nil)}, nil
}

// Login 用户登录
func (l *UserLogic) Login(req *types.LoginRequest, ip string) (*types.LoginResponse, error) {
	u := l.db().SysUser
	user, err := u.WithContext(l.ctx).Where(u.Username.Eq(req.Username), u.IsDelete.Is(false)).First()
	if err != nil {
		l.recordLoginLog(0, req.Username, ip, 0, "用户名或密码错误", "password")
		return nil, errors.New("用户名或密码错误")
	}

	if utils.MD5(req.Password) != user.Password {
		l.recordLoginLog(user.ID, req.Username, ip, 0, "用户名或密码错误", "password")
		return nil, errors.New("用户名或密码错误")
	}

	if model.GetInt32(user.Status) != 1 {
		l.recordLoginLog(user.ID, req.Username, ip, 0, "用户已被禁用", "password")
		return nil, errors.New("用户已被禁用")
	}

	if auth.IsDisable(user.ID) {
		l.recordLoginLog(user.ID, req.Username, ip, 0, "账号已被封禁", "password")
		return nil, errors.New("账号已被封禁")
	}

	token, err := auth.Login(user.ID)
	if err != nil {
		l.recordLoginLog(user.ID, req.Username, ip, 0, "登录失败", "password")
		return nil, errors.New("登录失败")
	}

	now := time.Now()
	u.WithContext(l.ctx).Where(u.ID.Eq(user.ID)).Updates(map[string]any{
		"last_login_at": now,
		"last_login_ip": ip,
	})

	l.saveUserToken(user.ID, token, ip, now)
	l.recordLoginLog(user.ID, req.Username, ip, 1, "登录成功", "password")

	// 查询用户角色
	roles, _ := l.getUserRoles(user.ID)
	return &types.LoginResponse{Token: token, UserInfo: types.ToUserInfoWithRoles(user, roles)}, nil
}

func (l *UserLogic) saveUserToken(userID int64, token string, ip string, now time.Time) {
	expireAt := now.Add(24 * time.Hour)
	userToken := &model.SysUserToken{
		UserID:       model.Int64Ptr(userID),
		Token:        model.StringPtr(token),
		Device:       model.StringPtr("pc"),
		Platform:     model.StringPtr("web"),
		IP:           model.StringPtr(ip),
		ExpireAt:     &expireAt,
		LastActiveAt: &now,
		IsDelete:     model.BoolPtr(false),
	}
	l.db().SysUserToken.WithContext(l.ctx).Create(userToken)
}

func (l *UserLogic) recordLoginLog(userID int64, username string, ip string, status int8, message string, loginType string) {
	log := &model.SysLoginLog{
		UserID:    model.Int64Ptr(userID),
		Username:  model.StringPtr(username),
		IP:        model.StringPtr(ip),
		Status:    model.Int32Ptr(int32(status)),
		Message:   model.StringPtr(message),
		LoginType: model.StringPtr(loginType),
		IsDelete:  model.BoolPtr(false),
	}
	l.db().SysLoginLog.WithContext(l.ctx).Create(log)
}

// Logout 用户登出
func (l *UserLogic) Logout(token string) error {
	t := l.db().SysUserToken
	t.WithContext(l.ctx).Where(t.Token.Eq(token)).Delete()
	return auth.LogoutByToken(token)
}

// GetUserInfo 获取用户信息（含角色）
func (l *UserLogic) GetUserInfo(userID int64) (*types.UserInfo, error) {
	u := l.db().SysUser
	user, err := u.WithContext(l.ctx).Where(u.ID.Eq(userID), u.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}

	// 查询用户角色
	roles, _ := l.getUserRoles(userID)
	return types.ToUserInfoWithRoles(user, roles), nil
}

// getUserRoles 获取用户的角色列表
func (l *UserLogic) getUserRoles(userID int64) ([]*model.SysRole, error) {
	ur := l.db().SysUserRole
	userRoles, err := ur.WithContext(l.ctx).Where(ur.UserID.Eq(userID), ur.IsDelete.Is(false)).Find()
	if err != nil || len(userRoles) == 0 {
		return nil, err
	}

	roleIDs := make([]int64, len(userRoles))
	for i, r := range userRoles {
		roleIDs[i] = r.RoleID
	}

	r := l.db().SysRole
	return r.WithContext(l.ctx).Where(r.ID.In(roleIDs...), r.IsDelete.Is(false)).Find()
}

// CreateUser 创建用户
func (l *UserLogic) CreateUser(req *types.CreateUserRequest) (*types.UserInfo, error) {
	u := l.db().SysUser
	count, _ := u.WithContext(l.ctx).Where(u.Username.Eq(req.Username), u.IsDelete.Is(false)).Count()
	if count > 0 {
		return nil, errors.New("用户名已存在")
	}

	user := &model.SysUser{
		Username: req.Username,
		Password: utils.MD5(req.Password),
		Nickname: model.StringPtr(req.Nickname),
		Email:    model.StringPtr(req.Email),
		Phone:    model.StringPtr(req.Phone),
		Gender:   model.Int32Ptr(int32(req.Gender)),
		DeptID:   model.Int64Ptr(int64(req.DeptID)),
		Status:   model.Int32Ptr(1),
		Remark:   model.StringPtr(req.Remark),
		IsDelete: model.BoolPtr(false),
	}

	if err := u.WithContext(l.ctx).Create(user); err != nil {
		return nil, err
	}

	// 关联角色
	if len(req.RoleIDs) > 0 {
		ur := l.db().SysUserRole
		for _, roleID := range req.RoleIDs {
			ur.WithContext(l.ctx).Create(&model.SysUserRole{
				UserID:   user.ID,
				RoleID:   int64(roleID),
				IsDelete: model.BoolPtr(false),
			})
		}
	}

	return types.ToUserInfo(user), nil
}

// UpdateUser 更新用户
func (l *UserLogic) UpdateUser(req *types.UpdateUserRequest) error {
	u := l.db().SysUser
	_, err := u.WithContext(l.ctx).Where(u.ID.Eq(int64(req.ID))).Updates(map[string]any{
		"nickname": req.Nickname,
		"avatar":   req.Avatar,
		"email":    req.Email,
		"phone":    req.Phone,
		"gender":   req.Gender,
		"dept_id":  req.DeptID,
		"status":   req.Status,
		"remark":   req.Remark,
	})
	if err != nil {
		return err
	}

	// 只有当明确传递了 RoleIDs 时才更新角色关联
	if req.RoleIDs != nil {
		ur := l.db().SysUserRole
		userID := int64(req.ID)

		// 获取当前用户的所有角色关联
		existingRoles, _ := ur.WithContext(l.ctx).Where(ur.UserID.Eq(userID)).Find()
		existingMap := make(map[int64]*model.SysUserRole)
		for _, r := range existingRoles {
			existingMap[r.RoleID] = r
		}

		// 新的角色ID集合
		newRoleIDs := make(map[int64]bool)
		for _, roleID := range *req.RoleIDs {
			newRoleIDs[int64(roleID)] = true
		}

		// 处理需要删除的角色（在旧列表中但不在新列表中）
		for roleID, role := range existingMap {
			if !newRoleIDs[roleID] {
				// 标记删除
				ur.WithContext(l.ctx).Where(ur.UserID.Eq(userID), ur.RoleID.Eq(roleID)).Update(ur.IsDelete, true)
			} else if role.IsDelete != nil && *role.IsDelete {
				// 如果之前被删除了，现在需要恢复
				ur.WithContext(l.ctx).Where(ur.UserID.Eq(userID), ur.RoleID.Eq(roleID)).Update(ur.IsDelete, false)
			}
		}

		// 处理需要新增的角色（在新列表中但不在旧列表中）
		for roleID := range newRoleIDs {
			if _, exists := existingMap[roleID]; !exists {
				ur.WithContext(l.ctx).Create(&model.SysUserRole{
					UserID:   userID,
					RoleID:   roleID,
					IsDelete: model.BoolPtr(false),
				})
			}
		}
	}

	return nil
}

// DeleteUser 删除用户
func (l *UserLogic) DeleteUser(id int64) error {
	ur := l.db().SysUserRole
	ur.WithContext(l.ctx).Where(ur.UserID.Eq(id), ur.IsDelete.Is(false)).Update(ur.IsDelete, true)

	u := l.db().SysUser
	_, err := u.WithContext(l.ctx).Where(u.ID.Eq(id)).Update(u.IsDelete, true)
	return err
}

// ResetPassword 重置密码
func (l *UserLogic) ResetPassword(id int64, newPassword string) error {
	u := l.db().SysUser
	_, err := u.WithContext(l.ctx).Where(u.ID.Eq(id)).Update(u.Password, utils.MD5(newPassword))
	return err
}

// ChangePassword 修改密码
func (l *UserLogic) ChangePassword(id int64, oldPassword, newPassword string) error {
	u := l.db().SysUser
	user, err := u.WithContext(l.ctx).Where(u.ID.Eq(id)).First()
	if err != nil {
		return err
	}

	if utils.MD5(oldPassword) != user.Password {
		return errors.New("原密码错误")
	}

	_, err = u.WithContext(l.ctx).Where(u.ID.Eq(id)).Update(u.Password, utils.MD5(newPassword))
	return err
}

// ListUsers 获取用户列表（含角色）
func (l *UserLogic) ListUsers(req *types.ListUsersRequest) ([]*types.UserInfo, int64, error) {
	u := l.db().SysUser
	q := u.WithContext(l.ctx).Where(u.IsDelete.Is(false))

	if req.Username != "" {
		q = q.Where(u.Username.Like("%" + req.Username + "%"))
	}
	if req.Nickname != "" {
		q = q.Where(u.Nickname.Like("%" + req.Nickname + "%"))
	}
	if req.Phone != "" {
		q = q.Where(u.Phone.Like("%" + req.Phone + "%"))
	}
	if req.Status != nil {
		q = q.Where(u.Status.Eq(int32(*req.Status)))
	}
	if req.DeptID > 0 {
		q = q.Where(u.DeptID.Eq(int64(req.DeptID)))
	}

	total, _ := q.Count()

	if req.Page > 0 && req.PageSize > 0 {
		q = q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	users, err := q.Find()
	if err != nil {
		return nil, 0, err
	}

	// 批量查询用户角色
	list := make([]*types.UserInfo, len(users))
	for i, user := range users {
		roles, _ := l.getUserRoles(user.ID)
		list[i] = types.ToUserInfoWithRoles(user, roles)
	}

	return list, total, nil
}

// GetUserRoleIDs 获取用户的角色ID列表
func (l *UserLogic) GetUserRoleIDs(userID int64) ([]int64, error) {
	ur := l.db().SysUserRole
	roles, err := ur.WithContext(l.ctx).Where(ur.UserID.Eq(userID), ur.IsDelete.Is(false)).Find()
	if err != nil {
		return nil, err
	}

	ids := make([]int64, len(roles))
	for i, r := range roles {
		ids[i] = r.RoleID
	}
	return ids, nil
}
