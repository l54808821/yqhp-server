package scheduler

import (
	"github.com/redis/go-redis/v9"
)

// Option 配置选项函数
type Option func(*Options)

// Options 调度器配置
type Options struct {
	RedisClient *redis.Client
	LogCallback LogCallback
}

// WithRedisLocker 启用 Redis 分布式锁，多实例部署时同一任务只执行一次
func WithRedisLocker(client *redis.Client) Option {
	return func(o *Options) {
		o.RedisClient = client
	}
}

// WithLogCallback 设置日志回调，用于记录任务执行日志
func WithLogCallback(cb LogCallback) Option {
	return func(o *Options) {
		o.LogCallback = cb
	}
}
