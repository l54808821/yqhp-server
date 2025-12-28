package types

import (
	"time"
)

const TimeFormat = "2006-01-02 15:04:05"

// DateTime 时间类型，格式化为 "2006-01-02 15:04:05"
type DateTime time.Time

func (t DateTime) MarshalJSON() ([]byte, error) {
	tt := time.Time(t)
	if tt.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + tt.Format(TimeFormat) + `"`), nil
}

func (t *DateTime) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" || s == `""` {
		return nil
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	parsed, err := time.ParseInLocation(TimeFormat, s, time.Local)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return err
		}
	}
	*t = DateTime(parsed)
	return nil
}

// ToDateTime 将 *time.Time 转换为 *DateTime
func ToDateTime(t *time.Time) *DateTime {
	if t == nil {
		return nil
	}
	dt := DateTime(*t)
	return &dt
}

// 辅助函数：安全获取指针值
func GetString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func GetInt32(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}

func GetInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func GetBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
