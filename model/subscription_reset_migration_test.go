package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNormalizePurchaseAnchoredSubscriptionResetRepairsLegacyRowIdempotently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SubscriptionPlan{}, &UserSubscription{}))

	now := time.Now().In(subscriptionBusinessLocation).Truncate(time.Second)
	start := now.Add(-72 * time.Hour)
	end := start.Add(7 * 24 * time.Hour)
	plan := SubscriptionPlan{
		Title:            "legacy daily",
		DurationUnit:     SubscriptionDurationWeek,
		DurationValue:    1,
		QuotaResetPeriod: SubscriptionResetDaily,
	}
	require.NoError(t, db.Create(&plan).Error)
	legacyLast := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, subscriptionBusinessLocation).Add(-24 * time.Hour)
	legacyNext := legacyLast.Add(24 * time.Hour)
	sub := UserSubscription{
		PlanId:        plan.Id,
		StartTime:     start.Unix(),
		EndTime:       end.Unix(),
		Status:        "active",
		AmountTotal:   100,
		AmountUsed:    42,
		LastResetTime: legacyLast.Unix(),
		NextResetTime: legacyNext.Unix(),
		PlanSnapshot:  "",
		CreatedAt:     now.Unix(),
		UpdatedAt:     now.Unix(),
	}
	require.NoError(t, db.Create(&sub).Error)

	var locked UserSubscription
	require.NoError(t, db.First(&locked, sub.Id).Error)
	changed, err := normalizePurchaseAnchoredSubscriptionResetTx(db, &locked, &plan, now.Unix())
	require.NoError(t, err)
	require.True(t, changed)
	wantLast, wantNext := purchaseAnchoredResetSchedule(&plan, start.Unix(), end.Unix(), now.Unix())
	require.Equal(t, wantLast, locked.LastResetTime)
	require.Equal(t, wantNext, locked.NextResetTime)
	require.Zero(t, locked.AmountUsed)

	var refreshed UserSubscription
	require.NoError(t, db.First(&refreshed, sub.Id).Error)
	changed, err = normalizePurchaseAnchoredSubscriptionResetTx(db, &refreshed, &plan, now.Unix())
	require.NoError(t, err)
	require.False(t, changed, "running the repair twice must be idempotent")
}
