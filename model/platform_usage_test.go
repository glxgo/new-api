package model

import "testing"

func TestPlatformQuotaFromHistoricalLog(t *testing.T) {
	row := platformUsageLogRow{
		Quota: 95,
		Other: `{"group_ratio":0.5,"ingress_original_quota":100,"priority_doubled":true}`,
	}
	got, exact := platformQuotaFromLog(row)
	if !exact {
		t.Fatal("expected exact historical calculation")
	}
	if got != 200 {
		t.Fatalf("platformQuotaFromLog() = %d, want 200", got)
	}
}

func TestPlatformQuotaFromExplicitLogValue(t *testing.T) {
	row := platformUsageLogRow{
		Quota: 95,
		Other: `{"platform_base_quota":200,"group_ratio":0.5}`,
	}
	got, exact := platformQuotaFromLog(row)
	if !exact || got != 200 {
		t.Fatalf("platformQuotaFromLog() = (%d, %v), want (200, true)", got, exact)
	}
}
