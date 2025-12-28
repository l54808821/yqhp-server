package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"yqhp/admin/internal/auth"
	"yqhp/admin/internal/model"
	"yqhp/admin/internal/types"
	"yqhp/common/utils"

	"gorm.io/gorm"
)

// OAuthLogic OAuth逻辑
type OAuthLogic struct {
	db *gorm.DB
}

// NewOAuthLogic 创建OAuth逻辑
func NewOAuthLogic(db *gorm.DB) *OAuthLogic {
	return &OAuthLogic{db: db}
}

// GetAuthURL 获取授权URL
func (l *OAuthLogic) GetAuthURL(providerCode, state string) (string, error) {
	provider, err := l.GetProvider(providerCode)
	if err != nil {
		return "", err
	}

	if provider.Status != 1 {
		return "", errors.New("该登录方式已禁用")
	}

	params := url.Values{}
	params.Set("client_id", provider.ClientID)
	params.Set("redirect_uri", provider.RedirectURI)
	params.Set("response_type", "code")
	params.Set("state", state)
	if provider.Scope != "" {
		params.Set("scope", provider.Scope)
	}

	return fmt.Sprintf("%s?%s", provider.AuthURL, params.Encode()), nil
}

// HandleCallback 处理OAuth回调
func (l *OAuthLogic) HandleCallback(providerCode, code, ip string) (*types.OAuthLoginResponse, error) {
	provider, err := l.GetProvider(providerCode)
	if err != nil {
		return nil, err
	}

	// 获取access_token
	tokenData, err := l.getAccessToken(provider, code)
	if err != nil {
		return nil, err
	}

	// 获取用户信息
	userInfo, err := l.getUserInfo(provider, tokenData)
	if err != nil {
		return nil, err
	}

	// 查找或创建用户
	return l.findOrCreateUser(provider, userInfo, tokenData, ip)
}

// getAccessToken 获取access_token
func (l *OAuthLogic) getAccessToken(provider *model.OAuthProvider, code string) (map[string]any, error) {
	params := url.Values{}
	params.Set("client_id", provider.ClientID)
	params.Set("client_secret", provider.ClientSecret)
	params.Set("code", code)
	params.Set("redirect_uri", provider.RedirectURI)
	params.Set("grant_type", "authorization_code")

	// 创建请求
	req, err := http.NewRequest("POST", provider.TokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// GitHub 需要 Accept: application/json 才会返回 JSON 格式
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	// 先尝试解析为 JSON
	if err := json.Unmarshal(body, &result); err != nil {
		// 如果不是 JSON，尝试解析为 URL 编码格式（如：access_token=xxx&token_type=bearer）
		values, parseErr := url.ParseQuery(string(body))
		if parseErr != nil {
			return nil, fmt.Errorf("解析响应失败: %v, 原始响应: %s", err, string(body))
		}
		result = make(map[string]any)
		for key, val := range values {
			if len(val) > 0 {
				result[key] = val[0]
			}
		}
	}

	if errMsg, ok := result["error"]; ok {
		errDesc := result["error_description"]
		if errDesc == nil {
			errDesc = errMsg
		}
		return nil, fmt.Errorf("获取access_token失败: %v", errDesc)
	}

	return result, nil
}

// getUserInfo 获取用户信息
func (l *OAuthLogic) getUserInfo(provider *model.OAuthProvider, tokenData map[string]any) (map[string]any, error) {
	accessToken, ok := tokenData["access_token"].(string)
	if !ok {
		return nil, errors.New("access_token无效")
	}

	req, err := http.NewRequest("GET", provider.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// findOrCreateUser 查找或创建用户
func (l *OAuthLogic) findOrCreateUser(provider *model.OAuthProvider, userInfo, tokenData map[string]any, ip string) (*types.OAuthLoginResponse, error) {
	// 解析用户信息
	openID := getStringValue(userInfo, "id", "openid", "open_id")
	unionID := getStringValue(userInfo, "unionid", "union_id")
	nickname := getStringValue(userInfo, "name", "nickname", "login")
	avatar := getStringValue(userInfo, "avatar_url", "avatar", "headimgurl")

	if openID == "" {
		return nil, errors.New("获取用户标识失败")
	}

	// 查找OAuth绑定记录
	var oauthUser model.OAuthUser
	err := l.db.Where("provider_code = ? AND open_id = ?", provider.Code, openID).First(&oauthUser).Error
	isNew := false

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 新用户，创建账号
		isNew = true
		user := &model.User{
			Username: fmt.Sprintf("%s_%s", provider.Code, openID[:8]),
			Password: utils.MD5(utils.GenerateRandomString(16)),
			Nickname: nickname,
			Avatar:   avatar,
			Status:   1,
		}
		if err := l.db.Create(user).Error; err != nil {
			return nil, err
		}

		// 创建OAuth绑定
		rawData, _ := json.Marshal(userInfo)
		accessToken, _ := tokenData["access_token"].(string)
		refreshToken, _ := tokenData["refresh_token"].(string)
		expiresIn, _ := tokenData["expires_in"].(float64)

		oauthUser = model.OAuthUser{
			UserID:       user.ID,
			ProviderCode: provider.Code,
			OpenID:       openID,
			UnionID:      unionID,
			Nickname:     nickname,
			Avatar:       avatar,
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresAt:    time.Now().Unix() + int64(expiresIn),
			RawData:      string(rawData),
		}
		if err := l.db.Create(&oauthUser).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	// 获取用户信息
	var user model.User
	if err := l.db.Preload("Roles").First(&user, oauthUser.UserID).Error; err != nil {
		return nil, err
	}

	// 检查用户状态
	if user.Status != 1 {
		return nil, errors.New("用户已被禁用")
	}

	// 执行登录
	token, err := auth.Login(user.ID)
	if err != nil {
		return nil, errors.New("登录失败")
	}

	// 更新最后登录时间
	now := time.Now()
	l.db.Model(&user).Updates(map[string]any{
		"last_login_at": now,
		"last_login_ip": ip,
	})

	return &types.OAuthLoginResponse{
		Token:    token,
		UserInfo: &user,
		IsNew:    isNew,
	}, nil
}

// GetProvider 获取OAuth提供商
func (l *OAuthLogic) GetProvider(code string) (*model.OAuthProvider, error) {
	var provider model.OAuthProvider
	if err := l.db.Where("code = ?", code).First(&provider).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

// ListProviders 获取所有启用的OAuth提供商
func (l *OAuthLogic) ListProviders() ([]model.OAuthProvider, error) {
	var providers []model.OAuthProvider
	if err := l.db.Where("status = 1").Order("sort ASC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// CreateProvider 创建OAuth提供商
func (l *OAuthLogic) CreateProvider(req *types.CreateProviderRequest) (*model.OAuthProvider, error) {
	provider := &model.OAuthProvider{
		Name:         req.Name,
		Code:         req.Code,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURI:  req.RedirectURI,
		AuthURL:      req.AuthURL,
		TokenURL:     req.TokenURL,
		UserInfoURL:  req.UserInfoURL,
		Scope:        req.Scope,
		Status:       req.Status,
		Sort:         req.Sort,
		Icon:         req.Icon,
		Remark:       req.Remark,
	}

	if err := l.db.Create(provider).Error; err != nil {
		return nil, err
	}

	return provider, nil
}

// UpdateProvider 更新OAuth提供商
func (l *OAuthLogic) UpdateProvider(req *types.UpdateProviderRequest) error {
	updates := map[string]any{
		"name":          req.Name,
		"client_id":     req.ClientID,
		"client_secret": req.ClientSecret,
		"redirect_uri":  req.RedirectURI,
		"auth_url":      req.AuthURL,
		"token_url":     req.TokenURL,
		"user_info_url": req.UserInfoURL,
		"scope":         req.Scope,
		"status":        req.Status,
		"sort":          req.Sort,
		"icon":          req.Icon,
		"remark":        req.Remark,
	}

	return l.db.Model(&model.OAuthProvider{}).Where("id = ?", req.ID).Updates(updates).Error
}

// DeleteProvider 删除OAuth提供商
func (l *OAuthLogic) DeleteProvider(id uint) error {
	return l.db.Delete(&model.OAuthProvider{}, id).Error
}

// BindOAuth 绑定第三方账号
func (l *OAuthLogic) BindOAuth(userID uint, providerCode, code string) error {
	provider, err := l.GetProvider(providerCode)
	if err != nil {
		return err
	}

	tokenData, err := l.getAccessToken(provider, code)
	if err != nil {
		return err
	}

	userInfo, err := l.getUserInfo(provider, tokenData)
	if err != nil {
		return err
	}

	openID := getStringValue(userInfo, "id", "openid", "open_id")
	unionID := getStringValue(userInfo, "unionid", "union_id")
	nickname := getStringValue(userInfo, "name", "nickname", "login")
	avatar := getStringValue(userInfo, "avatar_url", "avatar", "headimgurl")

	// 检查是否已绑定
	var count int64
	l.db.Model(&model.OAuthUser{}).Where("provider_code = ? AND open_id = ?", providerCode, openID).Count(&count)
	if count > 0 {
		return errors.New("该账号已被其他用户绑定")
	}

	rawData, _ := json.Marshal(userInfo)
	accessToken, _ := tokenData["access_token"].(string)
	refreshToken, _ := tokenData["refresh_token"].(string)
	expiresIn, _ := tokenData["expires_in"].(float64)

	oauthUser := &model.OAuthUser{
		UserID:       userID,
		ProviderCode: providerCode,
		OpenID:       openID,
		UnionID:      unionID,
		Nickname:     nickname,
		Avatar:       avatar,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Unix() + int64(expiresIn),
		RawData:      string(rawData),
	}

	return l.db.Create(oauthUser).Error
}

// UnbindOAuth 解绑第三方账号
func (l *OAuthLogic) UnbindOAuth(userID uint, providerCode string) error {
	return l.db.Where("user_id = ? AND provider_code = ?", userID, providerCode).Delete(&model.OAuthUser{}).Error
}

// GetUserBindings 获取用户绑定的第三方账号
func (l *OAuthLogic) GetUserBindings(userID uint) ([]model.OAuthUser, error) {
	var bindings []model.OAuthUser
	if err := l.db.Where("user_id = ?", userID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

// getStringValue 从map中获取字符串值
func getStringValue(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch val := v.(type) {
			case string:
				return val
			case float64:
				// JSON 数字默认解析为 float64
				return fmt.Sprintf("%.0f", val)
			case int:
				return fmt.Sprintf("%d", val)
			case int64:
				return fmt.Sprintf("%d", val)
			}
		}
	}
	return ""
}
