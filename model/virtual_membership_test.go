package model

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
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

func TestVirtualMembershipOriginalPriceForDisplayUsesTierPrice(t *testing.T) {
	plan := &VirtualMembershipPlan{
		OriginalPriceAmount: 20, TwoGroupOriginalPrice: 18,
		ThreeGroupOriginalPrice: 16, FourGroupOriginalPrice: 14,
	}

	price, err := VirtualMembershipOriginalPriceForDisplay(plan, 3)
	if err != nil {
		t.Fatalf("original price error: %v", err)
	}
	if price != 16 {
		t.Fatalf("original price = %v, want 16", price)
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
	previousRatio := ratio_setting.GroupRatio2JSONString()
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	isolatedOptionMap := make(map[string]string, len(previousOptionMap))
	for key, value := range previousOptionMap {
		isolatedOptionMap[key] = value
	}
	common.OptionMap = isolatedOptionMap
	common.OptionMapRWMutex.Unlock()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:virtual-membership-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &Option{}, &VirtualMembershipPlan{}, &UserVirtualMembership{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
		_ = ratio_setting.UpdateGroupRatioByJSONString(previousRatio)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})
	return db
}

func TestListUserVirtualMembershipsReturnsOnlyCurrentActiveInstances(t *testing.T) {
	db := setupVirtualMembershipTestDB(t)
	now := common.GetTimestamp()
	memberships := []UserVirtualMembership{
		{UserId: 41, PlanId: 1, PlanTitle: "active", Status: VirtualMembershipStatusActive, StartTime: now - 60, EndTime: now + 60},
		{UserId: 41, PlanId: 1, PlanTitle: "elapsed", Status: VirtualMembershipStatusActive, StartTime: now - 120, EndTime: now - 60},
		{UserId: 41, PlanId: 1, PlanTitle: "expired", Status: VirtualMembershipStatusExpired, StartTime: now - 120, EndTime: now - 60},
		{UserId: 41, PlanId: 1, PlanTitle: "future", Status: VirtualMembershipStatusActive, StartTime: now + 60, EndTime: now + 120},
	}
	if err := db.Create(&memberships).Error; err != nil {
		t.Fatalf("create memberships: %v", err)
	}

	got, err := ListUserVirtualMemberships(41)
	if err != nil {
		t.Fatalf("list memberships: %v", err)
	}
	if len(got) != 1 || got[0].PlanTitle != "active" {
		t.Fatalf("memberships = %#v, want only active", got)
	}
	var elapsed UserVirtualMembership
	if err := db.Where("plan_title = ?", "elapsed").First(&elapsed).Error; err != nil {
		t.Fatalf("load elapsed membership: %v", err)
	}
	if elapsed.Status != VirtualMembershipStatusExpired {
		t.Fatalf("elapsed status = %q, want expired", elapsed.Status)
	}
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

func TestEnsureVirtualMembershipGroupRegisteredAddsNonSelectableRoutingGroup(t *testing.T) {
	db := setupVirtualMembershipTestDB(t)
	if err := db.AutoMigrate(&Option{}); err != nil {
		t.Fatalf("migrate option: %v", err)
	}
	if err := db.Create(&Option{Key: "GroupRatio", Value: `{"default":1}`}).Error; err != nil {
		t.Fatalf("create group ratio option: %v", err)
	}
	if err := db.Create(&Option{Key: "UserUsableGroups", Value: `{"default":"默认分组"}`}).Error; err != nil {
		t.Fatalf("create usable groups option: %v", err)
	}

	previousRatio := ratio_setting.GroupRatio2JSONString()
	common.OptionMapRWMutex.RLock()
	previousOptionMap := common.OptionMap
	common.OptionMapRWMutex.RUnlock()
	t.Cleanup(func() {
		_ = ratio_setting.UpdateGroupRatioByJSONString(previousRatio)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})
	common.OptionMapRWMutex.Lock()
	common.OptionMap = nil
	common.OptionMapRWMutex.Unlock()

	if err := EnsureVirtualMembershipGroupRegistered(VirtualMembershipDefaultAllowedGroup); err != nil {
		t.Fatalf("register virtual membership group: %v", err)
	}

	var ratioOption Option
	if err := db.First(&ratioOption, "`key` = ?", "GroupRatio").Error; err != nil {
		t.Fatalf("load group ratio option: %v", err)
	}
	var ratios map[string]float64
	if err := json.Unmarshal([]byte(ratioOption.Value), &ratios); err != nil {
		t.Fatalf("decode group ratios: %v", err)
	}
	if ratios[VirtualMembershipDefaultAllowedGroup] != 1 {
		t.Fatalf("membership group ratio = %v, want 1", ratios[VirtualMembershipDefaultAllowedGroup])
	}
	if !ratio_setting.ContainsGroupRatio(VirtualMembershipDefaultAllowedGroup) {
		t.Fatal("membership group was not refreshed in memory")
	}

	var usableOption Option
	if err := db.First(&usableOption, "`key` = ?", "UserUsableGroups").Error; err != nil {
		t.Fatalf("load usable groups option: %v", err)
	}
	var usable map[string]string
	if err := json.Unmarshal([]byte(usableOption.Value), &usable); err != nil {
		t.Fatalf("decode usable groups: %v", err)
	}
	if _, exists := usable[VirtualMembershipDefaultAllowedGroup]; exists {
		t.Fatal("membership-only group must not become globally selectable")
	}
}

func TestAdminDeleteVirtualMembershipUnbindsTokensAndPreservesAudit(t *testing.T) {
	db := setupVirtualMembershipTestDB(t)
	if err := db.AutoMigrate(&Token{}, &VirtualMembershipPreConsumeRecord{}); err != nil {
		t.Fatalf("migrate delete dependencies: %v", err)
	}
	user := User{Username: "delete-member", Status: common.UserStatusEnabled}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	membership := UserVirtualMembership{
		UserId: user.Id, PlanId: 1, PlanTitle: "GPT Plus", PlanCode: "plus",
		GroupSize: 1, WeeklyQuota: 50_000_000, Status: VirtualMembershipStatusActive,
	}
	if err := db.Create(&membership).Error; err != nil {
		t.Fatalf("create membership: %v", err)
	}
	token := Token{
		UserId: user.Id, Name: "member-key", Key: "delete-member-key",
		Group: VirtualMembershipDefaultAllowedGroup, VirtualMembershipId: membership.Id,
	}
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	audit := VirtualMembershipPreConsumeRecord{
		RequestId: "settled-delete-audit", MembershipId: membership.Id, UserId: user.Id,
		PreConsumed: 100, FinalQuota: 100, Status: VirtualMembershipRecordSettled,
	}
	if err := db.Create(&audit).Error; err != nil {
		t.Fatalf("create settled audit: %v", err)
	}

	unbound, err := AdminDeleteVirtualMembership(membership.Id)
	if err != nil {
		t.Fatalf("delete membership: %v", err)
	}
	if unbound != 1 {
		t.Fatalf("unbound tokens = %d, want 1", unbound)
	}
	var membershipCount int64
	if err := db.Model(&UserVirtualMembership{}).Where("id = ?", membership.Id).Count(&membershipCount).Error; err != nil {
		t.Fatalf("count membership: %v", err)
	}
	if membershipCount != 0 {
		t.Fatalf("membership count = %d, want 0", membershipCount)
	}
	var updatedToken Token
	if err := db.First(&updatedToken, token.Id).Error; err != nil {
		t.Fatalf("load token: %v", err)
	}
	if updatedToken.VirtualMembershipId != 0 || updatedToken.Group != VirtualMembershipDefaultAllowedGroup {
		t.Fatalf("token binding/group = %d/%q", updatedToken.VirtualMembershipId, updatedToken.Group)
	}
	var auditCount int64
	if err := db.Model(&VirtualMembershipPreConsumeRecord{}).Where("id = ?", audit.Id).Count(&auditCount).Error; err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit count = %d, want 1", auditCount)
	}
}

func TestAdminDeleteVirtualMembershipBlocksPendingSettlement(t *testing.T) {
	db := setupVirtualMembershipTestDB(t)
	if err := db.AutoMigrate(&Token{}, &VirtualMembershipPreConsumeRecord{}); err != nil {
		t.Fatalf("migrate delete dependencies: %v", err)
	}
	membership := UserVirtualMembership{
		UserId: 7, PlanId: 1, PlanTitle: "GPT Plus", PlanCode: "plus",
		GroupSize: 1, WeeklyQuota: 50_000_000, Status: VirtualMembershipStatusActive,
	}
	if err := db.Create(&membership).Error; err != nil {
		t.Fatalf("create membership: %v", err)
	}
	pending := VirtualMembershipPreConsumeRecord{
		RequestId: "pending-delete-audit", MembershipId: membership.Id, UserId: membership.UserId,
		PreConsumed: 100, Status: VirtualMembershipRecordPending,
	}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatalf("create pending audit: %v", err)
	}

	if _, err := AdminDeleteVirtualMembership(membership.Id); err == nil {
		t.Fatal("delete should fail while a settlement is pending")
	}
	var membershipCount int64
	if err := db.Model(&UserVirtualMembership{}).Where("id = ?", membership.Id).Count(&membershipCount).Error; err != nil {
		t.Fatalf("count membership: %v", err)
	}
	if membershipCount != 1 {
		t.Fatalf("membership count = %d, want 1", membershipCount)
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
