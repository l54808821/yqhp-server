package executor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/types"
)

const (
	// DBExecutorType 是数据库执行器的类型标识符。
	DBExecutorType = "db"

	// 数据库操作的默认超时时间。
	defaultDBTimeout = 30 * time.Second
)

// DBDriver 数据库驱动类型
type DBDriver string

const (
	DBDriverMySQL    DBDriver = "mysql"
	DBDriverPostgres DBDriver = "postgres"
	DBDriverSQLite   DBDriver = "sqlite"
	DBDriverMongoDB  DBDriver = "mongodb"
	DBDriverRedis    DBDriver = "redis"
)

// DBConfig 数据库配置
type DBConfig struct {
	Driver         DBDriver      `yaml:"driver" json:"driver"`                                       // 驱动: mysql/postgres/sqlite/mongodb/redis
	DSN            string        `yaml:"dsn" json:"dsn"`                                             // 数据源名称
	MaxConnections int           `yaml:"max_connections,omitempty" json:"max_connections,omitempty"` // 最大连接数
	Timeout        time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`                 // 查询超时
}

// DBOperation 数据库操作
type DBOperation struct {
	Action string `yaml:"action" json:"action"`                     // 操作: query/execute/count/exists
	SQL    string `yaml:"sql,omitempty" json:"sql,omitempty"`       // SQL 语句
	Params []any  `yaml:"params,omitempty" json:"params,omitempty"` // 参数
	TxID   string `yaml:"tx_id,omitempty" json:"tx_id,omitempty"`   // 事务 ID
}

// DBAdapter 数据库适配器接口
type DBAdapter interface {
	// Connect 连接到数据库
	Connect(ctx context.Context, config *DBConfig) error
	// Query 查询，返回列表
	Query(ctx context.Context, sql string, params ...any) ([]map[string]any, error)
	// Execute 执行，返回影响行数
	Execute(ctx context.Context, sql string, params ...any) (int64, error)
	// Count 统计
	Count(ctx context.Context, sql string, params ...any) (int64, error)
	// Exists 存在检查
	Exists(ctx context.Context, sql string, params ...any) (bool, error)
	// BeginTx 开始事务
	BeginTx(ctx context.Context) (string, error)
	// CommitTx 提交事务
	CommitTx(ctx context.Context, txID string) error
	// RollbackTx 回滚事务
	RollbackTx(ctx context.Context, txID string) error
	// Close 关闭连接
	Close(ctx context.Context) error
	// IsConnected 检查是否已连接
	IsConnected() bool
}

// DBResult 数据库操作结果
type DBResult struct {
	Success      bool             `json:"success"`
	Data         []map[string]any `json:"data,omitempty"`
	RowsAffected int64            `json:"rows_affected,omitempty"`
	Count        int64            `json:"count,omitempty"`
	Exists       bool             `json:"exists,omitempty"`
	TxID         string           `json:"tx_id,omitempty"`
	Error        string           `json:"error,omitempty"`
}

// DBExecutor 执行数据库操作。
type DBExecutor struct {
	*BaseExecutor
	config   *DBConfig
	adapters map[DBDriver]DBAdapter
}

// NewDBExecutor 创建一个新的数据库执行器。
func NewDBExecutor() *DBExecutor {
	return &DBExecutor{
		BaseExecutor: NewBaseExecutor(DBExecutorType),
		adapters:     make(map[DBDriver]DBAdapter),
	}
}

// RegisterAdapter 注册数据库适配器
func (e *DBExecutor) RegisterAdapter(driver DBDriver, adapter DBAdapter) {
	e.adapters[driver] = adapter
}

// Init 使用配置初始化数据库执行器。
func (e *DBExecutor) Init(ctx context.Context, config map[string]any) error {
	if err := e.BaseExecutor.Init(ctx, config); err != nil {
		return err
	}

	e.config = &DBConfig{
		Driver:         DBDriverSQLite,
		MaxConnections: 10,
		Timeout:        defaultDBTimeout,
	}

	// 解析配置
	if driver, ok := config["driver"].(string); ok {
		e.config.Driver = DBDriver(strings.ToLower(driver))
	}
	if dsn, ok := config["dsn"].(string); ok {
		e.config.DSN = dsn
	}
	if maxConns, ok := config["max_connections"].(int); ok {
		e.config.MaxConnections = maxConns
	}
	if timeout, ok := config["timeout"].(string); ok {
		if d, err := time.ParseDuration(timeout); err == nil {
			e.config.Timeout = d
		}
	}

	return nil
}

// Execute 执行数据库操作步骤。
func (e *DBExecutor) Execute(ctx context.Context, step *types.Step, execCtx *ExecutionContext) (*types.StepResult, error) {
	startTime := time.Now()

	// 解析操作配置
	op, err := e.parseOperation(step.Config)
	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	// 解析步骤级配置
	stepConfig := e.parseStepConfig(step.Config)

	// 变量解析
	if execCtx != nil {
		evalCtx := execCtx.ToEvaluationContext()
		op.SQL = resolveString(op.SQL, evalCtx)
		stepConfig.DSN = resolveString(stepConfig.DSN, evalCtx)
		// 解析参数中的变量
		for i, param := range op.Params {
			if s, ok := param.(string); ok {
				op.Params[i] = resolveString(s, evalCtx)
			}
		}
	}

	// 获取适配器
	adapter, ok := e.adapters[stepConfig.Driver]
	if !ok {
		// 使用内存适配器作为默认
		adapter = NewInMemoryDBAdapter()
		e.adapters[stepConfig.Driver] = adapter
	}

	// 确保已连接
	if !adapter.IsConnected() {
		if err := adapter.Connect(ctx, stepConfig); err != nil {
			return CreateFailedResult(step.ID, startTime, NewExecutionError(step.ID, "failed to connect to database", err)), nil
		}
	}

	// 设置超时
	timeout := stepConfig.Timeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 执行操作
	var result *DBResult
	switch op.Action {
	case "query":
		result, err = e.executeQuery(ctx, adapter, op)
	case "execute":
		result, err = e.executeExec(ctx, adapter, op)
	case "count":
		result, err = e.executeCount(ctx, adapter, op)
	case "exists":
		result, err = e.executeExists(ctx, adapter, op)
	case "begin":
		result, err = e.executeBeginTx(ctx, adapter)
	case "commit":
		result, err = e.executeCommitTx(ctx, adapter, op)
	case "rollback":
		result, err = e.executeRollbackTx(ctx, adapter, op)
	default:
		err = NewConfigError(fmt.Sprintf("unknown DB action: %s", op.Action), nil)
	}

	if err != nil {
		return CreateFailedResult(step.ID, startTime, err), nil
	}

	return CreateSuccessResult(step.ID, startTime, result), nil
}

// executeQuery 执行查询
func (e *DBExecutor) executeQuery(ctx context.Context, adapter DBAdapter, op *DBOperation) (*DBResult, error) {
	data, err := adapter.Query(ctx, op.SQL, op.Params...)
	if err != nil {
		return nil, NewExecutionError("db", "query failed", err)
	}
	return &DBResult{
		Success: true,
		Data:    data,
	}, nil
}

// executeExec 执行语句
func (e *DBExecutor) executeExec(ctx context.Context, adapter DBAdapter, op *DBOperation) (*DBResult, error) {
	rowsAffected, err := adapter.Execute(ctx, op.SQL, op.Params...)
	if err != nil {
		return nil, NewExecutionError("db", "execute failed", err)
	}
	return &DBResult{
		Success:      true,
		RowsAffected: rowsAffected,
	}, nil
}

// executeCount 执行统计
func (e *DBExecutor) executeCount(ctx context.Context, adapter DBAdapter, op *DBOperation) (*DBResult, error) {
	count, err := adapter.Count(ctx, op.SQL, op.Params...)
	if err != nil {
		return nil, NewExecutionError("db", "count failed", err)
	}
	return &DBResult{
		Success: true,
		Count:   count,
	}, nil
}

// executeExists 执行存在检查
func (e *DBExecutor) executeExists(ctx context.Context, adapter DBAdapter, op *DBOperation) (*DBResult, error) {
	exists, err := adapter.Exists(ctx, op.SQL, op.Params...)
	if err != nil {
		return nil, NewExecutionError("db", "exists check failed", err)
	}
	return &DBResult{
		Success: true,
		Exists:  exists,
	}, nil
}

// executeBeginTx 开始事务
func (e *DBExecutor) executeBeginTx(ctx context.Context, adapter DBAdapter) (*DBResult, error) {
	txID, err := adapter.BeginTx(ctx)
	if err != nil {
		return nil, NewExecutionError("db", "begin transaction failed", err)
	}
	return &DBResult{
		Success: true,
		TxID:    txID,
	}, nil
}

// executeCommitTx 提交事务
func (e *DBExecutor) executeCommitTx(ctx context.Context, adapter DBAdapter, op *DBOperation) (*DBResult, error) {
	err := adapter.CommitTx(ctx, op.TxID)
	if err != nil {
		return nil, NewExecutionError("db", "commit transaction failed", err)
	}
	return &DBResult{
		Success: true,
	}, nil
}

// executeRollbackTx 回滚事务
func (e *DBExecutor) executeRollbackTx(ctx context.Context, adapter DBAdapter, op *DBOperation) (*DBResult, error) {
	err := adapter.RollbackTx(ctx, op.TxID)
	if err != nil {
		return nil, NewExecutionError("db", "rollback transaction failed", err)
	}
	return &DBResult{
		Success: true,
	}, nil
}

// parseOperation 解析操作配置
func (e *DBExecutor) parseOperation(config map[string]any) (*DBOperation, error) {
	op := &DBOperation{}

	if action, ok := config["action"].(string); ok {
		op.Action = strings.ToLower(action)
	} else {
		return nil, NewConfigError("DB step requires 'action' configuration", nil)
	}

	if sql, ok := config["sql"].(string); ok {
		op.SQL = sql
	}
	if params, ok := config["params"].([]any); ok {
		op.Params = params
	}
	if txID, ok := config["tx_id"].(string); ok {
		op.TxID = txID
	}

	return op, nil
}

// parseStepConfig 解析步骤级配置
func (e *DBExecutor) parseStepConfig(config map[string]any) *DBConfig {
	stepConfig := &DBConfig{
		Driver:         e.config.Driver,
		DSN:            e.config.DSN,
		MaxConnections: e.config.MaxConnections,
		Timeout:        e.config.Timeout,
	}

	if driver, ok := config["driver"].(string); ok {
		stepConfig.Driver = DBDriver(strings.ToLower(driver))
	}
	if dsn, ok := config["dsn"].(string); ok {
		stepConfig.DSN = dsn
	}
	if maxConns, ok := config["max_connections"].(int); ok {
		stepConfig.MaxConnections = maxConns
	}
	if timeout, ok := config["timeout"].(string); ok {
		if d, err := time.ParseDuration(timeout); err == nil {
			stepConfig.Timeout = d
		}
	}

	return stepConfig
}

// Cleanup 释放数据库执行器持有的资源。
func (e *DBExecutor) Cleanup(ctx context.Context) error {
	for _, adapter := range e.adapters {
		if err := adapter.Close(ctx); err != nil {
			return err
		}
	}
	return nil
}

// init 在默认注册表中注册数据库执行器。
func init() {
	MustRegister(NewDBExecutor())
}
