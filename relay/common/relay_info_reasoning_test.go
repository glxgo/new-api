package common

import "testing"

func TestCaptureEffectiveReasoningEffort(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "chat completions top-level field",
			payload: `{"model":"gpt-5.6","reasoning_effort":"high"}`,
			want:    "high",
		},
		{
			name:    "responses nested field",
			payload: `{"model":"gpt-5.6","reasoning":{"effort":"xhigh"}}`,
			want:    "xhigh",
		},
		{
			name:    "normalizes whitespace and case",
			payload: `{"model":"gpt-5.6","reasoning_effort":" MEDIUM "}`,
			want:    "medium",
		},
		{
			name:    "chat field takes precedence when both are present",
			payload: `{"model":"gpt-5.6","reasoning_effort":"low","reasoning":{"effort":"high"}}`,
			want:    "low",
		},
		{
			name:    "falls back to model suffix",
			payload: `{"model":"gpt-5.6-minimal"}`,
			want:    "minimal",
		},
		{
			name:    "missing effort clears stale value",
			payload: `{"model":"gpt-5.6"}`,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &RelayInfo{ReasoningEffort: "stale"}
			info.CaptureEffectiveReasoningEffort([]byte(tt.payload))
			if info.ReasoningEffort != tt.want {
				t.Fatalf("ReasoningEffort = %q, want %q", info.ReasoningEffort, tt.want)
			}
		})
	}
}

func TestCaptureEffectiveReasoningEffortNilSafe(t *testing.T) {
	var info *RelayInfo
	info.CaptureEffectiveReasoningEffort([]byte(`{"reasoning_effort":"high"}`))

	present := &RelayInfo{ReasoningEffort: "stale"}
	present.CaptureEffectiveReasoningEffort(nil)
	if present.ReasoningEffort != "" {
		t.Fatalf("ReasoningEffort = %q, want empty for nil payload", present.ReasoningEffort)
	}
}

func TestCaptureEffectiveReasoningEffortClaude(t *testing.T) {
	for _, tc := range []struct{ name, payload, want string }{
		{"adaptive high", `{"model":"claude-opus-4-6","thinking":{"type":"adaptive"},"output_config":{"effort":"high"}}`, "high"},
		{"max", `{"output_config":{"effort":"max"}}`, "max"},
		{"normalize", `{"output_config":{"effort":" LOW "}}`, "low"},
		{"explicit overrides model suffix", `{"model":"claude-opus-4-6-high","output_config":{"effort":"medium"}}`, "medium"},
		{"budget is not a named effort", `{"model":"claude-sonnet-4-5","thinking":{"type":"enabled","budget_tokens":8192}}`, ""},
		{"no effort does not invent default", `{"model":"claude-opus-4-6","thinking":{"type":"adaptive"}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &RelayInfo{ReasoningEffort: "stale"}
			info.CaptureEffectiveReasoningEffort([]byte(tc.payload))
			if info.ReasoningEffort != tc.want {
				t.Fatalf("got %q, want %q", info.ReasoningEffort, tc.want)
			}
		})
	}
}
