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

func setupLuckyWheelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:lucky-wheel-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&User{}, &TopUp{}, &SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{},
		&LuckyCampaign{}, &LuckyRuleSet{}, &LuckyCard{}, &LuckyDraw{},
		&LuckyRechargeEvent{}, &LuckyRechargeProgress{}, &LuckyRewardBucket{},
		&LuckyPausePeriod{}, &SubscriptionConsumptionPriority{}, &Option{},
	))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })
	require.NoError(t, EnsureDefaultLuckyCampaign())
	return db
}

func TestLuckyRechargeBonusMigrationChangesSixtyToFortyOnce(t *testing.T) {
	db := setupLuckyWheelTestDB(t)
	var rule LuckyRuleSet
	require.NoError(t, db.First(&rule).Error)
	require.NoError(t, db.Model(&rule).Update("recharge_bonus_usd_micros", int64(60_000_000)).Error)

	require.NoError(t, EnsureLuckyRechargeBonusForty())
	require.NoError(t, db.First(&rule, rule.Id).Error)
	require.EqualValues(t, 40_000_000, rule.RechargeBonusUsdMicros)

	require.NoError(t, db.Model(&rule).Update("recharge_bonus_usd_micros", int64(55_000_000)).Error)
	require.NoError(t, EnsureLuckyRechargeBonusForty())
	require.NoError(t, db.First(&rule, rule.Id).Error)
	require.EqualValues(t, 55_000_000, rule.RechargeBonusUsdMicros, "迁移完成后不得覆盖管理员后续配置")
}

func TestLuckyPrizePoolsTotalExactlyOneMillion(t *testing.T) {
	subscription, recharge := defaultLuckyPools()
	require.NoError(t, validateLuckyPool(subscription, true))
	require.NoError(t, validateLuckyPool(recharge, false))
	require.Equal(t, []LuckyPrizeConfig{
		{LuckyPrizeQuota5, 5_000_000, 360_000},
		{LuckyPrizeQuota10, 10_000_000, 320_000},
		{LuckyPrizeQuota20, 20_000_000, 200_000},
		{LuckyPrizeQuota50, 50_000_000, 30_000},
		{LuckyPrizeQuota100, 100_000_000, 10_000},
		{LuckyPrizeGift5, 5_000_000, 47_000},
		{LuckyPrizeGift10, 10_000_000, 20_000},
		{LuckyPrizeGift20, 20_000_000, 10_000},
		{LuckyPrizeDouble, 0, 250},
		{LuckyPrizeFullReset, 0, 1_500},
		{LuckyPrizeCrazy5H, 0, 1_250},
	}, subscription)
	require.Equal(t, []LuckyPrizeConfig{
		{LuckyPrizeQuota5, 5_000_000, 360_000},
		{LuckyPrizeQuota10, 10_000_000, 320_000},
		{LuckyPrizeQuota20, 20_000_000, 200_000},
		{LuckyPrizeQuota50, 50_000_000, 30_000},
		{LuckyPrizeQuota100, 100_000_000, 10_000},
		{LuckyPrizeGift5, 5_000_000, 48_750},
		{LuckyPrizeGift10, 10_000_000, 20_000},
		{LuckyPrizeGift20, 20_000_000, 10_000},
		{LuckyPrizeCrazy5H, 0, 1_250},
	}, recharge)

	var subWeight, rechargeWeight int64
	for _, prize := range subscription {
		subWeight += prize.Weight
	}
	for _, prize := range recharge {
		rechargeWeight += prize.Weight
		require.NotEqual(t, LuckyPrizeDouble, prize.Code)
		require.NotEqual(t, LuckyPrizeFullReset, prize.Code)
	}
	require.EqualValues(t, LuckyWeightScale, subWeight)
	require.EqualValues(t, LuckyWeightScale, rechargeWeight)
}

func TestLuckyThresholdProgressAndRechargeIdempotency(t *testing.T) {
	db := setupLuckyWheelTestDB(t)
	require.EqualValues(t, 5_000, LuckyThresholdCents(1))
	require.EqualValues(t, 80_000, LuckyThresholdCents(6))
	require.EqualValues(t, 100_000, LuckyThresholdCents(7))
	require.EqualValues(t, 140_000, LuckyThresholdCents(9))

	require.NoError(t, db.Model(&LuckyCampaign{}).Where("code = ?", LuckyCampaignCode).
		Updates(map[string]interface{}{"issuance_paused": false, "draw_paused": false}).Error)
	user := User{Username: "lucky-user", Password: "hashed", AffCode: "lucky1"}
	require.NoError(t, db.Create(&user).Error)
	topUp := TopUp{
		UserId: user.Id, Amount: 0, Money: 450, TradeNo: "lucky-topup-1",
		Status: common.TopUpStatusSuccess, CompleteTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&topUp).Error)
	require.True(t, topUp.LuckyRechargeEligible)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		cards, err := RecordLuckyRechargeTx(tx, &topUp)
		require.Len(t, cards, 4)
		return err
	}))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		cards, err := RecordLuckyRechargeTx(tx, &topUp)
		require.Empty(t, cards)
		return err
	}))

	var progress LuckyRechargeProgress
	require.NoError(t, db.First(&progress, user.Id).Error)
	require.EqualValues(t, 45_000, progress.EligibleCents)
	require.EqualValues(t, 4, progress.HighestAwardedStage)
	require.EqualValues(t, 60_000, progress.NextThresholdCents)

	var count int64
	require.NoError(t, db.Model(&LuckyCard{}).Count(&count).Error)
	require.EqualValues(t, 4, count)

	var issued []LuckyCard
	require.NoError(t, db.Order("id asc").Find(&issued).Error)
	require.NoError(t, db.Model(&issued[0]).Update("status", LuckyCardConsumed).Error)
	draw := LuckyDraw{
		UserId: user.Id, CardId: issued[0].Id, RuleSetId: issued[0].RuleSetId,
		IdempotencyKey: "lucky-reversal-draw", PrizeType: LuckyPrizeGift5,
		Status: "awarded", AwardedAt: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&draw).Error)
	var reversal LuckySourceReversalResult
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var err error
		reversal, err = ReverseLuckyRechargeTx(tx, &topUp, "payment chargeback")
		return err
	}))
	require.True(t, reversal.EventCreated)
	require.EqualValues(t, 3, reversal.RevokedCards)
	require.EqualValues(t, 1, reversal.ReviewCards)
	require.EqualValues(t, 1, reversal.ReviewDraws)

	require.NoError(t, db.First(&progress, user.Id).Error)
	require.Zero(t, progress.EligibleCents)
	require.EqualValues(t, 4, progress.HighestAwardedStage)
	require.EqualValues(t, 60_000, progress.NextThresholdCents)
	require.NoError(t, db.First(&draw, draw.Id).Error)
	require.Equal(t, "review_required", draw.Status)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		repeated, err := ReverseLuckyRechargeTx(tx, &topUp, "duplicate callback")
		require.False(t, repeated.EventCreated)
		require.Zero(t, repeated.RevokedCards)
		return err
	}))
}

func TestRevokeUserAvailableLuckyCardsPreservesAuditHistory(t *testing.T) {
	db := setupLuckyWheelTestDB(t)
	now := common.GetTimestamp()
	cards := []LuckyCard{
		{UserId: 5, PoolType: LuckyPoolRecharge, SourceType: "admin_compensation", SourceRef: "user-5", GrantKey: "user-5:available", Status: LuckyCardAvailable, ExpiresAt: now + 3600},
		{UserId: 5, PoolType: LuckyPoolRecharge, SourceType: "admin_compensation", SourceRef: "user-5", GrantKey: "user-5:consumed", Status: LuckyCardConsumed, ExpiresAt: now + 3600, ConsumedAt: now},
		{UserId: 6, PoolType: LuckyPoolRecharge, SourceType: "admin_compensation", SourceRef: "user-6", GrantKey: "user-6:available", Status: LuckyCardAvailable, ExpiresAt: now + 3600},
	}
	require.NoError(t, db.Create(&cards).Error)
	draw := LuckyDraw{
		UserId: 5, CardId: cards[1].Id, IdempotencyKey: "user-5-draw",
		PrizeType: LuckyPrizeGift5, Status: "awarded", AwardedAt: now,
	}
	require.NoError(t, db.Create(&draw).Error)

	var revoked int64
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var err error
		revoked, err = RevokeUserAvailableLuckyCardsTx(tx, 5, "管理员手动清空")
		return err
	}))
	require.EqualValues(t, 1, revoked)

	var stored []LuckyCard
	require.NoError(t, db.Order("id asc").Find(&stored).Error)
	require.Equal(t, LuckyCardRevoked, stored[0].Status)
	require.NotZero(t, stored[0].RevokedAt)
	require.Equal(t, "管理员手动清空", stored[0].RevokeReason)
	require.Equal(t, LuckyCardConsumed, stored[1].Status)
	require.Equal(t, LuckyCardAvailable, stored[2].Status)

	var drawCount int64
	require.NoError(t, db.Model(&LuckyDraw{}).Where("card_id = ?", cards[1].Id).Count(&drawCount).Error)
	require.EqualValues(t, 1, drawCount)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		repeated, err := RevokeUserAvailableLuckyCardsTx(tx, 5, "重复清空")
		require.Zero(t, repeated)
		return err
	}))
}

func TestListLuckyAdminDrawsIncludesUserPrizeAndCardSource(t *testing.T) {
	db := setupLuckyWheelTestDB(t)
	now := common.GetTimestamp()
	users := []User{
		{Username: "alice-lucky", DisplayName: "Alice", Password: "hashed-password", AffCode: "lucky-admin-a"},
		{Username: "bob-lucky", DisplayName: "Bob", Password: "hashed-password", AffCode: "lucky-admin-b"},
	}
	require.NoError(t, db.Create(&users).Error)
	cards := []LuckyCard{
		{
			UserId: users[0].Id, PoolType: LuckyPoolSubscription,
			SourceType: "subscription_purchase", SourceRef: "order-a",
			SourceOrderId: 101, SourceSubscriptionId: 201,
			SourceCycleKey: "purchase", SourceEffectiveEndTime: now + 86400,
			GrantKey: "admin-history-a", Status: LuckyCardConsumed,
			IssuedAt: now - 300, ExpiresAt: now + 86400, ConsumedAt: now - 100,
		},
		{
			UserId: users[1].Id, PoolType: LuckyPoolRecharge,
			SourceType: "wallet_topup", SourceRef: "topup-b",
			SourceOrderId: 102, GrantKey: "admin-history-b",
			Status: LuckyCardConsumed, IssuedAt: now - 200,
			ExpiresAt: now + 86400, ConsumedAt: now - 50,
		},
	}
	require.NoError(t, db.Create(&cards).Error)
	draws := []LuckyDraw{
		{
			UserId: users[0].Id, CardId: cards[0].Id,
			IdempotencyKey: "admin-history-draw-a", PrizeType: LuckyPrizeQuota10,
			DisplayUsdMicros: 10_000_000, ActualUsdMicros: 10_000_000,
			RewardSubscriptionId: 301, Status: "awarded", AwardedAt: now - 100,
		},
		{
			UserId: users[1].Id, CardId: cards[1].Id,
			IdempotencyKey: "admin-history-draw-b", PrizeType: LuckyPrizeGift5,
			DisplayUsdMicros: 5_000_000, ActualUsdMicros: 5_000_000,
			GiftQuotaAwarded: 5_000_000, Status: "review_required", AwardedAt: now - 50,
		},
	}
	require.NoError(t, db.Create(&draws).Error)

	records, total, err := ListLuckyAdminDraws(LuckyAdminDrawFilter{
		Keyword: "alice", PrizeType: LuckyPrizeQuota10,
		StartTime: now - 150, EndTime: now - 75, Page: 1, PageSize: 10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, records, 1)
	require.Equal(t, users[0].Id, records[0].UserId)
	require.Equal(t, "alice-lucky", records[0].Username)
	require.Equal(t, "Alice", records[0].DisplayName)
	require.Equal(t, LuckyPoolSubscription, records[0].PoolType)
	require.Equal(t, "subscription_purchase", records[0].SourceType)
	require.Equal(t, "order-a", records[0].SourceRef)
	require.Equal(t, 201, records[0].SourceSubscriptionId)
	require.Equal(t, 301, records[0].RewardSubscriptionId)

	records, total, err = ListLuckyAdminDraws(LuckyAdminDrawFilter{
		Keyword: fmt.Sprintf("%d", users[1].Id), Status: "review_required",
		Page: 1, PageSize: 10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, records, 1)
	require.Equal(t, LuckyPrizeGift5, records[0].PrizeType)
	require.Equal(t, "topup-b", records[0].SourceRef)
}

func TestUnboundSubscriptionOrderUsesSavedPriority(t *testing.T) {
	db := setupLuckyWheelTestDB(t)
	now := common.GetTimestamp()
	first := UserSubscription{UserId: 9, Status: "active", StartTime: now - 10, EndTime: now + 100, AmountTotal: 100}
	second := UserSubscription{UserId: 9, Status: "active", StartTime: now - 10, EndTime: now + 200, AmountTotal: 100}
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)
	require.NoError(t, db.Create(&SubscriptionConsumptionPriority{
		UserId: 9, GroupName: "default", SubscriptionId: second.Id, Priority: 1, Revision: 1,
	}).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		subs, err := subscriptionCandidatesTx(tx, 9, "default", now, nil)
		require.NoError(t, err)
		require.Equal(t, []int{second.Id, first.Id}, []int{subs[0].Id, subs[1].Id})
		return nil
	}))
}

func TestLuckySubscriptionProgressUsesEarliestEligibleRealInstance(t *testing.T) {
	db := setupLuckyWheelTestDB(t)
	now := time.Now().Unix()
	eligiblePlan := SubscriptionPlan{
		Title: "周期赠卡月卡", PriceAmount: 9.9, DurationUnit: SubscriptionDurationMonth,
		DurationValue: 1, QuotaResetPeriod: SubscriptionResetWeekly,
		LuckyCardOnReset: true,
	}
	plainPlan := SubscriptionPlan{
		Title: "无重置周卡", PriceAmount: 4.9, DurationUnit: SubscriptionDurationWeek,
		DurationValue: 1, QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, db.Create(&eligiblePlan).Error)
	require.NoError(t, db.Create(&plainPlan).Error)
	eligibleSnapshot, err := BuildSubscriptionPlanSnapshot(&eligiblePlan)
	require.NoError(t, err)
	plainSnapshot, err := BuildSubscriptionPlanSnapshot(&plainPlan)
	require.NoError(t, err)

	require.NoError(t, db.Create(&UserSubscription{
		UserId: 41, PlanId: plainPlan.Id, PlanSnapshot: plainSnapshot,
		Status: "active", StartTime: now - 60, EndTime: now + 8*86400,
	}).Error)
	require.NoError(t, db.Create(&UserSubscription{
		UserId: 41, PlanId: eligiblePlan.Id, PlanSnapshot: eligibleSnapshot,
		Status: "active", StartTime: now - 60, EndTime: now + 30*86400,
		NextResetTime: now + 6*86400,
	}).Error)
	require.NoError(t, db.Create(&UserSubscription{
		UserId: 41, PlanId: eligiblePlan.Id, PlanSnapshot: eligibleSnapshot,
		Status: "active", StartTime: now - 60, EndTime: now + 30*86400,
		NextResetTime: now + 2*86400, LuckyCardDisabled: true,
	}).Error)

	progress, err := GetLuckySubscriptionProgress(41, now)
	require.NoError(t, err)
	require.True(t, progress.Subscribed)
	require.True(t, progress.Eligible)
	require.EqualValues(t, now+6*86400, progress.NextCardAt)
}

func TestLuckySubscriptionProgressDistinguishesNoResetAndNoSubscription(t *testing.T) {
	db := setupLuckyWheelTestDB(t)
	now := time.Now().Unix()
	plan := SubscriptionPlan{
		Title: "无重置周卡", PriceAmount: 4.9, DurationUnit: SubscriptionDurationWeek,
		DurationValue: 1, QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, db.Create(&plan).Error)
	snapshot, err := BuildSubscriptionPlanSnapshot(&plan)
	require.NoError(t, err)
	require.NoError(t, db.Create(&UserSubscription{
		UserId: 42, PlanId: plan.Id, PlanSnapshot: snapshot,
		Status: "active", StartTime: now - 60, EndTime: now + 7*86400,
	}).Error)

	active, err := GetLuckySubscriptionProgress(42, now)
	require.NoError(t, err)
	require.True(t, active.Subscribed)
	require.False(t, active.Eligible)
	require.Zero(t, active.NextCardAt)

	missing, err := GetLuckySubscriptionProgress(43, now)
	require.NoError(t, err)
	require.False(t, missing.Subscribed)
	require.False(t, missing.Eligible)
	require.Zero(t, missing.NextCardAt)
}

func TestLuckySubscriptionProgressSkipsMalformedLegacySubscription(t *testing.T) {
	db := setupLuckyWheelTestDB(t)
	now := time.Now().Unix()
	require.NoError(t, db.Create(&UserSubscription{
		UserId: 44, PlanId: 0, PlanSnapshot: "",
		Status: "active", StartTime: now - 60, EndTime: now + 7*86400,
	}).Error)

	progress, err := GetLuckySubscriptionProgress(44, now)
	require.NoError(t, err)
	require.True(t, progress.Subscribed)
	require.False(t, progress.Eligible)
	require.Zero(t, progress.NextCardAt)
}
