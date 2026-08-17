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

func TestResolvePreDiscountQuotaOnlyRemovesGroupDiscount(t *testing.T) {
	tests := []struct {
		name  string
		quota int
		other map[string]interface{}
		want  int
	}{
		{
			name:  "entrance discount is preserved when group has no discount",
			quota: 31683,
			other: map[string]interface{}{
				"group_ratio":         1.0,
				"ingress_multiplier":  0.97,
				"platform_base_quota": 32663,
			},
			want: 31683,
		},
		{
			name:  "group discount is removed while entrance price is preserved",
			quota: 3392,
			other: map[string]interface{}{
				"group_ratio":        0.12,
				"ingress_multiplier": 1.0,
			},
			want: 28267,
		},
		{
			name:  "missing group ratio falls back to charged quota",
			quota: 800,
			other: map[string]interface{}{
				"platform_base_quota": 1000,
			},
			want: 800,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolvePreDiscountQuota(test.quota, test.other); got != test.want {
				t.Fatalf("resolvePreDiscountQuota() = %d, want %d", got, test.want)
			}
		})
	}
}
