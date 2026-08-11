package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RechargeSourceWalletTopUp       = "topup"
	RechargeSourceSubscription      = "subscription"
	RechargeSourceVirtualMembership = "virtual_membership"
	RechargeSourceAdmin             = "admin"
	RechargeSourceAdminLog          = "admin_log"

	RechargeCommissionLegacy        = "legacy"
	RechargeCommissionPending       = "pending"
	RechargeCommissionDone          = "done"
	RechargeCommissionSkippedSource = "skipped_source"
	RechargeCommissionPolicyV1      = 1
)

var ErrUnsupportedPaymentCurrency = errors.New("unsupported payment currency snapshot")
var ErrPaymentSnapshotMismatch = errors.New("verified payment amount or currency does not match order snapshot")

// PaymentSnapshot is captured only after a provider webhook has been
// authenticated. AmountMinor uses the ISO currency's minor unit; this policy
// currently accepts only CNY and USD, both with two decimal places.
type PaymentSnapshot struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

func NewPaymentSnapshotFromMinor(amountMinor int64, currency string) (PaymentSnapshot, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if amountMinor <= 0 || (currency != "CNY" && currency != "USD") {
		return PaymentSnapshot{}, ErrUnsupportedPaymentCurrency
	}
	return PaymentSnapshot{AmountMinor: amountMinor, Currency: currency}, nil
}

func NewPaymentSnapshotFromDisplayAmount(amount, currency string) (PaymentSnapshot, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency != "CNY" && currency != "USD" {
		return PaymentSnapshot{}, ErrUnsupportedPaymentCurrency
	}
	value, err := decimal.NewFromString(strings.TrimSpace(amount))
	if err != nil || !value.IsPositive() {
		return PaymentSnapshot{}, errors.New("invalid payment amount")
	}
	minor := value.Mul(decimal.NewFromInt(100))
	if !minor.Equal(minor.Round(0)) {
		return PaymentSnapshot{}, errors.New("payment amount has unsupported precision")
	}
	return NewPaymentSnapshotFromMinor(minor.IntPart(), currency)
}

func NewPaymentSnapshotFromMoney(amount float64, currency string) (PaymentSnapshot, error) {
	return NewPaymentSnapshotFromDisplayAmount(decimal.NewFromFloat(amount).StringFixed(2), currency)
}

func CommissionBaseQuotaForPayment(snapshot PaymentSnapshot) (int64, error) {
	snapshot, err := NewPaymentSnapshotFromMinor(snapshot.AmountMinor, snapshot.Currency)
	if err != nil {
		return 0, err
	}
	if snapshot.Currency == "CNY" {
		return CNYCentsToCommissionBaseQuota(snapshot.AmountMinor)
	}
	if common.QuotaPerUnit <= 0 {
		return 0, errors.New("invalid quota conversion configuration")
	}
	return decimal.NewFromInt(snapshot.AmountMinor).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Div(decimal.NewFromInt(100)).Round(0).IntPart(), nil
}

func RechargeCentsForPayment(snapshot PaymentSnapshot) (int64, error) {
	snapshot, err := NewPaymentSnapshotFromMinor(snapshot.AmountMinor, snapshot.Currency)
	if err != nil {
		return 0, err
	}
	if snapshot.Currency == "CNY" {
		return snapshot.AmountMinor, nil
	}
	if operation_setting.Price <= 0 {
		return 0, errors.New("invalid CNY sale price configuration")
	}
	return decimal.NewFromInt(snapshot.AmountMinor).Mul(decimal.NewFromFloat(operation_setting.Price)).Round(0).IntPart(), nil
}

func ValidatePaymentSnapshot(expectedAmountMinor int64, expectedCurrency string, actual PaymentSnapshot) error {
	normalized, err := NewPaymentSnapshotFromMinor(actual.AmountMinor, actual.Currency)
	if err != nil {
		return err
	}
	if expectedAmountMinor <= 0 || strings.ToUpper(strings.TrimSpace(expectedCurrency)) != normalized.Currency || expectedAmountMinor != normalized.AmountMinor {
		return ErrPaymentSnapshotMismatch
	}
	return nil
}

// The recharge commission policy is fixed business policy, not a pricing or
// cost setting. Rates are basis points to avoid floating point drift.
const (
	rechargeCommissionOrdinaryDirectBP   int64 = 500
	rechargeCommissionOrdinaryIndirectBP int64 = 200
	rechargeCommissionAgentDirectBP      int64 = 800
	rechargeCommissionAgentIndirectBP    int64 = 400
	rechargeCommissionAdminDirectBP      int64 = 1500
	rechargeCommissionAdminIndirectBP    int64 = 500
	rechargeCommissionRootBP             int64 = 500
)

func RechargeCommissionSourceRef(sourceType, sourceRef string) string {
	sourceType = strings.TrimSpace(sourceType)
	sourceRef = strings.TrimSpace(sourceRef)
	raw := "recharge:" + sourceType + ":" + sourceRef
	if len(raw) <= 64 {
		return raw
	}
	sum := sha256.Sum256([]byte(raw))
	prefix := sourceType
	if len(prefix) > 20 {
		prefix = prefix[:20]
	}
	return fmt.Sprintf("recharge:%s:%x", prefix, sum[:16])
}

func rechargeSourcePaysCommission(sourceType string) bool {
	switch strings.TrimSpace(sourceType) {
	case RechargeSourceWalletTopUp, RechargeSourceSubscription, RechargeSourceVirtualMembership:
		return true
	default:
		return false
	}
}

// CNYCentsToCommissionBaseQuota snapshots the quota-equivalent value of an
// actual CNY payment. operation_setting.Price is the site's CNY sale price for
// one wallet dollar; USDExchangeRate is intentionally not used here.
func CNYCentsToCommissionBaseQuota(amountCents int64) (int64, error) {
	if amountCents <= 0 || common.QuotaPerUnit <= 0 || operation_setting.Price <= 0 {
		return 0, errors.New("invalid CNY commission conversion configuration")
	}
	return decimal.NewFromInt(amountCents).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Div(decimal.NewFromInt(100)).
		Div(decimal.NewFromFloat(operation_setting.Price)).
		Round(0).IntPart(), nil
}

func rechargeCommissionAmount(baseQuota, basisPoints int64) int {
	if baseQuota <= 0 || basisPoints <= 0 {
		return 0
	}
	return int(decimal.NewFromInt(baseQuota).
		Mul(decimal.NewFromInt(basisPoints)).
		Div(decimal.NewFromInt(10_000)).
		Round(0).IntPart())
}

func rechargeCommissionDirectRate(role int) int64 {
	if role == common.RoleAgentUser {
		return rechargeCommissionAgentDirectBP
	}
	return rechargeCommissionOrdinaryDirectBP
}

func rechargeCommissionIndirectRate(role int) int64 {
	if role == common.RoleAgentUser {
		return rechargeCommissionAgentIndirectBP
	}
	return rechargeCommissionOrdinaryIndirectBP
}

// RechargeCommissionReferralRates exposes the immutable referral percentages
// for the public affiliate page. All paid sources use the same policy.
func RechargeCommissionReferralRates(role int) (direct, indirect float64) {
	return float64(rechargeCommissionDirectRate(role)) / 10_000,
		float64(rechargeCommissionIndirectRate(role)) / 10_000
}

// settleRechargeCommissionTx atomically writes audit rows and moves the
// corresponding gift/withdrawable balances. sourceRef is the idempotency key.
func settleRechargeCommissionTx(tx *gorm.DB, buyer *User, baseQuota, amountCents int64, sourceType, sourceRef string, createdAt int64) ([]int, error) {
	if tx == nil {
		return nil, errors.New("recharge commission transaction is nil")
	}
	if buyer == nil || buyer.Id <= 0 || baseQuota <= 0 || buyer.Role >= common.RoleRootUser {
		return nil, nil
	}
	auditRef := RechargeCommissionSourceRef(sourceType, sourceRef)
	if auditRef == "" {
		return nil, errors.New("recharge commission source is required")
	}
	getUser := func(id int) (*User, error) {
		if id <= 0 {
			return nil, nil
		}
		var user User
		if err := tx.Omit("password").Where("id = ?", id).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			return nil, err
		}
		return &user, nil
	}

	inviter, err := getUser(buyer.InviterId)
	if err != nil {
		return nil, err
	}
	inviter2Id := 0
	if inviter != nil {
		inviter2Id = inviter.InviterId
	}
	inviter2, err := getUser(inviter2Id)
	if err != nil {
		return nil, err
	}
	admin, err := getUser(buyer.AffAdminId)
	if err != nil {
		return nil, err
	}
	var root User
	var rootPtr *User
	if err := tx.Omit("password").Where("role = ?", common.RoleRootUser).Order("id asc").First(&root).Error; err == nil {
		rootPtr = &root
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if createdAt <= 0 {
		createdAt = common.GetTimestamp()
	}
	batchId := "recharge-" + fmt.Sprint(createdAt)
	records := make([]*DividendRecord, 0, 4)
	appendRecord := func(recipient *User, dividendType int, basisPoints int64) {
		if recipient == nil || recipient.Id <= 0 || basisPoints <= 0 {
			return
		}
		amount := rechargeCommissionAmount(baseQuota, basisPoints)
		if amount <= 0 {
			return
		}
		commissionKey := fmt.Sprintf("%s:%d:%d", auditRef, recipient.Id, dividendType)
		records = append(records, &DividendRecord{
			BatchId: batchId, UserId: recipient.Id, SourceUserId: buyer.Id,
			Type: dividendType, GrossProfit: 0, Amount: amount,
			SourceRechargeCents: amountCents, SourceRef: auditRef,
			CommissionKey: &commissionKey, PolicyVersion: RechargeCommissionPolicyV1, CreatedAt: createdAt,
		})
	}

	if inviter != nil && inviter.Role < common.RoleAdminUser {
		appendRecord(inviter, DividendTypeDirect, rechargeCommissionDirectRate(inviter.Role))
	}
	if inviter2 != nil && inviter2.Role < common.RoleAdminUser {
		appendRecord(inviter2, DividendTypeIndirect, rechargeCommissionIndirectRate(inviter2.Role))
	}
	// Root receives only the unconditional 5% root share. The additional
	// administrator share belongs exclusively to the ordinary admin role.
	if admin != nil && admin.Role == common.RoleAdminUser {
		switch {
		case buyer.InviterId == admin.Id:
			appendRecord(admin, DividendTypeAdmin, rechargeCommissionAdminDirectBP)
		case inviter2Id == admin.Id:
			appendRecord(admin, DividendTypeAdmin, rechargeCommissionAdminIndirectBP)
		}
	}
	if rootPtr != nil {
		appendRecord(rootPtr, DividendTypeRoot, rechargeCommissionRootBP)
	}

	accumGift := map[int]int{}
	accumDividend := map[int]int{}
	for _, record := range records {
		result := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "commission_key"}}, DoNothing: true,
		}).Create(record)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		var recipient User
		if err := tx.Select("id", "role").First(&recipient, record.UserId).Error; err != nil {
			return nil, err
		}
		if (record.Type == DividendTypeDirect || record.Type == DividendTypeIndirect) && !common.AffiliateRewardIsWithdrawable(recipient.Role) {
			accumGift[recipient.Id] += record.Amount
		} else {
			accumDividend[recipient.Id] += record.Amount
		}
	}
	giftRecipients := make([]int, 0, len(accumGift))
	for userId, amount := range accumGift {
		result := tx.Model(&User{}).Where("id = ?", userId).
			Update("gift_quota", gorm.Expr("gift_quota + ?", amount))
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected != 1 {
			return nil, fmt.Errorf("recharge commission gift recipient %d not found", userId)
		}
		giftRecipients = append(giftRecipients, userId)
	}
	for userId, amount := range accumDividend {
		result := tx.Model(&User{}).Where("id = ?", userId).Updates(map[string]interface{}{
			"dividend_balance": gorm.Expr("dividend_balance + ?", amount),
			"dividend_total":   gorm.Expr("dividend_total + ?", amount),
		})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected != 1 {
			return nil, fmt.Errorf("recharge commission recipient %d not found", userId)
		}
	}
	return giftRecipients, nil
}

func SettleRechargeCreditCommissionTx(tx *gorm.DB, credit *RechargeCredit) ([]int, error) {
	if tx == nil || credit == nil || credit.Id <= 0 {
		return nil, errors.New("invalid recharge credit commission")
	}
	var locked RechargeCredit
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, credit.Id).Error; err != nil {
		return nil, err
	}
	if locked.CommissionState == RechargeCommissionDone || locked.CommissionState == RechargeCommissionSkippedSource {
		*credit = locked
		return nil, nil
	}
	cutoverAt, err := rechargeCommissionCutoverAtTx(tx)
	if err != nil {
		return nil, err
	}
	if cutoverAt > 0 && locked.CreatedAt < cutoverAt {
		// Historical credits still belong to the cumulative-recharge ledger, but
		// they must never be promoted into the new fixed commission policy.
		locked.CommissionState = RechargeCommissionLegacy
		locked.CommissionPolicyVersion = 0
		locked.CommissionSettledAt = 0
		if err := tx.Model(&RechargeCredit{}).Where("id = ?", locked.Id).Updates(map[string]interface{}{
			"commission_state":          locked.CommissionState,
			"commission_policy_version": locked.CommissionPolicyVersion,
			"commission_settled_at":     locked.CommissionSettledAt,
		}).Error; err != nil {
			return nil, err
		}
		*credit = locked
		return nil, nil
	}
	if !rechargeSourcePaysCommission(locked.SourceType) {
		locked.CommissionState = RechargeCommissionSkippedSource
		locked.CommissionPolicyVersion = RechargeCommissionPolicyV1
		locked.CommissionSettledAt = common.GetTimestamp()
		if err := tx.Model(&RechargeCredit{}).Where("id = ?", locked.Id).Updates(map[string]interface{}{
			"commission_state":          locked.CommissionState,
			"commission_policy_version": locked.CommissionPolicyVersion,
			"commission_settled_at":     locked.CommissionSettledAt,
		}).Error; err != nil {
			return nil, err
		}
		*credit = locked
		return nil, nil
	}
	if locked.CommissionState != RechargeCommissionPending {
		// Rows created before the fixed-recharge policy are immutable history.
		*credit = locked
		return nil, nil
	}
	var buyer User
	if err := tx.Omit("password").Where("id = ?", locked.UserId).First(&buyer).Error; err != nil {
		return nil, err
	}
	giftRecipients, err := settleRechargeCommissionTx(
		tx, &buyer, locked.CommissionBaseQuota, locked.AmountCents,
		locked.SourceType, locked.SourceRef, locked.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	locked.CommissionState = RechargeCommissionDone
	locked.CommissionPolicyVersion = RechargeCommissionPolicyV1
	locked.CommissionSettledAt = common.GetTimestamp()
	if err := tx.Model(&RechargeCredit{}).Where("id = ? AND commission_state = ?", locked.Id, RechargeCommissionPending).
		Updates(map[string]interface{}{
			"commission_state":          locked.CommissionState,
			"commission_policy_version": locked.CommissionPolicyVersion,
			"commission_settled_at":     locked.CommissionSettledAt,
		}).Error; err != nil {
		return nil, err
	}
	*credit = locked
	return giftRecipients, nil
}

// InvalidateRechargeCommissionRecipientCaches runs after the payment
// transaction commits. Replays are harmless and do not move balances.
func InvalidateRechargeCommissionRecipientCaches(sourceType, sourceRef string) {
	auditRef := RechargeCommissionSourceRef(sourceType, sourceRef)
	if auditRef == "" || DB == nil {
		return
	}
	var userIds []int
	if err := DB.Model(&DividendRecord{}).Where("source_ref = ?", auditRef).
		Distinct("user_id").Pluck("user_id", &userIds).Error; err != nil {
		common.SysError("load recharge commission recipients failed: " + err.Error())
		return
	}
	invalidateDividendGiftCaches(userIds)
}

func invalidateDividendGiftCaches(userIds []int) {
	for _, userId := range userIds {
		if err := invalidateUserCache(userId); err != nil {
			common.SysError(fmt.Sprintf("invalidate commission recipient cache failed uid=%d: %v", userId, err))
		}
	}
}
