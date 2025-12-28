package utils

import (
	"github.com/duke-git/lancet/v2/maputil"
)

// MapKeys 获取Map的所有键
func MapKeys[K comparable, V any](m map[K]V) []K {
	return maputil.Keys(m)
}

// MapValues 获取Map的所有值
func MapValues[K comparable, V any](m map[K]V) []V {
	return maputil.Values(m)
}

// MapMerge 合并多个Map
func MapMerge[K comparable, V any](maps ...map[K]V) map[K]V {
	return maputil.Merge(maps...)
}

// MapFilter 过滤Map
func MapFilter[K comparable, V any](m map[K]V, fn func(key K, value V) bool) map[K]V {
	return maputil.Filter(m, fn)
}

// MapForEach 遍历Map
func MapForEach[K comparable, V any](m map[K]V, fn func(key K, value V)) {
	maputil.ForEach(m, fn)
}

// MapContainsKey 判断Map是否包含某个键
func MapContainsKey[K comparable, V any](m map[K]V, key K) bool {
	_, ok := m[key]
	return ok
}

// MapGetOrDefault 获取Map值，如果不存在则返回默认值
func MapGetOrDefault[K comparable, V any](m map[K]V, key K, defaultValue V) V {
	if v, ok := m[key]; ok {
		return v
	}
	return defaultValue
}

