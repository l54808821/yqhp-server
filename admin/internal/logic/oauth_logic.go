package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"yqhp/admin/internal/auth"
	"yqhp/admin/internal/ctxutil"
	"yqhp/admin/internal/model"
	"yqhp/admin/internal/query"
	"yqhp/admin/internal/svc"
	"yqhp/admin/internal/types"
	"yqhp/common/utils"

	"github.com/gofiber/fiber/v2"
)

// OAuthLogic OAuth逻辑
type OAuthLogic struct {
	ctx   context.Context
	fiber *fiber.Ctx
}

// NewOAuthLogic 创建OAuth逻辑
func NewOAuthLogic(c *fiber.Ctx) *OAuthLogic {
	return &OAuthLogic{ctx: c.UserContext(), fiber: c}
}

func (l *OAuthLogic) db() *query.Query {
	return svc.Ctx.Query
}

// GetAuthURL 获取授权URL
func (l *OAuthLogic) GetAuthURL(providerCode, state string) (string, error) {
	provider, err := l.GetProvider(providerCode)
	if err != nil {
		return "", err
	}

	if model.GetInt32(provider.Status) != 1 {
		return "", errors.New("该登录方式已禁用")
	}

	params := url.Values{}
	params.Set("client_id", model.GetString(provider.ClientID))
	params.Set("redirect_uri", model.GetString(provider.RedirectURI))
	params.Set("response_type", "code")
	params.Set("state", state)
	if scope := model.GetString(provider.Scope); scope != "" {
		params.Set("scope", scope)
	}

	return fmt.Sprintf("%s?%s", model.GetString(provider.AuthURL), params.Encode()), nil
}

// HandleCallback 处理OAuth回调
func (l *OAuthLogic) HandleCallback(providerCode, code, ip string) (*types.OAuthLoginResponse, error) {
	provider, err := l.GetProvider(providerCode)
	if err != nil {
		return nil, err
	}

	tokenData, err := l.getAccessToken(provider, code)
	if err != nil {
		return nil, err
	}

	userInfo, err := l.getUserInfo(provider, tokenData)
	if err != nil {
		return nil, err
	}

	return l.findOrCreateUser(provider, userInfo, tokenData, ip)
}

// getAccessToken 获取access_token
func (l *OAuthLogic) getAccessToken(provider *model.SysOauthProvider, code string) (map[string]any, error) {
	params := url.Values{}
	params.Set("client_id", model.GetString(provider.ClientID))
	params.Set("client_secret", model.GetString(provider.ClientSecret))
	params.Set("code", code)
	params.Set("redirect_uri", model.GetString(provider.RedirectURI))
	params.Set("grant_type", "authorization_code")

	req, err := http.NewRequest("POST", model.GetString(provider.TokenURL), strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
	if err := json.Unmarshal(body, &result); err != nil {
		values, parseErr := url.ParseQuery(string(body))
		if parseErr != nil {
			return nil, fmt.Errorf("解析响应失败: %v", err)
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
func (l *OAuthLogic) getUserInfo(provider *model.SysOauthProvider, tokenData map[string]any) (map[string]any, error) {
	accessToken, ok := tokenData["access_token"].(string)
	if !ok {
		return nil, errors.New("access_token无效")
	}

	req, err := http.NewRequest("GET", model.GetString(provider.UserInfoURL), nil)
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
func (l *OAuthLogic) findOrCreateUser(provider *model.SysOauthProvider, userInfo, tokenData map[string]any, ip string) (*types.OAuthLoginResponse, error) {
	openID := getStringValue(userInfo, "id", "openid", "open_id")
	unionID := getStringValue(userInfo, "unionid", "union_id")
	nickname := getStringValue(userInfo, "name", "nickname", "login")
	avatar := getStringValue(userInfo, "avatar_url", "avatar", "headimgurl")

	if openID == "" {
		return nil, errors.New("获取用户标识失败")
	}

	ou := l.db().SysOauthUser
	oauthUser, err := ou.WithContext(l.ctx).Where(ou.ProviderCode.Eq(provider.Code), ou.OpenID.Eq(openID), ou.IsDelete.Is(false)).First()
	isNew := false

	if err != nil {
		// 新用户，创建账号
		isNew = true
		user := &model.SysUser{
			Username: fmt.Sprintf("%s_%s", provider.Code, openID[:8]),
			Password: utils.MD5(utils.GenerateRandomString(16)),
			Nickname: model.StringPtr(nickname),
			Avatar:   model.StringPtr(avatar),
			Status:   model.Int32Ptr(1),
			IsDelete: model.BoolPtr(false),
		}
		if err := l.db().SysUser.WithContext(l.ctx).Create(user); err != nil {
			return nil, err
		}

		rawData, _ := json.Marshal(userInfo)
		accessToken, _ := tokenData["access_token"].(string)
		refreshToken, _ := tokenData["refresh_token"].(string)
		expiresIn, _ := tokenData["expires_in"].(float64)

		oauthUser = &model.SysOauthUser{
			UserID:       model.Int64Ptr(user.ID),
			ProviderCode: model.StringPtr(provider.Code),
			OpenID:       model.StringPtr(openID),
			UnionID:      model.StringPtr(unionID),
			Nickname:     model.StringPtr(nickname),
			Avatar:       model.StringPtr(avatar),
			AccessToken:  model.StringPtr(accessToken),
			RefreshToken: model.StringPtr(refreshToken),
			ExpiresAt:    model.Int64Ptr(time.Now().Unix() + int64(expiresIn)),
			RawData:      model.StringPtr(string(rawData)),
			IsDelete:     model.BoolPtr(false),
		}
		if err := ou.WithContext(l.ctx).Create(oauthUser); err != nil {
			return nil, err
		}
	}

	u := l.db().SysUser
	user, err := u.WithContext(l.ctx).Where(u.ID.Eq(model.GetInt64(oauthUser.UserID)), u.IsDelete.Is(false)).First()
	if err != nil {
		return nil, err
	}

	if model.GetInt32(user.Status) != 1 {
		return nil, errors.New("用户已被禁用")
	}

	token, err := auth.Login(user.ID)
	if err != nil {
		return nil, errors.New("登录失败")
	}

	now := time.Now()
	u.WithContext(l.ctx).Where(u.ID.Eq(user.ID)).Updates(map[string]any{
		"last_login_at": now,
		"last_login_ip": ip,
	})

	// 保存用户token记录
	l.saveUserToken(user.ID, token, ip, now)

	return &types.OAuthLoginResponse{
		Token:    token,
		UserInfo: types.ToUserInfo(user),
		IsNew:    isNew,
	}, nil
}

// GetProvider 获取OAuth提供商
func (l *OAuthLogic) GetProvider(code string) (*model.SysOauthProvider, error) {
	p := l.db().SysOauthProvider
	return p.WithContext(l.ctx).Where(p.Code.Eq(code), p.IsDelete.Is(false)).First()
}

// ListProviders 获取OAuth提供商列表
func (l *OAuthLogic) ListProviders(req *types.ListProvidersRequest) ([]*types.OAuthProviderInfo, int64, error) {
	p := l.db().SysOauthProvider
	q := p.WithContext(l.ctx).Where(p.IsDelete.Is(false))

	if req.Name != "" {
		q = q.Where(p.Name.Like("%" + req.Name + "%"))
	}
	if req.Status != nil {
		q = q.Where(p.Status.Eq(int32(*req.Status)))
	}

	total, _ := q.Count()

	if req.Page > 0 && req.PageSize > 0 {
		q = q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	}

	providers, err := q.Order(p.Sort).Find()
	if err != nil {
		return nil, 0, err
	}
	return types.ToOAuthProviderInfoList(providers), total, nil
}

// ListAllProviders 获取所有启用的OAuth提供商
func (l *OAuthLogic) ListAllProviders() ([]*types.OAuthProviderInfo, error) {
	p := l.db().SysOauthProvider
	providers, err := p.WithContext(l.ctx).Where(p.Status.Eq(1), p.IsDelete.Is(false)).Order(p.Sort).Find()
	if err != nil {
		return nil, err
	}
	return types.ToOAuthProviderInfoList(providers), nil
}

// CreateProvider 创建OAuth提供商
func (l *OAuthLogic) CreateProvider(req *types.CreateProviderRequest) (*types.OAuthProviderInfo, error) {
	userID := ctxutil.GetUserID(l.ctx)
	provider := &model.SysOauthProvider{
		Name:         req.Name,
		Code:         req.Code,
		ClientID:     model.StringPtr(req.ClientID),
		ClientSecret: model.StringPtr(req.ClientSecret),
		RedirectURI:  model.StringPtr(req.RedirectURI),
		AuthURL:      model.StringPtr(req.AuthURL),
		TokenURL:     model.StringPtr(req.TokenURL),
		UserInfoURL:  model.StringPtr(req.UserInfoURL),
		Scope:        model.StringPtr(req.Scope),
		Status:       model.Int32Ptr(int32(req.Status)),
		Sort:         model.Int64Ptr(int64(req.Sort)),
		Icon:         model.StringPtr(req.Icon),
		Remark:       model.StringPtr(req.Remark),
		IsDelete:     model.BoolPtr(false),
		CreatedBy:    model.Int64Ptr(userID),
		UpdatedBy:    model.Int64Ptr(userID),
	}

	if err := l.db().SysOauthProvider.WithContext(l.ctx).Create(provider); err != nil {
		return nil, err
	}

	return types.ToOAuthProviderInfo(provider), nil
}

// UpdateProvider 更新OAuth提供商
func (l *OAuthLogic) UpdateProvider(req *types.UpdateProviderRequest) error {
	userID := ctxutil.GetUserID(l.ctx)
	p := l.db().SysOauthProvider
	_, err := p.WithContext(l.ctx).Where(p.ID.Eq(int64(req.ID))).Updates(map[string]any{
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
		"updated_by":    userID,
	})
	return err
}

// DeleteProvider 删除OAuth提供商
func (l *OAuthLogic) DeleteProvider(id int64) error {
	p := l.db().SysOauthProvider
	_, err := p.WithContext(l.ctx).Where(p.ID.Eq(id)).Update(p.IsDelete, true)
	return err
}

// BindOAuth 绑定第三方账号
func (l *OAuthLogic) BindOAuth(userID int64, providerCode, code string) error {
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

	ou := l.db().SysOauthUser
	count, _ := ou.WithContext(l.ctx).Where(ou.ProviderCode.Eq(providerCode), ou.OpenID.Eq(openID), ou.IsDelete.Is(false)).Count()
	if count > 0 {
		return errors.New("该账号已被其他用户绑定")
	}

	rawData, _ := json.Marshal(userInfo)
	accessToken, _ := tokenData["access_token"].(string)
	refreshToken, _ := tokenData["refresh_token"].(string)
	expiresIn, _ := tokenData["expires_in"].(float64)

	oauthUser := &model.SysOauthUser{
		UserID:       model.Int64Ptr(userID),
		ProviderCode: model.StringPtr(providerCode),
		OpenID:       model.StringPtr(openID),
		UnionID:      model.StringPtr(unionID),
		Nickname:     model.StringPtr(nickname),
		Avatar:       model.StringPtr(avatar),
		AccessToken:  model.StringPtr(accessToken),
		RefreshToken: model.StringPtr(refreshToken),
		ExpiresAt:    model.Int64Ptr(time.Now().Unix() + int64(expiresIn)),
		RawData:      model.StringPtr(string(rawData)),
		IsDelete:     model.BoolPtr(false),
	}

	return ou.WithContext(l.ctx).Create(oauthUser)
}

// UnbindOAuth 解绑第三方账号
func (l *OAuthLogic) UnbindOAuth(userID int64, providerCode string) error {
	ou := l.db().SysOauthUser
	_, err := ou.WithContext(l.ctx).Where(ou.UserID.Eq(userID), ou.ProviderCode.Eq(providerCode), ou.IsDelete.Is(false)).Update(ou.IsDelete, true)
	return err
}

// GetUserBindings 获取用户绑定的第三方账号
func (l *OAuthLogic) GetUserBindings(userID int64) ([]*types.OAuthBindingInfo, error) {
	ou := l.db().SysOauthUser
	bindings, err := ou.WithContext(l.ctx).Where(ou.UserID.Eq(userID), ou.IsDelete.Is(false)).Find()
	if err != nil {
		return nil, err
	}
	return types.ToOAuthBindingInfoList(bindings), nil
}

// saveUserToken 保存用户token记录
func (l *OAuthLogic) saveUserToken(userID int64, token string, ip string, now time.Time) {
	expireAt := now.Add(24 * time.Hour)
	var userAgent string
	if l.fiber != nil {
		userAgent = l.fiber.Get("User-Agent")
	}
	userToken := &model.SysUserToken{
		UserID:       model.Int64Ptr(userID),
		Token:        model.StringPtr(token),
		Device:       model.StringPtr("pc"),
		Platform:     model.StringPtr("web"),
		IP:           model.StringPtr(ip),
		UserAgent:    model.StringPtr(userAgent),
		ExpireAt:     &expireAt,
		LastActiveAt: &now,
		IsDelete:     model.BoolPtr(false),
	}
	l.db().SysUserToken.WithContext(l.ctx).Create(userToken)
}

// getStringValue 从map中获取字符串值
func getStringValue(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch val := v.(type) {
			case string:
				return val
			case float64:
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
