package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/kiro"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestKiroPublicPricingGroupsAndPrivacy(t *testing.T) {
	const config = `{"mode":"charge","groups":["kiro","private"],"price_per_credit":"0.16","cost_per_credit":"0.10","quota_per_currency":500000,"reserve_credits":"10","trusted_channels":[104]}`
	require.NoError(t, kiro.Configure(config))
	t.Cleanup(func() { _ = kiro.Configure(kiro.DefaultConfig) })
	pricing := []model.Pricing{
		{ModelName: "opus", ModelRatio: 2, CompletionRatio: 5, EnableGroup: []string{"kiro", "private", "normal"}, SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeAnthropic}},
		{ModelName: "image", EnableGroup: []string{"kiro"}, SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeImageGeneration}},
	}
	quotes := withKiroCreditPricing(pricing, map[string]string{"kiro": "", "normal": ""}, map[string]float64{"kiro": 1, "normal": 0.5}, "")
	require.Len(t, quotes[0].KiroCreditPrices, 1)
	require.Equal(t, "0.16", quotes[0].KiroCreditPrices["kiro"].EffectivePrice)
	require.Empty(t, quotes[1].KiroCreditPrices, "unsupported image endpoints must not advertise credits")
	require.Empty(t, pricing[0].KiroCreditPrices, "shared pricing cache must not be mutated")
	require.Equal(t, 2.0, quotes[0].ModelRatio, "normal-group prices remain intact")
	payload, err := common.Marshal(quotes)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "cost_per_credit")
	require.NotContains(t, string(payload), "trusted_channels")
	other := withKiroCreditPricing(pricing, map[string]string{"normal": ""}, nil, "")
	require.Empty(t, other[0].KiroCreditPrices, "inaccessible credit groups must not leak a quote")

	old := ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() { _ = ratio_setting.UpdateGroupGroupRatioByJSONString(old) })
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"fixture-user":{"kiro":0.5}}`))
	personal := withKiroCreditPricing(pricing, map[string]string{"kiro": ""}, map[string]float64{"kiro": 1}, "fixture-user")
	require.Equal(t, "0.08", personal[0].KiroCreditPrices["kiro"].EffectivePrice, "quote must match the effective billing snapshot")
}
