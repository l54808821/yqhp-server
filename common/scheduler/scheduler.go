package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	redislock "github.com/go-co-op/gocron-redis-lock/v2"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	scheduler   gocron.Scheduler
	mu          sync.RWMutex
	jobs        map[string]gocron.Job
	cronExprs   map[string]string
	dynamicJobs map[string]bool
	taskFuncs   map[string]func()
	logCallback LogCallback
}

// NewScheduler 创建调度器实例
func NewScheduler(opts ...Option) (*Scheduler, error) {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	var gocronOpts []gocron.SchedulerOption

	if options.RedisClient != nil {
		locker, err := redislock.NewRedisLocker(options.RedisClient, redislock.WithTries(1))
		if err != nil {
			return nil, fmt.Errorf("创建 Redis 分布式锁失败: %w", err)
		}
		gocronOpts = append(gocronOpts, gocron.WithDistributedLocker(locker))
	}

	s, err := gocron.NewScheduler(gocronOpts...)
	if err != nil {
		return nil, fmt.Errorf("创建调度器失败: %w", err)
	}

	return &Scheduler{
		scheduler:   s,
		jobs:        make(map[string]gocron.Job),
		cronExprs:   make(map[string]string),
		dynamicJobs: make(map[string]bool),
		taskFuncs:   make(map[string]func()),
		logCallback: options.LogCallback,
	}, nil
}

// Register 注册静态任务（启动时注册）
func (s *Scheduler) Register(job Job, cronExpr string) error {
	return s.addJob(job.Name(), cronExpr, job.Run, false)
}

// AddDynamic 添加动态任务（运行时创建）
func (s *Scheduler) AddDynamic(name string, cronExpr string, fn func(ctx context.Context) error) error {
	return s.addJob(name, cronExpr, fn, true)
}

func (s *Scheduler) addJob(name string, cronExpr string, fn func(ctx context.Context) error, isDynamic bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[name]; exists {
		return fmt.Errorf("任务 %s 已存在", name)
	}

	wrappedFn := s.wrapWithLog(name, fn)

	j, err := s.scheduler.NewJob(
		gocron.CronJob(cronExpr, true),
		gocron.NewTask(wrappedFn),
		gocron.WithName(name),
		gocron.WithTags(name),
	)
	if err != nil {
		return fmt.Errorf("创建任务 %s 失败: %w", name, err)
	}

	s.jobs[name] = j
	s.cronExprs[name] = cronExpr
	s.dynamicJobs[name] = isDynamic
	s.taskFuncs[name] = wrappedFn
	return nil
}

// UpdateCron 更新任务的 cron 表达式
func (s *Scheduler) UpdateCron(name string, cronExpr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldJob, exists := s.jobs[name]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", name)
	}

	taskFn, ok := s.taskFuncs[name]
	if !ok {
		return fmt.Errorf("任务 %s 的执行函数不存在", name)
	}

	newJob, err := s.scheduler.Update(
		oldJob.ID(),
		gocron.CronJob(cronExpr, true),
		gocron.NewTask(taskFn),
		gocron.WithName(name),
		gocron.WithTags(name),
	)
	if err != nil {
		return fmt.Errorf("更新任务 %s 失败: %w", name, err)
	}

	s.jobs[name] = newJob
	s.cronExprs[name] = cronExpr
	return nil
}

// Remove 删除任务
func (s *Scheduler) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[name]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", name)
	}

	if err := s.scheduler.RemoveJob(job.ID()); err != nil {
		return fmt.Errorf("删除任务 %s 失败: %w", name, err)
	}

	delete(s.jobs, name)
	delete(s.cronExprs, name)
	delete(s.dynamicJobs, name)
	delete(s.taskFuncs, name)
	return nil
}

// TriggerOnce 立即触发任务执行一次
func (s *Scheduler) TriggerOnce(name string) error {
	s.mu.RLock()
	job, exists := s.jobs[name]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("任务 %s 不存在", name)
	}

	return job.RunNow()
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.scheduler.Start()
}

// Stop 停止调度器
func (s *Scheduler) Stop() error {
	return s.scheduler.Shutdown()
}

// ListJobs 列出所有任务
func (s *Scheduler) ListJobs() []JobInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]JobInfo, 0, len(s.jobs))
	for name, job := range s.jobs {
		info := JobInfo{
			Name:      name,
			CronExpr:  s.cronExprs[name],
			IsDynamic: s.dynamicJobs[name],
		}
		nextRun, _ := job.NextRun()
		info.NextRun = nextRun
		lastRun, _ := job.LastRun()
		info.LastRun = lastRun
		list = append(list, info)
	}
	return list
}

// GetJob 获取任务信息
func (s *Scheduler) GetJob(name string) (*JobInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[name]
	if !exists {
		return nil, errors.New("任务不存在")
	}

	info := &JobInfo{
		Name:      name,
		CronExpr:  s.cronExprs[name],
		IsDynamic: s.dynamicJobs[name],
	}
	nextRun, _ := job.NextRun()
	info.NextRun = nextRun
	lastRun, _ := job.LastRun()
	info.LastRun = lastRun
	return info, nil
}

// HasJob 检查任务是否存在
func (s *Scheduler) HasJob(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.jobs[name]
	return exists
}

// wrapWithLog 包装任务函数，记录执行日志
func (s *Scheduler) wrapWithLog(name string, fn func(ctx context.Context) error) func() {
	return func() {
		startTime := time.Now()
		err := fn(context.Background())
		endTime := time.Now()
		duration := endTime.Sub(startTime).Milliseconds()

		if s.logCallback != nil {
			entry := &JobLogEntry{
				JobName:   name,
				Status:    1,
				StartTime: startTime,
				EndTime:   endTime,
				Duration:  duration,
			}
			if err != nil {
				entry.Status = 0
				entry.Error = err.Error()
			}
			s.logCallback(entry)
		}
	}
}
