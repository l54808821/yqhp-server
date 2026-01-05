package executor

import (
	"context"
	"testing"

	"yqhp/workflow-engine/pkg/types"
)

// TestDBExecutor_Query tests database query.
func TestDBExecutor_Query(t *testing.T) {
	executor := NewDBExecutor()
	ctx := context.Background()

	adapter := NewInMemoryDBAdapter()
	executor.RegisterAdapter(DBDriverSQLite, adapter)

	err := executor.Init(ctx, map[string]any{
		"driver": "sqlite",
		"dsn":    ":memory:",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	// 插入测试数据
	adapter.InsertRow("users", map[string]any{"id": 1, "name": "Alice", "age": 30})
	adapter.InsertRow("users", map[string]any{"id": 2, "name": "Bob", "age": 25})

	step := &types.Step{
		ID: "test-query",
		Config: map[string]any{
			"action": "query",
			"sql":    "SELECT * FROM users",
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusSuccess {
		t.Errorf("expected success, got %s: %s", result.Status, result.Error)
	}

	output := result.Output.(*DBResult)
	if !output.Success {
		t.Errorf("expected success, got error: %s", output.Error)
	}
	if len(output.Data) != 2 {
		t.Errorf("expected 2 rows, got %d", len(output.Data))
	}
}

// TestDBExecutor_QueryWithWhere tests query with WHERE clause.
func TestDBExecutor_QueryWithWhere(t *testing.T) {
	executor := NewDBExecutor()
	ctx := context.Background()

	adapter := NewInMemoryDBAdapter()
	executor.RegisterAdapter(DBDriverSQLite, adapter)

	err := executor.Init(ctx, map[string]any{
		"driver": "sqlite",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	adapter.InsertRow("users", map[string]any{"id": 1, "name": "Alice"})
	adapter.InsertRow("users", map[string]any{"id": 2, "name": "Bob"})

	step := &types.Step{
		ID: "test-query",
		Config: map[string]any{
			"action": "query",
			"sql":    "SELECT * FROM users WHERE id = ?",
			"params": []any{1},
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := result.Output.(*DBResult)
	if len(output.Data) != 1 {
		t.Errorf("expected 1 row, got %d", len(output.Data))
	}
	if output.Data[0]["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", output.Data[0]["name"])
	}
}

// TestDBExecutor_Execute tests execute statement.
func TestDBExecutor_Execute(t *testing.T) {
	executor := NewDBExecutor()
	ctx := context.Background()

	adapter := NewInMemoryDBAdapter()
	executor.RegisterAdapter(DBDriverSQLite, adapter)

	err := executor.Init(ctx, map[string]any{
		"driver": "sqlite",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	step := &types.Step{
		ID: "test-insert",
		Config: map[string]any{
			"action": "execute",
			"sql":    "INSERT INTO users (id, name) VALUES (?, ?)",
			"params": []any{1, "Charlie"},
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusSuccess {
		t.Errorf("expected success, got %s: %s", result.Status, result.Error)
	}

	output := result.Output.(*DBResult)
	if output.RowsAffected != 1 {
		t.Errorf("expected 1 row affected, got %d", output.RowsAffected)
	}

	// 验证数据已插入
	rows := adapter.GetTable("users")
	if len(rows) != 1 {
		t.Errorf("expected 1 row in table, got %d", len(rows))
	}
}

// TestDBExecutor_Count tests count operation.
func TestDBExecutor_Count(t *testing.T) {
	executor := NewDBExecutor()
	ctx := context.Background()

	adapter := NewInMemoryDBAdapter()
	executor.RegisterAdapter(DBDriverSQLite, adapter)

	err := executor.Init(ctx, map[string]any{
		"driver": "sqlite",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	adapter.InsertRow("users", map[string]any{"id": 1, "status": "active"})
	adapter.InsertRow("users", map[string]any{"id": 2, "status": "active"})
	adapter.InsertRow("users", map[string]any{"id": 3, "status": "inactive"})

	step := &types.Step{
		ID: "test-count",
		Config: map[string]any{
			"action": "count",
			"sql":    "SELECT COUNT(*) FROM users WHERE status = ?",
			"params": []any{"active"},
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := result.Output.(*DBResult)
	if output.Count != 2 {
		t.Errorf("expected count 2, got %d", output.Count)
	}
}

// TestDBExecutor_Exists tests exists operation.
func TestDBExecutor_Exists(t *testing.T) {
	executor := NewDBExecutor()
	ctx := context.Background()

	adapter := NewInMemoryDBAdapter()
	executor.RegisterAdapter(DBDriverSQLite, adapter)

	err := executor.Init(ctx, map[string]any{
		"driver": "sqlite",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	adapter.InsertRow("users", map[string]any{"id": 1, "name": "Alice"})

	// 存在的记录
	step := &types.Step{
		ID: "test-exists",
		Config: map[string]any{
			"action": "exists",
			"sql":    "SELECT 1 FROM users WHERE name = ?",
			"params": []any{"Alice"},
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := result.Output.(*DBResult)
	if !output.Exists {
		t.Error("expected exists to be true")
	}

	// 不存在的记录
	step.Config["params"] = []any{"NonExistent"}
	result, err = executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output = result.Output.(*DBResult)
	if output.Exists {
		t.Error("expected exists to be false")
	}
}

// TestDBExecutor_Delete tests delete operation.
func TestDBExecutor_Delete(t *testing.T) {
	executor := NewDBExecutor()
	ctx := context.Background()

	adapter := NewInMemoryDBAdapter()
	executor.RegisterAdapter(DBDriverSQLite, adapter)

	err := executor.Init(ctx, map[string]any{
		"driver": "sqlite",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	adapter.InsertRow("users", map[string]any{"id": 1, "name": "Alice"})
	adapter.InsertRow("users", map[string]any{"id": 2, "name": "Bob"})

	step := &types.Step{
		ID: "test-delete",
		Config: map[string]any{
			"action": "execute",
			"sql":    "DELETE FROM users WHERE id = ?",
			"params": []any{1},
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := result.Output.(*DBResult)
	if output.RowsAffected != 1 {
		t.Errorf("expected 1 row deleted, got %d", output.RowsAffected)
	}

	// 验证只剩一行
	rows := adapter.GetTable("users")
	if len(rows) != 1 {
		t.Errorf("expected 1 row remaining, got %d", len(rows))
	}
}

// TestDBExecutor_Transaction tests transaction operations.
func TestDBExecutor_Transaction(t *testing.T) {
	executor := NewDBExecutor()
	ctx := context.Background()

	adapter := NewInMemoryDBAdapter()
	executor.RegisterAdapter(DBDriverSQLite, adapter)

	err := executor.Init(ctx, map[string]any{
		"driver": "sqlite",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	adapter.InsertRow("users", map[string]any{"id": 1, "name": "Alice"})

	// 开始事务
	beginStep := &types.Step{
		ID: "begin",
		Config: map[string]any{
			"action": "begin",
		},
	}

	result, err := executor.Execute(ctx, beginStep, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := result.Output.(*DBResult)
	txID := output.TxID
	if txID == "" {
		t.Error("expected transaction ID")
	}

	// 插入数据
	insertStep := &types.Step{
		ID: "insert",
		Config: map[string]any{
			"action": "execute",
			"sql":    "INSERT INTO users (id, name) VALUES (?, ?)",
			"params": []any{2, "Bob"},
		},
	}
	executor.Execute(ctx, insertStep, nil)

	// 回滚事务
	rollbackStep := &types.Step{
		ID: "rollback",
		Config: map[string]any{
			"action": "rollback",
			"tx_id":  txID,
		},
	}

	result, err = executor.Execute(ctx, rollbackStep, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusSuccess {
		t.Errorf("expected success, got %s", result.Status)
	}

	// 验证数据已回滚
	rows := adapter.GetTable("users")
	if len(rows) != 1 {
		t.Errorf("expected 1 row after rollback, got %d", len(rows))
	}
}

// TestDBExecutor_TransactionCommit tests transaction commit.
func TestDBExecutor_TransactionCommit(t *testing.T) {
	executor := NewDBExecutor()
	ctx := context.Background()

	adapter := NewInMemoryDBAdapter()
	executor.RegisterAdapter(DBDriverSQLite, adapter)

	err := executor.Init(ctx, map[string]any{
		"driver": "sqlite",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	// 开始事务
	beginStep := &types.Step{
		ID: "begin",
		Config: map[string]any{
			"action": "begin",
		},
	}

	result, _ := executor.Execute(ctx, beginStep, nil)
	txID := result.Output.(*DBResult).TxID

	// 插入数据
	insertStep := &types.Step{
		ID: "insert",
		Config: map[string]any{
			"action": "execute",
			"sql":    "INSERT INTO users (id, name) VALUES (?, ?)",
			"params": []any{1, "Alice"},
		},
	}
	executor.Execute(ctx, insertStep, nil)

	// 提交事务
	commitStep := &types.Step{
		ID: "commit",
		Config: map[string]any{
			"action": "commit",
			"tx_id":  txID,
		},
	}

	result, err = executor.Execute(ctx, commitStep, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusSuccess {
		t.Errorf("expected success, got %s", result.Status)
	}

	// 验证数据已提交
	rows := adapter.GetTable("users")
	if len(rows) != 1 {
		t.Errorf("expected 1 row after commit, got %d", len(rows))
	}
}

// TestDBExecutor_VariableResolution tests variable resolution.
func TestDBExecutor_VariableResolution(t *testing.T) {
	executor := NewDBExecutor()
	ctx := context.Background()

	adapter := NewInMemoryDBAdapter()
	executor.RegisterAdapter(DBDriverSQLite, adapter)

	err := executor.Init(ctx, map[string]any{
		"driver": "sqlite",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	adapter.InsertRow("users", map[string]any{"id": 1, "name": "Alice"})

	// 创建执行上下文
	execCtx := NewExecutionContext()
	execCtx.SetVariable("user_id", "1")

	step := &types.Step{
		ID: "test-query",
		Config: map[string]any{
			"action": "query",
			"sql":    "SELECT * FROM users WHERE id = ?",
			"params": []any{"${user_id}"},
		},
	}

	result, err := executor.Execute(ctx, step, execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := result.Output.(*DBResult)
	if len(output.Data) != 1 {
		t.Errorf("expected 1 row, got %d", len(output.Data))
	}
}

// TestDBExecutor_InvalidAction tests invalid action.
func TestDBExecutor_InvalidAction(t *testing.T) {
	executor := NewDBExecutor()
	ctx := context.Background()

	adapter := NewInMemoryDBAdapter()
	executor.RegisterAdapter(DBDriverSQLite, adapter)

	err := executor.Init(ctx, map[string]any{
		"driver": "sqlite",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}

	step := &types.Step{
		ID: "invalid",
		Config: map[string]any{
			"action": "invalid",
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusFailed {
		t.Errorf("expected failed status, got %s", result.Status)
	}
}

// TestDBExecutor_MissingAction tests missing action.
func TestDBExecutor_MissingAction(t *testing.T) {
	executor := NewDBExecutor()
	ctx := context.Background()

	err := executor.Init(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}

	step := &types.Step{
		ID:     "missing",
		Config: map[string]any{},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != types.ResultStatusFailed {
		t.Errorf("expected failed status, got %s", result.Status)
	}
}

// TestDBExecutor_QueryWithLimit tests query with LIMIT.
func TestDBExecutor_QueryWithLimit(t *testing.T) {
	executor := NewDBExecutor()
	ctx := context.Background()

	adapter := NewInMemoryDBAdapter()
	executor.RegisterAdapter(DBDriverSQLite, adapter)

	err := executor.Init(ctx, map[string]any{
		"driver": "sqlite",
	})
	if err != nil {
		t.Fatalf("failed to init executor: %v", err)
	}
	defer executor.Cleanup(ctx)

	for i := 1; i <= 10; i++ {
		adapter.InsertRow("users", map[string]any{"id": i, "name": "User"})
	}

	step := &types.Step{
		ID: "test-query",
		Config: map[string]any{
			"action": "query",
			"sql":    "SELECT * FROM users LIMIT 5",
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := result.Output.(*DBResult)
	if len(output.Data) != 5 {
		t.Errorf("expected 5 rows, got %d", len(output.Data))
	}
}

// TestInMemoryDBAdapter_Clear tests clearing data.
func TestInMemoryDBAdapter_Clear(t *testing.T) {
	adapter := NewInMemoryDBAdapter()
	ctx := context.Background()

	adapter.Connect(ctx, &DBConfig{})
	adapter.InsertRow("users", map[string]any{"id": 1})
	adapter.InsertRow("orders", map[string]any{"id": 1})

	if len(adapter.GetTable("users")) != 1 {
		t.Error("expected 1 user")
	}
	if len(adapter.GetTable("orders")) != 1 {
		t.Error("expected 1 order")
	}

	adapter.Clear()

	if len(adapter.GetTable("users")) != 0 {
		t.Error("expected 0 users after clear")
	}
	if len(adapter.GetTable("orders")) != 0 {
		t.Error("expected 0 orders after clear")
	}
}
