package types

import (
	"database/sql/driver"
	"fmt"
	"time"
)

const (
	// DateTimeFormat 日期时间格式
	DateTimeFormat = "2006-01-02 15:04:05"
	// DateFormat 日期格式
	DateFormat = "2006-01-02"
	// TimeFormat 时间格式
	TimeFormat = "15:04:05"
)

// DateTime 自定义时间类型，JSON序列化为 "yyyy-MM-dd HH:mm:ss" 格式
type DateTime time.Time

// Now 返回当前时间的DateTime
func Now() DateTime {
	return DateTime(time.Now())
}

// NewDateTime 从time.Time创建DateTime
func NewDateTime(t time.Time) DateTime {
	return DateTime(t)
}

// Time 转换为time.Time
func (t DateTime) Time() time.Time {
	return time.Time(t)
}

// IsZero 判断是否为零值
func (t DateTime) IsZero() bool {
	return time.Time(t).IsZero()
}

// String 实现Stringer接口
func (t DateTime) String() string {
	return time.Time(t).Format(DateTimeFormat)
}

// MarshalJSON 实现json.Marshaler接口
func (t DateTime) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf(`"%s"`, t.String())), nil
}

// UnmarshalJSON 实现json.Unmarshaler接口
func (t *DateTime) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" || string(data) == `""` {
		*t = DateTime{}
		return nil
	}

	// 去掉引号
	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	// 尝试多种格式解析
	formats := []string{
		DateTimeFormat,
		"2006-01-02T15:04:05Z07:00",     // RFC3339
		"2006-01-02T15:04:05.000Z07:00", // RFC3339 with milliseconds
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000",
		time.RFC3339,
		time.RFC3339Nano,
	}

	var parseErr error
	for _, format := range formats {
		parsed, err := time.Parse(format, str)
		if err == nil {
			*t = DateTime(parsed)
			return nil
		}
		parseErr = err
	}

	return fmt.Errorf("无法解析时间格式: %s, 错误: %v", str, parseErr)
}

// Value 实现driver.Valuer接口（用于GORM写入数据库）
func (t DateTime) Value() (driver.Value, error) {
	if t.IsZero() {
		return nil, nil
	}
	return time.Time(t), nil
}

// Scan 实现sql.Scanner接口（用于GORM从数据库读取）
func (t *DateTime) Scan(value any) error {
	if value == nil {
		*t = DateTime{}
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		*t = DateTime(v)
		return nil
	case string:
		parsed, err := time.Parse(DateTimeFormat, v)
		if err != nil {
			// 尝试其他格式
			parsed, err = time.Parse("2006-01-02T15:04:05Z07:00", v)
			if err != nil {
				return fmt.Errorf("无法解析时间字符串: %s", v)
			}
		}
		*t = DateTime(parsed)
		return nil
	case []byte:
		return t.Scan(string(v))
	default:
		return fmt.Errorf("无法将 %T 转换为 DateTime", value)
	}
}

// GormDataType 实现GORM的DataType接口
func (t DateTime) GormDataType() string {
	return "datetime"
}

// Date 自定义日期类型，JSON序列化为 "yyyy-MM-dd" 格式
type Date time.Time

// NewDate 从time.Time创建Date
func NewDate(t time.Time) Date {
	return Date(t)
}

// Today 返回今天的Date
func Today() Date {
	now := time.Now()
	return Date(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()))
}

// Time 转换为time.Time
func (d Date) Time() time.Time {
	return time.Time(d)
}

// IsZero 判断是否为零值
func (d Date) IsZero() bool {
	return time.Time(d).IsZero()
}

// String 实现Stringer接口
func (d Date) String() string {
	return time.Time(d).Format(DateFormat)
}

// MarshalJSON 实现json.Marshaler接口
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf(`"%s"`, d.String())), nil
}

// UnmarshalJSON 实现json.Unmarshaler接口
func (d *Date) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" || string(data) == `""` {
		*d = Date{}
		return nil
	}

	// 去掉引号
	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	// 只取日期部分
	if len(str) > 10 {
		str = str[:10]
	}

	parsed, err := time.Parse(DateFormat, str)
	if err != nil {
		return fmt.Errorf("无法解析日期格式: %s, 错误: %v", str, err)
	}
	*d = Date(parsed)
	return nil
}

// Value 实现driver.Valuer接口
func (d Date) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil
	}
	return time.Time(d), nil
}

// Scan 实现sql.Scanner接口
func (d *Date) Scan(value any) error {
	if value == nil {
		*d = Date{}
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		*d = Date(v)
		return nil
	case string:
		if len(v) > 10 {
			v = v[:10]
		}
		parsed, err := time.Parse(DateFormat, v)
		if err != nil {
			return fmt.Errorf("无法解析日期字符串: %s", v)
		}
		*d = Date(parsed)
		return nil
	case []byte:
		return d.Scan(string(v))
	default:
		return fmt.Errorf("无法将 %T 转换为 Date", value)
	}
}

// GormDataType 实现GORM的DataType接口
func (d Date) GormDataType() string {
	return "date"
}

// NullDateTime 可空的DateTime类型
type NullDateTime struct {
	DateTime DateTime
	Valid    bool
}

// NewNullDateTime 创建NullDateTime
func NewNullDateTime(t time.Time, valid bool) NullDateTime {
	return NullDateTime{
		DateTime: DateTime(t),
		Valid:    valid,
	}
}

// MarshalJSON 实现json.Marshaler接口
func (n NullDateTime) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return n.DateTime.MarshalJSON()
}

// UnmarshalJSON 实现json.Unmarshaler接口
func (n *NullDateTime) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		n.Valid = false
		return nil
	}
	n.Valid = true
	return n.DateTime.UnmarshalJSON(data)
}

// Value 实现driver.Valuer接口
func (n NullDateTime) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.DateTime.Value()
}

// Scan 实现sql.Scanner接口
func (n *NullDateTime) Scan(value any) error {
	if value == nil {
		n.Valid = false
		return nil
	}
	n.Valid = true
	return n.DateTime.Scan(value)
}

