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
