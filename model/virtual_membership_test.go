package model

import "testing"

func TestVirtualMembershipVariantDividesQuotaByGroupSize(t *testing.T) {
	plan := &VirtualMembershipPlan{
		PriceAmount:     12,
		TwoGroupPrice:   8,
		WeeklyQuota:     100,
		FiveHourEnabled: true,
		FiveHourQuota:   20,
	}

	price, weekly, fiveHour, err := VirtualMembershipVariantForDisplay(plan, 2)
	if err != nil {
		t.Fatalf("variant error: %v", err)
	}
	if price != 8 || weekly != 50 || fiveHour != 10 {
		t.Fatalf("variant = (%v, %d, %d), want (8, 50, 10)", price, weekly, fiveHour)
	}
}

func TestVirtualMembershipVariantFallsBackToBasePrice(t *testing.T) {
	plan := &VirtualMembershipPlan{PriceAmount: 12, WeeklyQuota: 100}

	price, weekly, _, err := VirtualMembershipVariantForDisplay(plan, 4)
	if err != nil {
		t.Fatalf("variant error: %v", err)
	}
	if price != 12 || weekly != 25 {
		t.Fatalf("variant = (%v, %d), want (12, 25)", price, weekly)
	}
}

func TestVirtualMembershipQuotaPercent(t *testing.T) {
	if got := VirtualMembershipQuotaPercent(25, 100); got != 25 {
		t.Fatalf("quota percent = %d, want 25", got)
	}
	if got := VirtualMembershipQuotaPercent(100, 0); got != 0 {
		t.Fatalf("zero quota percent = %d, want 0", got)
	}
}

func TestVirtualMembershipPlanDefaultsToDedicatedGroup(t *testing.T) {
	plan := &VirtualMembershipPlan{}
	plan.Normalize()
	if plan.AllowedGroup != VirtualMembershipDefaultAllowedGroup {
		t.Fatalf("allowed group = %q, want %q", plan.AllowedGroup, VirtualMembershipDefaultAllowedGroup)
	}
}
