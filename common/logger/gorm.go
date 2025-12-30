package logger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

// GormLogger GORM 日志适配器
type GormLogger struct {
	SlowThreshold time.Duration
	LogLevel      gormlogger.LogLevel
}

// NewGormLogger 创建 GORM 日志适配器
func NewGormLogger() *GormLogger {
	return &GormLogger{
		SlowThreshold: 200 * time.Millisecond,
		LogLevel:      gormlogger.Info,
	}
}

func (l *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Info {
		L().Sugar().Infof(msg, data...)
	}
}

func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Warn {
		L().Sugar().Warnf(msg, data...)
	}
}

func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Error {
		L().Sugar().Errorf(msg, data...)
	}
}

// shortCaller 截取短路径，只保留包名/文件名:行号
func shortCaller(caller string) string {
	parts := strings.Split(caller, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return caller
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.LogLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()
	caller := shortCaller(utils.FileWithLineNum())

	if IsJson() {
		fields := []zap.Field{
			zap.String("caller", caller),
			zap.Duration("latency", elapsed),
			zap.Int64("rows", rows),
			zap.String("sql", sql),
		}
		switch {
		case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
			L().WithOptions(zap.WithCaller(false)).Error("SQL", append(fields, zap.Error(err))...)
		case elapsed > l.SlowThreshold && l.SlowThreshold != 0:
			L().WithOptions(zap.WithCaller(false)).Warn("SQL SLOW", fields...)
		case l.LogLevel >= gormlogger.Info:
			L().WithOptions(zap.WithCaller(false)).Debug("SQL", fields...)
		}
	} else {
		msg := fmt.Sprintf("[%.3fms] [rows:%d] %s", float64(elapsed.Microseconds())/1000, rows, sql)
		lg := L().WithOptions(zap.AddCallerSkip(-1), zap.WithCaller(false)).Named(caller)
		switch {
		case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
			lg.Error(msg, zap.Error(err))
		case elapsed > l.SlowThreshold && l.SlowThreshold != 0:
			lg.Warn("SLOW " + msg)
		case l.LogLevel >= gormlogger.Info:
			lg.Debug(msg)
		}
	}
}
