package model

import (
	"testing"
	"time"
)

func TestCalcNextResetTimeDailyUsesStartWallClockAndSevenWindows(t *testing.T) {
	start := time.Date(2026, 8, 18, 15, 30, 31, 0, subscriptionBusinessLocation)
	plan := &SubscriptionPlan{
		DurationUnit:     SubscriptionDurationWeek,
		DurationValue:    1,
		QuotaResetPeriod: SubscriptionResetDaily,
	}
	end := start.AddDate(0, 0, 7).Unix()
	base := start
	for reset := 1; reset <= 6; reset++ {
		next := calcNextResetTime(base, plan, end, start.Unix())
		want := start.AddDate(0, 0, reset).Unix()
		if next != want {
			t.Fatalf("reset %d = %d (%s), want %d (%s)", reset, next,
				time.Unix(next, 0).In(subscriptionBusinessLocation), want,
				start.AddDate(0, 0, reset))
		}
		base = time.Unix(next, 0).In(subscriptionBusinessLocation)
	}
	if next := calcNextResetTime(base, plan, end, start.Unix()); next != 0 {
		t.Fatalf("seventh boundary unexpectedly opens another window at %s", time.Unix(next, 0).In(subscriptionBusinessLocation))
	}

	// A late-night purchase must not be pulled back to midnight.
	late := time.Date(2026, 8, 18, 23, 59, 0, 0, subscriptionBusinessLocation)
	lateEnd := late.AddDate(0, 0, 7).Unix()
	if next := calcNextResetTime(late, plan, lateEnd, late.Unix()); next != late.AddDate(0, 0, 1).Unix() {
		t.Fatalf("late purchase next reset = %s, want %s",
			time.Unix(next, 0).In(subscriptionBusinessLocation), late.AddDate(0, 0, 1))
	}
}

func TestCalcNextResetTimeWeeklyUsesPurchaseWallClock(t *testing.T) {
	start := time.Date(2026, 8, 18, 15, 30, 31, 0, subscriptionBusinessLocation)
	plan := &SubscriptionPlan{
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		QuotaResetPeriod: SubscriptionResetWeekly,
	}
	end := start.AddDate(0, 1, 0).Unix()
	next := calcNextResetTime(start, plan, end, start.Unix())
	want := start.AddDate(0, 0, 7).Unix()
	if next != want {
		t.Fatalf("weekly reset = %s, want %s", time.Unix(next, 0).In(subscriptionBusinessLocation), start.AddDate(0, 0, 7))
	}
}

func TestProjectSubscriptionNextResetTimeSkipsPastDisplayBoundary(t *testing.T) {
	start := time.Date(2026, 8, 18, 15, 30, 31, 0, subscriptionBusinessLocation)
	plan := &SubscriptionPlan{
		DurationUnit:     SubscriptionDurationWeek,
		DurationValue:    2,
		QuotaResetPeriod: SubscriptionResetDaily,
	}
	item := &AdminSubscriptionSubscriber{
		StartTime:     start.Unix(),
		EndTime:       start.AddDate(0, 0, 14).Unix(),
		LastResetTime: start.Unix(),
		NextResetTime: start.AddDate(0, 0, 1).Unix(),
	}
	now := start.AddDate(0, 0, 3).Add(time.Hour).Unix()
	want := start.AddDate(0, 0, 4).Unix()
	if got := projectSubscriptionNextResetTime(item, plan, now); got != want {
		t.Fatalf("projected next reset = %s, want %s", time.Unix(got, 0).In(subscriptionBusinessLocation), time.Unix(want, 0).In(subscriptionBusinessLocation))
	}
}

func TestPurchaseAnchoredScheduleRepairsLegacyMidnightPhase(t *testing.T) {
	start := time.Date(2026, 8, 18, 15, 30, 31, 0, subscriptionBusinessLocation)
	plan := &SubscriptionPlan{
		DurationUnit:     SubscriptionDurationWeek,
		DurationValue:    1,
		QuotaResetPeriod: SubscriptionResetDaily,
	}
	end := start.AddDate(0, 0, 7).Unix()
	now := time.Date(2026, 8, 24, 16, 0, 0, 0, subscriptionBusinessLocation).Unix()
	last, next := purchaseAnchoredResetSchedule(plan, start.Unix(), end, now)
	wantLast := start.AddDate(0, 0, 6).Unix()
	if last != wantLast || next != 0 {
		t.Fatalf("purchase-aligned schedule = last %s next %s, want last %s and no next",
			time.Unix(last, 0).In(subscriptionBusinessLocation),
			time.Unix(next, 0).In(subscriptionBusinessLocation),
			time.Unix(wantLast, 0).In(subscriptionBusinessLocation))
	}
	legacyLast := time.Date(2026, 8, 24, 0, 0, 0, 0, subscriptionBusinessLocation).Unix()
	legacyNext := time.Date(2026, 8, 25, 0, 0, 0, 0, subscriptionBusinessLocation).Unix()
	if isPurchaseResetBoundary(legacyLast, start.Unix(), end, plan) ||
		isPurchaseResetBoundary(legacyNext, start.Unix(), end, plan) {
		t.Fatal("legacy midnight boundaries were incorrectly treated as purchase-aligned")
	}
}

func TestLuckyDoubleKeepsSourceResetPhase(t *testing.T) {
	sourceStart := time.Date(2026, 8, 18, 15, 30, 31, 0, subscriptionBusinessLocation)
	drawTime := sourceStart.Add(36 * time.Hour)
	plan := &SubscriptionPlan{
		DurationUnit:     SubscriptionDurationWeek,
		DurationValue:    1,
		QuotaResetPeriod: SubscriptionResetDaily,
	}
	end := sourceStart.AddDate(0, 0, 7).Unix()
	sourceLast, sourceNext := purchaseAnchoredResetSchedule(plan, sourceStart.Unix(), end, drawTime.Unix())
	luckyDouble := &UserSubscription{
		Source:        "lucky_double",
		StartTime:     drawTime.Unix(),
		EndTime:       end,
		LastResetTime: sourceLast,
		NextResetTime: sourceNext,
	}
	if purchaseResetPhaseNeedsRepair(luckyDouble, plan) {
		t.Fatal("lucky double reward was incorrectly treated as a new purchase phase")
	}
	if subscriptionResetBoundaryDueFor(luckyDouble, plan, drawTime.Unix()) {
		t.Fatal("lucky double reward unexpectedly projected a draw-time reset")
	}
}
