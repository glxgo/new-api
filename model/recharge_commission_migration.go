package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	rechargeCommissionMigrationKey      = "RechargeCommissionPolicyV1ReconciliationV2"
	rechargeCommissionLegacyCountKey    = "RechargeCommissionPolicyV1LegacyPendingCount"
	rechargeCommissionLegacyCutoffIDKey = "RechargeCommissionPolicyV1LegacyCutoffLogId"
	rechargeCommissionLogBatchV1        = "recharge_policy_v1"
)

var errHistoricalRechargeOwnerUnavailable = errors.New("historical recharge owner is unavailable")

// MigrateRechargeCommissionPolicyV1 inventories the obsolete consumption-
// profit queue without pretending unresolved rows were settled. Completed
// historical batches and dividend records remain immutable. All new real
// payments settle atomically when their RechargeCredit is inserted.
func MigrateRechargeCommissionPolicyV1() error {
	if !common.IsMasterNode {
		return nil
	}
	var marker Option
	if err := DB.Where(optionKeyWhereClause(), rechargeCommissionMigrationKey).First(&marker).Error; err == nil && marker.Value == "true" {
		return nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	// Do not rewrite every historical consumption log. Large installations can
	// have hundreds of thousands of unresolved rows; a bulk UPDATE would create
	// avoidable locks, undo records and binlog pressure during a blue-green
	// release. Snapshot the count and high-water ID instead. New-policy consume
	// logs are born settled, while any old-version rows written during drain are
	// counted cheaply above the high-water mark.
	var legacySnapshot struct {
		Count int64 `gorm:"column:pending_count"`
		MaxID int64 `gorm:"column:max_id"`
	}
	if LOG_DB != nil {
		if err := LOG_DB.Model(&Log{}).
			Select("COUNT(*) AS pending_count, COALESCE(MAX(id), 0) AS max_id").
			Where("type = ? AND settled = ?", LogTypeConsume, false).
			Scan(&legacySnapshot).Error; err != nil {
			return err
		}
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := reconcileIdentifiablePendingPaymentsTx(tx); err != nil {
			return err
		}
		if err := tx.Model(&AffiliateSettle{}).
			Where("status = ?", AffiliateSettleStatusRunning).
			Update("status", AffiliateSettleStatusFailed).Error; err != nil {
			return err
		}
		options := []Option{
			{Key: rechargeCommissionMigrationKey, Value: "true"},
			{Key: rechargeCommissionLegacyCountKey, Value: strconv.FormatInt(legacySnapshot.Count, 10)},
			{Key: rechargeCommissionLegacyCutoffIDKey, Value: strconv.FormatInt(legacySnapshot.MaxID, 10)},
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"}),
		}).Create(&options).Error
	})
}

// CountPendingLegacyProfitReconciliations returns the immutable cutover count
// plus only late old-version rows written while the previous container drains.
// New recharge-policy logs are marked settled at creation and never enter this
// legacy audit count.
func CountPendingLegacyProfitReconciliations() (int64, error) {
	if DB == nil || LOG_DB == nil {
		return 0, nil
	}
	var options []Option
	if err := DB.Where(commonKeyCol+" IN ?", []string{
		rechargeCommissionLegacyCountKey,
		rechargeCommissionLegacyCutoffIDKey,
	}).Find(&options).Error; err != nil {
		return 0, err
	}
	values := make(map[string]int64, len(options))
	for _, option := range options {
		value, err := strconv.ParseInt(strings.TrimSpace(option.Value), 10, 64)
		if err != nil || value < 0 {
			return 0, fmt.Errorf("invalid recharge commission cutover option %s", option.Key)
		}
		values[option.Key] = value
	}
	base, hasBase := values[rechargeCommissionLegacyCountKey]
	cutoffID, hasCutoff := values[rechargeCommissionLegacyCutoffIDKey]
	if !hasBase || !hasCutoff {
		var count int64
		err := LOG_DB.Model(&Log{}).Where("type = ? AND settled = ?", LogTypeConsume, false).Count(&count).Error
		return count, err
	}
	var late int64
	if err := LOG_DB.Model(&Log{}).
		Where("type = ? AND settled = ? AND id > ?", LogTypeConsume, false, cutoffID).
		Count(&late).Error; err != nil {
		return 0, err
	}
	return base + late, nil
}

// reconcileIdentifiablePendingPaymentsTx promotes only payment events that
// have an unambiguous entitlement and verified amount/currency. Anything else
// remains explicitly marked for manual review.
func reconcileIdentifiablePendingPaymentsTx(tx *gorm.DB) error {
	promote := func(userId int, snapshot PaymentSnapshot, baseQuota int64, sourceType, tradeNo string, completedAt int64) error {
		var activeOwnerCount int64
		if err := tx.Model(&User{}).Where("id = ?", userId).Count(&activeOwnerCount).Error; err != nil {
			return err
		}
		if activeOwnerCount != 1 {
			return errHistoricalRechargeOwnerUnavailable
		}
		amountCents, err := RechargeCentsForPayment(snapshot)
		if err != nil {
			return err
		}
		if baseQuota <= 0 {
			baseQuota, err = CommissionBaseQuotaForPayment(snapshot)
			if err != nil {
				return err
			}
		}
		var credit RechargeCredit
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_ref = ?", tradeNo).First(&credit).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_, err = RecordPaidRechargeCreditTx(tx, userId, amountCents, baseQuota, snapshot.Currency, sourceType, tradeNo, completedAt)
			return err
		}
		if err != nil {
			return err
		}
		if credit.CommissionState == RechargeCommissionDone {
			return nil
		}
		if err := tx.Model(&credit).Updates(map[string]interface{}{
			"amount_cents": amountCents, "commission_base_quota": baseQuota,
			"payment_currency": snapshot.Currency, "commission_state": RechargeCommissionPending,
		}).Error; err != nil {
			return err
		}
		credit.AmountCents, credit.CommissionBaseQuota = amountCents, baseQuota
		credit.PaymentCurrency, credit.CommissionState = snapshot.Currency, RechargeCommissionPending
		_, err = SettleRechargeCreditCommissionTx(tx, &credit)
		return err
	}

	if tx.Migrator().HasTable(&VirtualMembershipOrder{}) && tx.Migrator().HasTable(&UserVirtualMembership{}) {
		var orders []VirtualMembershipOrder
		if err := tx.Where("status = ? AND payment_provider = ? AND dividend_state IN ?", VirtualMembershipOrderSuccess, PaymentProviderEpay, []string{SubscriptionDividendPending, SubscriptionDividendProcessing}).Find(&orders).Error; err != nil {
			return err
		}
		for i := range orders {
			var entitlementCount int64
			if err := tx.Model(&UserVirtualMembership{}).Where("order_id = ?", orders[i].Id).Count(&entitlementCount).Error; err != nil {
				return err
			}
			if entitlementCount != 1 {
				continue
			}
			var historical int64
			tx.Model(&DividendRecord{}).Where("source_ref = ?", fmt.Sprintf("vm-order-%d", orders[i].Id)).Count(&historical)
			if historical == 0 {
				snapshot, snapshotErr := NewPaymentSnapshotFromMoney(orders[i].Money, "CNY")
				if snapshotErr != nil {
					return snapshotErr
				}
				if err := promote(orders[i].UserId, snapshot, orders[i].CommissionBaseQuota, RechargeSourceVirtualMembership, orders[i].TradeNo, orders[i].CompleteTime); err != nil {
					if errors.Is(err, errHistoricalRechargeOwnerUnavailable) {
						continue
					}
					return err
				}
			}
			if err := tx.Model(&orders[i]).Update("dividend_state", SubscriptionDividendDone).Error; err != nil {
				return err
			}
		}
	}

	if tx.Migrator().HasTable(&SubscriptionOrder{}) && tx.Migrator().HasTable(&UserSubscription{}) {
		var orders []SubscriptionOrder
		if err := tx.Where("status = ? AND payment_provider IN ?", common.TopUpStatusSuccess, []string{PaymentProviderEpay, PaymentProviderStripe, PaymentProviderCreem, PaymentProviderWaffoPancake}).Find(&orders).Error; err != nil {
			return err
		}
		for i := range orders {
			var subs []UserSubscription
			if err := tx.Where("user_id = ? AND plan_id = ? AND source = ? AND created_at BETWEEN ? AND ? AND dividend_state IN ?", orders[i].UserId, orders[i].PlanId, "order", orders[i].CompleteTime-10, orders[i].CompleteTime+10, []string{SubscriptionDividendPending, SubscriptionDividendProcessing}).Limit(2).Find(&subs).Error; err != nil {
				return err
			}
			if len(subs) != 1 {
				if err := markSubscriptionReconciliationTx(tx, &orders[i], "manual_review", "paid subscription could not be mapped to exactly one pending entitlement"); err != nil {
					return err
				}
				continue
			}
			var historical int64
			tx.Model(&DividendRecord{}).Where("source_ref = ?", fmt.Sprintf("sub-end-%d", subs[0].Id)).Count(&historical)
			if historical == 0 {
				snapshot, snapshotErr := historicalSubscriptionPaymentSnapshot(&orders[i])
				if snapshotErr != nil {
					if err := markSubscriptionReconciliationTx(tx, &orders[i], "manual_review", snapshotErr.Error()); err != nil {
						return err
					}
					continue
				}
				if err := promote(orders[i].UserId, snapshot, orders[i].CommissionBaseQuota, RechargeSourceSubscription, orders[i].TradeNo, orders[i].CompleteTime); err != nil {
					if errors.Is(err, errHistoricalRechargeOwnerUnavailable) {
						if markErr := markSubscriptionReconciliationTx(tx, &orders[i], "manual_review", "historical paid subscription owner is unavailable"); markErr != nil {
							return markErr
						}
						continue
					}
					return err
				}
				baseQuota := orders[i].CommissionBaseQuota
				if baseQuota <= 0 {
					baseQuota, _ = CommissionBaseQuotaForPayment(snapshot)
				}
				if err := tx.Model(&orders[i]).Updates(map[string]interface{}{
					"expected_payment_amount_minor": snapshot.AmountMinor, "expected_payment_currency": snapshot.Currency,
					"actual_payment_amount_minor": snapshot.AmountMinor, "actual_payment_currency": snapshot.Currency,
					"commission_base_quota": baseQuota, "commission_reconciliation_status": "resolved",
					"commission_reconciliation_reason": "historical verified provider payload reconciled under recharge policy v1",
				}).Error; err != nil {
					return err
				}
			} else if err := markSubscriptionReconciliationTx(tx, &orders[i], "resolved_historical", "historical subscription dividend already exists and was left unchanged"); err != nil {
				return err
			}
			if err := tx.Model(&subs[0]).Updates(map[string]interface{}{"dividend_state": SubscriptionDividendDone, "updated_at": common.GetTimestamp()}).Error; err != nil {
				return err
			}
		}
	}
	if tx.Migrator().HasTable(&TopUp{}) {
		var topups []TopUp
		if err := tx.Where("status = ? AND payment_provider IN ?", common.TopUpStatusSuccess, []string{PaymentProviderEpay, PaymentProviderStripe, PaymentProviderCreem, PaymentProviderWaffo, PaymentProviderWaffoPancake}).Find(&topups).Error; err != nil {
			return err
		}
		for i := range topups {
			var credit RechargeCredit
			creditErr := tx.Where("source_type = ? AND source_ref = ?", RechargeSourceWalletTopUp, topups[i].TradeNo).First(&credit).Error
			if creditErr == nil && credit.CommissionState == RechargeCommissionDone {
				continue
			}
			if creditErr != nil && !errors.Is(creditErr, gorm.ErrRecordNotFound) {
				return creditErr
			}
			if topups[i].ActualPaymentAmountMinor <= 0 || strings.TrimSpace(topups[i].ActualPaymentCurrency) == "" {
				if err := tx.Model(&topups[i]).Updates(map[string]interface{}{
					"commission_reconciliation_status": "manual_review",
					"commission_reconciliation_reason": "historical wallet order has no persisted verified provider amount/currency",
				}).Error; err != nil {
					return err
				}
				continue
			}
			snapshot, snapshotErr := NewPaymentSnapshotFromMinor(topups[i].ActualPaymentAmountMinor, topups[i].ActualPaymentCurrency)
			if snapshotErr != nil {
				if err := tx.Model(&topups[i]).Updates(map[string]interface{}{
					"commission_reconciliation_status": "manual_review",
					"commission_reconciliation_reason": "persisted payment snapshot has unsupported amount or currency",
				}).Error; err != nil {
					return err
				}
				continue
			}
			if err := promote(topups[i].UserId, snapshot, topups[i].CommissionBaseQuota, RechargeSourceWalletTopUp, topups[i].TradeNo, topups[i].CompleteTime); err != nil {
				if errors.Is(err, errHistoricalRechargeOwnerUnavailable) {
					if markErr := tx.Model(&topups[i]).Updates(map[string]interface{}{
						"commission_reconciliation_status": "manual_review",
						"commission_reconciliation_reason": "historical paid wallet order owner is unavailable",
					}).Error; markErr != nil {
						return markErr
					}
					continue
				}
				return err
			}
			if err := tx.Model(&topups[i]).Updates(map[string]interface{}{
				"commission_reconciliation_status": "resolved",
				"commission_reconciliation_reason": "persisted verified payment snapshot reconciled under recharge policy v1",
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func markSubscriptionReconciliationTx(tx *gorm.DB, order *SubscriptionOrder, status, reason string) error {
	return tx.Model(order).Updates(map[string]interface{}{
		"commission_reconciliation_status": status,
		"commission_reconciliation_reason": reason,
	}).Error
}

func historicalSubscriptionPaymentSnapshot(order *SubscriptionOrder) (PaymentSnapshot, error) {
	if order.ActualPaymentAmountMinor > 0 && strings.TrimSpace(order.ActualPaymentCurrency) != "" {
		return NewPaymentSnapshotFromMinor(order.ActualPaymentAmountMinor, order.ActualPaymentCurrency)
	}
	if order.PaymentProvider == PaymentProviderEpay {
		return NewPaymentSnapshotFromMoney(order.Money, "CNY")
	}
	var payload map[string]interface{}
	if err := common.UnmarshalJsonStr(order.ProviderPayload, &payload); err != nil {
		return PaymentSnapshot{}, fmt.Errorf("verified provider payload is not parseable: %w", err)
	}
	lookup := func(root map[string]interface{}, path ...string) interface{} {
		var current interface{} = root
		for _, key := range path {
			m, ok := current.(map[string]interface{})
			if !ok {
				return nil
			}
			current = m[key]
		}
		return current
	}
	switch order.PaymentProvider {
	case PaymentProviderStripe:
		minor, err := strconv.ParseInt(fmt.Sprint(lookup(payload, "amount_total")), 10, 64)
		if err != nil {
			return PaymentSnapshot{}, fmt.Errorf("Stripe verified amount_total is unavailable")
		}
		return NewPaymentSnapshotFromMinor(minor, fmt.Sprint(lookup(payload, "currency")))
	case PaymentProviderCreem:
		minor, err := strconv.ParseInt(fmt.Sprint(lookup(payload, "object", "order", "amount_paid")), 10, 64)
		if err != nil {
			return PaymentSnapshot{}, fmt.Errorf("Creem verified amount_paid is unavailable")
		}
		return NewPaymentSnapshotFromMinor(minor, fmt.Sprint(lookup(payload, "object", "order", "currency")))
	case PaymentProviderWaffoPancake:
		return NewPaymentSnapshotFromDisplayAmount(fmt.Sprint(lookup(payload, "data", "amount")), fmt.Sprint(lookup(payload, "data", "currency")))
	default:
		return PaymentSnapshot{}, ErrUnsupportedPaymentCurrency
	}
}
