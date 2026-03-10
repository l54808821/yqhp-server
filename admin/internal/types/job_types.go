package types

// CreateJobRequest 创建定时任务请求
type CreateJobRequest struct {
	Name           string `json:"name" validate:"required"`
	JobGroup       string `json:"jobGroup"`
	HandlerName    string `json:"handlerName" validate:"required"`
	CronExpression string `json:"cronExpression" validate:"required"`
	Params         string `json:"params"`
	Status         int32  `json:"status"`
	MisfirePolicy  int32  `json:"misfirePolicy"`
	Concurrent     int32  `json:"concurrent"`
	RetryCount     int32  `json:"retryCount"`
	RetryInterval  int32  `json:"retryInterval"`
	Remark         string `json:"remark"`
}

// UpdateJobRequest 更新定时任务请求
type UpdateJobRequest struct {
	ID             int64  `json:"id" validate:"required"`
	Name           string `json:"name"`
	JobGroup       string `json:"jobGroup"`
	HandlerName    string `json:"handlerName"`
	CronExpression string `json:"cronExpression"`
	Params         string `json:"params"`
	MisfirePolicy  int32  `json:"misfirePolicy"`
	Concurrent     int32  `json:"concurrent"`
	RetryCount     int32  `json:"retryCount"`
	RetryInterval  int32  `json:"retryInterval"`
	Remark         string `json:"remark"`
}

// ChangeJobStatusRequest 变更任务状态请求
type ChangeJobStatusRequest struct {
	Status int32 `json:"status" validate:"required"`
}

// ListJobsRequest 任务列表请求
type ListJobsRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	JobGroup string `json:"jobGroup"`
	Status   *int32 `json:"status"`
}

// JobInfo 任务响应
type JobInfo struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	JobGroup       string    `json:"jobGroup"`
	HandlerName    string    `json:"handlerName"`
	CronExpression string    `json:"cronExpression"`
	Params         string    `json:"params"`
	Status         int32     `json:"status"`
	Source         string    `json:"source"`
	SourceID       int64     `json:"sourceId"`
	MisfirePolicy  int32     `json:"misfirePolicy"`
	Concurrent     int32     `json:"concurrent"`
	RetryCount     int32     `json:"retryCount"`
	RetryInterval  int32     `json:"retryInterval"`
	Remark         string    `json:"remark"`
	CreatedBy      int64     `json:"createdBy"`
	UpdatedBy      int64     `json:"updatedBy"`
	CreatedAt      *DateTime `json:"createdAt"`
	UpdatedAt      *DateTime `json:"updatedAt"`
}

// ListJobLogsRequest 任务日志列表请求
type ListJobLogsRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	JobID    int64  `json:"jobId"`
	JobName  string `json:"jobName"`
	Status   *int32 `json:"status"`
}

// JobLogInfo 任务日志响应
type JobLogInfo struct {
	ID           int64     `json:"id"`
	JobID        int64     `json:"jobId"`
	JobName      string    `json:"jobName"`
	HandlerName  string    `json:"handlerName"`
	Params       string    `json:"params"`
	Status       int32     `json:"status"`
	ErrorMessage string    `json:"errorMessage"`
	StartTime    *DateTime `json:"startTime"`
	EndTime      *DateTime `json:"endTime"`
	Duration     int64     `json:"duration"`
	CreatedAt    *DateTime `json:"createdAt"`
}
