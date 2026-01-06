package executor

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// InMemoryDBAdapter 内存数据库适配器（用于测试）
type InMemoryDBAdapter struct {
	mu           sync.RWMutex
	connected    bool
	config       *DBConfig
	tables       map[string][]map[string]any
	transactions map[string]*inMemoryTx
	txCounter    int64
}

type inMemoryTx struct {
	id       string
	snapshot map[string][]map[string]any
}

// NewInMemoryDBAdapter 创建一个新的内存数据库适配器。
func NewInMemoryDBAdapter() *InMemoryDBAdapter {
	return &InMemoryDBAdapter{
		tables:       make(map[string][]map[string]any),
		transactions: make(map[string]*inMemoryTx),
	}
}

// Connect 连接到内存数据库。
func (a *InMemoryDBAdapter) Connect(ctx context.Context, config *DBConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.config = config
	a.connected = true
	return nil
}

// Query 执行查询并返回结果。
func (a *InMemoryDBAdapter) Query(ctx context.Context, sql string, params ...any) ([]map[string]any, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.connected {
		return nil, fmt.Errorf("not connected")
	}

	// 简单的 SELECT 解析
	tableName := a.extractTableName(sql, "FROM")
	if tableName == "" {
		return nil, fmt.Errorf("could not parse table name from SQL")
	}

	rows, ok := a.tables[tableName]
	if !ok {
		return []map[string]any{}, nil
	}

	// 应用 WHERE 条件（简化实现）
	result := a.applyWhere(rows, sql, params)

	// 应用 LIMIT
	result = a.applyLimit(result, sql)

	return result, nil
}

// Execute 执行语句并返回影响的行数。
func (a *InMemoryDBAdapter) Execute(ctx context.Context, sql string, params ...any) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.connected {
		return 0, fmt.Errorf("not connected")
	}

	sqlUpper := strings.ToUpper(strings.TrimSpace(sql))

	if strings.HasPrefix(sqlUpper, "INSERT") {
		return a.executeInsert(sql, params)
	} else if strings.HasPrefix(sqlUpper, "UPDATE") {
		return a.executeUpdate(sql, params)
	} else if strings.HasPrefix(sqlUpper, "DELETE") {
		return a.executeDelete(sql, params)
	} else if strings.HasPrefix(sqlUpper, "CREATE") {
		return a.executeCreate(sql)
	}

	return 0, fmt.Errorf("unsupported SQL statement")
}

// Count 执行计数查询。
func (a *InMemoryDBAdapter) Count(ctx context.Context, sql string, params ...any) (int64, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.connected {
		return 0, fmt.Errorf("not connected")
	}

	tableName := a.extractTableName(sql, "FROM")
	if tableName == "" {
		return 0, fmt.Errorf("could not parse table name from SQL")
	}

	rows, ok := a.tables[tableName]
	if !ok {
		return 0, nil
	}

	result := a.applyWhere(rows, sql, params)
	return int64(len(result)), nil
}

// Exists 检查是否有匹配查询的行。
func (a *InMemoryDBAdapter) Exists(ctx context.Context, sql string, params ...any) (bool, error) {
	count, err := a.Count(ctx, sql, params...)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// BeginTx 开始事务。
func (a *InMemoryDBAdapter) BeginTx(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.connected {
		return "", fmt.Errorf("not connected")
	}

	a.txCounter++
	txID := fmt.Sprintf("tx_%d", a.txCounter)

	// 创建快照
	snapshot := make(map[string][]map[string]any)
	for table, rows := range a.tables {
		rowsCopy := make([]map[string]any, len(rows))
		for i, row := range rows {
			rowCopy := make(map[string]any)
			for k, v := range row {
				rowCopy[k] = v
			}
			rowsCopy[i] = rowCopy
		}
		snapshot[table] = rowsCopy
	}

	a.transactions[txID] = &inMemoryTx{
		id:       txID,
		snapshot: snapshot,
	}

	return txID, nil
}

// CommitTx 提交事务。
func (a *InMemoryDBAdapter) CommitTx(ctx context.Context, txID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.transactions[txID]; !ok {
		return fmt.Errorf("transaction not found: %s", txID)
	}

	delete(a.transactions, txID)
	return nil
}

// RollbackTx 回滚事务。
func (a *InMemoryDBAdapter) RollbackTx(ctx context.Context, txID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	tx, ok := a.transactions[txID]
	if !ok {
		return fmt.Errorf("transaction not found: %s", txID)
	}

	// 恢复快照
	a.tables = tx.snapshot
	delete(a.transactions, txID)
	return nil
}

// Close 关闭连接。
func (a *InMemoryDBAdapter) Close(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.connected = false
	return nil
}

// IsConnected 返回适配器是否已连接。
func (a *InMemoryDBAdapter) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.connected
}

// InsertRow 向表中插入一行（用于测试）。
func (a *InMemoryDBAdapter) InsertRow(table string, row map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tables[table] = append(a.tables[table], row)
}

// GetTable 返回表中的所有行（用于测试）。
func (a *InMemoryDBAdapter) GetTable(table string) []map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.tables[table]
}

// Clear 清除所有数据（用于测试）。
func (a *InMemoryDBAdapter) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tables = make(map[string][]map[string]any)
	a.transactions = make(map[string]*inMemoryTx)
}

// extractTableName 从 SQL 中提取表名
func (a *InMemoryDBAdapter) extractTableName(sql string, keyword string) string {
	sqlUpper := strings.ToUpper(sql)
	idx := strings.Index(sqlUpper, keyword)
	if idx == -1 {
		return ""
	}

	rest := strings.TrimSpace(sql[idx+len(keyword):])
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return ""
	}

	// 移除可能的引号
	tableName := strings.Trim(parts[0], "`\"'")
	return tableName
}

// applyWhere 应用 WHERE 条件（简化实现）
func (a *InMemoryDBAdapter) applyWhere(rows []map[string]any, sql string, params []any) []map[string]any {
	sqlUpper := strings.ToUpper(sql)
	whereIdx := strings.Index(sqlUpper, "WHERE")
	if whereIdx == -1 {
		return rows
	}

	// 简单的 WHERE column = ? 解析
	whereClause := sql[whereIdx+5:]
	// 移除 ORDER BY, LIMIT 等
	for _, keyword := range []string{"ORDER BY", "LIMIT", "GROUP BY"} {
		if idx := strings.Index(strings.ToUpper(whereClause), keyword); idx != -1 {
			whereClause = whereClause[:idx]
		}
	}

	whereClause = strings.TrimSpace(whereClause)
	if whereClause == "" {
		return rows
	}

	// 解析条件
	var result []map[string]any
	paramIdx := 0

	for _, row := range rows {
		if a.matchesWhere(row, whereClause, params, &paramIdx) {
			result = append(result, row)
		}
		paramIdx = 0 // 重置参数索引
	}

	return result
}

// matchesWhere 检查行是否匹配 WHERE 条件
func (a *InMemoryDBAdapter) matchesWhere(row map[string]any, whereClause string, params []any, paramIdx *int) bool {
	// 简单实现：支持 column = ? 格式
	re := regexp.MustCompile(`(\w+)\s*=\s*\?`)
	matches := re.FindAllStringSubmatch(whereClause, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		column := match[1]
		if *paramIdx >= len(params) {
			return false
		}
		param := params[*paramIdx]
		*paramIdx++

		rowValue, ok := row[column]
		if !ok {
			return false
		}

		// 简单比较
		if fmt.Sprintf("%v", rowValue) != fmt.Sprintf("%v", param) {
			return false
		}
	}

	return true
}

// applyLimit 应用 LIMIT
func (a *InMemoryDBAdapter) applyLimit(rows []map[string]any, sql string) []map[string]any {
	sqlUpper := strings.ToUpper(sql)
	limitIdx := strings.Index(sqlUpper, "LIMIT")
	if limitIdx == -1 {
		return rows
	}

	limitClause := strings.TrimSpace(sql[limitIdx+5:])
	parts := strings.Fields(limitClause)
	if len(parts) == 0 {
		return rows
	}

	limit, err := strconv.Atoi(parts[0])
	if err != nil || limit <= 0 {
		return rows
	}

	if limit > len(rows) {
		return rows
	}
	return rows[:limit]
}

// executeInsert 执行 INSERT
func (a *InMemoryDBAdapter) executeInsert(sql string, params []any) (int64, error) {
	// 简单解析 INSERT INTO table (columns) VALUES (?)
	tableName := a.extractTableName(sql, "INTO")
	if tableName == "" {
		return 0, fmt.Errorf("could not parse table name")
	}

	// 提取列名
	re := regexp.MustCompile(`\(([^)]+)\)\s*VALUES`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 2 {
		return 0, fmt.Errorf("could not parse columns")
	}

	columns := strings.Split(matches[1], ",")
	for i := range columns {
		columns[i] = strings.TrimSpace(columns[i])
	}

	if len(columns) != len(params) {
		return 0, fmt.Errorf("column count doesn't match parameter count")
	}

	row := make(map[string]any)
	for i, col := range columns {
		row[col] = params[i]
	}

	a.tables[tableName] = append(a.tables[tableName], row)
	return 1, nil
}

// executeUpdate 执行 UPDATE
func (a *InMemoryDBAdapter) executeUpdate(sql string, params []any) (int64, error) {
	tableName := a.extractTableName(sql, "UPDATE")
	if tableName == "" {
		return 0, fmt.Errorf("could not parse table name")
	}

	rows, ok := a.tables[tableName]
	if !ok {
		return 0, nil
	}

	// 简化实现：更新所有匹配的行
	var affected int64
	for _, row := range rows {
		// 这里简化处理，实际应该解析 SET 和 WHERE
		affected++
		_ = row
	}

	return affected, nil
}

// executeDelete 执行 DELETE
func (a *InMemoryDBAdapter) executeDelete(sql string, params []any) (int64, error) {
	tableName := a.extractTableName(sql, "FROM")
	if tableName == "" {
		return 0, fmt.Errorf("could not parse table name")
	}

	rows, ok := a.tables[tableName]
	if !ok {
		return 0, nil
	}

	// 应用 WHERE 条件
	remaining := a.applyWhereNot(rows, sql, params)
	deleted := int64(len(rows) - len(remaining))
	a.tables[tableName] = remaining

	return deleted, nil
}

// applyWhereNot 返回不匹配 WHERE 条件的行
func (a *InMemoryDBAdapter) applyWhereNot(rows []map[string]any, sql string, params []any) []map[string]any {
	sqlUpper := strings.ToUpper(sql)
	whereIdx := strings.Index(sqlUpper, "WHERE")
	if whereIdx == -1 {
		// 没有 WHERE，删除所有
		return []map[string]any{}
	}

	whereClause := sql[whereIdx+5:]
	whereClause = strings.TrimSpace(whereClause)

	var result []map[string]any
	paramIdx := 0

	for _, row := range rows {
		if !a.matchesWhere(row, whereClause, params, &paramIdx) {
			result = append(result, row)
		}
		paramIdx = 0
	}

	return result
}

// executeCreate 执行 CREATE TABLE
func (a *InMemoryDBAdapter) executeCreate(sql string) (int64, error) {
	// 简单解析 CREATE TABLE name
	re := regexp.MustCompile(`CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)
	matches := re.FindStringSubmatch(strings.ToUpper(sql))
	if len(matches) < 2 {
		return 0, fmt.Errorf("could not parse table name")
	}

	tableName := strings.ToLower(matches[1])
	if _, ok := a.tables[tableName]; !ok {
		a.tables[tableName] = []map[string]any{}
	}

	return 0, nil
}
