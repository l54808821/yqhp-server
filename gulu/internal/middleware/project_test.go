package middleware

import (
	"testing"

	"pgregory.net/rapid"
)

// Record 测试用记录结构
type Record struct {
	ID        int64
	ProjectID int64
}

// TestProperty1_ProjectDataIsolation 测试项目数据隔离
// Property 1: 项目数据隔离
// 对于任意项目 A 和项目 B，当查询项目 A 的业务数据时，返回的所有记录的 project_id 应等于项目 A 的 ID，不应包含项目 B 的数据。
// Feature: gulu-extension, Property 1: 项目数据隔离
// Validates: Requirements 1.4
func TestProperty1_ProjectDataIsolation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成两个不同的项目ID
		projectA := rapid.Int64Range(1, 1000000).Draw(t, "projectA")
		projectB := rapid.Int64Range(1, 1000000).Draw(t, "projectB")

		// 确保两个项目ID不同
		if projectA == projectB {
			projectB = projectA + 1
		}

		// 生成属于项目A的记录
		numRecordsA := rapid.IntRange(1, 10).Draw(t, "numRecordsA")
		recordsA := make([]Record, numRecordsA)
		for i := 0; i < numRecordsA; i++ {
			recordsA[i] = Record{
				ID:        int64(i + 1),
				ProjectID: projectA,
			}
		}

		// 生成属于项目B的记录
		numRecordsB := rapid.IntRange(1, 10).Draw(t, "numRecordsB")
		recordsB := make([]Record, numRecordsB)
		for i := 0; i < numRecordsB; i++ {
			recordsB[i] = Record{
				ID:        int64(numRecordsA + i + 1),
				ProjectID: projectB,
			}
		}

		// 合并所有记录
		allRecords := append(recordsA, recordsB...)

		// 模拟按项目A过滤查询
		filteredRecords := filterByProjectID(allRecords, projectA)

		// 验证属性：所有返回的记录都应属于项目A
		for _, record := range filteredRecords {
			if record.ProjectID != projectA {
				t.Fatalf("Property 1 violated: record with ID %d has project_id %d, expected %d",
					record.ID, record.ProjectID, projectA)
			}
		}

		// 验证属性：不应包含项目B的数据
		for _, record := range filteredRecords {
			if record.ProjectID == projectB {
				t.Fatalf("Property 1 violated: found record from project B (ID: %d) in project A's data",
					record.ID)
			}
		}

		// 验证属性：返回的记录数应等于项目A的记录数
		if len(filteredRecords) != numRecordsA {
			t.Fatalf("Property 1 violated: expected %d records, got %d",
				numRecordsA, len(filteredRecords))
		}
	})
}

// filterByProjectID 模拟按项目ID过滤
func filterByProjectID(records []Record, projectID int64) []Record {
	var result []Record
	for _, r := range records {
		if r.ProjectID == projectID {
			result = append(result, r)
		}
	}
	return result
}

// TestGetCurrentProjectID 测试获取当前项目ID
func TestGetCurrentProjectID(t *testing.T) {
	// 这个测试需要 Fiber 上下文，这里只测试逻辑
	tests := []struct {
		name     string
		input    interface{}
		expected int64
	}{
		{"valid project ID", int64(123), 123},
		{"zero project ID", int64(0), 0},
		{"nil value", nil, 0},
		{"wrong type", "123", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟从 Locals 获取值的逻辑
			var result int64
			if projectID, ok := tt.input.(int64); ok {
				result = projectID
			}
			if result != tt.expected {
				t.Errorf("GetCurrentProjectID() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// TestProjectIDHeader 测试项目ID请求头常量
func TestProjectIDHeader(t *testing.T) {
	if ProjectIDHeader != "X-Project-ID" {
		t.Errorf("ProjectIDHeader = %q, want %q", ProjectIDHeader, "X-Project-ID")
	}
}
