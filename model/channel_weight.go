package model

import "math/rand"

// weightedChoiceIndex returns an index using request-count weights. Positive
// weights participate proportionally; when every weight is zero, candidates
// are distributed uniformly instead of pinning traffic to the first channel.
func weightedChoiceIndex(length int, weightAt func(int) int, randomIntn func(int) int) int {
	if length <= 1 {
		return 0
	}

	total := 0
	maxInt := int(^uint(0) >> 1)
	weights := make([]int, length)
	for i := 0; i < length; i++ {
		weight := weightAt(i)
		if weight <= 0 {
			continue
		}
		if weight > maxInt-total {
			// Defensive fallback for malformed values that would overflow int.
			return randomIntn(length)
		}
		weights[i] = weight
		total += weight
	}

	if total <= 0 {
		return randomIntn(length)
	}
	roll := randomIntn(total)
	for i, weight := range weights {
		if roll < weight {
			return i
		}
		roll -= weight
	}
	return length - 1
}

func selectWeightedChannel(channels []*Channel) *Channel {
	if len(channels) == 0 {
		return nil
	}
	index := weightedChoiceIndex(len(channels), func(i int) int {
		return channels[i].GetWeight()
	}, rand.Intn)
	return channels[index]
}

func selectWeightedAbility(abilities []Ability) *Ability {
	if len(abilities) == 0 {
		return nil
	}
	index := weightedChoiceIndex(len(abilities), func(i int) int {
		return int(abilities[i].Weight)
	}, rand.Intn)
	return &abilities[index]
}
