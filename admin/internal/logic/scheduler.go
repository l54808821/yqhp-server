package logic

import (
	"yqhp/common/scheduler"
)

var globalScheduler *scheduler.Scheduler

// SetScheduler 设置全局调度器实例
func SetScheduler(s *scheduler.Scheduler) {
	globalScheduler = s
}

// GetScheduler 获取全局调度器实例
func GetScheduler() *scheduler.Scheduler {
	return globalScheduler
}
