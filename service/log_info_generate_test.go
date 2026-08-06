package service

import (
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestAppendIngressBillingInfoRecordsRouteAndDiscountedPrice(t *testing.T) {
	info := &relaycommon.RelayInfo{
		IngressCode:          "direct",
		IngressDisplayName:   "海外直链 URL",
		IngressMultiplierPPM: 950_000,
		FinalConsumedQuota:   95,
	}
	other := map[string]interface{}{}
	appendIngressBillingInfo(info, other)
	if other["ingress_code"] != "direct" || other["ingress_display_name"] != "海外直链 URL" {
		t.Fatalf("route fields = %#v", other)
	}
	if other["ingress_multiplier"] != 0.95 || other["ingress_billed_quota"] != 95 || other["ingress_original_quota"] != 100 {
		t.Fatalf("billing fields = %#v", other)
	}
}

func TestRelayFirstTokenDurationUsesUpstreamStartTime(t *testing.T) {
	requestStart := time.Unix(1_700_000_000, 0)
	upstreamStart := requestStart.Add(3 * time.Second)
	firstResponse := upstreamStart.Add(1250 * time.Millisecond)

	info := &relaycommon.RelayInfo{
		StartTime:         requestStart,
		UpstreamStartTime: upstreamStart,
		FirstResponseTime: firstResponse,
	}

	if got := relayFirstTokenDurationMs(info); got != float64(1250) {
		t.Fatalf("frt = %v, want 1250ms from upstream request to server receipt", got)
	}
}

func TestRelayFirstTokenDurationFallsBackWhenNoUpstreamStartExists(t *testing.T) {
	requestStart := time.Unix(1_700_000_000, 0)
	info := &relaycommon.RelayInfo{
		StartTime:         requestStart,
		FirstResponseTime: requestStart.Add(900 * time.Millisecond),
	}

	if got := relayFirstTokenDurationMs(info); got != float64(900) {
		t.Fatalf("frt = %v, want 900ms fallback from request start", got)
	}
}
