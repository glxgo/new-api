package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionPlanUpdateMapPersistsLuckyCardSettings(t *testing.T) {
	plan := model.SubscriptionPlan{
		LuckyCardGrantCount: 15,
		LuckyCardOnReset:    true,
	}

	updateMap := subscriptionPlanUpdateMap(&plan)

	require.EqualValues(t, 15, updateMap["lucky_card_grant_count"])
	require.Equal(t, true, updateMap["lucky_card_on_reset"])
}

func TestSubscriptionPlanUpdateMapCanClearLuckyCardSettings(t *testing.T) {
	plan := model.SubscriptionPlan{
		LuckyCardGrantCount: 0,
		LuckyCardOnReset:    false,
	}

	updateMap := subscriptionPlanUpdateMap(&plan)

	require.EqualValues(t, 0, updateMap["lucky_card_grant_count"])
	require.Equal(t, false, updateMap["lucky_card_on_reset"])
}
