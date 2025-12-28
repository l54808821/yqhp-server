package service

import (
	"errors"
	"time"

	"yqhp/admin/internal/auth"
	"yqhp/admin/internal/model"
	"yqhp/common/types"
	"yqhp/common/utils"

	"gorm.io/gorm"
)

// UserService 用户服务
type UserService struct {
	db *gorm.DB
}

// NewUserService 创建用户服务
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username        string `json:"username" validate:"required"`
	Password        string `json:"password" validate:"required"`
	ConfirmPassword string `json:"confirmPassword" validate:"required"`
	Nickname        string `json:"nickname"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
}

// Register 用户注册
func (s *UserService) Register(req *RegisterRequest, ip string) (*LoginResponse, error) {
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
	s.db.Model(&model.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		return nil, errors.New("用户名已存在")
	}

	// 检查邮箱是否存在
	if req.Email != "" {
		s.db.Model(&model.User{}).Where("email = ?", req.Email).Count(&count)
		if count > 0 {
			return nil, errors.New("邮箱已被使用")
		}
	}

	// 检查手机号是否存在
	if req.Phone != "" {
		s.db.Model(&model.User{}).Where("phone = ?", req.Phone).Count(&count)
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

	if err := s.db.Create(user).Error; err != nil {
		return nil, errors.New("注册失败，请稍后重试")
	}

	// 自动登录
	token, err := auth.Login(user.ID)
	if err != nil {
		return nil, errors.New("注册成功，但自动登录失败")
	}

	// 更新最后登录时间和IP
	now := time.Now()
	s.db.Model(user).Updates(map[string]any{
		"last_login_at": now,
		"last_login_ip": ip,
	})

	// 保存Token到数据库
	s.saveUserToken(user.ID, token, ip, now)

	// 记录登录日志
	s.recordLoginLog(user.ID, req.Username, ip, 1, "注册并登录成功", "register")

	return &LoginResponse{
		Token:    token,
		UserInfo: user,
	}, nil
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token    string      `json:"token"`
	UserInfo *model.User `json:"userInfo"`
}

// Login 用户登录
func (s *UserService) Login(req *LoginRequest, ip string) (*LoginResponse, error) {
	// 查询用户
	var user model.User
	if err := s.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 记录失败的登录日志
			s.recordLoginLog(0, req.Username, ip, 0, "用户名或密码错误", "password")
			return nil, errors.New("用户名或密码错误")
		}
		return nil, err
	}

	// 验证密码
	if utils.MD5(req.Password) != user.Password {
		// 记录失败的登录日志
		s.recordLoginLog(user.ID, req.Username, ip, 0, "用户名或密码错误", "password")
		return nil, errors.New("用户名或密码错误")
	}

	// 检查用户状态
	if user.Status != 1 {
		s.recordLoginLog(user.ID, req.Username, ip, 0, "用户已被禁用", "password")
		return nil, errors.New("用户已被禁用")
	}

	// 检查是否被封禁
	if auth.IsDisable(user.ID) {
		s.recordLoginLog(user.ID, req.Username, ip, 0, "账号已被封禁", "password")
		return nil, errors.New("账号已被封禁")
	}

	// 执行登录
	token, err := auth.Login(user.ID)
	if err != nil {
		s.recordLoginLog(user.ID, req.Username, ip, 0, "登录失败", "password")
		return nil, errors.New("登录失败")
	}

	// 更新最后登录时间和IP
	now := time.Now()
	s.db.Model(&user).Updates(map[string]any{
		"last_login_at": now,
		"last_login_ip": ip,
	})

	// 保存Token到数据库
	s.saveUserToken(user.ID, token, ip, now)

	// 记录成功的登录日志
	s.recordLoginLog(user.ID, req.Username, ip, 1, "登录成功", "password")

	// 预加载角色
	s.db.Preload("Roles").First(&user, user.ID)

	return &LoginResponse{
		Token:    token,
		UserInfo: &user,
	}, nil
}

// saveUserToken 保存用户Token到数据库
func (s *UserService) saveUserToken(userID uint, token string, ip string, now time.Time) {
	// 获取Token有效期配置（默认24小时）
	expireAt := now.Add(24 * time.Hour)
	
	userToken := &model.UserToken{
		UserID:       userID,
		Token:        token,
		Device:       "pc",
		Platform:     "web",
		IP:           ip,
		ExpireAt:     types.NewDateTime(expireAt),
		LastActiveAt: types.NewDateTime(now),
	}
	
	s.db.Create(userToken)
}

// recordLoginLog 记录登录日志
func (s *UserService) recordLoginLog(userID uint, username string, ip string, status int8, message string, loginType string) {
	log := &model.LoginLog{
		UserID:    userID,
		Username:  username,
		IP:        ip,
		Status:    status,
		Message:   message,
		LoginType: loginType,
	}
	s.db.Create(log)
}

// Logout 用户登出
func (s *UserService) Logout(token string) error {
	// 从数据库删除Token记录
	s.db.Where("token = ?", token).Delete(&model.UserToken{})
	
	// 调用sa-token登出
	return auth.LogoutByToken(token)
}

// GetUserInfo 获取用户信息
func (s *UserService) GetUserInfo(userID uint) (*model.User, error) {
	var user model.User
	if err := s.db.Preload("Roles").First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Gender   int8   `json:"gender"`
	DeptID   uint   `json:"deptId"`
	RoleIDs  []uint `json:"roleIds"`
	Remark   string `json:"remark"`
}

// CreateUser 创建用户
func (s *UserService) CreateUser(req *CreateUserRequest) (*model.User, error) {
	// 检查用户名是否存在
	var count int64
	s.db.Model(&model.User{}).Where("username = ?", req.Username).Count(&count)
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

	if err := s.db.Create(user).Error; err != nil {
		return nil, err
	}

	// 关联角色
	if len(req.RoleIDs) > 0 {
		for _, roleID := range req.RoleIDs {
			s.db.Create(&model.UserRole{
				UserID: user.ID,
				RoleID: roleID,
			})
		}
	}

	return user, nil
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	ID       uint   `json:"id" validate:"required"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Gender   int8   `json:"gender"`
	DeptID   uint   `json:"deptId"`
	Status   int8   `json:"status"`
	RoleIDs  []uint `json:"roleIds"`
	Remark   string `json:"remark"`
}

// UpdateUser 更新用户
func (s *UserService) UpdateUser(req *UpdateUserRequest) error {
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

	if err := s.db.Model(&model.User{}).Where("id = ?", req.ID).Updates(updates).Error; err != nil {
		return err
	}

	// 更新角色关联
	s.db.Where("user_id = ?", req.ID).Delete(&model.UserRole{})
	if len(req.RoleIDs) > 0 {
		for _, roleID := range req.RoleIDs {
			s.db.Create(&model.UserRole{
				UserID: req.ID,
				RoleID: roleID,
			})
		}
	}

	return nil
}

// DeleteUser 删除用户
func (s *UserService) DeleteUser(id uint) error {
	// 删除用户角色关联
	s.db.Where("user_id = ?", id).Delete(&model.UserRole{})
	// 删除用户
	return s.db.Delete(&model.User{}, id).Error
}

// ResetPassword 重置密码
func (s *UserService) ResetPassword(id uint, newPassword string) error {
	return s.db.Model(&model.User{}).Where("id = ?", id).Update("password", utils.MD5(newPassword)).Error
}

// ChangePassword 修改密码
func (s *UserService) ChangePassword(id uint, oldPassword, newPassword string) error {
	var user model.User
	if err := s.db.First(&user, id).Error; err != nil {
		return err
	}

	if utils.MD5(oldPassword) != user.Password {
		return errors.New("原密码错误")
	}

	return s.db.Model(&user).Update("password", utils.MD5(newPassword)).Error
}

// ListUsersRequest 用户列表请求
type ListUsersRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Phone    string `json:"phone"`
	Status   *int8  `json:"status"`
	DeptID   uint   `json:"deptId"`
}

// ListUsers 获取用户列表
func (s *UserService) ListUsers(req *ListUsersRequest) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	query := s.db.Model(&model.User{})

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

	if err := query.Preload("Roles").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

