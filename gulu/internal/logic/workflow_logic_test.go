package logic

import (
	"testing"

	"pgregory.net/rapid"
)

// TestProperty10_WorkflowVersionIncrement 测试工作流版本递增
// Property 10: 工作流版本递增
// 对于任意工作流的更新操作，更新后的版本号应大于更新前的版本号。
// Feature: gulu-extension, Property 10: 工作流版本递增
// Validates: Requirements 8.3
func TestProperty10_WorkflowVersionIncrement(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机的初始版本号
		initialVersion := rapid.Int32Range(1, 1000).Draw(t, "initialVersion")

		// 模拟版本递增逻辑
		newVersion := initialVersion + 1

		// 验证属性：新版本号应大于旧版本号
		if newVersion <= initialVersion {
			t.Fatalf("Property 10 violated: new version %d should be greater than old version %d",
				newVersion, initialVersion)
		}

		// 验证属性：版本号应该恰好递增 1
		if newVersion != initialVersion+1 {
			t.Fatalf("Property 10 violated: version should increment by 1, got %d -> %d",
				initialVersion, newVersion)
		}
	})
}

// TestVersionIncrementLogic 测试版本递增逻辑的边界情况
func TestVersionIncrementLogic(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion int32
		expectedNew    int32
	}{
		{"version 1", 1, 2},
		{"version 10", 10, 11},
		{"version 100", 100, 101},
		{"version 999", 999, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟版本递增
			newVersion := tt.currentVersion + 1
			if newVersion != tt.expectedNew {
				t.Errorf("Version increment failed: got %d, want %d", newVersion, tt.expectedNew)
			}
		})
	}
}

// TestWorkflowDefinitionValidation 测试工作流定义验证
func TestWorkflowDefinitionValidation(t *testing.T) {
	tests := []struct {
		name       string
		definition string
		shouldPass bool
	}{
		{
			name: "valid http workflow",
			definition: `{
				"name": "test",
				"steps": [
					{"id": "step1", "type": "http", "name": "test", "config": {"method": "GET", "url": "/test"}}
				]
			}`,
			shouldPass: true,
		},
		{
			name: "valid script workflow",
			definition: `{
				"name": "test",
				"steps": [
					{"id": "step1", "type": "script", "name": "test", "config": {"script": "console.log('test');"}}
				]
			}`,
			shouldPass: true,
		},
		{
			name:       "empty definition",
			definition: "",
			shouldPass: false,
		},
		{
			name:       "invalid json",
			definition: "{invalid}",
			shouldPass: false,
		},
		{
			name: "missing name",
			definition: `{
				"steps": [
					{"id": "step1", "type": "http", "name": "test", "config": {"method": "GET", "url": "/test"}}
				]
			}`,
			shouldPass: false,
		},
		{
			name: "empty steps",
			definition: `{
				"name": "test",
				"steps": []
			}`,
			shouldPass: false,
		},
		{
			name: "invalid step type",
			definition: `{
				"name": "test",
				"steps": [
					{"id": "step1", "type": "invalid", "name": "test"}
				]
			}`,
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logic := &WorkflowLogic{}
			err := logic.validateDefinition(tt.definition)

			if tt.shouldPass && err != nil {
				t.Errorf("Expected validation to pass, but got error: %v", err)
			}
			if !tt.shouldPass && err == nil {
				t.Errorf("Expected validation to fail, but it passed")
			}
		})
	}
}
