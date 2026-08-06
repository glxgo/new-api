package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSubscriptionBindingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:subscription-binding-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&User{},
		&Token{},
		&SubscriptionPlan{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&SubscriptionPreConsumeRecord{},
		&TokenSubscriptionBindingHistory{},
	))
	oldDB := DB
	DB = db
	t.Cleanup(func() {
		DB = oldDB
	})
	return db
}

func testSubscriptionPlan(title, group string) SubscriptionPlan {
	return SubscriptionPlan{
		Title:            title,
		PriceAmount:      20,
		Currency:         "USD",
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		QuotaResetPeriod: SubscriptionResetNever,
		Enabled:          true,
		TotalAmount:      10_000,
		AllowedGroup:     group,
	}
}

func TestHasSubscriptionPlanByGroupReservesPackageGroup(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	plan := testSubscriptionPlan("package group", "package-only")
	require.NoError(t, db.Create(&plan).Error)

	reserved, err := HasSubscriptionPlanByGroup("package-only")
	require.NoError(t, err)
	require.True(t, reserved)

	reserved, err = HasSubscriptionPlanByGroup("wallet-group")
	require.NoError(t, err)
	require.False(t, reserved)
}

func TestSubscriptionOrderSnapshotDoesNotDriftAfterPlanEdit(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	plan := testSubscriptionPlan("old terms", "team-a")
	require.NoError(t, db.Create(&plan).Error)
	order, err := NewSubscriptionOrderFromPlan(1, &plan, "snapshot-order", PaymentMethodBalance, PaymentProviderBalance)
	require.NoError(t, err)

	require.NoError(t, db.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).
		Updates(map[string]any{
			"title":         "new terms",
			"total_amount":  int64(50_000),
			"allowed_group": "team-b",
		}).Error)

	snapshot, err := ParseSubscriptionPlanSnapshot(order.PlanSnapshot)
	require.NoError(t, err)
	require.Equal(t, "old terms", snapshot.Title)
	require.Equal(t, int64(10_000), snapshot.TotalAmount)
	require.Equal(t, "team-a", snapshot.AllowedGroup)
}

func TestSubscriptionSummaryKeepsDisabledPlanPresentation(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := common.GetTimestamp()

	legacyPlan := testSubscriptionPlan("legacy offline plan", "team-a")
	legacyPlan.Enabled = false
	legacyPlan.PlanVersion = PlanVersionAdvanced
	require.NoError(t, db.Create(&legacyPlan).Error)
	legacySubscription := UserSubscription{
		UserId:      31,
		PlanId:      legacyPlan.Id,
		Status:      "active",
		StartTime:   now - 60,
		EndTime:     now + 3600,
		AmountTotal: 1000,
	}
	require.NoError(t, db.Create(&legacySubscription).Error)

	snapshotPlan := testSubscriptionPlan("purchased snapshot name", "team-b")
	snapshotPlan.PlanVersion = PlanVersionPro
	require.NoError(t, db.Create(&snapshotPlan).Error)
	snapshot, err := BuildSubscriptionPlanSnapshot(&snapshotPlan)
	require.NoError(t, err)
	snapshotSubscription := UserSubscription{
		UserId:       31,
		PlanId:       snapshotPlan.Id,
		PlanSnapshot: snapshot,
		Status:       "active",
		StartTime:    now - 60,
		EndTime:      now + 7200,
		AmountTotal:  2000,
	}
	require.NoError(t, db.Create(&snapshotSubscription).Error)
	require.NoError(t, db.Model(&snapshotPlan).Updates(map[string]any{
		"title":        "renamed after purchase",
		"plan_version": PlanVersionEnterprise,
		"enabled":      false,
	}).Error)

	summaries, err := GetAllUserSubscriptions(31)
	require.NoError(t, err)
	require.Len(t, summaries, 2)

	byId := make(map[int]*UserSubscription, len(summaries))
	for _, summary := range summaries {
		byId[summary.Subscription.Id] = summary.Subscription
	}
	require.Equal(t, "legacy offline plan", byId[legacySubscription.Id].PlanTitle)
	require.Equal(t, PlanVersionAdvanced, byId[legacySubscription.Id].PlanVersion)
	require.Equal(t, "purchased snapshot name", byId[snapshotSubscription.Id].PlanTitle)
	require.Equal(t, PlanVersionPro, byId[snapshotSubscription.Id].PlanVersion)
}

func TestAdminBindSubscriptionUsesRequestedStartTime(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	plan := testSubscriptionPlan("scheduled admin plan", "team-a")
	require.NoError(t, db.Create(&plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })

	start := time.Date(2026, time.August, 8, 14, 30, 0, 0, time.Local).Unix()
	_, err := AdminBindSubscriptionAt(77, plan.Id, start, "")
	require.NoError(t, err)

	var subscription UserSubscription
	require.NoError(t, db.Where("user_id = ?", 77).First(&subscription).Error)
	require.Equal(t, start, subscription.StartTime)
	require.Equal(t, time.Unix(start, 0).AddDate(0, 1, 0).Unix(), subscription.EndTime)
	require.Equal(t, "admin", subscription.Source)
}

func TestScheduledAdminSubscriptionDefersLegacyGroupUpgrade(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	user := User{Username: "scheduled-upgrade-user", Group: "default"}
	require.NoError(t, db.Create(&user).Error)
	plan := testSubscriptionPlan("scheduled legacy upgrade", "")
	plan.UpgradeGroup = "vip"
	require.NoError(t, db.Create(&plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })

	start := common.GetTimestamp() + 3600
	_, err := AdminBindSubscriptionAt(user.Id, plan.Id, start, "")
	require.NoError(t, err)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.Equal(t, "default", user.Group)

	require.NoError(t, db.Model(&UserSubscription{}).Where("user_id = ?", user.Id).
		Update("start_time", common.GetTimestamp()-1).Error)
	activated, err := ActivateDueSubscriptionGroups(10)
	require.NoError(t, err)
	require.Equal(t, 1, activated)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.Equal(t, "vip", user.Group)

	var subscription UserSubscription
	require.NoError(t, db.Where("user_id = ?", user.Id).First(&subscription).Error)
	require.Equal(t, "default", subscription.PrevUserGroup)
}

func TestTokenBindingRebindAndUnbindAreAudited(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := common.GetTimestamp()
	sub1 := UserSubscription{UserId: 7, PlanId: 1, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 10_000, AllowedGroup: "team-a"}
	sub2 := UserSubscription{UserId: 7, PlanId: 2, Status: "active", StartTime: now - 60, EndTime: now + 7200, AmountTotal: 10_000, AllowedGroup: "team-a"}
	require.NoError(t, db.Create(&sub1).Error)
	require.NoError(t, db.Create(&sub2).Error)
	token := Token{UserId: 7, Name: "project-key", Key: "binding-test-key", Group: "team-a", Status: common.TokenStatusEnabled}
	require.NoError(t, db.Create(&token).Error)

	updated, err := UpdateTokenSubscriptionBinding(7, token.Id, TokenSubscriptionBindingInput{
		Mode:           TokenSubscriptionModeInstance,
		SubscriptionId: sub1.Id,
		AllowRenewal:   true,
	}, "user")
	require.NoError(t, err)
	require.Equal(t, sub1.Id, updated.SubscriptionId)

	updated, err = UpdateTokenSubscriptionBinding(7, token.Id, TokenSubscriptionBindingInput{
		Mode:           TokenSubscriptionModeInstance,
		SubscriptionId: sub2.Id,
		AllowSameGroup: true,
	}, "user")
	require.NoError(t, err)
	require.Equal(t, sub2.Id, updated.SubscriptionId)

	updated, err = UpdateTokenSubscriptionBinding(7, token.Id, TokenSubscriptionBindingInput{
		Mode:          TokenSubscriptionModeAuto,
		CancelPlanned: true,
	}, "user")
	require.NoError(t, err)
	require.Equal(t, TokenSubscriptionModeAuto, updated.SubscriptionMode)
	require.Zero(t, updated.SubscriptionId)

	var history []TokenSubscriptionBindingHistory
	require.NoError(t, db.Where("token_id = ?", token.Id).Order("id asc").Find(&history).Error)
	require.Len(t, history, 3)
	require.Equal(t, []string{
		TokenSubscriptionActionBind,
		TokenSubscriptionActionRebind,
		TokenSubscriptionActionUnbind,
	}, []string{history[0].Action, history[1].Action, history[2].Action})
}

func TestTokenFieldAndSubscriptionUpdateIsAtomic(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := common.GetTimestamp()
	sub := UserSubscription{UserId: 14, PlanId: 1, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 1000, AllowedGroup: "team-a"}
	require.NoError(t, db.Create(&sub).Error)
	token := Token{UserId: 14, Name: "before", Key: "atomic-update-key", Group: "team-a", Status: common.TokenStatusEnabled}
	require.NoError(t, db.Create(&token).Error)

	desired := token
	desired.Name = "after"
	updated, err := UpdateTokenWithSubscriptionBinding(14, &desired, TokenSubscriptionBindingInput{
		Mode:           TokenSubscriptionModeInstance,
		SubscriptionId: sub.Id,
	}, "user")
	require.NoError(t, err)
	require.Equal(t, "after", updated.Name)
	require.Equal(t, sub.Id, updated.SubscriptionId)

	invalid := *updated
	invalid.Name = "must-not-persist"
	invalid.Group = "team-b"
	_, err = UpdateTokenWithSubscriptionBinding(14, &invalid, TokenSubscriptionBindingInput{
		Mode:           TokenSubscriptionModeInstance,
		SubscriptionId: sub.Id,
	}, "user")
	require.ErrorIs(t, err, ErrTokenSubscriptionGroupMismatch)
	require.NoError(t, db.First(&token, token.Id).Error)
	require.Equal(t, "after", token.Name)
	require.Equal(t, "team-a", token.Group)
	require.Equal(t, sub.Id, token.SubscriptionId)
}

func TestBoundTokenUsesRenewalSuccessorBeforeSameGroup(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := common.GetTimestamp()
	plan := testSubscriptionPlan("plan", "team-a")
	require.NoError(t, db.Create(&plan).Error)
	snapshot, err := BuildSubscriptionPlanSnapshot(&plan)
	require.NoError(t, err)
	current := UserSubscription{UserId: 8, PlanId: plan.Id, Status: "expired", StartTime: now - 7200, EndTime: now - 1, AmountTotal: 100, AmountUsed: 100, AllowedGroup: "team-a", PlanSnapshot: snapshot}
	require.NoError(t, db.Create(&current).Error)
	successor := UserSubscription{UserId: 8, PlanId: plan.Id, Status: "active", StartTime: now - 1, EndTime: now + 3600, AmountTotal: 1000, AllowedGroup: "team-a", PlanSnapshot: snapshot, RenewedFromId: &current.Id}
	require.NoError(t, db.Create(&successor).Error)
	other := UserSubscription{UserId: 8, PlanId: plan.Id, Status: "active", StartTime: now - 1, EndTime: now + 1800, AmountTotal: 1000, AllowedGroup: "team-a", PlanSnapshot: snapshot}
	require.NoError(t, db.Create(&other).Error)
	token := Token{
		UserId:                     8,
		Name:                       "renewal-key",
		Key:                        "renewal-test-key",
		Group:                      "team-a",
		Status:                     common.TokenStatusEnabled,
		SubscriptionMode:           TokenSubscriptionModeInstance,
		SubscriptionId:             current.Id,
		SubscriptionAllowRenewal:   true,
		SubscriptionAllowSameGroup: true,
	}
	require.NoError(t, db.Create(&token).Error)

	result, err := PreConsumeUserSubscriptionForToken("renewal-source-order", 8, "gpt-test", 0, 10, "team-a", &token)
	require.NoError(t, err)
	require.Equal(t, successor.Id, result.UserSubscriptionId)
	require.NoError(t, db.First(&token, token.Id).Error)
	require.Equal(t, successor.Id, token.SubscriptionId)
}

func TestLegacyTokenWithoutBindingKeepsAutomaticAllocation(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := common.GetTimestamp()
	plan := testSubscriptionPlan("legacy plan", "")
	require.NoError(t, db.Create(&plan).Error)
	snapshot, err := BuildSubscriptionPlanSnapshot(&plan)
	require.NoError(t, err)
	later := UserSubscription{UserId: 13, PlanId: plan.Id, Status: "active", StartTime: now - 1, EndTime: now + 7200, AmountTotal: 1000, PlanSnapshot: snapshot}
	earlier := UserSubscription{UserId: 13, PlanId: plan.Id, Status: "active", StartTime: now - 1, EndTime: now + 3600, AmountTotal: 1000, PlanSnapshot: snapshot}
	require.NoError(t, db.Create(&later).Error)
	require.NoError(t, db.Create(&earlier).Error)
	legacy := Token{UserId: 13, Group: "default", SubscriptionMode: ""}

	result, err := PreConsumeUserSubscriptionForToken("legacy-auto-source", 13, "gpt-test", 0, 10, "default", &legacy)
	require.NoError(t, err)
	require.Equal(t, earlier.Id, result.UserSubscriptionId)
	require.Zero(t, legacy.SubscriptionId)
}

func TestWalletFallbackLimitIsAtomicAndRefundable(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	token := Token{
		UserId:                    9,
		Name:                      "wallet-key",
		Key:                       "wallet-fallback-key",
		Group:                     "team-a",
		Status:                    common.TokenStatusEnabled,
		SubscriptionMode:          TokenSubscriptionModeInstance,
		SubscriptionId:            10,
		SubscriptionAllowWallet:   true,
		SubscriptionWalletLimit:   100,
		SubscriptionWalletCycleId: 10,
		SubscriptionWalletUsed:    0,
	}
	require.NoError(t, db.Create(&token).Error)
	_, err := ReserveTokenWalletFallback(token.Id, token.UserId, 60)
	require.NoError(t, err)
	_, err = ReserveTokenWalletFallback(token.Id, token.UserId, 50)
	require.ErrorIs(t, err, ErrTokenWalletFallbackLimitReached)
	used, err := ReleaseTokenWalletFallback(token.Id, token.UserId, 20)
	require.NoError(t, err)
	require.Equal(t, int64(40), used)
}

func TestRenewalCreatesLinearSuccessorAndSchedulesAtomicGroupSwitch(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := common.GetTimestamp()
	plan := testSubscriptionPlan("current renewal terms", "team-b")
	require.NoError(t, db.Create(&plan).Error)
	source := UserSubscription{
		UserId:       11,
		PlanId:       plan.Id,
		PlanTitle:    "old plan",
		Remark:       "project alpha",
		Status:       "active",
		StartTime:    now - 3600,
		EndTime:      now + 1800,
		AmountTotal:  1000,
		AllowedGroup: "team-a",
	}
	require.NoError(t, db.Create(&source).Error)
	token := Token{
		UserId:                     source.UserId,
		Name:                       "renew-bound-key",
		Key:                        "renew-binding-key",
		Group:                      "team-a",
		Status:                     common.TokenStatusEnabled,
		SubscriptionMode:           TokenSubscriptionModeInstance,
		SubscriptionId:             source.Id,
		SubscriptionAllowRenewal:   true,
		SubscriptionAllowSameGroup: true,
	}
	require.NoError(t, db.Create(&token).Error)
	order, err := NewSubscriptionOrderFromPlan(
		source.UserId,
		&plan,
		"renewal-schedule-order",
		PaymentMethodBalance,
		PaymentProviderBalance,
	)
	require.NoError(t, err)
	order.RenewFromSubscriptionId = &source.Id

	var successor *UserSubscription
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var createErr error
		successor, _, createErr = createRenewalSubscriptionFromOrderTx(tx, order, &plan, 200)
		if createErr != nil {
			return createErr
		}
		return tx.Create(order).Error
	}))
	require.NotNil(t, successor)
	require.Equal(t, source.EndTime, successor.StartTime)
	require.Equal(t, "project alpha（续费）", successor.Remark)
	require.NotNil(t, successor.RenewedFromId)
	require.Equal(t, source.Id, *successor.RenewedFromId)

	require.NoError(t, db.First(&token, token.Id).Error)
	require.Equal(t, source.Id, token.SubscriptionId)
	require.Equal(t, successor.Id, token.PlannedSubscriptionId)
	require.Equal(t, "team-b", token.PlannedSubscriptionGroup)
	require.Equal(t, successor.StartTime, token.PlannedSubscriptionEffective)
	require.NotEmpty(t, order.RenewalBindingSnapshot)

	secondOrder, err := NewSubscriptionOrderFromPlan(
		source.UserId,
		&plan,
		"renewal-schedule-order-2",
		PaymentMethodBalance,
		PaymentProviderBalance,
	)
	require.NoError(t, err)
	secondOrder.RenewFromSubscriptionId = &source.Id
	err = db.Transaction(func(tx *gorm.DB) error {
		_, _, createErr := createRenewalSubscriptionFromOrderTx(tx, secondOrder, &plan, 200)
		return createErr
	})
	require.ErrorIs(t, err, ErrSubscriptionRenewalAlreadyExists)
}

func TestAdminRenewalCreatesLinkedSuccessorWithoutChargingUser(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := common.GetTimestamp()
	plan := testSubscriptionPlan("admin renewal terms", "team-b")
	require.NoError(t, db.Create(&plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })
	source := UserSubscription{
		UserId:       28,
		PlanId:       plan.Id,
		Remark:       "managed project",
		Status:       "active",
		StartTime:    now - 3600,
		EndTime:      now + 1800,
		AmountTotal:  1000,
		AllowedGroup: "team-a",
	}
	require.NoError(t, db.Create(&source).Error)
	token := Token{
		UserId:                   source.UserId,
		Name:                     "admin-renew-key",
		Key:                      "admin-renew-key-secret",
		Group:                    "team-a",
		Status:                   common.TokenStatusEnabled,
		SubscriptionMode:         TokenSubscriptionModeInstance,
		SubscriptionId:           source.Id,
		SubscriptionAllowRenewal: true,
	}
	require.NoError(t, db.Create(&token).Error)

	preview, err := AdminRenewUserSubscription(source.Id)
	require.NoError(t, err)
	require.Equal(t, source.EndTime, preview.StartTime)
	require.Equal(t, plan.Id, preview.Plan.Id)
	require.Len(t, preview.BindingChanges, 1)

	var successor UserSubscription
	require.NoError(t, db.Where("renewed_from_id = ?", source.Id).First(&successor).Error)
	require.Equal(t, "admin_renewal", successor.Source)
	require.Equal(t, int64(0), successor.PaidRevenueQuota)
	require.Equal(t, "managed project（续费）", successor.Remark)
	require.Equal(t, source.EndTime, successor.StartTime)

	require.NoError(t, db.First(&token, token.Id).Error)
	require.Equal(t, source.Id, token.SubscriptionId)
	require.Equal(t, successor.Id, token.PlannedSubscriptionId)
	require.Equal(t, "team-b", token.PlannedSubscriptionGroup)
	require.Equal(t, successor.StartTime, token.PlannedSubscriptionEffective)

	_, err = AdminRenewUserSubscription(source.Id)
	require.ErrorIs(t, err, ErrSubscriptionRenewalAlreadyExists)
}

func TestRenewalOrderInsertRejectsSecondPendingOrder(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := common.GetTimestamp()
	plan := testSubscriptionPlan("renewal plan", "team-a")
	require.NoError(t, db.Create(&plan).Error)
	source := UserSubscription{
		UserId:      12,
		PlanId:      plan.Id,
		Status:      "active",
		StartTime:   now - 60,
		EndTime:     now + 3600,
		AmountTotal: 1000,
	}
	require.NoError(t, db.Create(&source).Error)

	first, err := NewSubscriptionOrderFromPlan(12, &plan, "pending-renewal-1", PaymentMethodStripe, PaymentProviderStripe)
	require.NoError(t, err)
	first.RenewFromSubscriptionId = &source.Id
	require.NoError(t, first.Insert())

	second, err := NewSubscriptionOrderFromPlan(12, &plan, "pending-renewal-2", PaymentMethodStripe, PaymentProviderStripe)
	require.NoError(t, err)
	second.RenewFromSubscriptionId = &source.Id
	require.ErrorIs(t, second.Insert(), ErrSubscriptionRenewalOrderPending)
}

func TestHardDeleteBlockedButInvalidateAllowedForBoundInstance(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := common.GetTimestamp()
	sub := UserSubscription{UserId: 10, PlanId: 1, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 1000}
	require.NoError(t, db.Create(&sub).Error)
	token := Token{
		UserId:           10,
		Name:             "bound-key",
		Key:              "delete-guard-key",
		Group:            "default",
		Status:           common.TokenStatusEnabled,
		SubscriptionMode: TokenSubscriptionModeInstance,
		SubscriptionId:   sub.Id,
	}
	require.NoError(t, db.Create(&token).Error)

	_, err := AdminDeleteUserSubscription(sub.Id)
	require.Error(t, err)
	_, err = AdminInvalidateUserSubscription(sub.Id)
	require.NoError(t, err)
	require.NoError(t, db.First(&sub, sub.Id).Error)
	require.Equal(t, "cancelled", sub.Status)
}
