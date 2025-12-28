package auth

import (
	"yqhp/admin/internal/model"

	"gorm.io/gorm"
)

// PermissionService 权限服务
type PermissionService struct {
	db *gorm.DB
}

// NewPermissionService 创建权限服务
func NewPermissionService(db *gorm.DB) *PermissionService {
	return &PermissionService{db: db}
}

// GetUserRoles 获取用户角色列表
func (s *PermissionService) GetUserRoles(userID int64) ([]string, error) {
	var roleCodes []string
	err := s.db.Model(&model.SysUserRole{}).
		Select("sys_role.code").
		Joins("JOIN sys_role ON sys_role.id = sys_user_role.role_id").
		Where("sys_user_role.user_id = ? AND sys_user_role.is_delete = ? AND sys_role.is_delete = ? AND sys_role.status = 1", userID, false, false).
		Pluck("code", &roleCodes).Error
	if err != nil {
		return nil, err
	}
	return roleCodes, nil
}

// GetUserPermissions 获取用户权限列表
func (s *PermissionService) GetUserPermissions(userID int64) ([]string, error) {
	var permissions []string

	// 获取用户的所有角色
	var roleIDs []int64
	err := s.db.Model(&model.SysUserRole{}).
		Where("user_id = ? AND is_delete = ?", userID, false).
		Pluck("role_id", &roleIDs).Error
	if err != nil {
		return nil, err
	}

	if len(roleIDs) == 0 {
		return permissions, nil
	}

	// 获取角色关联的资源权限
	var resourceIDs []int64
	err = s.db.Model(&model.SysRoleResource{}).
		Where("role_id IN ? AND is_delete = ?", roleIDs, false).
		Pluck("resource_id", &resourceIDs).Error
	if err != nil {
		return nil, err
	}

	if len(resourceIDs) == 0 {
		return permissions, nil
	}

	// 获取资源的权限标识
	err = s.db.Model(&model.SysResource{}).
		Where("id IN ? AND code != '' AND code IS NOT NULL AND status = 1 AND is_delete = ?", resourceIDs, false).
		Pluck("code", &permissions).Error
	if err != nil {
		return nil, err
	}

	return permissions, nil
}

// HasRole 判断用户是否拥有角色
func (s *PermissionService) HasRole(userID int64, roleCode string) (bool, error) {
	roles, err := s.GetUserRoles(userID)
	if err != nil {
		return false, err
	}

	for _, role := range roles {
		if role == roleCode {
			return true, nil
		}
	}
	return false, nil
}

// HasPermission 判断用户是否拥有权限
func (s *PermissionService) HasPermission(userID int64, permissionCode string) (bool, error) {
	permissions, err := s.GetUserPermissions(userID)
	if err != nil {
		return false, err
	}

	for _, perm := range permissions {
		if perm == permissionCode || matchWildcard(perm, permissionCode) {
			return true, nil
		}
	}
	return false, nil
}

// HasAnyRole 判断用户是否拥有任一角色
func (s *PermissionService) HasAnyRole(userID int64, roleCodes ...string) (bool, error) {
	roles, err := s.GetUserRoles(userID)
	if err != nil {
		return false, err
	}

	roleMap := make(map[string]bool)
	for _, role := range roles {
		roleMap[role] = true
	}

	for _, code := range roleCodes {
		if roleMap[code] {
			return true, nil
		}
	}
	return false, nil
}

// HasAnyPermission 判断用户是否拥有任一权限
func (s *PermissionService) HasAnyPermission(userID int64, permissionCodes ...string) (bool, error) {
	permissions, err := s.GetUserPermissions(userID)
	if err != nil {
		return false, err
	}

	for _, perm := range permissions {
		for _, code := range permissionCodes {
			if perm == code || matchWildcard(perm, code) {
				return true, nil
			}
		}
	}
	return false, nil
}

// matchWildcard 通配符匹配
// 支持 * 匹配任意字符
// 如: user:* 匹配 user:add, user:edit, user:delete
// 如: user:*:view 匹配 user:info:view, user:list:view
func matchWildcard(pattern, target string) bool {
	if pattern == "*" {
		return true
	}

	pLen, tLen := len(pattern), len(target)
	pIdx, tIdx := 0, 0
	starIdx, matchIdx := -1, 0

	for tIdx < tLen {
		if pIdx < pLen && (pattern[pIdx] == target[tIdx] || pattern[pIdx] == '?') {
			pIdx++
			tIdx++
		} else if pIdx < pLen && pattern[pIdx] == '*' {
			starIdx = pIdx
			matchIdx = tIdx
			pIdx++
		} else if starIdx != -1 {
			pIdx = starIdx + 1
			matchIdx++
			tIdx = matchIdx
		} else {
			return false
		}
	}

	for pIdx < pLen && pattern[pIdx] == '*' {
		pIdx++
	}

	return pIdx == pLen
}
