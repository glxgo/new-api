package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

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

func TestVirtualMembershipVariantSplitsCapacityLimits(t *testing.T) {
	plan := &VirtualMembershipPlan{
		PriceAmount: 12, TwoGroupPrice: 8, WeeklyQuota: 100,
		ConcurrencyLimit: 10, RPMLimit: 10,
	}

	_, _, _, concurrency, rpm, err := VirtualMembershipVariantLimitsForDisplay(plan, 2)
	if err != nil {
		t.Fatalf("variant error: %v", err)
	}
	if concurrency != 5 || rpm != 5 {
		t.Fatalf("capacity = (%d, %d), want (5, 5)", concurrency, rpm)
	}

	plan.ConcurrencyLimit = 1
	plan.RPMLimit = 0
	_, _, _, concurrency, rpm, err = VirtualMembershipVariantLimitsForDisplay(plan, 4)
	if err != nil {
		t.Fatalf("small-limit variant error: %v", err)
	}
	if concurrency != 1 || rpm != 0 {
		t.Fatalf("small capacity = (%d, %d), want (1, 0)", concurrency, rpm)
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

func setupVirtualMembershipTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:virtual-membership-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &VirtualMembershipPlan{}, &UserVirtualMembership{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })
	return db
}

func TestListAdminVirtualMembershipsIncludesUserAndRefreshesDueQuota(t *testing.T) {
	db := setupVirtualMembershipTestDB(t)
	now := common.GetTimestamp()
	user := User{Username: "member-owner", DisplayName: "会员用户", Email: "member@example.com"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	membership := UserVirtualMembership{
		UserId: user.Id, PlanId: 1, PlanTitle: "GPT Plus", PlanCode: "plus",
		GroupSize: 1, WeeklyQuota: 50_000_000, WeeklyUsed: 10_000_000,
		WeeklyResetAt: now - 1, StartTime: now - 86400, EndTime: now + 86400,
		Status: VirtualMembershipStatusActive,
	}
	if err := db.Create(&membership).Error; err != nil {
		t.Fatalf("create membership: %v", err)
	}

	records, err := ListAdminVirtualMemberships()
	if err != nil {
		t.Fatalf("list admin memberships: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	record := records[0]
	if record.Username != user.Username || record.DisplayName != user.DisplayName || record.Email != user.Email {
		t.Fatalf("user identity = %+v", record)
	}
	if record.Membership.WeeklyUsed != 0 || record.Membership.WeeklyResetAt <= now {
		t.Fatalf("membership quota was not refreshed: %+v", record.Membership)
	}
}

func TestAdminGrantVirtualMembershipCreatesZeroMoneyAuditOrder(t *testing.T) {
	db := setupVirtualMembershipTestDB(t)
	if err := db.AutoMigrate(&VirtualMembershipOrder{}); err != nil {
		t.Fatalf("migrate order: %v", err)
	}
	user := User{Username: "grant-owner", Status: common.UserStatusEnabled}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	plan := VirtualMembershipPlan{
		Code: "pro5x", Title: "GPT Pro 5x", PriceAmount: 199,
		TwoGroupPrice: 120, DurationDays: 30, WeeklyQuota: 100_000_000,
		FiveHourEnabled: true, FiveHourQuota: 20_000_000,
		ConcurrencyLimit: 10, RPMLimit: 60, Enabled: true,
	}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}

	order, membership, err := AdminGrantVirtualMembership(user.Id, plan.Id, 2)
	if err != nil {
		t.Fatalf("grant membership: %v", err)
	}
	if order.Money != 0 || order.PaymentProvider != VirtualMembershipAdminGrant || order.Status != VirtualMembershipOrderSuccess {
		t.Fatalf("audit order = %+v", order)
	}
	if membership.UserId != user.Id || membership.GroupSize != 2 || membership.WeeklyQuota != plan.WeeklyQuota/2 {
		t.Fatalf("membership = %+v", membership)
	}
	if membership.ConcurrencyLimit != 5 || membership.RPMLimit != 30 {
		t.Fatalf("membership limits = %d/%d", membership.ConcurrencyLimit, membership.RPMLimit)
	}
}

func TestSaveVirtualMembershipPlanSyncsFiveHourPolicyToActiveMemberships(t *testing.T) {
	db := setupVirtualMembershipTestDB(t)
	now := time.Now().Unix()
	plan := &VirtualMembershipPlan{
		Code: "plus", Title: "GPT Plus", PriceAmount: 10, WeeklyQuota: 50_000_000,
		DurationDays: 30, Enabled: true,
	}
	if err := SaveVirtualMembershipPlan(plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	memberships := []UserVirtualMembership{
		{
			PlanId: plan.Id, UserId: 1, GroupSize: 1, Status: VirtualMembershipStatusActive,
			StartTime: now - 60, EndTime: now + 86400, FiveHourUsed: 9,
		},
		{
			PlanId: plan.Id, UserId: 2, GroupSize: 2, Status: VirtualMembershipStatusActive,
			StartTime: now - 60, EndTime: now + 86400,
		},
		{
			PlanId: plan.Id, UserId: 3, GroupSize: 1, Status: VirtualMembershipStatusExpired,
			StartTime: now - 86400, EndTime: now - 60,
		},
	}
	if err := db.Create(&memberships).Error; err != nil {
		t.Fatalf("create memberships: %v", err)
	}

	plan.FiveHourEnabled = true
	plan.FiveHourQuota = 10_000_000
	if err := SaveVirtualMembershipPlan(plan); err != nil {
		t.Fatalf("enable 5-hour policy: %v", err)
	}
	var active []UserVirtualMembership
	if err := db.Where("status = ?", VirtualMembershipStatusActive).Order("id").Find(&active).Error; err != nil {
		t.Fatalf("load active memberships: %v", err)
	}
	if !active[0].FiveHourActive || active[0].FiveHourQuota != 10_000_000 || active[0].FiveHourUsed != 0 {
		t.Fatalf("single membership after enable = %+v", active[0])
	}
	if !active[1].FiveHourActive || active[1].FiveHourQuota != 5_000_000 || active[1].FiveHourUsed != 0 {
		t.Fatalf("two-person membership after enable = %+v", active[1])
	}
	firstResetAt := active[0].FiveHourResetAt
	if firstResetAt <= now {
		t.Fatalf("reset time = %d, want after %d", firstResetAt, now)
	}

	if err := db.Model(&UserVirtualMembership{}).Where("id = ?", active[0].Id).
		Update("five_hour_used", 2_000_000).Error; err != nil {
		t.Fatalf("seed used quota: %v", err)
	}
	plan.FiveHourQuota = 20_000_000
	if err := SaveVirtualMembershipPlan(plan); err != nil {
		t.Fatalf("change 5-hour quota: %v", err)
	}
	var updated UserVirtualMembership
	if err := db.First(&updated, active[0].Id).Error; err != nil {
		t.Fatalf("reload membership: %v", err)
	}
	if updated.FiveHourQuota != 20_000_000 || updated.FiveHourUsed != 2_000_000 || updated.FiveHourResetAt != firstResetAt {
		t.Fatalf("membership after quota change = %+v", updated)
	}

	plan.FiveHourEnabled = false
	if err := SaveVirtualMembershipPlan(plan); err != nil {
		t.Fatalf("disable 5-hour policy: %v", err)
	}
	if err := db.First(&updated, active[0].Id).Error; err != nil {
		t.Fatalf("reload disabled membership: %v", err)
	}
	if updated.FiveHourActive || updated.FiveHourQuota != 0 || updated.FiveHourUsed != 0 || updated.FiveHourResetAt != 0 {
		t.Fatalf("membership after disable = %+v", updated)
	}

	var expired UserVirtualMembership
	if err := db.Where("status = ?", VirtualMembershipStatusExpired).First(&expired).Error; err != nil {
		t.Fatalf("load expired membership: %v", err)
	}
	if expired.FiveHourActive || expired.FiveHourQuota != 0 {
		t.Fatalf("expired membership was changed = %+v", expired)
	}
}
