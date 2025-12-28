package types

import "yqhp/admin/internal/model"

// OAuthLoginResponse OAuth登录响应
type OAuthLoginResponse struct {
	Token    string      `json:"token"`
	UserInfo *model.User `json:"userInfo"`
	IsNew    bool        `json:"isNew"` // 是否新用户
}

// CreateProviderRequest 创建OAuth提供商请求
type CreateProviderRequest struct {
	Name         string `json:"name" validate:"required"`
	Code         string `json:"code" validate:"required"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RedirectURI  string `json:"redirectUri"`
	AuthURL      string `json:"authUrl"`
	TokenURL     string `json:"tokenUrl"`
	UserInfoURL  string `json:"userInfoUrl"`
	Scope        string `json:"scope"`
	Status       int8   `json:"status"`
	Sort         int    `json:"sort"`
	Icon         string `json:"icon"`
	Remark       string `json:"remark"`
}

// UpdateProviderRequest 更新OAuth提供商请求
type UpdateProviderRequest struct {
	ID           uint   `json:"id" validate:"required"`
	Name         string `json:"name"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RedirectURI  string `json:"redirectUri"`
	AuthURL      string `json:"authUrl"`
	TokenURL     string `json:"tokenUrl"`
	UserInfoURL  string `json:"userInfoUrl"`
	Scope        string `json:"scope"`
	Status       int8   `json:"status"`
	Sort         int    `json:"sort"`
	Icon         string `json:"icon"`
	Remark       string `json:"remark"`
}

// ListProvidersRequest OAuth提供商列表请求
type ListProvidersRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Status   *int8  `json:"status"`
}
