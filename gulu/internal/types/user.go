package types

// UserInfoResp 用户信息响应
type UserInfoResp struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

// MenuResp 菜单响应
type MenuResp struct {
	ID        int64  `json:"id"`
	ParentID  int64  `json:"parentId"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Redirect  string `json:"redirect"`
	Icon      string `json:"icon"`
	Sort      int64  `json:"sort"`
	IsHidden  bool   `json:"isHidden"`
	IsCache   bool   `json:"isCache"`
	IsFrame   bool   `json:"isFrame"`
	Type      int32  `json:"type"`
}
