package types

import "yqhp/admin/internal/model"

// ToUserInfo 将 model.SysUser 转换为 UserInfo（不含角色）
func ToUserInfo(u *model.SysUser) *UserInfo {
	if u == nil {
		return nil
	}
	return &UserInfo{
		ID:          u.ID,
		Username:    u.Username,
		Nickname:    model.GetString(u.Nickname),
		Avatar:      model.GetString(u.Avatar),
		Email:       model.GetString(u.Email),
		Phone:       model.GetString(u.Phone),
		Gender:      model.GetInt32(u.Gender),
		Status:      model.GetInt32(u.Status),
		DeptID:      model.GetInt64(u.DeptID),
		Remark:      model.GetString(u.Remark),
		LastLoginAt: ToDateTime(u.LastLoginAt),
		LastLoginIP: model.GetString(u.LastLoginIP),
		CreatedAt:   ToDateTime(u.CreatedAt),
		UpdatedAt:   ToDateTime(u.UpdatedAt),
		Roles:       []RoleRef{},
	}
}

// ToUserInfoWithRoles 将 model.SysUser 转换为 UserInfo（含角色）
func ToUserInfoWithRoles(u *model.SysUser, roles []*model.SysRole) *UserInfo {
	info := ToUserInfo(u)
	if info == nil {
		return nil
	}
	info.Roles = ToRoleRefList(roles)
	return info
}

// ToUserInfoList 批量转换（不含角色）
func ToUserInfoList(users []*model.SysUser) []*UserInfo {
	list := make([]*UserInfo, len(users))
	for i, u := range users {
		list[i] = ToUserInfo(u)
	}
	return list
}

// ToRoleRef 转换角色引用
func ToRoleRef(r *model.SysRole) RoleRef {
	if r == nil {
		return RoleRef{}
	}
	return RoleRef{
		ID:     r.ID,
		Name:   r.Name,
		Code:   r.Code,
		Status: model.GetInt32(r.Status),
	}
}

// ToRoleRefList 批量转换角色引用
func ToRoleRefList(roles []*model.SysRole) []RoleRef {
	list := make([]RoleRef, len(roles))
	for i, r := range roles {
		list[i] = ToRoleRef(r)
	}
	return list
}

// ToRoleInfo 将 model.SysRole 转换为 RoleInfo
func ToRoleInfo(r *model.SysRole) *RoleInfo {
	if r == nil {
		return nil
	}
	return &RoleInfo{
		ID:        r.ID,
		AppID:     r.AppID,
		Name:      r.Name,
		Code:      r.Code,
		Sort:      model.GetInt64(r.Sort),
		Status:    model.GetInt32(r.Status),
		Remark:    model.GetString(r.Remark),
		CreatedAt: ToDateTime(r.CreatedAt),
		UpdatedAt: ToDateTime(r.UpdatedAt),
	}
}

// ToRoleInfoList 批量转换
func ToRoleInfoList(roles []*model.SysRole) []*RoleInfo {
	list := make([]*RoleInfo, len(roles))
	for i, r := range roles {
		list[i] = ToRoleInfo(r)
	}
	return list
}

// ToDeptInfo 将 model.SysDept 转换为 DeptInfo
func ToDeptInfo(d *model.SysDept) *DeptInfo {
	if d == nil {
		return nil
	}
	return &DeptInfo{
		ID:        d.ID,
		ParentID:  model.GetInt64(d.ParentID),
		Name:      d.Name,
		Code:      model.GetString(d.Code),
		Leader:    model.GetString(d.Leader),
		Phone:     model.GetString(d.Phone),
		Email:     model.GetString(d.Email),
		Sort:      model.GetInt64(d.Sort),
		Status:    model.GetInt32(d.Status),
		Remark:    model.GetString(d.Remark),
		CreatedAt: ToDateTime(d.CreatedAt),
		UpdatedAt: ToDateTime(d.UpdatedAt),
	}
}

// ToDeptInfoList 批量转换
func ToDeptInfoList(depts []*model.SysDept) []*DeptInfo {
	list := make([]*DeptInfo, len(depts))
	for i, d := range depts {
		list[i] = ToDeptInfo(d)
	}
	return list
}

// ToResourceInfo 将 model.SysResource 转换为 ResourceInfo
func ToResourceInfo(r *model.SysResource) *ResourceInfo {
	if r == nil {
		return nil
	}
	return &ResourceInfo{
		ID:        r.ID,
		AppID:     r.AppID,
		ParentID:  model.GetInt64(r.ParentID),
		Name:      r.Name,
		Code:      model.GetString(r.Code),
		Type:      r.Type,
		Path:      model.GetString(r.Path),
		Component: model.GetString(r.Component),
		Redirect:  model.GetString(r.Redirect),
		Icon:      model.GetString(r.Icon),
		Sort:      model.GetInt64(r.Sort),
		IsHidden:  model.GetBool(r.IsHidden),
		IsCache:   model.GetBool(r.IsCache),
		IsFrame:   model.GetBool(r.IsFrame),
		Status:    model.GetInt32(r.Status),
		Remark:    model.GetString(r.Remark),
		CreatedAt: ToDateTime(r.CreatedAt),
		UpdatedAt: ToDateTime(r.UpdatedAt),
	}
}

// ToResourceInfoList 批量转换
func ToResourceInfoList(resources []*model.SysResource) []*ResourceInfo {
	list := make([]*ResourceInfo, len(resources))
	for i, r := range resources {
		list[i] = ToResourceInfo(r)
	}
	return list
}

// ToDictTypeInfo 转换字典类型
func ToDictTypeInfo(d *model.SysDictType) *DictTypeInfo {
	if d == nil {
		return nil
	}
	return &DictTypeInfo{
		ID:        d.ID,
		Name:      d.Name,
		Code:      d.Code,
		Status:    model.GetInt32(d.Status),
		Remark:    model.GetString(d.Remark),
		CreatedAt: ToDateTime(d.CreatedAt),
		UpdatedAt: ToDateTime(d.UpdatedAt),
	}
}

// ToDictTypeInfoList 批量转换
func ToDictTypeInfoList(types []*model.SysDictType) []*DictTypeInfo {
	list := make([]*DictTypeInfo, len(types))
	for i, t := range types {
		list[i] = ToDictTypeInfo(t)
	}
	return list
}

// ToDictDataInfo 转换字典数据
func ToDictDataInfo(d *model.SysDictDatum) *DictDataInfo {
	if d == nil {
		return nil
	}
	return &DictDataInfo{
		ID:        d.ID,
		TypeCode:  d.TypeCode,
		Label:     d.Label,
		Value:     d.Value,
		Sort:      model.GetInt64(d.Sort),
		Status:    model.GetInt32(d.Status),
		IsDefault: model.GetBool(d.IsDefault),
		CssClass:  model.GetString(d.CSSClass),
		ListClass: model.GetString(d.ListClass),
		Remark:    model.GetString(d.Remark),
		CreatedAt: ToDateTime(d.CreatedAt),
		UpdatedAt: ToDateTime(d.UpdatedAt),
	}
}

// ToDictDataInfoList 批量转换
func ToDictDataInfoList(data []*model.SysDictDatum) []*DictDataInfo {
	list := make([]*DictDataInfo, len(data))
	for i, d := range data {
		list[i] = ToDictDataInfo(d)
	}
	return list
}

// ToConfigInfo 转换配置
func ToConfigInfo(c *model.SysConfig) *ConfigInfo {
	if c == nil {
		return nil
	}
	return &ConfigInfo{
		ID:        c.ID,
		Name:      c.Name,
		Key:       c.Key,
		Value:     model.GetString(c.Value),
		Type:      model.GetString(c.Type),
		IsBuilt:   model.GetBool(c.IsBuilt),
		Remark:    model.GetString(c.Remark),
		CreatedAt: ToDateTime(c.CreatedAt),
		UpdatedAt: ToDateTime(c.UpdatedAt),
	}
}

// ToConfigInfoList 批量转换
func ToConfigInfoList(configs []*model.SysConfig) []*ConfigInfo {
	list := make([]*ConfigInfo, len(configs))
	for i, c := range configs {
		list[i] = ToConfigInfo(c)
	}
	return list
}

// ToAppInfo 转换应用
func ToAppInfo(a *model.SysApplication) *AppInfo {
	if a == nil {
		return nil
	}
	return &AppInfo{
		ID:          a.ID,
		Name:        a.Name,
		Code:        a.Code,
		Description: model.GetString(a.Description),
		Icon:        model.GetString(a.Icon),
		Sort:        model.GetInt64(a.Sort),
		Status:      model.GetInt32(a.Status),
		CreatedAt:   ToDateTime(a.CreatedAt),
		UpdatedAt:   ToDateTime(a.UpdatedAt),
	}
}

// ToAppInfoList 批量转换
func ToAppInfoList(apps []*model.SysApplication) []*AppInfo {
	list := make([]*AppInfo, len(apps))
	for i, a := range apps {
		list[i] = ToAppInfo(a)
	}
	return list
}

// ToTokenInfo 转换令牌
func ToTokenInfo(t *model.SysUserToken) *TokenInfo {
	if t == nil {
		return nil
	}
	return &TokenInfo{
		ID:           t.ID,
		UserID:       model.GetInt64(t.UserID),
		Token:        model.GetString(t.Token),
		Device:       model.GetString(t.Device),
		Platform:     model.GetString(t.Platform),
		IP:           model.GetString(t.IP),
		ExpireAt:     ToDateTime(t.ExpireAt),
		LastActiveAt: ToDateTime(t.LastActiveAt),
		CreatedAt:    ToDateTime(t.CreatedAt),
	}
}

// ToTokenInfoList 批量转换
func ToTokenInfoList(tokens []*model.SysUserToken) []*TokenInfo {
	list := make([]*TokenInfo, len(tokens))
	for i, t := range tokens {
		list[i] = ToTokenInfo(t)
	}
	return list
}

// ToLoginLogInfo 转换登录日志
func ToLoginLogInfo(l *model.SysLoginLog) *LoginLogInfo {
	if l == nil {
		return nil
	}
	return &LoginLogInfo{
		ID:        l.ID,
		UserID:    model.GetInt64(l.UserID),
		Username:  model.GetString(l.Username),
		IP:        model.GetString(l.IP),
		Status:    model.GetInt32(l.Status),
		Message:   model.GetString(l.Message),
		LoginType: model.GetString(l.LoginType),
		CreatedAt: ToDateTime(l.CreatedAt),
	}
}

// ToLoginLogInfoList 批量转换
func ToLoginLogInfoList(logs []*model.SysLoginLog) []*LoginLogInfo {
	list := make([]*LoginLogInfo, len(logs))
	for i, l := range logs {
		list[i] = ToLoginLogInfo(l)
	}
	return list
}

// ToOperationLogInfo 转换操作日志
func ToOperationLogInfo(l *model.SysOperationLog) *OperationLogInfo {
	if l == nil {
		return nil
	}
	return &OperationLogInfo{
		ID:        l.ID,
		UserID:    model.GetInt64(l.UserID),
		Username:  model.GetString(l.Username),
		Module:    model.GetString(l.Module),
		Action:    model.GetString(l.Action),
		Method:    model.GetString(l.Method),
		Path:      model.GetString(l.Path),
		IP:        model.GetString(l.IP),
		Status:    model.GetInt32(l.Status),
		Duration:  model.GetInt64(l.Duration),
		CreatedAt: ToDateTime(l.CreatedAt),
	}
}

// ToOperationLogInfoList 批量转换
func ToOperationLogInfoList(logs []*model.SysOperationLog) []*OperationLogInfo {
	list := make([]*OperationLogInfo, len(logs))
	for i, l := range logs {
		list[i] = ToOperationLogInfo(l)
	}
	return list
}

// ToOAuthProviderInfo 转换OAuth提供商
func ToOAuthProviderInfo(p *model.SysOauthProvider) *OAuthProviderInfo {
	if p == nil {
		return nil
	}
	return &OAuthProviderInfo{
		ID:           p.ID,
		Name:         p.Name,
		Code:         p.Code,
		ClientID:     model.GetString(p.ClientID),
		ClientSecret: model.GetString(p.ClientSecret),
		RedirectURI:  model.GetString(p.RedirectURI),
		AuthURL:      model.GetString(p.AuthURL),
		TokenURL:     model.GetString(p.TokenURL),
		UserInfoURL:  model.GetString(p.UserInfoURL),
		Scope:        model.GetString(p.Scope),
		Status:       model.GetInt32(p.Status),
		Sort:         model.GetInt64(p.Sort),
		Icon:         model.GetString(p.Icon),
		Remark:       model.GetString(p.Remark),
		CreatedAt:    ToDateTime(p.CreatedAt),
		UpdatedAt:    ToDateTime(p.UpdatedAt),
	}
}

// ToOAuthProviderInfoList 批量转换
func ToOAuthProviderInfoList(providers []*model.SysOauthProvider) []*OAuthProviderInfo {
	list := make([]*OAuthProviderInfo, len(providers))
	for i, p := range providers {
		list[i] = ToOAuthProviderInfo(p)
	}
	return list
}

// ToDeptTreeInfo 转换部门树节点
func ToDeptTreeInfo(d *model.SysDept) DeptTreeInfo {
	return DeptTreeInfo{
		DeptInfo: *ToDeptInfo(d),
		Children: nil,
	}
}

// BuildDeptTree 构建部门树
func BuildDeptTree(depts []*model.SysDept, parentID int64) []DeptTreeInfo {
	var tree []DeptTreeInfo
	for _, dept := range depts {
		deptParentID := model.GetInt64(dept.ParentID)
		if deptParentID == parentID {
			node := ToDeptTreeInfo(dept)
			children := BuildDeptTree(depts, dept.ID)
			if len(children) > 0 {
				node.Children = children
			}
			tree = append(tree, node)
		}
	}
	return tree
}

// ToResourceTreeInfo 转换资源树节点
func ToResourceTreeInfo(r *model.SysResource) ResourceTreeInfo {
	return ResourceTreeInfo{
		ResourceInfo: *ToResourceInfo(r),
		Children:     nil,
	}
}

// BuildResourceTree 构建资源树
func BuildResourceTree(resources []*model.SysResource, parentID int64) []ResourceTreeInfo {
	var tree []ResourceTreeInfo
	for _, resource := range resources {
		resParentID := model.GetInt64(resource.ParentID)
		if resParentID == parentID {
			node := ToResourceTreeInfo(resource)
			children := BuildResourceTree(resources, resource.ID)
			if len(children) > 0 {
				node.Children = children
			}
			tree = append(tree, node)
		}
	}
	return tree
}

// ToOAuthBindingInfo 转换OAuth绑定信息
func ToOAuthBindingInfo(b *model.SysOauthUser) *OAuthBindingInfo {
	if b == nil {
		return nil
	}
	return &OAuthBindingInfo{
		ID:           b.ID,
		ProviderCode: model.GetString(b.ProviderCode),
		OpenID:       model.GetString(b.OpenID),
		Nickname:     model.GetString(b.Nickname),
		Avatar:       model.GetString(b.Avatar),
		CreatedAt:    ToDateTime(b.CreatedAt),
	}
}

// ToOAuthBindingInfoList 批量转换OAuth绑定信息
func ToOAuthBindingInfoList(bindings []*model.SysOauthUser) []*OAuthBindingInfo {
	list := make([]*OAuthBindingInfo, len(bindings))
	for i, b := range bindings {
		list[i] = ToOAuthBindingInfo(b)
	}
	return list
}
