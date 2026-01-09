package logic

import (
	"testing"

	"pgregory.net/rapid"
)

// TestProperty14_ExecutorLabelFiltering 测试执行机标签筛选
// Property 14: 执行机标签筛选
// 对于任意标签筛选条件，返回的执行机列表中的每个执行机都应包含指定的标签。
// Feature: gulu-extension, Property 14: 执行机标签筛选
// Validates: Requirements 7.5
func TestProperty14_ExecutorLabelFiltering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成随机的执行机标签
		numLabels := rapid.IntRange(0, 5).Draw(t, "numLabels")
		executorLabels := make(map[string]string)
		for i := 0; i < numLabels; i++ {
			key := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "labelKey")
			value := rapid.StringMatching(`[a-z0-9]{1,10}`).Draw(t, "labelValue")
			executorLabels[key] = value
		}

		// 生成随机的筛选标签（从执行机标签中选择子集或生成新的）
		numFilterLabels := rapid.IntRange(0, 3).Draw(t, "numFilterLabels")
		filterLabels := make(map[string]string)

		// 决定是否使用执行机已有的标签
		useExistingLabels := rapid.Bool().Draw(t, "useExistingLabels")

		if useExistingLabels && len(executorLabels) > 0 {
			// 从执行机标签中选择子集
			keys := make([]string, 0, len(executorLabels))
			for k := range executorLabels {
				keys = append(keys, k)
			}
			for i := 0; i < numFilterLabels && i < len(keys); i++ {
				idx := rapid.IntRange(0, len(keys)-1).Draw(t, "keyIndex")
				key := keys[idx]
				filterLabels[key] = executorLabels[key]
			}
		} else {
			// 生成新的筛选标签
			for i := 0; i < numFilterLabels; i++ {
				key := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "filterKey")
				value := rapid.StringMatching(`[a-z0-9]{1,10}`).Draw(t, "filterValue")
				filterLabels[key] = value
			}
		}

		// 执行标签匹配
		matched := MatchLabels(executorLabels, filterLabels)

		// 验证属性：如果匹配成功，执行机必须包含所有筛选标签
		if matched {
			for key, value := range filterLabels {
				if executorLabels[key] != value {
					t.Fatalf("Property 14 violated: matched executor should contain filter label %s=%s, but has %s=%s",
						key, value, key, executorLabels[key])
				}
			}
		}

		// 验证属性：如果筛选标签为空，应该总是匹配
		if len(filterLabels) == 0 && !matched {
			t.Fatal("Property 14 violated: empty filter labels should always match")
		}

		// 验证属性：如果执行机标签为空但筛选标签不为空，应该不匹配
		if len(executorLabels) == 0 && len(filterLabels) > 0 && matched {
			t.Fatal("Property 14 violated: empty executor labels should not match non-empty filter labels")
		}
	})
}

// TestMatchLabels_EdgeCases 测试标签匹配的边界情况
func TestMatchLabels_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		executorLabels map[string]string
		filterLabels   map[string]string
		expected       bool
	}{
		{
			name:           "both empty",
			executorLabels: map[string]string{},
			filterLabels:   map[string]string{},
			expected:       true,
		},
		{
			name:           "filter empty",
			executorLabels: map[string]string{"env": "prod"},
			filterLabels:   map[string]string{},
			expected:       true,
		},
		{
			name:           "executor empty, filter not empty",
			executorLabels: map[string]string{},
			filterLabels:   map[string]string{"env": "prod"},
			expected:       false,
		},
		{
			name:           "exact match",
			executorLabels: map[string]string{"env": "prod", "region": "cn-east"},
			filterLabels:   map[string]string{"env": "prod", "region": "cn-east"},
			expected:       true,
		},
		{
			name:           "subset match",
			executorLabels: map[string]string{"env": "prod", "region": "cn-east", "team": "qa"},
			filterLabels:   map[string]string{"env": "prod"},
			expected:       true,
		},
		{
			name:           "value mismatch",
			executorLabels: map[string]string{"env": "prod"},
			filterLabels:   map[string]string{"env": "dev"},
			expected:       false,
		},
		{
			name:           "key not found",
			executorLabels: map[string]string{"env": "prod"},
			filterLabels:   map[string]string{"region": "cn-east"},
			expected:       false,
		},
		{
			name:           "partial match fails",
			executorLabels: map[string]string{"env": "prod"},
			filterLabels:   map[string]string{"env": "prod", "region": "cn-east"},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchLabels(tt.executorLabels, tt.filterLabels)
			if result != tt.expected {
				t.Errorf("MatchLabels(%v, %v) = %v, want %v",
					tt.executorLabels, tt.filterLabels, result, tt.expected)
			}
		})
	}
}

// TestExecutorTypeValidation 测试执行机类型验证
func TestExecutorTypeValidation(t *testing.T) {
	tests := []struct {
		executorType string
		valid        bool
	}{
		{"performance", true},
		{"normal", true},
		{"debug", true},
		{"invalid", false},
		{"", false},
		{"PERFORMANCE", false}, // 大小写敏感
		{"Normal", false},
	}

	for _, tt := range tests {
		t.Run(tt.executorType, func(t *testing.T) {
			result := isValidExecutorType(tt.executorType)
			if result != tt.valid {
				t.Errorf("isValidExecutorType(%q) = %v, want %v", tt.executorType, result, tt.valid)
			}
		})
	}
}
