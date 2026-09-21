package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionSpendLimitWindowUsesBeijingClock(t *testing.T) {
	now := time.Date(2026, 9, 15, 14, 37, 42, 0, subscriptionBusinessLocation)

	hourStart, hourEnd, ok := subscriptionSpendLimitWindow(SubscriptionSpendLimitHour, now.Unix())
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 9, 15, 14, 0, 0, 0, subscriptionBusinessLocation).Unix(), hourStart)
	require.Equal(t, time.Date(2026, 9, 15, 15, 0, 0, 0, subscriptionBusinessLocation).Unix(), hourEnd)

	dayStart, dayEnd, ok := subscriptionSpendLimitWindow(SubscriptionSpendLimitDay, now.Unix())
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 9, 15, 0, 0, 0, 0, subscriptionBusinessLocation).Unix(), dayStart)
	require.Equal(t, time.Date(2026, 9, 16, 0, 0, 0, 0, subscriptionBusinessLocation).Unix(), dayEnd)
}

func TestPreConsumeUserSubscriptionEnforcesDailySpendLimit(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := common.GetTimestamp()
	plan := testSubscriptionPlan("weekly monthly card", "")
	plan.Id = 900001
	plan.QuotaResetPeriod = SubscriptionResetWeekly
	snapshot, err := BuildSubscriptionPlanSnapshot(&plan)
	require.NoError(t, err)

	sub := UserSubscription{
		UserId:           81,
		PlanId:           plan.Id,
		PlanSnapshot:     snapshot,
		Status:           "active",
		StartTime:        now - 60,
		EndTime:          now + 30*24*60*60,
		AmountTotal:      10_000,
		SpendLimitPeriod: SubscriptionSpendLimitDay,
		SpendLimitQuota:  1_000,
	}
	require.NoError(t, db.Create(&sub).Error)
	require.NoError(t, db.Create(&SubscriptionPreConsumeRecord{
		RequestId:          "already-used",
		UserId:             sub.UserId,
		UserSubscriptionId: sub.Id,
		PreConsumed:        600,
		FinalSaleQuota:     600,
		Status:             SubscriptionCostStatusFinal,
	}).Error)

	_, err = PreConsumeUserSubscription("limit-blocked", sub.UserId, "gpt-test", 0, 401, "")
	require.ErrorContains(t, err, "subscription spend limit reached")

	result, err := PreConsumeUserSubscription("limit-allowed", sub.UserId, "gpt-test", 0, 400, "")
	require.NoError(t, err)
	require.Equal(t, sub.Id, result.UserSubscriptionId)
	require.Equal(t, int64(400), result.PreConsumed)
}

func TestUpdateUserSubscriptionSpendLimitCanBeCleared(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := common.GetTimestamp()
	plan := testSubscriptionPlan("weekly monthly card", "")
	plan.Id = 900002
	snapshot, err := BuildSubscriptionPlanSnapshot(&plan)
	require.NoError(t, err)
	sub := UserSubscription{
		UserId:       82,
		PlanId:       plan.Id,
		PlanSnapshot: snapshot,
		Status:       "active",
		StartTime:    now - 60,
		EndTime:      now + 30*24*60*60,
		AmountTotal:  10_000,
	}
	require.NoError(t, db.Create(&sub).Error)

	updated, err := UpdateUserSubscriptionSpendLimit(sub.UserId, sub.Id, SubscriptionSpendLimitHour, 250)
	require.NoError(t, err)
	require.Equal(t, SubscriptionSpendLimitHour, updated.SpendLimitPeriod)
	require.Equal(t, int64(250), updated.SpendLimitQuota)

	cleared, err := UpdateUserSubscriptionSpendLimit(sub.UserId, sub.Id, "", 0)
	require.NoError(t, err)
	require.Empty(t, cleared.SpendLimitPeriod)
	require.Zero(t, cleared.SpendLimitQuota)
}

func TestBuildSubscriptionSummariesPopulatesSpendLimitUsage(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := common.GetTimestamp()
	plan := testSubscriptionPlan("weekly monthly card", "")
	plan.Id = 900003
	snapshot, err := BuildSubscriptionPlanSnapshot(&plan)
	require.NoError(t, err)
	sub := UserSubscription{
		UserId:           83,
		PlanId:           plan.Id,
		PlanSnapshot:     snapshot,
		Status:           "active",
		StartTime:        now - 60,
		EndTime:          now + 30*24*60*60,
		AmountTotal:      10_000,
		SpendLimitPeriod: SubscriptionSpendLimitDay,
		SpendLimitQuota:  1_000,
	}
	require.NoError(t, db.Create(&sub).Error)
	require.NoError(t, db.Create(&SubscriptionPreConsumeRecord{
		RequestId:          "summary-used",
		UserId:             sub.UserId,
		UserSubscriptionId: sub.Id,
		PreConsumed:        345,
		Status:             SubscriptionCostStatusReserved,
	}).Error)

	summaries, err := GetVisibleUserSubscriptions(sub.UserId)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	require.Equal(t, int64(345), summaries[0].Subscription.SpendLimitUsed)
	require.NotZero(t, summaries[0].Subscription.SpendLimitWindowStart)
}
