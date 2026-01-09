package utils

import (
	"fmt"
	"runtime/debug"
)

// SafeGo 安全地启动一个 goroutine，自动捕获 panic 并记录日志
// 使用方式: utils.SafeGo(func() { ... })
func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// 记录 panic 信息和堆栈
				fmt.Printf("[SafeGo] goroutine panic recovered: %v\n", r)
				fmt.Printf("[SafeGo] stack trace:\n%s\n", debug.Stack())
			}
		}()
		fn()
	}()
}

// SafeGoWithCallback 安全地启动一个 goroutine，支持自定义 panic 处理回调
// 使用方式: utils.SafeGoWithCallback(func() { ... }, func(r interface{}) { ... })
func SafeGoWithCallback(fn func(), onPanic func(r interface{})) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// 记录 panic 信息和堆栈
				fmt.Printf("[SafeGoWithCallback] goroutine panic recovered: %v\n", r)
				fmt.Printf("[SafeGoWithCallback] stack trace:\n%s\n", debug.Stack())
				// 调用自定义回调
				if onPanic != nil {
					onPanic(r)
				}
			}
		}()
		fn()
	}()
}

// SafeGoWithName 安全地启动一个带名称的 goroutine，便于日志追踪
// 使用方式: utils.SafeGoWithName("monitor-execution", func() { ... })
func SafeGoWithName(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// 记录 panic 信息和堆栈
				fmt.Printf("[SafeGo:%s] goroutine panic recovered: %v\n", name, r)
				fmt.Printf("[SafeGo:%s] stack trace:\n%s\n", name, debug.Stack())
			}
		}()
		fn()
	}()
}
