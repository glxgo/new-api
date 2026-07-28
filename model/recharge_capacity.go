package model

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const rechargeCapacityMigrationKey = "RechargeCapacityCreditsMigratedV3"

type RechargeCapacityTier struct {
	MinimumCents     int64 `json:"minimum_cents"`
	MaximumCents     int64 `json:"maximum_cents"`
	ConcurrencyLimit int   `json:"concurrency_limit"`
	RPMLimit         int   `json:"rpm_limit"`
}

var rechargeCapacityTiers = []RechargeCapacityTier{
	{MinimumCents: 0, MaximumCents: 1000, ConcurrencyLimit: 8, RPMLimit: 15},
	{MinimumCents: 1000, MaximumCents: 5000, ConcurrencyLimit: 15, RPMLimit: 30},
	{MinimumCents: 5000, MaximumCents: 20000, ConcurrencyLimit: 20, RPMLimit: 50},
	{MinimumCents: 20000, MaximumCents: 50000, ConcurrencyLimit: 30, RPMLimit: 80},
	{MinimumCents: 50000, MaximumCents: 100000, ConcurrencyLimit: 50, RPMLimit: 100},
	{MinimumCents: 100000, MaximumCents: 0, ConcurrencyLimit: 70, RPMLimit: 150},
}

type RechargeCapacityProgress struct {
	Enabled          bool                   `json:"enabled"`
	TotalCents       int64                  `json:"total_cents"`
	CurrentTier      RechargeCapacityTier   `json:"current_tier"`
	NextTier         *RechargeCapacityTier  `json:"next_tier,omitempty"`
	RemainingCents   int64                  `json:"remaining_cents"`
	Progress         float64                `json:"progress"`
	Tiers            []RechargeCapacityTier `json:"tiers"`
	ConcurrencyLimit int                    `json:"concurrency_limit"`
	RPMLimit         int                    `json:"rpm_limit"`
}

// RechargeCredit is an append-only, idempotent qualification ledger. It keeps
// paid orders and administrator recharge actions auditable without treating
// gifts, redemptions, check-ins, refunds, or balance overrides as recharge.
type RechargeCredit struct {
	Id          int64  `json:"id" gorm:"primaryKey"`
	UserId      int    `json:"user_id" gorm:"not null;index"`
	AmountCents int64  `json:"amount_cents" gorm:"not null"`
	SourceType  string `json:"source_type" gorm:"type:varchar(32);not null;uniqueIndex:idx_recharge_credit_source,priority:1"`
	SourceRef   string `json:"source_ref" gorm:"type:varchar(255);not null;uniqueIndex:idx_recharge_credit_source,priority:2"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint;not null;index"`
}

func (RechargeCredit) TableName() string {
	return "recharge_credits"
}

func RechargeCapacityTiers() []RechargeCapacityTier {
	result := make([]RechargeCapacityTier, len(rechargeCapacityTiers))
	copy(result, rechargeCapacityTiers)
	return result
}

func RechargeCapacityForCents(totalCents int64) RechargeCapacityTier {
	if totalCents < 0 {
		totalCents = 0
	}
	for i := len(rechargeCapacityTiers) - 1; i >= 0; i-- {
		if totalCents >= rechargeCapacityTiers[i].MinimumCents {
			return rechargeCapacityTiers[i]
		}
	}
	return rechargeCapacityTiers[0]
}

func BuildRechargeCapacityProgress(totalCents int64, effectiveConcurrency int, effectiveRPM int) RechargeCapacityProgress {
	if totalCents < 0 {
		totalCents = 0
	}
	currentIndex := 0
	for i := len(rechargeCapacityTiers) - 1; i >= 0; i-- {
		if totalCents >= rechargeCapacityTiers[i].MinimumCents {
			currentIndex = i
			break
		}
	}
	current := rechargeCapacityTiers[currentIndex]
	result := RechargeCapacityProgress{
		Enabled:          common.RechargeCapacityEnabled,
		TotalCents:       totalCents,
		CurrentTier:      current,
		Tiers:            RechargeCapacityTiers(),
		ConcurrencyLimit: effectiveConcurrency,
		RPMLimit:         effectiveRPM,
		Progress:         1,
	}
	if currentIndex+1 >= len(rechargeCapacityTiers) {
		return result
	}
	next := rechargeCapacityTiers[currentIndex+1]
	result.NextTier = &next
	result.RemainingCents = next.MinimumCents - totalCents
	if result.RemainingCents < 0 {
		result.RemainingCents = 0
	}
	span := next.MinimumCents - current.MinimumCents
	if span > 0 {
		result.Progress = float64(totalCents-current.MinimumCents) / float64(span)
		if result.Progress < 0 {
			result.Progress = 0
		}
		if result.Progress > 1 {
			result.Progress = 1
		}
	}
	return result
}

func MoneyToRechargeCents(money float64) int64 {
	if money <= 0 {
		return 0
	}
	return decimal.NewFromFloat(money).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
}

func RecordRechargeCreditTx(tx *gorm.DB, userId int, amountCents int64, sourceType string, sourceRef string, createdAt int64) (bool, error) {
	if tx == nil || userId <= 0 || amountCents <= 0 {
		return false, errors.New("invalid recharge credit")
	}
	sourceType = strings.TrimSpace(sourceType)
	sourceRef = strings.TrimSpace(sourceRef)
	if sourceType == "" || sourceRef == "" {
		return false, errors.New("recharge credit source is required")
	}
	if createdAt <= 0 {
		createdAt = common.GetTimestamp()
	}
	credit := RechargeCredit{
		UserId:      userId,
		AmountCents: amountCents,
		SourceType:  sourceType,
		SourceRef:   sourceRef,
		CreatedAt:   createdAt,
	}
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "source_type"}, {Name: "source_ref"}},
		DoNothing: true,
	}).Create(&credit)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	update := tx.Model(&User{}).
		Where("id = ?", userId).
		Update("recharge_total_cents", gorm.Expr("recharge_total_cents + ?", amountCents))
	if update.Error != nil {
		return false, update.Error
	}
	if update.RowsAffected == 0 {
		var user User
		if err := tx.Select("id", "recharge_total_cents").Where("id = ?", userId).First(&user).Error; err != nil {
			return false, fmt.Errorf("user %d not found for recharge credit: %w", userId, err)
		}
		var ledgerTotal int64
		if err := tx.Model(&RechargeCredit{}).
			Where("user_id = ?", userId).
			Select("COALESCE(SUM(amount_cents), 0)").
			Scan(&ledgerTotal).Error; err != nil {
			return false, err
		}
		if user.RechargeTotalCents != ledgerTotal {
			if err := tx.Model(&User{}).
				Where("id = ?", userId).
				Update("recharge_total_cents", ledgerTotal).Error; err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

func RecordTopUpRechargeCreditTx(tx *gorm.DB, topUp *TopUp) (bool, error) {
	if topUp == nil || topUp.TradeNo == "" || topUp.Status != common.TopUpStatusSuccess {
		return false, nil
	}
	return RecordRechargeCreditTx(
		tx,
		topUp.UserId,
		MoneyToRechargeCents(topUp.Money),
		"topup",
		topUp.TradeNo,
		topUp.CompleteTime,
	)
}

func IncreaseUserQuotaWithRechargeCredit(userId int, quota int, amountCents int64, sourceRef string) error {
	if userId <= 0 || quota <= 0 || amountCents <= 0 || strings.TrimSpace(sourceRef) == "" {
		return errors.New("invalid administrator recharge")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		created, err := RecordRechargeCreditTx(tx, userId, amountCents, "admin", sourceRef, common.GetTimestamp())
		if err != nil {
			return err
		}
		if !created {
			return nil
		}
		if result := tx.Model(&User{}).
			Where("id = ?", userId).
			Update("quota", gorm.Expr("quota + ?", quota)); result.Error != nil {
			return result.Error
		} else if result.RowsAffected != 1 {
			return fmt.Errorf("user %d not found", userId)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := InvalidateUserCache(userId); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate recharge capacity cache for user %d: %s", userId, err.Error()))
	}
	return nil
}

func QuotaToRechargeCents(quota int) int64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	usd := decimal.NewFromInt(int64(quota)).Div(decimal.NewFromFloat(common.QuotaPerUnit))
	switch {
	case common.RechargeCapacityEnabled:
		// Administrator quota inputs use the configured display amount. The
		// current API sends quota units, so restore that entered amount here.
		switch operation_setting.GetQuotaDisplayType() {
		case operation_setting.QuotaDisplayTypeCNY:
			return usd.Mul(decimal.NewFromFloat(operation_setting.USDExchangeRate)).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
		case operation_setting.QuotaDisplayTypeCustom:
			rate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
			if rate <= 0 {
				rate = 1
			}
			return usd.Mul(decimal.NewFromFloat(rate)).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
		case operation_setting.QuotaDisplayTypeTokens:
			return decimal.NewFromInt(int64(quota)).Mul(decimal.NewFromInt(100)).IntPart()
		default:
			return usd.Mul(decimal.NewFromInt(100)).Round(0).IntPart()
		}
	default:
		return usd.Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	}
}

var firstDecimalNumber = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)

func parseHistoricalAdminRechargeCents(log Log) int64 {
	if strings.TrimSpace(log.Other) == "" {
		return 0
	}
	var other map[string]interface{}
	if err := common.UnmarshalJsonStr(log.Other, &other); err != nil {
		return 0
	}
	op, ok := other["op"].(map[string]interface{})
	if !ok || fmt.Sprintf("%v", op["action"]) != "user.quota_add" {
		return 0
	}
	params, ok := op["params"].(map[string]interface{})
	if !ok {
		return 0
	}
	match := firstDecimalNumber.FindString(fmt.Sprintf("%v", params["quota"]))
	if match == "" {
		return 0
	}
	amount, err := decimal.NewFromString(match)
	if err != nil || !amount.IsPositive() {
		return 0
	}
	return amount.Mul(decimal.NewFromInt(100)).Round(0).IntPart()
}

// MigrateRechargeCapacityCreditsV3 backfills successful paid top-ups,
// externally paid subscription orders, and structured administrator quota-add
// audit records. The ledger uniqueness makes a restart during migration safe.
func MigrateRechargeCapacityCreditsV3() error {
	if !common.IsMasterNode {
		return nil
	}
	var option Option
	if err := DB.Where(optionKeyWhereClause(), rechargeCapacityMigrationKey).First(&option).Error; err == nil && option.Value == "true" {
		return nil
	}

	var topUps []TopUp
	if err := DB.Where("status = ? AND money > 0", common.TopUpStatusSuccess).Find(&topUps).Error; err != nil {
		return err
	}
	type missingPaidSubscription struct {
		UserId       int
		Money        float64
		TradeNo      string
		CompleteTime int64
	}
	var missingSubscriptions []missingPaidSubscription
	if err := DB.
		Table("subscription_orders AS o").
		Select("o.user_id, o.money, o.trade_no, o.complete_time").
		Joins("JOIN users AS u ON u.id = o.user_id AND u.deleted_at IS NULL").
		Joins("LEFT JOIN recharge_credits AS r ON r.source_type = ? AND r.source_ref = TRIM(o.trade_no)", "topup").
		Where("o.status = ? AND o.money > 0", common.TopUpStatusSuccess).
		Where("LOWER(TRIM(COALESCE(o.payment_provider, ''))) <> ?", PaymentProviderBalance).
		Where("LOWER(TRIM(COALESCE(o.payment_method, ''))) <> ?", PaymentMethodBalance).
		Where("r.id IS NULL").
		Scan(&missingSubscriptions).Error; err != nil {
		return err
	}
	var logs []Log
	if LOG_DB != nil {
		if err := LOG_DB.
			Where("type = ? AND other LIKE ?", LogTypeManage, "%user.quota_add%").
			Find(&logs).Error; err != nil {
			return err
		}
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		for i := range topUps {
			var userCount int64
			if err := tx.Model(&User{}).Where("id = ?", topUps[i].UserId).Count(&userCount).Error; err != nil {
				return err
			}
			if userCount == 0 {
				continue
			}
			if _, err := RecordTopUpRechargeCreditTx(tx, &topUps[i]); err != nil {
				return err
			}
		}
		for i := range missingSubscriptions {
			order := &missingSubscriptions[i]
			if _, err := RecordRechargeCreditTx(
				tx,
				order.UserId,
				MoneyToRechargeCents(order.Money),
				"topup",
				order.TradeNo,
				order.CompleteTime,
			); err != nil {
				return err
			}
		}
		for _, log := range logs {
			amountCents := parseHistoricalAdminRechargeCents(log)
			if amountCents <= 0 {
				continue
			}
			var userCount int64
			if err := tx.Model(&User{}).Where("id = ?", log.UserId).Count(&userCount).Error; err != nil {
				return err
			}
			if userCount == 0 {
				continue
			}
			if _, err := RecordRechargeCreditTx(
				tx,
				log.UserId,
				amountCents,
				"admin_log",
				strconv.FormatInt(int64(log.Id), 10),
				log.CreatedAt,
			); err != nil {
				return err
			}
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"}),
		}).Create(&Option{Key: rechargeCapacityMigrationKey, Value: "true"}).Error
	})
}
