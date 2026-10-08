package kiro

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestKiroPublicCreditPricing(t *testing.T) {
	t.Cleanup(func() { _ = Configure(DefaultConfig) })
	require.NoError(t, Configure(testConfig))
	quotes := PublicGroupPrices(map[string]float64{"kiro": 0.5})
	require.Equal(t, "0.055", quotes["kiro"].EffectivePrice)
	require.Equal(t, "CNY", quotes["kiro"].Currency)
	payload, err := common.Marshal(quotes)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "cost")
	require.NotContains(t, string(payload), "trusted")
	require.Equal(t, "0.11", PublicGroupPrices(nil)["kiro"].EffectivePrice)
	require.Equal(t, "0", PublicGroupPrices(map[string]float64{"kiro": 0})["kiro"].EffectivePrice)
	require.NoError(t, Configure(strings.Replace(testConfig, "0.11", "0.16", 1)))
	require.Equal(t, "0.16", PublicGroupPrices(nil)["kiro"].EffectivePrice)
	for _, mode := range []string{"shadow", "off"} {
		require.NoError(t, Configure(strings.Replace(testConfig, "charge", mode, 1)))
		require.Empty(t, PublicGroupPrices(nil), "non-charge mode must advertise legacy pricing")
	}
}
