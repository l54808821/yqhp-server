package types

// OAuthLoginResponse OAuth登录响应
type OAuthLoginResponse struct {
	Token    string    `json:"token"`
	UserInfo *UserInfo `json:"userInfo"`
	IsNew    bool      `json:"isNew"`
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

// OAuthProviderInfo OAuth提供商响应（不包含敏感信息）
type OAuthProviderInfo struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	ClientID    string    `json:"clientId"`
	RedirectURI string    `json:"redirectUri"`
	AuthURL     string    `json:"authUrl"`
	TokenURL    string    `json:"tokenUrl"`
	UserInfoURL string    `json:"userInfoUrl"`
	Scope       string    `json:"scope"`
	Status      int32     `json:"status"`
	Sort        int64     `json:"sort"`
	Icon        string    `json:"icon"`
	Remark      string    `json:"remark"`
	CreatedBy   int64     `json:"createdBy"`
	UpdatedBy   int64     `json:"updatedBy"`
	CreatedAt   *DateTime `json:"createdAt"`
	UpdatedAt   *DateTime `json:"updatedAt"`
}

// OAuthBindingInfo 用户绑定的第三方账号信息
type OAuthBindingInfo struct {
	ID           int64     `json:"id"`
	ProviderCode string    `json:"providerCode"`
	OpenID       string    `json:"openId"`
	Nickname     string    `json:"nickname"`
	Avatar       string    `json:"avatar"`
	CreatedAt    *DateTime `json:"createdAt"`
}
