package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
)

func TestApplyPrioritySurcharge(t *testing.T) {
	// tieredExpr == "" means non-tiered billing; a non-empty value simulates a
	// tiered_expr model with that expression string.
	tests := []struct {
		name           string
		channelType    int
		serviceTier    string
		tieredExpr     string
		requestBody    string
		quota          int
		wantQuota      int
		wantSurcharged bool
	}{
		{"openai priority applies 2.5x surcharge", constant.ChannelTypeOpenAI, "priority", "", "", 100, 250, true},
		{"openai priority large quota applies 2.5x surcharge", constant.ChannelTypeOpenAI, "priority", "", "", 12345, 30863, true},
		{"openai default tier unchanged", constant.ChannelTypeOpenAI, "default", "", "", 100, 100, false},
		{"openai empty tier unchanged", constant.ChannelTypeOpenAI, "", "", "", 100, 100, false},
		{"non-openai priority unchanged", constant.ChannelTypeAzure, "priority", "", "", 100, 100, false},
		{"openai priority tiered expr without service tier applies 2.5x", constant.ChannelTypeOpenAI, "priority", `len <= 272000 ? tier("standard", p*2.5+c*15) : tier("long_context", p*5+c*30)`, `{"service_tier":"priority"}`, 100, 250, true},
		{"openai priority tiered expr that handles service tier is not stacked", constant.ChannelTypeOpenAI, "priority", `param("service_tier") == "priority" ? tier("fast", (p*5+c*25)*2) : tier("base", p*5+c*25)`, `{"service_tier":"priority"}`, 100, 100, false},
		{"openai priority tiered expr with overridden service tier applies 2.5x", constant.ChannelTypeOpenAI, "priority", `param("service_tier") == "priority" ? tier("fast", (p*5+c*25)*2) : tier("base", p*5+c*25)`, `{"service_tier":"default"}`, 100, 250, true},
		{"openai priority zero quota unchanged", constant.ChannelTypeOpenAI, "priority", "", "", 0, 0, false},
		{"openai priority negative quota unchanged", constant.ChannelTypeOpenAI, "priority", "", "", -5, -5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &RelayInfo{
				ChannelMeta: &ChannelMeta{ChannelType: tt.channelType},
				ServiceTier: tt.serviceTier,
			}
			if tt.tieredExpr != "" {
				info.TieredBillingSnapshot = &billingexpr.BillingSnapshot{ExprString: tt.tieredExpr}
			}
			if tt.requestBody != "" {
				info.BillingRequestInput = &billingexpr.RequestInput{Body: []byte(tt.requestBody)}
			}
			got := info.ApplyPrioritySurcharge(tt.quota)
			if got != tt.wantQuota {
				t.Errorf("quota = %d, want %d", got, tt.wantQuota)
			}
			if info.PriorityDoubled != tt.wantSurcharged {
				t.Errorf("PriorityDoubled = %v, want %v", info.PriorityDoubled, tt.wantSurcharged)
			}
		})
	}
}

func TestCaptureEffectiveServiceTier(t *testing.T) {
	info := &RelayInfo{}
	info.CaptureEffectiveServiceTier([]byte(`{"service_tier":"priority"}`))
	if info.ServiceTier != "priority" {
		t.Fatalf("ServiceTier = %q, want priority", info.ServiceTier)
	}
	info.CaptureEffectiveServiceTier([]byte(`{"model":"gpt-5.6"}`))
	if info.ServiceTier != "" {
		t.Fatalf("ServiceTier = %q, want empty after final payload removed it", info.ServiceTier)
	}
	info.CaptureEffectiveServiceTier([]byte(`{"service_tier":" PRIORITY "}`))
	if info.ServiceTier != "priority" {
		t.Fatalf("ServiceTier = %q, want normalized priority", info.ServiceTier)
	}
	info.CaptureEffectiveServiceTier(nil)
	if info.ServiceTier != "" {
		t.Fatalf("ServiceTier = %q, want empty for nil payload", info.ServiceTier)
	}
}
