package utils

import (
	"github.com/duke-git/lancet/v2/random"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/duke-git/lancet/v2/cryptor"
)

// IsEmpty 判断字符串是否为空
func IsEmpty(s string) bool {
	return strutil.IsBlank(s)
}

// IsNotEmpty 判断字符串是否不为空
func IsNotEmpty(s string) bool {
	return !IsEmpty(s)
}

// Trim 去除字符串两端空格
func Trim(s string) string {
	return strutil.Trim(s)
}

// GenerateUUID 生成UUID
func GenerateUUID() string {
	uuid, _ := random.UUIdV4()
	return uuid
}

// GenerateRandomString 生成随机字符串
func GenerateRandomString(length int) string {
	return random.RandString(length)
}

// MD5 MD5加密
func MD5(s string) string {
	return cryptor.Md5String(s)
}

// SHA256 SHA256加密
func SHA256(s string) string {
	return cryptor.Sha256(s)
}

// Contains 判断字符串是否包含子串
func Contains(s, substr string) bool {
	return strutil.ContainsAny(s, []string{substr})
}

// CamelCase 转驼峰命名
func CamelCase(s string) string {
	return strutil.CamelCase(s)
}

// SnakeCase 转蛇形命名
func SnakeCase(s string) string {
	return strutil.SnakeCase(s)
}

