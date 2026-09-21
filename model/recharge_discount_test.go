package model

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRechargeDiscountThresholds(t *testing.T) {
	for _, test := range []struct {
		cents int64
		rate  float64
	}{
		{-1, 1}, {0, 1}, {4999, 1}, {5000, .99}, {19999, .99}, {20000, .98},
		{49999, .98}, {50000, .97}, {99999, .97}, {100000, .96}, {299999, .96}, {300000, .95},
	} {
		t.Run(fmt.Sprint(test.cents), func(t *testing.T) {
			progress := BuildRechargeDiscountProgress(test.cents)
			require.Equal(t, test.rate, progress.CurrentTier.Rate)
			require.InDelta(t, 100*test.rate, ApplyRechargeDiscount(100, test.cents), 1e-9)
			require.Len(t, progress.Tiers, 5)
		})
	}
	progress := BuildRechargeDiscountProgress(12500)
	require.EqualValues(t, 7500, progress.RemainingCents)
	require.Equal(t, .5, progress.Progress)
	require.Nil(t, BuildRechargeDiscountProgress(300000).NextTier)
}
