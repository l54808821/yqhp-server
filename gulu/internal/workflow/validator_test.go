package workflow

import (
	"testing"
)

func TestValidateAIStep_ValidToolNames(t *testing.T) {
	step := &Step{
		Config: map[string]interface{}{
			"prompt": "test prompt",
			"tools":  []any{"http_request", "var_read", "var_write", "json_parse"},
		},
	}
	errs := validateAIStep(step, "steps[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid tools, got %v", errs)
	}
}

func TestValidateAIStep_UnknownToolName(t *testing.T) {
	step := &Step{
		Config: map[string]interface{}{
			"prompt": "test prompt",
			"tools":  []any{"http_request", "unknown_tool"},
		},
	}
	errs := validateAIStep(step, "steps[0]")
	found := false
	for _, e := range errs {
		if e.Field == "steps[0].config.tools[1]" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for unknown tool at index 1, got %v", errs)
	}
}

func TestValidateAIStep_ValidMCPServerIDs(t *testing.T) {
	step := &Step{
		Config: map[string]interface{}{
			"prompt":         "test prompt",
			"mcp_server_ids": []any{float64(1), float64(5), float64(100)},
		},
	}
	errs := validateAIStep(step, "steps[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid mcp_server_ids, got %v", errs)
	}
}

func TestValidateAIStep_InvalidMCPServerIDs(t *testing.T) {
	tests := []struct {
		name    string
		ids     []any
		wantErr bool
	}{
		{"zero id", []any{float64(0)}, true},
		{"negative id", []any{float64(-1)}, true},
		{"non-integer float", []any{float64(1.5)}, true},
		{"non-number type", []any{"abc"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &Step{
				Config: map[string]interface{}{
					"prompt":         "test prompt",
					"mcp_server_ids": tt.ids,
				},
			}
			errs := validateAIStep(step, "steps[0]")
			hasIDErr := false
			for _, e := range errs {
				if e.Field == "steps[0].config.mcp_server_ids[0]" {
					hasIDErr = true
					break
				}
			}
			if tt.wantErr && !hasIDErr {
				t.Errorf("expected mcp_server_ids error, got %v", errs)
			}
		})
	}
}

func TestValidateAIStep_ValidMaxToolRounds(t *testing.T) {
	for _, v := range []float64{1, 10, 25, 50} {
		step := &Step{
			Config: map[string]interface{}{
				"prompt":          "test prompt",
				"max_tool_rounds": v,
			},
		}
		errs := validateAIStep(step, "steps[0]")
		if len(errs) != 0 {
			t.Errorf("expected no errors for max_tool_rounds=%v, got %v", v, errs)
		}
	}
}

func TestValidateAIStep_InvalidMaxToolRounds(t *testing.T) {
	for _, v := range []float64{0, -1, 51, 100} {
		step := &Step{
			Config: map[string]interface{}{
				"prompt":          "test prompt",
				"max_tool_rounds": v,
			},
		}
		errs := validateAIStep(step, "steps[0]")
		found := false
		for _, e := range errs {
			if e.Field == "steps[0].config.max_tool_rounds" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected max_tool_rounds error for value %v, got %v", v, errs)
		}
	}
}

func TestValidateAIStep_NoToolFields_NoErrors(t *testing.T) {
	step := &Step{
		Config: map[string]interface{}{
			"prompt": "test prompt",
		},
	}
	errs := validateAIStep(step, "steps[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors when tool fields are absent, got %v", errs)
	}
}
