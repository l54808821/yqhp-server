package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	log    *zap.Logger
	once   sync.Once
	isJson bool
)

// Config 日志配置
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, text
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
		isJson = cfg != nil && cfg.Format == "json"
	})
}

// fixedWidthCallerEncoder 固定宽度的 caller encoder
func fixedWidthCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	const width = 30
	s := caller.TrimmedPath()
	if len(s) < width {
		s = s + strings.Repeat(" ", width-len(s))
	}
	enc.AppendString(s)
}

// fixedWidthNameEncoder 固定宽度的 name encoder（用于 SQL 日志的 caller）
func fixedWidthNameEncoder(name string, enc zapcore.PrimitiveArrayEncoder) {
	const width = 30
	if len(name) < width {
		name = name + strings.Repeat(" ", width-len(name))
	}
	enc.AppendString(name)
}

// customConsoleEncoder 自定义 console encoder
type customConsoleEncoder struct {
	zapcore.Encoder
	pool buffer.Pool
}

func newCustomConsoleEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	cfg.EncodeCaller = fixedWidthCallerEncoder
	cfg.EncodeName = fixedWidthNameEncoder
	return &customConsoleEncoder{
		Encoder: zapcore.NewConsoleEncoder(cfg),
	}
}

func (e *customConsoleEncoder) Clone() zapcore.Encoder {
	return &customConsoleEncoder{Encoder: e.Encoder.Clone()}
}

// newLogger 创建日志实例
func newLogger(cfg *Config) *zap.Logger {
	if cfg == nil {
		cfg = &Config{
			Level:  "info",
			Format: "text",
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
		encoder = newCustomConsoleEncoder(encoderConfig)
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

// IsJson 是否为 JSON 格式
func IsJson() bool {
	return isJson
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
	traceId := ""
	if t := c.Locals("requestid"); t != nil {
		traceId = t.(string)
	}
	if isJson {
		return L().With(zap.String("traceId", traceId))
	}
	return L()
}

// Middleware HTTP 日志中间件
func Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		status := c.Response().StatusCode()
		latency := time.Since(start)
		traceId := ""
		if t := c.Locals("requestid"); t != nil {
			traceId = t.(string)
		}

		if isJson {
			fields := []zap.Field{
				zap.Int("status", status),
				zap.String("method", c.Method()),
				zap.String("path", c.Path()),
				zap.String("ip", c.IP()),
				zap.Duration("latency", latency),
				zap.String("traceId", traceId),
			}
			if err != nil {
				fields = append(fields, zap.Error(err))
			}
			if status >= 500 {
				L().Error("HTTP", fields...)
			} else if status >= 400 {
				L().Warn("HTTP", fields...)
			} else {
				L().Info("HTTP", fields...)
			}
		} else {
			msg := fmt.Sprintf("[%s] %d %s %s %v", traceId[:8], status, c.Method(), c.Path(), latency)
			if status >= 500 {
				L().Error(msg)
			} else if status >= 400 {
				L().Warn(msg)
			} else {
				L().Info(msg)
			}
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
