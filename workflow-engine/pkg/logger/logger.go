// Package logger 提供简单的日志工具
package logger

import (
	"fmt"
	"os"
	"strings"
)

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	// 当前日志级别，默认为 Info
	currentLevel = LevelInfo
)

// SetLevel 设置日志级别
func SetLevel(level Level) {
	currentLevel = level
}

// SetLevelFromString 从字符串设置日志级别
func SetLevelFromString(level string) {
	switch strings.ToLower(level) {
	case "debug":
		currentLevel = LevelDebug
	case "info":
		currentLevel = LevelInfo
	case "warn", "warning":
		currentLevel = LevelWarn
	case "error":
		currentLevel = LevelError
	default:
		currentLevel = LevelInfo
	}
}

// EnableDebug 启用调试日志
func EnableDebug() {
	currentLevel = LevelDebug
}

// DisableDebug 禁用调试日志
func DisableDebug() {
	currentLevel = LevelInfo
}

// IsDebugEnabled 检查是否启用调试日志
func IsDebugEnabled() bool {
	return currentLevel <= LevelDebug
}

// Debug 输出调试日志
func Debug(format string, args ...interface{}) {
	if currentLevel <= LevelDebug {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// Info 输出信息日志
func Info(format string, args ...interface{}) {
	if currentLevel <= LevelInfo {
		fmt.Fprintf(os.Stderr, "[INFO] "+format+"\n", args...)
	}
}

// Warn 输出警告日志
func Warn(format string, args ...interface{}) {
	if currentLevel <= LevelWarn {
		fmt.Fprintf(os.Stderr, "[WARN] "+format+"\n", args...)
	}
}

// Error 输出错误日志
func Error(format string, args ...interface{}) {
	if currentLevel <= LevelError {
		fmt.Fprintf(os.Stderr, "[ERROR] "+format+"\n", args...)
	}
}
