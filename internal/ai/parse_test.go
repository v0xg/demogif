package ai

import "testing"

func TestParseActionsJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLen   int
		wantFirst string // selector of the first action
		wantErr   bool
	}{
		{"plain array", `[{"action":"click","selector":"#a"}]`, 1, "#a", false},
		{"empty array", `[]`, 0, "", false},
		{"markdown fence", "```json\n[{\"action\":\"click\",\"selector\":\"#a\"}]\n```", 1, "#a", false},
		{"brackets in selector", `Here: [{"action":"type","selector":"input[name=\"q\"]","text":"x"}]`, 1, `input[name="q"]`, false},
		{"bracket in preamble", `Steps [1-2] follow: [{"action":"click","selector":"#b"},{"action":"wait","wait":500}]`, 2, "#b", false},
		{"unbalanced bracket in text", `[{"action":"type","selector":"#q","text":"a]b"}]`, 1, "#q", false},
		{"trailing commentary", `[{"action":"click","selector":"#a"}] Let me know if [anything] else.`, 1, "#a", false},
		{"no array", `I can't do that.`, 0, "", true},
		{"malformed", `[{"action": "click",`, 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseActionsJSON(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("got %d actions, want %d: %+v", len(got), tt.wantLen, got)
			}
			if tt.wantLen > 0 && got[0].Selector != tt.wantFirst {
				t.Fatalf("first selector = %q, want %q", got[0].Selector, tt.wantFirst)
			}
		})
	}
}
