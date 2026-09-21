package controller

import (
	"testing"
	"time"
)

func TestUserQuotaRangeAllowsTheFullCalendarMonthWindow(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).Unix()
	end := time.Date(2026, 7, 31, 23, 59, 59, 0, time.UTC).Unix()

	if !isUserQuotaRangeWithinLimit(start, end) {
		t.Fatalf("expected a 31-day calendar month range to be accepted")
	}
}

func TestUserQuotaRangeRejectsMoreThan31Days(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).Unix()
	end := start + int64(31*24*time.Hour/time.Second) + 1

	if isUserQuotaRangeWithinLimit(start, end) {
		t.Fatalf("expected a range longer than 31 days to be rejected")
	}
}
