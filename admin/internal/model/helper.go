package model

// 类型别名，方便迁移
type (
	Application   = SysApplication
	User          = SysUser
	Role          = SysRole
	Resource      = SysResource
	Dept          = SysDept
	DictType      = SysDictType
	DictData      = SysDictDatum
	OAuthProvider = SysOauthProvider
	OAuthUser     = SysOauthUser
	UserToken     = SysUserToken
	LoginLog      = SysLoginLog
	OperationLog  = SysOperationLog
	UserRole      = SysUserRole
	UserApp       = SysUserApp
	RoleResource  = SysRoleResource
	TableView     = SysTableView
)

// 内置应用编码常量
const (
	AppCodeAdmin = "admin" // 后台管理系统
)

// 辅助函数：创建 bool 指针
func BoolPtr(b bool) *bool {
	return &b
}

// 辅助函数：创建 int32 指针
func Int32Ptr(i int32) *int32 {
	return &i
}

// 辅助函数：创建 int64 指针
func Int64Ptr(i int64) *int64 {
	return &i
}

// 辅助函数：创建 string 指针
func StringPtr(s string) *string {
	return &s
}

// 辅助函数：安全获取 bool 值
func GetBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// 辅助函数：安全获取 int32 值
func GetInt32(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}

// 辅助函数：安全获取 int64 值
func GetInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

// 辅助函数：安全获取 string 值
func GetString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ResourceWithChildren 带子节点的资源（用于树形结构）
type ResourceWithChildren struct {
	SysResource
	Children []ResourceWithChildren `json:"children,omitempty"`
}

// DeptWithChildren 带子节点的部门（用于树形结构）
type DeptWithChildren struct {
	SysDept
	Children []DeptWithChildren `json:"children,omitempty"`
}
