package extractor

import (
	"context"
	"testing"

	"yqhp/workflow-engine/internal/keyword"
)

func TestHeader_Execute(t *testing.T) {
	kw := Header()
	ctx := context.Background()

	tests := []struct {
		name       string
		headers    map[string]string
		headerName string
		to         string
		want       string
		wantErr    bool
	}{
		{
			name:       "simple header",
			headers:    map[string]string{"Content-Type": "application/json"},
			headerName: "Content-Type",
			to:         "contentType",
			want:       "application/json",
		},
		{
			name:       "case insensitive",
			headers:    map[string]string{"Content-Type": "application/json"},
			headerName: "content-type",
			to:         "contentType",
			want:       "application/json",
		},
		{
			name:       "not found",
			headers:    map[string]string{"Content-Type": "application/json"},
			headerName: "X-Custom",
			to:         "custom",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execCtx := keyword.NewExecutionContext()
			execCtx.SetResponse(&keyword.ResponseData{
				Headers: tt.headers,
			})

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"name": tt.headerName,
				"to":   tt.to,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantErr {
				if result.Success {
					t.Error("expected failure")
				}
				return
			}

			if !result.Success {
				t.Errorf("expected success, got failure: %s", result.Message)
				return
			}

			val, _ := execCtx.GetVariable(tt.to)
			if val != tt.want {
				t.Errorf("got %v, want %v", val, tt.want)
			}
		})
	}
}

func TestCookie_Execute(t *testing.T) {
	kw := Cookie()
	ctx := context.Background()

	tests := []struct {
		name       string
		setCookie  string
		cookieName string
		to         string
		want       string
		wantErr    bool
	}{
		{
			name:       "simple cookie",
			setCookie:  "session=abc123; Path=/; HttpOnly",
			cookieName: "session",
			to:         "sessionId",
			want:       "abc123",
		},
		{
			name:       "cookie not found",
			setCookie:  "session=abc123; Path=/",
			cookieName: "token",
			to:         "token",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execCtx := keyword.NewExecutionContext()
			execCtx.SetResponse(&keyword.ResponseData{
				Headers: map[string]string{"Set-Cookie": tt.setCookie},
			})

			result, err := kw.Execute(ctx, execCtx, map[string]any{
				"name": tt.cookieName,
				"to":   tt.to,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantErr {
				if result.Success {
					t.Error("expected failure")
				}
				return
			}

			if !result.Success {
				t.Errorf("expected success, got failure: %s", result.Message)
				return
			}

			val, _ := execCtx.GetVariable(tt.to)
			if val != tt.want {
				t.Errorf("got %v, want %v", val, tt.want)
			}
		})
	}
}

func TestStatus_Execute(t *testing.T) {
	kw := Status()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()
	execCtx.SetResponse(&keyword.ResponseData{
		Status: 200,
	})

	result, err := kw.Execute(ctx, execCtx, map[string]any{
		"to": "statusCode",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Message)
	}

	val, _ := execCtx.GetVariable("statusCode")
	if val != 200 {
		t.Errorf("got %v, want 200", val)
	}
}

func TestBody_Execute(t *testing.T) {
	kw := Body()
	ctx := context.Background()
	execCtx := keyword.NewExecutionContext()
	execCtx.SetResponse(&keyword.ResponseData{
		Body: `{"message": "hello"}`,
	})

	result, err := kw.Execute(ctx, execCtx, map[string]any{
		"to": "responseBody",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Message)
	}

	val, _ := execCtx.GetVariable("responseBody")
	if val != `{"message": "hello"}` {
		t.Errorf("got %v, want body", val)
	}
}

func TestRegisterHeaderExtractors(t *testing.T) {
	registry := keyword.NewRegistry()
	RegisterHeaderExtractors(registry)

	expected := []string{"header", "cookie", "status", "body"}
	for _, name := range expected {
		if !registry.Has(name) {
			t.Errorf("expected keyword '%s' to be registered", name)
		}
	}
}
