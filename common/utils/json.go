package utils

import (
	"github.com/bytedance/sonic"
)

// ToJSON 将对象转换为JSON字符串
func ToJSON(v any) (string, error) {
	bytes, err := sonic.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ToJSONBytes 将对象转换为JSON字节数组
func ToJSONBytes(v any) ([]byte, error) {
	return sonic.Marshal(v)
}

// ToJSONPretty 将对象转换为格式化的JSON字符串
func ToJSONPretty(v any) (string, error) {
	bytes, err := sonic.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// FromJSON 将JSON字符串转换为对象
func FromJSON[T any](s string) (T, error) {
	var v T
	err := sonic.UnmarshalString(s, &v)
	return v, err
}

// FromJSONBytes 将JSON字节数组转换为对象
func FromJSONBytes[T any](data []byte) (T, error) {
	var v T
	err := sonic.Unmarshal(data, &v)
	return v, err
}

// Unmarshal 将JSON字节数组解析到指定对象
func Unmarshal(data []byte, v any) error {
	return sonic.Unmarshal(data, v)
}

// UnmarshalString 将JSON字符串解析到指定对象
func UnmarshalString(s string, v any) error {
	return sonic.UnmarshalString(s, v)
}

// Marshal 将对象序列化为JSON字节数组
func Marshal(v any) ([]byte, error) {
	return sonic.Marshal(v)
}

// MarshalString 将对象序列化为JSON字符串
func MarshalString(v any) (string, error) {
	return sonic.MarshalString(v)
}

// ToMap 将对象转换为Map
func ToMap(v any) (map[string]any, error) {
	bytes, err := sonic.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := sonic.Unmarshal(bytes, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// FromMap 将Map转换为对象
func FromMap[T any](m map[string]any) (T, error) {
	var v T
	bytes, err := sonic.Marshal(m)
	if err != nil {
		return v, err
	}
	if err := sonic.Unmarshal(bytes, &v); err != nil {
		return v, err
	}
	return v, nil
}

// Valid 验证是否为有效的JSON
func Valid(data []byte) bool {
	return sonic.Valid(data)
}

// ValidString 验证字符串是否为有效的JSON
func ValidString(s string) bool {
	return sonic.ValidString(s)
}

// Get 从JSON中获取指定路径的值（使用sonic的ast功能）
func Get(data []byte, path ...any) (any, error) {
	node, err := sonic.Get(data, path...)
	if err != nil {
		return nil, err
	}
	return node.Interface()
}

// GetString 从JSON中获取指定路径的字符串值
func GetString(data []byte, path ...any) (string, error) {
	node, err := sonic.Get(data, path...)
	if err != nil {
		return "", err
	}
	return node.String()
}

// GetInt 从JSON中获取指定路径的整数值
func GetInt(data []byte, path ...any) (int64, error) {
	node, err := sonic.Get(data, path...)
	if err != nil {
		return 0, err
	}
	return node.Int64()
}

// GetFloat 从JSON中获取指定路径的浮点数值
func GetFloat(data []byte, path ...any) (float64, error) {
	node, err := sonic.Get(data, path...)
	if err != nil {
		return 0, err
	}
	return node.Float64()
}

// GetBool 从JSON中获取指定路径的布尔值
func GetBool(data []byte, path ...any) (bool, error) {
	node, err := sonic.Get(data, path...)
	if err != nil {
		return false, err
	}
	return node.Bool()
}
