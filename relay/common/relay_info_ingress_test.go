package common

import "testing"

func TestIngressMultiplierUsesFixedPointRounding(t *testing.T) {
	info := &RelayInfo{IngressMultiplierPPM: 950_000}

	if got := info.ApplyIngressMultiplier(100); got != 95 {
		t.Fatalf("discounted quota = %d, want 95", got)
	}
	if got := info.RestoreIngressMultiplier(95); got != 100 {
		t.Fatalf("restored quota = %d, want 100", got)
	}
}

func TestIngressMultiplierDefaultsToOriginalPrice(t *testing.T) {
	info := &RelayInfo{}

	if got := info.ApplyIngressMultiplier(123); got != 123 {
		t.Fatalf("default multiplier quota = %d, want 123", got)
	}
}
