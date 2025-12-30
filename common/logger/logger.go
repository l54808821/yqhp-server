package logger

import (
	"os"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	log  *zap.Logger
	once sync.Once
)

// Config 日志配置
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, console
	Output     string // stdout, file, both
	FilePath   string
	MaxSize    int // MB
	MaxBackups int
	MaxAge     int // days
}

// Init 初始化日志
func Init(cfg *Config) {
	once.Do(func() {
		log = newLogger(cfg)
	})
}

// newLogger 创建日志实例
func newLogger(cfg *Config) *zap.Logger {
	if cfg == nil {
		cfg = &Config{
			Level:  "info",
			Format: "console",
			Output: "stdout",
		}
	}

	// 解析日志级别
	level := zapcore.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	// 编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 选择编码器
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 配置输出
	var cores []zapcore.Core
	if cfg.Output == "stdout" || cfg.Output == "both" || cfg.Output == "" {
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level))
	}
	if cfg.Output == "file" || cfg.Output == "both" {
		if cfg.FilePath != "" {
			writer := &lumberjack.Logger{
				Filename:   cfg.FilePath,
				MaxSize:    cfg.MaxSize,
				MaxBackups: cfg.MaxBackups,
				MaxAge:     cfg.MaxAge,
			}
			cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(writer), level))
		}
	}

	core := zapcore.NewTee(cores...)
	return zap.New(core, zap.AddCaller())
}

// L 获取全局日志实例
func L() *zap.Logger {
	if log == nil {
		Init(nil)
	}
	return log
}

// WithTraceId 创建带 traceId 的 logger
func WithTraceId(c *fiber.Ctx) *zap.Logger {
	if traceId := c.Locals("requestid"); traceId != nil {
		return L().With(zap.String("traceId", traceId.(string)))
	}
	return L()
}

// Middleware HTTP 日志中间件
func Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		fields := []zap.Field{
			zap.Int("status", c.Response().StatusCode()),
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.String("ip", c.IP()),
			zap.Duration("latency", time.Since(start)),
		}
		if traceId := c.Locals("requestid"); traceId != nil {
			fields = append(fields, zap.String("traceId", traceId.(string)))
		}
		if err != nil {
			fields = append(fields, zap.Error(err))
		}

		status := c.Response().StatusCode()
		if status >= 500 {
			L().Error("HTTP", fields...)
		} else if status >= 400 {
			L().Warn("HTTP", fields...)
		} else {
			L().Info("HTTP", fields...)
		}
		return err
	}
}

// Debug 调试日志
func Debug(msg string, fields ...zap.Field) {
	L().WithOptions(zap.AddCallerSkip(1)).Debug(msg, fields...)
}

// Info 信息日志
func Info(msg string, fields ...zap.Field) {
	L().WithOptions(zap.AddCallerSkip(1)).Info(msg, fields...)
}

// Warn 警告日志
func Warn(msg string, fields ...zap.Field) {
	L().WithOptions(zap.AddCallerSkip(1)).Warn(msg, fields...)
}

// Error 错误日志
func Error(msg string, fields ...zap.Field) {
	L().WithOptions(zap.AddCallerSkip(1)).Error(msg, fields...)
}

// Fatal 致命错误日志
func Fatal(msg string, fields ...zap.Field) {
	L().WithOptions(zap.AddCallerSkip(1)).Fatal(msg, fields...)
}

// Sync 同步日志
func Sync() {
	if log != nil {
		_ = log.Sync()
	}
}
