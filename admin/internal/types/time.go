package types

import (
	"database/sql/driver"
	"fmt"
	"time"
)

const TimeFormat = "2006-01-02 15:04:05"

// DateTime 自定义时间类型，JSON 序列化为 "2006-01-02 15:04:05" 格式
type DateTime time.Time

func (t DateTime) MarshalJSON() ([]byte, error) {
	tt := time.Time(t)
	if tt.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf(`"%s"`, tt.Format(TimeFormat))), nil
}

func (t *DateTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		return nil
	}
	s := string(data)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	tt, err := time.ParseInLocation(TimeFormat, s, time.Local)
	if err != nil {
		return err
	}
	*t = DateTime(tt)
	return nil
}

func (t DateTime) Value() (driver.Value, error) {
	return time.Time(t), nil
}

func (t *DateTime) Scan(v any) error {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case time.Time:
		*t = DateTime(val)
	case *time.Time:
		if val != nil {
			*t = DateTime(*val)
		}
	}
	return nil
}

func (t DateTime) String() string {
	return time.Time(t).Format(TimeFormat)
}

// ToDateTime 将 *time.Time 转换为 *DateTime
func ToDateTime(t *time.Time) *DateTime {
	if t == nil {
		return nil
	}
	dt := DateTime(*t)
	return &dt
}
