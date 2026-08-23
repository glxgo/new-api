package model

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// NormalizePurchaseAnchoredSubscriptionResets repairs active legacy daily and
// weekly instances after the reset-anchor rule changed from server midnight to
// purchase time.  It is intentionally idempotent and does not backfill lucky
// cards for historical boundaries (the old scheduler may already have issued
// them); future boundaries continue through the normal reset path.
func NormalizePurchaseAnchoredSubscriptionResets() (int, error) {
	if DB == nil {
		return 0, errors.New("database is not initialized")
	}
	now := GetDBTimestamp()
	var subscriptions []UserSubscription
	if err := DB.Where("status = ? AND start_time > 0 AND end_time > ?", "active", now).
		Order("id asc").Find(&subscriptions).Error; err != nil {
		return 0, err
	}
	changed := 0
	for _, candidate := range subscriptions {
		subscription := candidate
		plan, err := planForUserSubscriptionTx(DB, &subscription)
		if err != nil || plan == nil || !subscriptionUsesPurchaseResetAnchorFor(&subscription, plan) {
			// Deleted/malformed plans are handled by the existing subscription
			// validation path; they must not prevent unrelated rows from starting.
			continue
		}
		// Avoid opening a transaction for rows that already use the purchase
		// phase.  On large installations this keeps startup migration close to
		// a read-only scan while still repairing legacy midnight rows.
		if !purchaseResetPhaseNeedsRepair(&subscription, plan) {
			continue
		}
		err = DB.Transaction(func(tx *gorm.DB) error {
			var locked UserSubscription
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Where("id = ? AND status = ?", subscription.Id, "active").
				First(&locked).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil
				}
				return err
			}
			lockedPlan, err := planForUserSubscriptionTx(tx, &locked)
			if err != nil || lockedPlan == nil || !subscriptionUsesPurchaseResetAnchorFor(&locked, lockedPlan) {
				return nil
			}
			if !purchaseResetPhaseNeedsRepair(&locked, lockedPlan) {
				return nil
			}
			wasChanged, err := normalizePurchaseAnchoredSubscriptionResetTx(tx, &locked, lockedPlan, now)
			if wasChanged {
				changed++
			}
			return err
		})
		if err != nil {
			return changed, err
		}
	}
	if changed > 0 {
		common.SysLog("normalized purchase-anchored subscription reset instances=" + strconv.Itoa(changed))
	}
	return changed, nil
}
