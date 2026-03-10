package scheduler

import (
	"context"
	"time"
)

// Job 静态任务接口，各项目实现此接口注册任务
type Job interface {
	Name() string
	Run(ctx context.Context) error
}

// LogCallback 日志回调函数，由各项目注入具体的日志写入逻辑
type LogCallback func(entry *JobLogEntry)

// JobLogEntry 任务执行日志条目
type JobLogEntry struct {
	JobName   string
	Params    string
	Status    int // 0-失败 1-成功
	Error     string
	StartTime time.Time
	EndTime   time.Time
	Duration  int64 // 毫秒
}

// JobInfo 任务信息（用于查询）
type JobInfo struct {
	Name       string    `json:"name"`
	CronExpr   string    `json:"cronExpr"`
	NextRun    time.Time `json:"nextRun"`
	LastRun    time.Time `json:"lastRun"`
	IsRunning  bool      `json:"isRunning"`
	IsDynamic  bool      `json:"isDynamic"`
}
