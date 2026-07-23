package service

import (
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

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
