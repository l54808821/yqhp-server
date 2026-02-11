package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"yqhp/workflow-engine/pkg/types"
)

// mockTool 用于测试的模拟工具
type mockTool struct {
	name string
}

func (m *mockTool) Definition() *types.ToolDefinition {
	return &types.ToolDefinition{
		Name:        m.name,
		Description: "测试工具: " + m.name,
		Parameters:  json.RawMessage(`{"type":"object"}`),
	}
}

func (m *mockTool) Execute(ctx context.Context, arguments string, execCtx *ExecutionContext) (*types.ToolResult, error) {
	return &types.ToolResult{Content: "ok"}, nil
}

func TestNewToolRegistry(t *testing.T) {
	r := NewToolRegistry()
	assert.NotNil(t, r)
	assert.Empty(t, r.List())
}

func TestToolRegistry_Register(t *testing.T) {
	r := NewToolRegistry()

	err := r.Register(&mockTool{name: "test_tool"})
	require.NoError(t, err)
	assert.True(t, r.Has("test_tool"))
}

func TestToolRegistry_Register_NilTool(t *testing.T) {
	r := NewToolRegistry()
	err := r.Register(nil)
	assert.Error(t, err)
}

func TestToolRegistry_Register_DuplicateName(t *testing.T) {
	r := NewToolRegistry()

	err := r.Register(&mockTool{name: "dup"})
	require.NoError(t, err)

	err = r.Register(&mockTool{name: "dup"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已注册")
}

func TestToolRegistry_Get(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&mockTool{name: "foo"})

	tool, ok := r.Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "foo", tool.Definition().Name)

	_, ok = r.Get("nonexistent")
	assert.False(t, ok)
}

func TestToolRegistry_List(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&mockTool{name: "a"})
	r.Register(&mockTool{name: "b"})

	defs := r.List()
	assert.Len(t, defs, 2)

	names := map[string]bool{}
	for _, d := range defs {
		names[d.Name] = true
	}
	assert.True(t, names["a"])
	assert.True(t, names["b"])
}

func TestToolRegistry_Has(t *testing.T) {
	r := NewToolRegistry()
	assert.False(t, r.Has("x"))

	r.Register(&mockTool{name: "x"})
	assert.True(t, r.Has("x"))
}

func TestDefaultToolRegistry_ConvenienceFunctions(t *testing.T) {
	// 保存并恢复全局注册表
	original := DefaultToolRegistry
	DefaultToolRegistry = NewToolRegistry()
	defer func() { DefaultToolRegistry = original }()

	err := RegisterTool(&mockTool{name: "global_tool"})
	require.NoError(t, err)

	assert.True(t, HasTool("global_tool"))

	tool, ok := GetTool("global_tool")
	assert.True(t, ok)
	assert.Equal(t, "global_tool", tool.Definition().Name)

	defs := ListTools()
	assert.Len(t, defs, 1)
	assert.Equal(t, "global_tool", defs[0].Name)
}
