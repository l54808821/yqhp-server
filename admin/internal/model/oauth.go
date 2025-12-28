package model

// OAuthProvider 第三方登录提供商模型
type OAuthProvider struct {
	BaseModel
	Name         string `gorm:"size:50;not null" json:"name"`
	Code         string `gorm:"size:50;uniqueIndex;not null" json:"code"` // wechat, feishu, github等
	ClientID     string `gorm:"size:255" json:"clientId"`
	ClientSecret string `gorm:"size:255" json:"-"`
	RedirectURI  string `gorm:"size:255" json:"redirectUri"`
	AuthURL      string `gorm:"size:255" json:"authUrl"`
	TokenURL     string `gorm:"size:255" json:"tokenUrl"`
	UserInfoURL  string `gorm:"size:255" json:"userInfoUrl"`
	Scope        string `gorm:"size:255" json:"scope"`
	Status       int8   `gorm:"default:1" json:"status"` // 0:禁用 1:启用
	Sort         int    `gorm:"default:0" json:"sort"`
	Icon         string `gorm:"size:100" json:"icon"`
	Remark       string `gorm:"size:500" json:"remark"`
}

// TableName 表名
func (OAuthProvider) TableName() string {
	return "sys_oauth_provider"
}

// OAuthUser 第三方登录用户绑定模型
type OAuthUser struct {
	BaseModel
	UserID       uint   `gorm:"index" json:"userId"`
	ProviderCode string `gorm:"size:50;index" json:"providerCode"`
	OpenID       string `gorm:"size:255;index" json:"openId"`
	UnionID      string `gorm:"size:255" json:"unionId"`
	Nickname     string `gorm:"size:100" json:"nickname"`
	Avatar       string `gorm:"size:255" json:"avatar"`
	AccessToken  string `gorm:"size:500" json:"-"`
	RefreshToken string `gorm:"size:500" json:"-"`
	ExpiresAt    int64  `json:"expiresAt"`
	RawData      string `gorm:"type:text" json:"-"` // 原始数据JSON
}

// TableName 表名
func (OAuthUser) TableName() string {
	return "sys_oauth_user"
}
