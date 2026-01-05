package keyword

import (
	"context"
	"testing"
)

// mockKeyword is a simple keyword implementation for testing.
type mockKeyword struct {
	BaseKeyword
}

func newMockKeyword(name string, category Category) *mockKeyword {
	return &mockKeyword{
		BaseKeyword: NewBaseKeyword(name, category, "mock keyword for testing"),
	}
}

func (m *mockKeyword) Execute(ctx context.Context, execCtx *ExecutionContext, params map[string]any) (*Result, error) {
	return NewSuccessResult("executed", nil), nil
}

func (m *mockKeyword) Validate(params map[string]any) error {
	return nil
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	// Test successful registration
	kw := newMockKeyword("test_keyword", CategoryAssertion)
	err := r.Register(kw)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Test duplicate registration
	err = r.Register(kw)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}

	// Test nil keyword
	err = r.Register(nil)
	if err == nil {
		t.Error("expected error for nil keyword")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	kw := newMockKeyword("test_keyword", CategoryAssertion)
	r.MustRegister(kw)

	// Test successful get
	got, err := r.Get("test_keyword")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if got.Name() != "test_keyword" {
		t.Errorf("expected name 'test_keyword', got '%s'", got.Name())
	}

	// Test not found
	_, err = r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent keyword")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()
	kw := newMockKeyword("test_keyword", CategoryAssertion)
	r.MustRegister(kw)

	// Test successful unregister
	err := r.Unregister("test_keyword")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Verify it's gone
	if r.Has("test_keyword") {
		t.Error("keyword should be unregistered")
	}

	// Test unregister nonexistent
	err = r.Unregister("nonexistent")
	if err == nil {
		t.Error("expected error for unregistering nonexistent keyword")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(newMockKeyword("assert1", CategoryAssertion))
	r.MustRegister(newMockKeyword("assert2", CategoryAssertion))
	r.MustRegister(newMockKeyword("extract1", CategoryExtractor))

	// Test list by category
	assertions := r.List(CategoryAssertion)
	if len(assertions) != 2 {
		t.Errorf("expected 2 assertions, got %d", len(assertions))
	}

	extractors := r.List(CategoryExtractor)
	if len(extractors) != 1 {
		t.Errorf("expected 1 extractor, got %d", len(extractors))
	}

	// Test list all
	all := r.List("")
	if len(all) != 3 {
		t.Errorf("expected 3 keywords, got %d", len(all))
	}
}

func TestRegistry_Has(t *testing.T) {
	r := NewRegistry()
	kw := newMockKeyword("test_keyword", CategoryAssertion)
	r.MustRegister(kw)

	if !r.Has("test_keyword") {
		t.Error("expected Has to return true")
	}

	if r.Has("nonexistent") {
		t.Error("expected Has to return false for nonexistent")
	}
}

func TestRegistry_Count(t *testing.T) {
	r := NewRegistry()
	if r.Count() != 0 {
		t.Errorf("expected count 0, got %d", r.Count())
	}

	r.MustRegister(newMockKeyword("kw1", CategoryAssertion))
	r.MustRegister(newMockKeyword("kw2", CategoryExtractor))

	if r.Count() != 2 {
		t.Errorf("expected count 2, got %d", r.Count())
	}
}

func TestRegistry_Clear(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(newMockKeyword("kw1", CategoryAssertion))
	r.MustRegister(newMockKeyword("kw2", CategoryExtractor))

	r.Clear()

	if r.Count() != 0 {
		t.Errorf("expected count 0 after clear, got %d", r.Count())
	}
}
