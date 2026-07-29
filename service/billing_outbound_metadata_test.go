package service

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestPreparePriorityBillingForOutboundCapturesReasoningEffort(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "chat completions",
			payload: `{"model":"gpt-5.6","reasoning_effort":"high"}`,
			want:    "high",
		},
		{
			name:    "responses",
			payload: `{"model":"gpt-5.6","reasoning":{"effort":"xhigh"}}`,
			want:    "xhigh",
		},
		{
			name:    "removed by final payload",
			payload: `{"model":"gpt-5.6"}`,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{ReasoningEffort: "stale"}
			if apiErr := PreparePriorityBillingForOutbound(info, []byte(tt.payload)); apiErr != nil {
				t.Fatalf("PreparePriorityBillingForOutbound() error = %v", apiErr)
			}
			if info.ReasoningEffort != tt.want {
				t.Fatalf("ReasoningEffort = %q, want %q", info.ReasoningEffort, tt.want)
			}
		})
	}
}
