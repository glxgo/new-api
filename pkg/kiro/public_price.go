package kiro

import (
	"math"

	"github.com/shopspring/decimal"
)

// PublicCreditPrice deliberately excludes procurement costs and channel IDs.
type PublicCreditPrice struct {
	Unit           string `json:"unit"`
	Currency       string `json:"currency"`
	Price          string `json:"price_per_credit"`
	GroupRatio     string `json:"group_ratio"`
	EffectivePrice string `json:"effective_price_per_credit"`
}

// PublicGroupPrices reads one live config snapshot. Shadow still uses legacy
// billing, so only charge mode may replace a public token-based quotation.
func PublicGroupPrices(ratios map[string]float64) map[string]PublicCreditPrice {
	c := current.Load()
	if c == nil || c.Mode != "charge" {
		return nil
	}
	price, err := Decimal(c.Price)
	if err != nil {
		return nil
	}
	quotes := make(map[string]PublicCreditPrice, len(c.Groups))
	for _, group := range c.Groups {
		ratio, exists := ratios[group]
		if !exists {
			ratio = 1
		}
		if ratio < 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
			continue
		}
		groupRatio := decimal.NewFromFloat(ratio)
		quotes[group] = PublicCreditPrice{
			Unit: Unit, Currency: "CNY", Price: price.String(),
			GroupRatio: groupRatio.String(), EffectivePrice: price.Mul(groupRatio).String(),
		}
	}
	return quotes
}
