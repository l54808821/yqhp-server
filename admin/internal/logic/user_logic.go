package logic

import (
	"errors"
	"time"

	"yqhp/admin/internal/auth"
	"yqhp/admin/internal/model"
	"yqhp/admin/internal/types"
	commontypes "yqhp/common/types"
	"yqhp/common/utils"

	"gorm.io/gorm"
)

// UserLogic 用户逻辑
type UserLogic struct {
	db *gorm.DB
}

// NewUserLogic 创建用户逻辑
func NewUserLogic(db *gorm.DB) *UserLogic {
	return &UserLogic{db: db}
}

// Register 用户注册
func (l *UserLogic) Register(req *types.RegisterRequest, ip string) (*types.LoginResponse, error) {
	// 验证密码
	if req.Password != req.ConfirmPassword {
		return nil, errors.New("两次密码输入不一致")
	}

	// 验证用户名长度
	if len(req.Username) < 4 || len(req.Username) > 20 {
		return nil, errors.New("用户名长度应为4-20个字符")
	}

	// 验证密码长度
	if len(req.Password) < 6 || len(req.Password) > 20 {
		return nil, errors.New("密码长度应为6-20个字符")
	}

	// 检查用户名是否存在
	var count int64
	l.db.Model(&model.User{}).Where("username = ? AND is_delete = ?", req.Username, false).Count(&count)
	if count > 0 {
		return nil, errors.New("用户名已存在")
	}

	// 检查邮箱是否存在
	if req.Email != "" {
		l.db.Model(&model.User{}).Where("email = ? AND is_delete = ?", req.Email, false).Count(&count)
		if count > 0 {
			return nil, errors.New("邮箱已被使用")
		}
	}

	// 检查手机号是否存在
	if req.Phone != "" {
		l.db.Model(&model.User{}).Where("phone = ? AND is_delete = ?", req.Phone, false).Count(&count)
		if count > 0 {
			return nil, errors.New("手机号已被使用")
		}
	}

	// 设置默认昵称
	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}

	// 创建用户
	user := &model.User{
		Username: req.Username,
		Password: utils.MD5(req.Password),
		Nickname: nickname,
		Email:    req.Email,
		Phone:    req.Phone,
		Status:   1,
	}

	if err := l.db.Create(user).Error; err != nil {
		return nil, errors.New("注册失败，请稍后重试")
	}

	// 自动登录
	token, err := auth.Login(user.ID)
	if err != nil {
		return nil, errors.New("注册成功，但自动登录失败")
	}

	// 更新最后登录时间和IP
	now := time.Now()
	l.db.Model(user).Updates(map[string]any{
		"last_login_at": now,
		"last_login_ip": ip,
	})

	// 保存Token到数据库
	l.saveUserToken(user.ID, token, ip, now)

	// 记录登录日志
	l.recordLoginLog(user.ID, req.Username, ip, 1, "注册并登录成功", "register")

	return &types.LoginResponse{
		Token:    token,
		UserInfo: user,
	}, nil
}

// Login 用户登录
func (l *UserLogic) Login(req *types.LoginRequest, ip string) (*types.LoginResponse, error) {
	// 查询用户
	var user model.User
	if err := l.db.Where("username = ? AND is_delete = ?", req.Username, false).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 记录失败的登录日志
			l.recordLoginLog(0, req.Username, ip, 0, "用户名或密码错误", "password")
			return nil, errors.New("用户名或密码错误")
		}
		return nil, err
	}

	// 验证密码
	if utils.MD5(req.Password) != user.Password {
		// 记录失败的登录日志
		l.recordLoginLog(user.ID, req.Username, ip, 0, "用户名或密码错误", "password")
		return nil, errors.New("用户名或密码错误")
	}

	// 检查用户状态
	if user.Status != 1 {
		l.recordLoginLog(user.ID, req.Username, ip, 0, "用户已被禁用", "password")
		return nil, errors.New("用户已被禁用")
	}

	// 检查是否被封禁
	if auth.IsDisable(user.ID) {
		l.recordLoginLog(user.ID, req.Username, ip, 0, "账号已被封禁", "password")
		return nil, errors.New("账号已被封禁")
	}

	// 执行登录
	token, err := auth.Login(user.ID)
	if err != nil {
		l.recordLoginLog(user.ID, req.Username, ip, 0, "登录失败", "password")
		return nil, errors.New("登录失败")
	}

	// 更新最后登录时间和IP
	now := time.Now()
	l.db.Model(&user).Updates(map[string]any{
		"last_login_at": now,
		"last_login_ip": ip,
	})

	// 保存Token到数据库
	l.saveUserToken(user.ID, token, ip, now)

	// 记录成功的登录日志
	l.recordLoginLog(user.ID, req.Username, ip, 1, "登录成功", "password")

	// 预加载角色
	l.db.Preload("Roles", "is_delete = ?", false).First(&user, user.ID)

	return &types.LoginResponse{
		Token:    token,
		UserInfo: &user,
	}, nil
}

// saveUserToken 保存用户Token到数据库
func (l *UserLogic) saveUserToken(userID uint, token string, ip string, now time.Time) {
	// 获取Token有效期配置（默认24小时）
	expireAt := now.Add(24 * time.Hour)

	userToken := &model.UserToken{
		UserID:       userID,
		Token:        token,
		Device:       "pc",
		Platform:     "web",
		IP:           ip,
		ExpireAt:     commontypes.NewDateTime(expireAt),
		LastActiveAt: commontypes.NewDateTime(now),
	}

	l.db.Create(userToken)
}

// recordLoginLog 记录登录日志
func (l *UserLogic) recordLoginLog(userID uint, username string, ip string, status int8, message string, loginType string) {
	log := &model.LoginLog{
		UserID:    userID,
		Username:  username,
		IP:        ip,
		Status:    status,
		Message:   message,
		LoginType: loginType,
	}
	l.db.Create(log)
}

// Logout 用户登出
func (l *UserLogic) Logout(token string) error {
	// 从数据库删除Token记录
	l.db.Where("token = ?", token).Delete(&model.UserToken{})

	// 调用sa-token登出
	return auth.LogoutByToken(token)
}

// GetUserInfo 获取用户信息
func (l *UserLogic) GetUserInfo(userID uint) (*model.User, error) {
	var user model.User
	if err := l.db.Preload("Roles", "is_delete = ?", false).Where("is_delete = ?", false).First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateUser 创建用户
func (l *UserLogic) CreateUser(req *types.CreateUserRequest) (*model.User, error) {
	// 检查用户名是否存在
	var count int64
	l.db.Model(&model.User{}).Where("username = ? AND is_delete = ?", req.Username, false).Count(&count)
	if count > 0 {
		return nil, errors.New("用户名已存在")
	}

	// 创建用户
	user := &model.User{
		Username: req.Username,
		Password: utils.MD5(req.Password),
		Nickname: req.Nickname,
		Email:    req.Email,
		Phone:    req.Phone,
		Gender:   req.Gender,
		DeptID:   req.DeptID,
		Status:   1,
		Remark:   req.Remark,
	}

	if err := l.db.Create(user).Error; err != nil {
		return nil, err
	}

	// 关联角色
	if len(req.RoleIDs) > 0 {
		for _, roleID := range req.RoleIDs {
			l.db.Create(&model.UserRole{
				UserID: user.ID,
				RoleID: roleID,
			})
		}
	}

	return user, nil
}

// UpdateUser 更新用户
func (l *UserLogic) UpdateUser(req *types.UpdateUserRequest) error {
	// 更新用户信息
	updates := map[string]any{
		"nickname": req.Nickname,
		"avatar":   req.Avatar,
		"email":    req.Email,
		"phone":    req.Phone,
		"gender":   req.Gender,
		"dept_id":  req.DeptID,
		"status":   req.Status,
		"remark":   req.Remark,
	}

	if err := l.db.Model(&model.User{}).Where("id = ?", req.ID).Updates(updates).Error; err != nil {
		return err
	}

	// 更新角色关联（软删除旧的，创建新的）
	l.db.Model(&model.UserRole{}).Where("user_id = ? AND is_delete = ?", req.ID, false).Update("is_delete", true)
	if len(req.RoleIDs) > 0 {
		for _, roleID := range req.RoleIDs {
			l.db.Create(&model.UserRole{
				UserID: req.ID,
				RoleID: roleID,
			})
		}
	}

	return nil
}

// DeleteUser 删除用户（软删除）
func (l *UserLogic) DeleteUser(id uint) error {
	// 软删除用户角色关联
	l.db.Model(&model.UserRole{}).Where("user_id = ? AND is_delete = ?", id, false).Update("is_delete", true)
	// 软删除用户
	return l.db.Model(&model.User{}).Where("id = ?", id).Update("is_delete", true).Error
}

// ResetPassword 重置密码
func (l *UserLogic) ResetPassword(id uint, newPassword string) error {
	return l.db.Model(&model.User{}).Where("id = ?", id).Update("password", utils.MD5(newPassword)).Error
}

// ChangePassword 修改密码
func (l *UserLogic) ChangePassword(id uint, oldPassword, newPassword string) error {
	var user model.User
	if err := l.db.First(&user, id).Error; err != nil {
		return err
	}

	if utils.MD5(oldPassword) != user.Password {
		return errors.New("原密码错误")
	}

	return l.db.Model(&user).Update("password", utils.MD5(newPassword)).Error
}

// ListUsers 获取用户列表
func (l *UserLogic) ListUsers(req *types.ListUsersRequest) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	query := l.db.Model(&model.User{}).Where("is_delete = ?", false)

	if req.Username != "" {
		query = query.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.Nickname != "" {
		query = query.Where("nickname LIKE ?", "%"+req.Nickname+"%")
	}
	if req.Phone != "" {
		query = query.Where("phone LIKE ?", "%"+req.Phone+"%")
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	if req.DeptID > 0 {
		query = query.Where("dept_id = ?", req.DeptID)
	}

	query.Count(&total)

	if req.Page > 0 && req.PageSize > 0 {
		offset := (req.Page - 1) * req.PageSize
		query = query.Offset(offset).Limit(req.PageSize)
	}

	if err := query.Preload("Roles", "is_delete = ?", false).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
