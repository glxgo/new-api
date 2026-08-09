package controller

import (
	"testing"
	"time"
)

func TestChinaDayRange(t *testing.T) {
	now := time.Date(2026, 8, 9, 2, 30, 0, 0, time.UTC)
	start, end := chinaDayRange(now)
	if end-start != int64(10*time.Hour/time.Second+30*time.Minute/time.Second) {
		t.Fatalf("unexpected day range: %d seconds", end-start)
	}
}
