package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
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

func TestPreparePriorityBillingForOutboundReservesTieredPrioritySurcharge(t *testing.T) {
	funding := &priorityFundingStub{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
		},
		PriceData: types.PriceData{
			QuotaToPreConsume: 100,
		},
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode: "tiered_expr",
			ExprString:  `tier("base", p*5+c*25)`,
		},
		IsPlayground: true,
	}
	info.Billing = &BillingSession{
		relayInfo: info,
		funding:   funding,
		trusted:   true,
	}

	require.Nil(t, PreparePriorityBillingForOutbound(info, []byte(`{"service_tier":"priority"}`)))
	require.True(t, info.PriorityDoubled)
	require.Equal(t, 200, info.Billing.GetPreConsumedQuota())
	require.Equal(t, 200, funding.settled)
}
