package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildSubscriptionSummariesMarksFinalResetCycle(t *testing.T) {
	plan := SubscriptionPlan{
		Id:               19,
		Title:            "sxd周限额月卡",
		PlanVersion:      PlanVersionPro,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		QuotaResetPeriod: SubscriptionResetWeekly,
		QuotaResetAnchor: SubscriptionResetAnchorMidnight,
		TotalAmount:      600_000_000,
	}
	snapshot, err := BuildSubscriptionPlanSnapshot(&plan)
	require.NoError(t, err)

	start := time.Date(2026, 8, 20, 16, 29, 56, 0, subscriptionBusinessLocation)
	sub := UserSubscription{
		Id:            234,
		PlanId:        plan.Id,
		PlanTitle:     plan.Title,
		PlanVersion:   plan.PlanVersion,
		PlanSnapshot:  snapshot,
		Status:        "active",
		StartTime:     start.Unix(),
		EndTime:       start.AddDate(0, 1, 0).Unix(),
		LastResetTime: time.Date(2026, 9, 10, 0, 0, 0, 0, subscriptionBusinessLocation).Unix(),
		NextResetTime: 0,
	}

	summaries := buildSubscriptionSummaries([]UserSubscription{sub})
	require.Len(t, summaries, 1)
	require.NotNil(t, summaries[0].Subscription)
	require.Equal(t, SubscriptionResetWeekly, summaries[0].Subscription.QuotaResetPeriod)
	require.True(t, summaries[0].Subscription.IsFinalResetCycle)
}

func TestBuildSubscriptionSummariesDoesNotMarkNeverOrScheduledReset(t *testing.T) {
	start := time.Date(2026, 8, 20, 12, 0, 0, 0, subscriptionBusinessLocation)
	end := start.AddDate(0, 1, 0)

	neverPlan := SubscriptionPlan{
		Id:               20,
		Title:            "no reset",
		PlanVersion:      PlanVersionPro,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	neverSnapshot, err := BuildSubscriptionPlanSnapshot(&neverPlan)
	require.NoError(t, err)

	weeklyPlan := SubscriptionPlan{
		Id:               21,
		Title:            "weekly",
		PlanVersion:      PlanVersionPro,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		QuotaResetPeriod: SubscriptionResetWeekly,
	}
	weeklySnapshot, err := BuildSubscriptionPlanSnapshot(&weeklyPlan)
	require.NoError(t, err)

	summaries := buildSubscriptionSummaries([]UserSubscription{
		{
			Id: 1, PlanId: neverPlan.Id, PlanTitle: neverPlan.Title, PlanVersion: neverPlan.PlanVersion,
			PlanSnapshot: neverSnapshot, StartTime: start.Unix(), EndTime: end.Unix(), NextResetTime: 0,
		},
		{
			Id: 2, PlanId: weeklyPlan.Id, PlanTitle: weeklyPlan.Title, PlanVersion: weeklyPlan.PlanVersion,
			PlanSnapshot: weeklySnapshot, StartTime: start.Unix(), EndTime: end.Unix(),
			LastResetTime: start.Unix(), NextResetTime: start.AddDate(0, 0, 7).Unix(),
		},
	})

	require.Len(t, summaries, 2)
	require.False(t, summaries[0].Subscription.IsFinalResetCycle)
	require.False(t, summaries[1].Subscription.IsFinalResetCycle)
}
