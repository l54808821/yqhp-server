package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"yqhp/gulu/internal/svc"
)

// AdminClient Admin 服务 API 客户端
type AdminClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewAdminClient 创建 Admin 客户端
func NewAdminClient() *AdminClient {
	return &AdminClient{
		baseURL: svc.Ctx.Config.Gulu.AdminURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// APIResponse Admin API 响应格式
type APIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

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
	ID        int64      `json:"id"`
	ParentID  int64      `json:"parentId"`
	Name      string     `json:"name"`
	Code      string     `json:"code"`
	Path      string     `json:"path"`
	Component string     `json:"component"`
	Redirect  string     `json:"redirect"`
	Icon      string     `json:"icon"`
	Sort      int64      `json:"sort"`
	IsHidden  bool       `json:"isHidden"`
	IsCache   bool       `json:"isCache"`
	IsFrame   bool       `json:"isFrame"`
	Type      int32      `json:"type"`
	Children  []MenuResp `json:"children,omitempty"`
}

// doRequest 执行 HTTP 请求
func (c *AdminClient) doRequest(method, path, token string) (*APIResponse, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("satoken", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API 错误: %s", apiResp.Message)
	}

	return &apiResp, nil
}

// GetUserInfo 获取用户信息
func (c *AdminClient) GetUserInfo(token string) (*UserInfoResp, error) {
	resp, err := c.doRequest("GET", "/api/auth/user-info", token)
	if err != nil {
		return nil, err
	}

	var userInfo UserInfoResp
	if err := json.Unmarshal(resp.Data, &userInfo); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %w", err)
	}

	return &userInfo, nil
}

// GetUserMenus 获取用户菜单（按应用过滤）
func (c *AdminClient) GetUserMenus(token, appCode string) ([]MenuResp, error) {
	path := fmt.Sprintf("/api/app/%s/menus", appCode)
	resp, err := c.doRequest("GET", path, token)
	if err != nil {
		return nil, err
	}

	var menus []MenuResp
	if err := json.Unmarshal(resp.Data, &menus); err != nil {
		return nil, fmt.Errorf("解析菜单失败: %w", err)
	}

	return menus, nil
}

// GetUserPermissionCodes 获取用户权限码（按应用过滤）
func (c *AdminClient) GetUserPermissionCodes(token, appCode string) ([]string, error) {
	path := fmt.Sprintf("/api/app/%s/permissions", appCode)
	resp, err := c.doRequest("GET", path, token)
	if err != nil {
		return nil, err
	}

	var codes []string
	if err := json.Unmarshal(resp.Data, &codes); err != nil {
		return nil, fmt.Errorf("解析权限码失败: %w", err)
	}

	return codes, nil
}
