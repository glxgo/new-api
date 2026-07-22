package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWeightedChoiceIndexUsesExactWeightRanges(t *testing.T) {
	weights := []int{3, 2}
	for roll := 0; roll < 5; roll++ {
		roll := roll
		index := weightedChoiceIndex(len(weights), func(i int) int {
			return weights[i]
		}, func(total int) int {
			require.Equal(t, 5, total)
			return roll
		})
		if roll < 3 {
			require.Equal(t, 0, index)
		} else {
			require.Equal(t, 1, index)
		}
	}
}

func TestWeightedChoiceIndexDistributesUniformlyWhenAllWeightsAreZero(t *testing.T) {
	index := weightedChoiceIndex(3, func(int) int { return 0 }, func(total int) int {
		require.Equal(t, 3, total)
		return 2
	})
	require.Equal(t, 2, index)
}
