package types

// ListTokensRequest 令牌列表请求
type ListTokensRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
}

// ListLoginLogsRequest 登录日志列表请求
type ListLoginLogsRequest struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	Username  string `json:"username"`
	Status    *int8  `json:"status"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// ListOperationLogsRequest 操作日志列表请求
type ListOperationLogsRequest struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	Username  string `json:"username"`
	Module    string `json:"module"`
	Status    *int8  `json:"status"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// TokenInfo 令牌响应
type TokenInfo struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"userId"`
	Token        string    `json:"token"`
	Device       string    `json:"device"`
	Platform     string    `json:"platform"`
	IP           string    `json:"ip"`
	UserAgent    string    `json:"userAgent"`
	ExpireAt     *DateTime `json:"expireAt"`
	LastActiveAt *DateTime `json:"lastActiveAt"`
	CreatedAt    *DateTime `json:"createdAt"`
}

// LoginLogInfo 登录日志响应
type LoginLogInfo struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"userId"`
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	Location  string    `json:"location"`
	Browser   string    `json:"browser"`
	Os        string    `json:"os"`
	Status    int32     `json:"status"`
	Message   string    `json:"message"`
	LoginType string    `json:"loginType"`
	CreatedAt *DateTime `json:"createdAt"`
}

// OperationLogInfo 操作日志响应
type OperationLogInfo struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"userId"`
	Username  string    `json:"username"`
	Module    string    `json:"module"`
	Action    string    `json:"action"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	IP        string    `json:"ip"`
	Status    int32     `json:"status"`
	Duration  int64     `json:"duration"`
	CreatedAt *DateTime `json:"createdAt"`
}
