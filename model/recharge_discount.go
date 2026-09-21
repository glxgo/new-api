package model

import "github.com/shopspring/decimal"

// Discounts use completed, qualified CNY recharge, before the current order.
// Keep this policy separate from request capacity and from gift balances.
type RechargeDiscountTier struct {
	MinimumCents int64   `json:"minimum_cents"`
	Rate         float64 `json:"rate"`
}

var rechargeDiscountTiers = []RechargeDiscountTier{
	{0, 1}, {5000, 0.99}, {20000, 0.98}, {50000, 0.97}, {100000, 0.96}, {300000, 0.95},
}

type RechargeDiscountProgress struct {
	TotalCents     int64                  `json:"total_cents"`
	CurrentTier    RechargeDiscountTier   `json:"current_tier"`
	NextTier       *RechargeDiscountTier  `json:"next_tier,omitempty"`
	RemainingCents int64                  `json:"remaining_cents"`
	Progress       float64                `json:"progress"`
	Tiers          []RechargeDiscountTier `json:"tiers"`
}

func BuildRechargeDiscountProgress(totalCents int64) RechargeDiscountProgress {
	totalCents = max(0, totalCents)
	i := len(rechargeDiscountTiers) - 1
	for i > 0 && totalCents < rechargeDiscountTiers[i].MinimumCents {
		i--
	}
	current := rechargeDiscountTiers[i]
	result := RechargeDiscountProgress{TotalCents: totalCents, CurrentTier: current, Progress: 1,
		Tiers: append([]RechargeDiscountTier(nil), rechargeDiscountTiers[1:]...)}
	if i+1 < len(rechargeDiscountTiers) {
		next := rechargeDiscountTiers[i+1]
		result.NextTier = &next
		result.RemainingCents = next.MinimumCents - totalCents
		result.Progress = float64(totalCents-current.MinimumCents) / float64(next.MinimumCents-current.MinimumCents)
	}
	return result
}

func GetUserRechargeDiscount(userID int) (RechargeDiscountProgress, error) {
	var user User
	err := DB.Select("id", "recharge_total_cents").First(&user, userID).Error
	return BuildRechargeDiscountProgress(user.RechargeTotalCents), err
}

// Apply before coupon and fee calculation; only the gateway rounds the final amount.
func ApplyRechargeDiscount(money float64, totalCents int64) float64 {
	rate := BuildRechargeDiscountProgress(totalCents).CurrentTier.Rate
	result, _ := decimal.NewFromFloat(money).Mul(decimal.NewFromFloat(rate)).Float64()
	return result
}
