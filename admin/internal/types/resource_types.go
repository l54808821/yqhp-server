package types

// CreateResourceRequest 创建资源请求
type CreateResourceRequest struct {
	ParentID  uint   `json:"parentId"`
	Name      string `json:"name" validate:"required"`
	Code      string `json:"code"`
	Type      int8   `json:"type" validate:"required"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Redirect  string `json:"redirect"`
	Icon      string `json:"icon"`
	Sort      int    `json:"sort"`
	IsHidden  bool   `json:"isHidden"`
	IsCache   bool   `json:"isCache"`
	IsFrame   bool   `json:"isFrame"`
	Status    int8   `json:"status"`
	Remark    string `json:"remark"`
}

// UpdateResourceRequest 更新资源请求
type UpdateResourceRequest struct {
	ID        uint   `json:"id" validate:"required"`
	ParentID  uint   `json:"parentId"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Type      int8   `json:"type"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Redirect  string `json:"redirect"`
	Icon      string `json:"icon"`
	Sort      int    `json:"sort"`
	IsHidden  bool   `json:"isHidden"`
	IsCache   bool   `json:"isCache"`
	IsFrame   bool   `json:"isFrame"`
	Status    int8   `json:"status"`
	Remark    string `json:"remark"`
}
