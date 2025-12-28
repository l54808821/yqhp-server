package redis

import (
	"context"
	"fmt"
	"time"

	"yqhp/common/config"

	"github.com/redis/go-redis/v9"
)

var client *redis.Client
var ctx = context.Background()

// Init 初始化Redis连接
func Init(cfg *config.RedisConfig) error {
	client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 测试连接
	_, err := client.Ping(ctx).Result()
	return err
}

// GetClient 获取Redis客户端
func GetClient() *redis.Client {
	return client
}

// Close 关闭Redis连接
func Close() error {
	if client != nil {
		return client.Close()
	}
	return nil
}

// Set 设置键值
func Set(key string, value any, expiration time.Duration) error {
	return client.Set(ctx, key, value, expiration).Err()
}

// Get 获取值
func Get(key string) (string, error) {
	return client.Get(ctx, key).Result()
}

// Del 删除键
func Del(keys ...string) error {
	return client.Del(ctx, keys...).Err()
}

// Exists 判断键是否存在
func Exists(keys ...string) (int64, error) {
	return client.Exists(ctx, keys...).Result()
}

// Expire 设置过期时间
func Expire(key string, expiration time.Duration) error {
	return client.Expire(ctx, key, expiration).Err()
}

// TTL 获取过期时间
func TTL(key string) (time.Duration, error) {
	return client.TTL(ctx, key).Result()
}

// HSet 设置Hash字段
func HSet(key string, values ...any) error {
	return client.HSet(ctx, key, values...).Err()
}

// HGet 获取Hash字段
func HGet(key, field string) (string, error) {
	return client.HGet(ctx, key, field).Result()
}

// HGetAll 获取所有Hash字段
func HGetAll(key string) (map[string]string, error) {
	return client.HGetAll(ctx, key).Result()
}

// HDel 删除Hash字段
func HDel(key string, fields ...string) error {
	return client.HDel(ctx, key, fields...).Err()
}

// SAdd 添加集合成员
func SAdd(key string, members ...any) error {
	return client.SAdd(ctx, key, members...).Err()
}

// SMembers 获取集合所有成员
func SMembers(key string) ([]string, error) {
	return client.SMembers(ctx, key).Result()
}

// SIsMember 判断是否是集合成员
func SIsMember(key string, member any) (bool, error) {
	return client.SIsMember(ctx, key, member).Result()
}

// SRem 移除集合成员
func SRem(key string, members ...any) error {
	return client.SRem(ctx, key, members...).Err()
}

// LPush 左侧入队
func LPush(key string, values ...any) error {
	return client.LPush(ctx, key, values...).Err()
}

// RPush 右侧入队
func RPush(key string, values ...any) error {
	return client.RPush(ctx, key, values...).Err()
}

// LPop 左侧出队
func LPop(key string) (string, error) {
	return client.LPop(ctx, key).Result()
}

// RPop 右侧出队
func RPop(key string) (string, error) {
	return client.RPop(ctx, key).Result()
}

// LRange 获取列表范围
func LRange(key string, start, stop int64) ([]string, error) {
	return client.LRange(ctx, key, start, stop).Result()
}

// Incr 自增
func Incr(key string) (int64, error) {
	return client.Incr(ctx, key).Result()
}

// Decr 自减
func Decr(key string) (int64, error) {
	return client.Decr(ctx, key).Result()
}

