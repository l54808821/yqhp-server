package utils

import (
	"time"

	"github.com/duke-git/lancet/v2/datetime"
)

const (
	DateFormat     = "2006-01-02"
	TimeFormat     = "15:04:05"
	DateTimeFormat = "2006-01-02 15:04:05"
)

// Now 获取当前时间
func Now() time.Time {
	return time.Now()
}

// NowUnix 获取当前时间戳(秒)
func NowUnix() int64 {
	return time.Now().Unix()
}

// NowUnixMilli 获取当前时间戳(毫秒)
func NowUnixMilli() int64 {
	return time.Now().UnixMilli()
}

// FormatDate 格式化日期
func FormatDate(t time.Time) string {
	return datetime.FormatTimeToStr(t, DateFormat)
}

// FormatDateTime 格式化日期时间
func FormatDateTime(t time.Time) string {
	return datetime.FormatTimeToStr(t, DateTimeFormat)
}

// ParseDate 解析日期字符串
func ParseDate(s string) (time.Time, error) {
	return datetime.FormatStrToTime(s, DateFormat)
}

// ParseDateTime 解析日期时间字符串
func ParseDateTime(s string) (time.Time, error) {
	return datetime.FormatStrToTime(s, DateTimeFormat)
}

// AddDays 增加天数
func AddDays(t time.Time, days int) time.Time {
	return datetime.AddDay(t, int64(days))
}

// AddHours 增加小时数
func AddHours(t time.Time, hours int) time.Time {
	return datetime.AddHour(t, int64(hours))
}

// AddMinutes 增加分钟数
func AddMinutes(t time.Time, minutes int) time.Time {
	return datetime.AddMinute(t, int64(minutes))
}

// DaysBetween 计算两个日期之间的天数差
func DaysBetween(start, end time.Time) int {
	return int(end.Sub(start).Hours() / 24)
}

// IsToday 判断是否是今天
func IsToday(t time.Time) bool {
	now := time.Now()
	return t.Year() == now.Year() && t.Month() == now.Month() && t.Day() == now.Day()
}

// StartOfDay 获取一天的开始时间
func StartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// EndOfDay 获取一天的结束时间
func EndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
}

