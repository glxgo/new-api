package common

import "testing"

func TestPlatformBaseQuotaKeepsPriorityAndRemovesCustomerMultipliers(t *testing.T) {
	// Base 100, priority 2x, group 0.5x, ingress 0.95x => billed 95.
	got, exact := PlatformBaseQuota(95, 0.5, 100)
	if !exact {
		t.Fatal("expected exact calculation")
	}
	if got != 200 {
		t.Fatalf("PlatformBaseQuota() = %d, want 200", got)
	}
}

func TestPlatformBaseQuotaFallsBackWhenGroupRatioIsInvalid(t *testing.T) {
	got, exact := PlatformBaseQuota(95, 0, 100)
	if exact {
		t.Fatal("expected inexact fallback")
	}
	if got != 100 {
		t.Fatalf("PlatformBaseQuota() = %d, want 100", got)
	}
}
