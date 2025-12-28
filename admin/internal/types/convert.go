package types

import (
	"time"

	"github.com/jinzhu/copier"

	"yqhp/admin/internal/model"
)

// copierOption 配置 copier 转换选项
var copierOption = copier.Option{
	IgnoreEmpty: false,
	DeepCopy:    true,
	Converters: []copier.TypeConverter{
		// *time.Time -> *DateTime
		{
			SrcType: (*time.Time)(nil),
			DstType: (*DateTime)(nil),
			Fn: func(src interface{}) (interface{}, error) {
				if t, ok := src.(*time.Time); ok && t != nil {
					dt := DateTime(*t)
					return &dt, nil
				}
				return nil, nil
			},
		},
		// *string -> string
		{
			SrcType: (*string)(nil),
			DstType: "",
			Fn: func(src interface{}) (interface{}, error) {
				if s, ok := src.(*string); ok && s != nil {
					return *s, nil
				}
				return "", nil
			},
		},
		// *int32 -> int32
		{
			SrcType: (*int32)(nil),
			DstType: int32(0),
			Fn: func(src interface{}) (interface{}, error) {
				if i, ok := src.(*int32); ok && i != nil {
					return *i, nil
				}
				return int32(0), nil
			},
		},
		// *int64 -> int64
		{
			SrcType: (*int64)(nil),
			DstType: int64(0),
			Fn: func(src interface{}) (interface{}, error) {
				if i, ok := src.(*int64); ok && i != nil {
					return *i, nil
				}
				return int64(0), nil
			},
		},
		// *bool -> bool
		{
			SrcType: (*bool)(nil),
			DstType: false,
			Fn: func(src interface{}) (interface{}, error) {
				if b, ok := src.(*bool); ok && b != nil {
					return *b, nil
				}
				return false, nil
			},
		},
	},
}

// Copy 通用转换函数
func Copy[D any](src any) *D {
	if src == nil {
		return nil
	}
	var dst D
	copier.CopyWithOption(&dst, src, copierOption)
	return &dst
}

// CopyList 通用列表转换
func CopyList[D any, S any](src []*S) []*D {
	if src == nil {
		return nil
	}
	list := make([]*D, len(src))
	for i, s := range src {
		list[i] = Copy[D](s)
	}
	return list
}

// ========== User 相关 ==========

func ToUserInfo(u *model.SysUser) *UserInfo {
	info := Copy[UserInfo](u)
	if info != nil {
		info.Roles = []RoleRef{} // 初始化空数组
	}
	return info
}

func ToUserInfoWithRoles(u *model.SysUser, roles []*model.SysRole) *UserInfo {
	info := ToUserInfo(u)
	if info != nil {
		info.Roles = ToRoleRefList(roles)
	}
	return info
}

func ToUserInfoList(users []*model.SysUser) []*UserInfo {
	return CopyList[UserInfo](users)
}

func ToRoleRef(r *model.SysRole) *RoleRef {
	return Copy[RoleRef](r)
}

func ToRoleRefList(roles []*model.SysRole) []RoleRef {
	list := make([]RoleRef, len(roles))
	for i, r := range roles {
		if ref := ToRoleRef(r); ref != nil {
			list[i] = *ref
		}
	}
	return list
}

// ========== Role 相关 ==========

func ToRoleInfo(r *model.SysRole) *RoleInfo {
	return Copy[RoleInfo](r)
}

func ToRoleInfoList(roles []*model.SysRole) []*RoleInfo {
	return CopyList[RoleInfo](roles)
}

// ========== Dept 相关 ==========

func ToDeptInfo(d *model.SysDept) *DeptInfo {
	return Copy[DeptInfo](d)
}

func ToDeptInfoList(depts []*model.SysDept) []*DeptInfo {
	return CopyList[DeptInfo](depts)
}

func BuildDeptTree(depts []*model.SysDept, parentID int64) []DeptTreeInfo {
	var tree []DeptTreeInfo
	for _, dept := range depts {
		pid := GetInt64(dept.ParentID)
		if pid == parentID {
			node := DeptTreeInfo{DeptInfo: *ToDeptInfo(dept)}
			if children := BuildDeptTree(depts, dept.ID); len(children) > 0 {
				node.Children = children
			}
			tree = append(tree, node)
		}
	}
	return tree
}

// ========== Resource 相关 ==========

func ToResourceInfo(r *model.SysResource) *ResourceInfo {
	return Copy[ResourceInfo](r)
}

func ToResourceInfoList(resources []*model.SysResource) []*ResourceInfo {
	return CopyList[ResourceInfo](resources)
}

func BuildResourceTree(resources []*model.SysResource, parentID int64) []ResourceTreeInfo {
	var tree []ResourceTreeInfo
	for _, res := range resources {
		pid := GetInt64(res.ParentID)
		if pid == parentID {
			node := ResourceTreeInfo{ResourceInfo: *ToResourceInfo(res)}
			if children := BuildResourceTree(resources, res.ID); len(children) > 0 {
				node.Children = children
			}
			tree = append(tree, node)
		}
	}
	return tree
}

// ========== Dict 相关 ==========

func ToDictTypeInfo(d *model.SysDictType) *DictTypeInfo {
	return Copy[DictTypeInfo](d)
}

func ToDictTypeInfoList(types []*model.SysDictType) []*DictTypeInfo {
	return CopyList[DictTypeInfo](types)
}

func ToDictDataInfo(d *model.SysDictDatum) *DictDataInfo {
	return Copy[DictDataInfo](d)
}

func ToDictDataInfoList(data []*model.SysDictDatum) []*DictDataInfo {
	return CopyList[DictDataInfo](data)
}

// ========== Config 相关 ==========

func ToConfigInfo(c *model.SysConfig) *ConfigInfo {
	return Copy[ConfigInfo](c)
}

func ToConfigInfoList(configs []*model.SysConfig) []*ConfigInfo {
	return CopyList[ConfigInfo](configs)
}

// ========== Application 相关 ==========

func ToAppInfo(a *model.SysApplication) *AppInfo {
	return Copy[AppInfo](a)
}

func ToAppInfoList(apps []*model.SysApplication) []*AppInfo {
	return CopyList[AppInfo](apps)
}

// ========== Token/Log 相关 ==========

func ToTokenInfo(t *model.SysUserToken) *TokenInfo {
	return Copy[TokenInfo](t)
}

func ToTokenInfoList(tokens []*model.SysUserToken) []*TokenInfo {
	return CopyList[TokenInfo](tokens)
}

func ToLoginLogInfo(l *model.SysLoginLog) *LoginLogInfo {
	return Copy[LoginLogInfo](l)
}

func ToLoginLogInfoList(logs []*model.SysLoginLog) []*LoginLogInfo {
	return CopyList[LoginLogInfo](logs)
}

func ToOperationLogInfo(l *model.SysOperationLog) *OperationLogInfo {
	return Copy[OperationLogInfo](l)
}

func ToOperationLogInfoList(logs []*model.SysOperationLog) []*OperationLogInfo {
	return CopyList[OperationLogInfo](logs)
}

// ========== OAuth 相关 ==========

func ToOAuthProviderInfo(p *model.SysOauthProvider) *OAuthProviderInfo {
	return Copy[OAuthProviderInfo](p)
}

func ToOAuthProviderInfoList(providers []*model.SysOauthProvider) []*OAuthProviderInfo {
	return CopyList[OAuthProviderInfo](providers)
}

func ToOAuthBindingInfo(b *model.SysOauthUser) *OAuthBindingInfo {
	return Copy[OAuthBindingInfo](b)
}

func ToOAuthBindingInfoList(bindings []*model.SysOauthUser) []*OAuthBindingInfo {
	return CopyList[OAuthBindingInfo](bindings)
}
