package controller

import (
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/kiro"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func withKiroCreditPricing(pricing []model.Pricing, usableGroups map[string]string, ratios map[string]float64, userGroup string) []model.Pricing {
	// Match the effective ratio currently frozen by ModelPriceHelper, including
	// existing user-group overrides, without changing any billing configuration.
	effectiveRatios := make(map[string]float64, len(usableGroups))
	for group := range usableGroups {
		ratio, ok := ratios[group]
		if !ok {
			ratio = 1
		}
		if userGroup != "" {
			if special, exists := ratio_setting.GetGroupGroupRatio(userGroup, group); exists {
				ratio = special
			}
		}
		effectiveRatios[group] = ratio
	}
	quotes := kiro.PublicGroupPrices(effectiveRatios)
	if len(quotes) == 0 {
		return pricing
	}
	// GetPricing is shared and cached. Attach fresh quotes to copies so option
	// updates and per-user group access cannot poison another user's cache.
	result := make([]model.Pricing, len(pricing))
	copy(result, pricing)
	for index, item := range result {
		supported := false
		for _, endpoint := range item.SupportedEndpointTypes {
			if endpoint == constant.EndpointTypeOpenAI || endpoint == constant.EndpointTypeOpenAIResponse || endpoint == constant.EndpointTypeAnthropic {
				supported = true
				break
			}
		}
		if !supported {
			continue
		}
		for _, group := range item.EnableGroup {
			if _, accessible := usableGroups[group]; !accessible {
				continue
			}
			if quote, exists := quotes[group]; exists {
				if result[index].KiroCreditPrices == nil {
					result[index].KiroCreditPrices = make(map[string]kiro.PublicCreditPrice)
				}
				result[index].KiroCreditPrices[group] = quote
			}
		}
	}
	return result
}
