package executor

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SQLDBAdapter 基于 database/sql 的通用数据库适配器
// 支持 MySQL、PostgreSQL 等注册了 database/sql 驱动的数据库
type SQLDBAdapter struct {
	mu           sync.RWMutex
	db           *sql.DB
	connected    bool
	driverName   string
	transactions map[string]*sql.Tx
	txCounter    int64
}

// NewSQLDBAdapter 创建通用 SQL 适配器
func NewSQLDBAdapter() *SQLDBAdapter {
	return &SQLDBAdapter{
		transactions: make(map[string]*sql.Tx),
	}
}

// Connect 连接到数据库
func (a *SQLDBAdapter) Connect(ctx context.Context, config *DBConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.connected && a.db != nil {
		return nil
	}

	driverName := a.resolveDriverName(config.Driver)
	if driverName == "" {
		return fmt.Errorf("不支持的数据库驱动: %s", config.Driver)
	}

	db, err := sql.Open(driverName, config.DSN)
	if err != nil {
		return fmt.Errorf("打开数据库连接失败: %w", err)
	}

	if config.MaxConnections > 0 {
		db.SetMaxOpenConns(config.MaxConnections)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	a.db = db
	a.driverName = driverName
	a.connected = true
	return nil
}

// resolveDriverName 将 DBDriver 映射为 database/sql 驱动名
func (a *SQLDBAdapter) resolveDriverName(driver DBDriver) string {
	switch driver {
	case DBDriverMySQL:
		return "mysql"
	case DBDriverPostgres:
		return "pgx"
	default:
		return string(driver)
	}
}

// Query 执行查询，返回结果列表
func (a *SQLDBAdapter) Query(ctx context.Context, sqlStr string, params ...any) ([]map[string]any, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.connected || a.db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}

	rows, err := a.db.QueryContext(ctx, sqlStr, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]any, len(columns))
		for i, col := range columns {
			val := values[i]
			switch v := val.(type) {
			case []byte:
				row[col] = string(v)
			case time.Time:
				row[col] = v.Format("2006-01-02 15:04:05")
			default:
				row[col] = val
			}
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if result == nil {
		result = []map[string]any{}
	}

	return result, nil
}

// Execute 执行 SQL 语句，返回影响行数
func (a *SQLDBAdapter) Execute(ctx context.Context, sqlStr string, params ...any) (int64, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.connected || a.db == nil {
		return 0, fmt.Errorf("数据库未连接")
	}

	result, err := a.db.ExecContext(ctx, sqlStr, params...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// Count 统计行数
func (a *SQLDBAdapter) Count(ctx context.Context, sqlStr string, params ...any) (int64, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.connected || a.db == nil {
		return 0, fmt.Errorf("数据库未连接")
	}

	// 包装为 COUNT 查询
	countSQL := sqlStr
	upperSQL := strings.TrimSpace(strings.ToUpper(sqlStr))
	if strings.HasPrefix(upperSQL, "SELECT") && !strings.Contains(upperSQL, "COUNT(") {
		countSQL = fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS _cnt", sqlStr)
	}

	var count int64
	err := a.db.QueryRowContext(ctx, countSQL, params...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Exists 检查是否存在匹配行
func (a *SQLDBAdapter) Exists(ctx context.Context, sqlStr string, params ...any) (bool, error) {
	count, err := a.Count(ctx, sqlStr, params...)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// BeginTx 开始事务
func (a *SQLDBAdapter) BeginTx(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.connected || a.db == nil {
		return "", fmt.Errorf("数据库未连接")
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}

	a.txCounter++
	txID := fmt.Sprintf("tx_%d", a.txCounter)
	a.transactions[txID] = tx

	return txID, nil
}

// CommitTx 提交事务
func (a *SQLDBAdapter) CommitTx(ctx context.Context, txID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	tx, ok := a.transactions[txID]
	if !ok {
		return fmt.Errorf("事务不存在: %s", txID)
	}

	delete(a.transactions, txID)
	return tx.Commit()
}

// RollbackTx 回滚事务
func (a *SQLDBAdapter) RollbackTx(ctx context.Context, txID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	tx, ok := a.transactions[txID]
	if !ok {
		return fmt.Errorf("事务不存在: %s", txID)
	}

	delete(a.transactions, txID)
	return tx.Rollback()
}

// Close 关闭连接
func (a *SQLDBAdapter) Close(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 回滚所有未完成事务
	for _, tx := range a.transactions {
		tx.Rollback()
	}
	a.transactions = make(map[string]*sql.Tx)

	if a.db != nil {
		err := a.db.Close()
		a.db = nil
		a.connected = false
		return err
	}

	a.connected = false
	return nil
}

// IsConnected 检查是否已连接
func (a *SQLDBAdapter) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.connected && a.db != nil
}
