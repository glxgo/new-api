package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type TopUp struct {
	Id      int     `json:"id"`
	UserId  int     `json:"user_id" gorm:"index"`
	Amount  int64   `json:"amount"`
	Money   float64 `json:"money"`
	TradeNo string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	// ProductNameSnapshot is the immutable, human-readable gateway product
	// name captured when the order is created. It is metadata only and is never
	// used for payment verification.
	ProductNameSnapshot string  `json:"product_name_snapshot" gorm:"type:varchar(128);not null;default:''"`
	PaymentMethod       string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider     string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	CouponId            int     `json:"coupon_id" gorm:"index;not null;default:0"`
	CouponCode          string  `json:"coupon_code" gorm:"type:varchar(64);not null;default:''"`
	CouponDiscount      float64 `json:"coupon_discount" gorm:"not null;default:1"`
	CreateTime          int64   `json:"create_time"`
	CompleteTime        int64   `json:"complete_time"`
	// BalanceAfter 充值完成后用户余额快照(quota 单位, 本金+赠金)，
	// 仅供财务流水展示，不参与计费/扣费/退款逻辑。历史订单为 0。
	BalanceAfter                   *int64 `json:"balance_after" gorm:"column:balance_after"`
	Status                         string `json:"status"`
	LuckyRuleSetId                 int64  `json:"lucky_rule_set_id" gorm:"index;not null;default:0"`
	LuckyRechargeEligible          bool   `json:"lucky_recharge_eligible" gorm:"not null;default:false"`
	ExpectedPaymentAmountMinor     int64  `json:"expected_payment_amount_minor" gorm:"not null;default:0"`
	ExpectedPaymentCurrency        string `json:"expected_payment_currency" gorm:"type:varchar(8);not null;default:''"`
	CommissionBaseQuota            int64  `json:"commission_base_quota" gorm:"not null;default:0"`
	ActualPaymentAmountMinor       int64  `json:"actual_payment_amount_minor" gorm:"not null;default:0"`
	ActualPaymentCurrency          string `json:"actual_payment_currency" gorm:"type:varchar(8);not null;default:''"`
	CommissionReconciliationStatus string `json:"commission_reconciliation_status" gorm:"type:varchar(32);not null;default:'';index"`
	CommissionReconciliationReason string `json:"commission_reconciliation_reason" gorm:"type:varchar(255);not null;default:''"`
}

func SetTopUpPaymentExpectation(topUp *TopUp, snapshot PaymentSnapshot) error {
	if topUp == nil {
		return errors.New("topup is nil")
	}
	baseQuota, err := CommissionBaseQuotaForPayment(snapshot)
	if err != nil {
		return err
	}
	topUp.ExpectedPaymentAmountMinor = snapshot.AmountMinor
	topUp.ExpectedPaymentCurrency = snapshot.Currency
	topUp.CommissionBaseQuota = baseQuota
	return nil
}

// manualTopUpPaymentSnapshot turns the amount stored with an order into the
// immutable settlement snapshot authorized by an administrator. New orders
// should already carry an expectation; provider-specific fallbacks keep legacy
// Stripe/Creem/Epay orders completable without guessing an ambiguous currency.
func manualTopUpPaymentSnapshot(topUp *TopUp) (PaymentSnapshot, error) {
	if topUp == nil {
		return PaymentSnapshot{}, errors.New("topup is nil")
	}
	if topUp.ExpectedPaymentAmountMinor > 0 && strings.TrimSpace(topUp.ExpectedPaymentCurrency) != "" {
		return NewPaymentSnapshotFromMinor(topUp.ExpectedPaymentAmountMinor, topUp.ExpectedPaymentCurrency)
	}

	var currency string
	switch topUp.PaymentProvider {
	case "", PaymentProviderEpay:
		currency = "CNY"
	case PaymentProviderStripe, PaymentProviderCreem, PaymentProviderWaffoPancake:
		currency = "USD"
	case PaymentProviderWaffo:
		return PaymentSnapshot{}, errors.New("充值订单缺少支付金额和币种快照，无法补单")
	default:
		return PaymentSnapshot{}, errors.New("不支持该支付渠道的人工补单")
	}
	return NewPaymentSnapshotFromMoney(topUp.Money, currency)
}

func applyVerifiedTopUpPayment(topUp *TopUp, actual PaymentSnapshot) error {
	if topUp == nil {
		return errors.New("topup is nil")
	}
	actual, err := NewPaymentSnapshotFromMinor(actual.AmountMinor, actual.Currency)
	if err != nil {
		return err
	}
	if topUp.ExpectedPaymentAmountMinor <= 0 || strings.TrimSpace(topUp.ExpectedPaymentCurrency) == "" {
		// Stripe promotion codes and Creem provider products can change the
		// amount independently of local display prices. Their authenticated
		// webhook becomes the first immutable expectation.
		if topUp.PaymentProvider != PaymentProviderStripe && topUp.PaymentProvider != PaymentProviderCreem {
			return ErrPaymentSnapshotMismatch
		}
		if err := SetTopUpPaymentExpectation(topUp, actual); err != nil {
			return err
		}
	}
	if err := ValidatePaymentSnapshot(topUp.ExpectedPaymentAmountMinor, topUp.ExpectedPaymentCurrency, actual); err != nil {
		return err
	}
	if topUp.ActualPaymentAmountMinor > 0 &&
		(topUp.ActualPaymentAmountMinor != actual.AmountMinor || !strings.EqualFold(topUp.ActualPaymentCurrency, actual.Currency)) {
		return ErrPaymentSnapshotMismatch
	}
	topUp.ActualPaymentAmountMinor = actual.AmountMinor
	topUp.ActualPaymentCurrency = actual.Currency
	topUp.CommissionReconciliationStatus = ""
	topUp.CommissionReconciliationReason = ""
	return nil
}

func reconcileVerifiedSuccessfulTopUpTx(tx *gorm.DB, topUp *TopUp) error {
	if tx == nil || topUp == nil || topUp.Status != common.TopUpStatusSuccess {
		return errors.New("invalid successful topup reconciliation")
	}
	if err := tx.Save(topUp).Error; err != nil {
		return err
	}
	if _, err := RecordTopUpRechargeCreditTx(tx, topUp); err != nil {
		return err
	}
	_, err := RecordLuckyRechargeTx(tx, topUp)
	return err
}

func (topUp *TopUp) BeforeCreate(tx *gorm.DB) error {
	if topUp.LuckyRuleSetId == 0 && !topUp.LuckyRechargeEligible {
		campaign, rule, err := GetLuckyCampaignTx(tx, false)
		if err == nil && !campaign.IssuancePaused {
			topUp.LuckyRuleSetId = rule.Id
			topUp.LuckyRechargeEligible = true
		}
	}
	if topUp.PaymentProvider == PaymentProviderEpay && topUp.ExpectedPaymentAmountMinor == 0 && topUp.Money > 0 {
		snapshot, err := NewPaymentSnapshotFromMoney(topUp.Money, "CNY")
		if err != nil {
			return err
		}
		return SetTopUpPaymentExpectation(topUp, snapshot)
	}
	return nil
}

const (
	PaymentMethodStripe       = "stripe"
	PaymentMethodCreem        = "creem"
	PaymentMethodWaffo        = "waffo"
	PaymentMethodWaffoPancake = "waffo_pancake"
	PaymentMethodBalance      = "balance"
)

const (
	PaymentProviderEpay         = "epay"
	PaymentProviderStripe       = "stripe"
	PaymentProviderCreem        = "creem"
	PaymentProviderWaffo        = "waffo"
	PaymentProviderWaffoPancake = "waffo_pancake"
	PaymentProviderBalance      = "balance"
)

var (
	ErrPaymentMethodMismatch = errors.New("payment method mismatch")
	ErrTopUpNotFound         = errors.New("topup not found")
	ErrTopUpStatusInvalid    = errors.New("topup status invalid")
)

func (topUp *TopUp) Insert() error {
	var err error
	err = DB.Create(topUp).Error
	return err
}

func (topUp *TopUp) Update() error {
	var err error
	err = DB.Save(topUp).Error
	return err
}

func GetTopUpById(id int) *TopUp {
	var topUp *TopUp
	var err error
	err = DB.Where("id = ?", id).First(&topUp).Error
	if err != nil {
		return nil
	}
	return topUp
}

func GetTopUpByTradeNo(tradeNo string) *TopUp {
	var topUp *TopUp
	var err error
	err = DB.Where("trade_no = ?", tradeNo).First(&topUp).Error
	if err != nil {
		return nil
	}
	return topUp
}

func UpdatePendingTopUpStatus(tradeNo string, expectedPaymentProvider string, targetStatus string) error {
	if tradeNo == "" {
		return errors.New("未提供支付单号")
	}

	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			return ErrTopUpNotFound
		}
		if expectedPaymentProvider != "" && topUp.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if topUp.Status != common.TopUpStatusPending {
			return ErrTopUpStatusInvalid
		}

		topUp.Status = targetStatus
		return tx.Save(topUp).Error
	})
}

// snapshotBalanceAfterRecharge 在充值事务内读取用户当前 (quota + gift_quota)，
// 加上本次充值新增额度，得到充值完成后的余额快照（quota 单位，本金+赠金）。
// 仅用于填充 topUp.BalanceAfter，不影响计费/扣费/退款逻辑。读取失败返回 nil。
// 注意：读取发生在 tx 内 quota 更新之前，所以 = 更新前余额 + 本次充值额度 = 更新后余额。
func snapshotBalanceAfterRecharge(tx *gorm.DB, userId int, quotaToAdd int) *int64 {
	var u User
	if err := tx.Select("quota", "gift_quota").Where("id = ?", userId).First(&u).Error; err != nil {
		return nil
	}
	balance := int64(u.Quota + u.GiftQuota + quotaToAdd)
	return &balance
}

func Recharge(referenceId string, customerId string, callerIp string, actual PaymentSnapshot) (err error) {
	if referenceId == "" {
		return errors.New("未提供支付单号")
	}

	var quota float64
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", referenceId).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		if topUp.PaymentProvider != PaymentProviderStripe {
			return ErrPaymentMethodMismatch
		}
		if err := applyVerifiedTopUpPayment(topUp, actual); err != nil {
			return err
		}

		if topUp.Status == common.TopUpStatusSuccess {
			return reconcileVerifiedSuccessfulTopUpTx(tx, topUp)
		}
		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		quota = topUp.Money * common.QuotaPerUnit
		topUp.BalanceAfter = snapshotBalanceAfterRecharge(tx, topUp.UserId, int(quota))
		err = tx.Save(topUp).Error
		if err != nil {
			return err
		}

		err = tx.Model(&User{}).Where("id = ?", topUp.UserId).Updates(map[string]interface{}{"stripe_customer": customerId, "quota": gorm.Expr("quota + ?", quota)}).Error
		if err != nil {
			return err
		}

		if _, err = RecordTopUpRechargeCreditTx(tx, topUp); err != nil {
			return err
		}
		_, err = RecordLuckyRechargeTx(tx, topUp)
		return err
	})

	if err != nil {
		common.SysError("topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}

	verifiedQuota := int(decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
	if logErr := EnsureTopupPaymentLog(
		topUp.UserId,
		"wallet_topup:stripe:"+topUp.TradeNo,
		fmt.Sprintf("使用在线充值成功，充值金额: %v，支付金额：%d", logger.FormatQuota(verifiedQuota), topUp.Amount),
		callerIp, topUp.PaymentMethod, PaymentMethodStripe,
	); logErr != nil {
		common.SysLog("failed to ensure stripe topup log: " + logErr.Error())
	}
	_ = InvalidateUserCache(topUp.UserId)
	InvalidateRechargeCommissionRecipientCaches(RechargeSourceWalletTopUp, topUp.TradeNo)

	return nil
}

func CompleteEpayTopUp(tradeNo string, actualPaymentMethod string, actual PaymentSnapshot) (*TopUp, int, error) {
	if strings.TrimSpace(tradeNo) == "" {
		return nil, 0, errors.New("未提供支付单号")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	var completed TopUp
	var quotaToAdd int
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(&completed).Error; err != nil {
			return ErrTopUpNotFound
		}
		if completed.PaymentProvider != PaymentProviderEpay {
			return ErrPaymentMethodMismatch
		}
		if err := applyVerifiedTopUpPayment(&completed, actual); err != nil {
			return err
		}
		if completed.Status == common.TopUpStatusSuccess {
			return reconcileVerifiedSuccessfulTopUpTx(tx, &completed)
		}
		if completed.Status != common.TopUpStatusPending {
			return ErrTopUpStatusInvalid
		}
		if strings.TrimSpace(actualPaymentMethod) != "" {
			completed.PaymentMethod = strings.TrimSpace(actualPaymentMethod)
		}
		completed.Status = common.TopUpStatusSuccess
		completed.CompleteTime = common.GetTimestamp()
		quotaToAdd = int(decimal.NewFromInt(completed.Amount).
			Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
			IntPart())
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}
		completed.BalanceAfter = snapshotBalanceAfterRecharge(tx, completed.UserId, quotaToAdd)
		if err := tx.Save(&completed).Error; err != nil {
			return err
		}
		result := tx.Model(&User{}).
			Where("id = ?", completed.UserId).
			Update("quota", gorm.Expr("quota + ?", quotaToAdd))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("user %d not found", completed.UserId)
		}
		if _, err := RecordTopUpRechargeCreditTx(tx, &completed); err != nil {
			return err
		}
		if err := completeTopUpCouponUseTx(tx, &completed); err != nil {
			return err
		}
		_, err := RecordLuckyRechargeTx(tx, &completed)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	if quotaToAdd > 0 {
		_ = InvalidateUserCache(completed.UserId)
	}
	InvalidateRechargeCommissionRecipientCaches(RechargeSourceWalletTopUp, completed.TradeNo)
	return &completed, quotaToAdd, nil
}

// topUpQueryWindowSeconds 限制充值记录查询的时间窗口（秒）。
const topUpQueryWindowSeconds int64 = 30 * 24 * 60 * 60

// topUpQueryCutoff 返回允许查询的最早 create_time（秒级 Unix 时间戳）。
func topUpQueryCutoff() int64 {
	return common.GetTimestamp() - topUpQueryWindowSeconds
}

func GetUserTopUps(userId int, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	// Start transaction
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	cutoff := topUpQueryCutoff()

	// Get total count within transaction
	err = tx.Model(&TopUp{}).Where("user_id = ? AND create_time >= ?", userId, cutoff).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated topups within same transaction
	err = tx.Where("user_id = ? AND create_time >= ?", userId, cutoff).Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Commit transaction
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return topups, total, nil
}

// GetAllTopUps 获取全平台的充值记录（管理员使用，不限制时间窗口）
func GetAllTopUps(pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err = tx.Model(&TopUp{}).Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return topups, total, nil
}

// searchTopUpCountHardLimit 搜索充值记录时 COUNT 的安全上限，
// 防止对超大表执行无界 COUNT 触发 DoS。
const searchTopUpCountHardLimit = 10000

// SearchUserTopUps 按订单号搜索某用户的充值记录
func SearchUserTopUps(userId int, keyword string, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&TopUp{}).Where("user_id = ? AND create_time >= ?", userId, topUpQueryCutoff())
	if keyword != "" {
		pattern, perr := sanitizeLikePattern(keyword)
		if perr != nil {
			tx.Rollback()
			return nil, 0, perr
		}
		query = query.Where("trade_no LIKE ? ESCAPE '!'", pattern)
	}

	if err = query.Limit(searchTopUpCountHardLimit).Count(&total).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to count search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return topups, total, nil
}

// SearchAllTopUps 按订单号搜索全平台充值记录（管理员使用，不限制时间窗口）
func SearchAllTopUps(keyword string, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&TopUp{})
	if keyword != "" {
		pattern, perr := sanitizeLikePattern(keyword)
		if perr != nil {
			tx.Rollback()
			return nil, 0, perr
		}
		query = query.Where("trade_no LIKE ? ESCAPE '!'", pattern)
	}

	if err = query.Limit(searchTopUpCountHardLimit).Count(&total).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to count search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return topups, total, nil
}

// ManualCompleteTopUp 管理员手动完成订单并给用户充值
func ManualCompleteTopUp(tradeNo string, callerIp string) error {
	if tradeNo == "" {
		return errors.New("未提供订单号")
	}

	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	var userId int
	var quotaToAdd int
	var payMoney float64
	var paymentMethod string
	var completedNow bool

	err := DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		// 行级锁，避免并发补单
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			return errors.New("充值订单不存在")
		}

		// 幂等处理：已成功直接返回
		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("订单状态不是待支付，无法补单")
		}

		// 计算应充值额度：
		// - Creem 订单：Amount 在下单时已经是内部 quota，不得再乘 QuotaPerUnit。
		// - Stripe 订单：Money 代表经分组倍率换算后的美元数量，直接 * QuotaPerUnit。
		// - 其他订单（如易支付）：Amount 为美元数量，* QuotaPerUnit。
		if topUp.PaymentProvider == PaymentProviderStripe {
			dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
			quotaToAdd = int(decimal.NewFromFloat(topUp.Money).Mul(dQuotaPerUnit).IntPart())
		} else if topUp.PaymentProvider == PaymentProviderCreem {
			quotaToAdd = int(topUp.Amount)
		} else {
			dAmount := decimal.NewFromInt(topUp.Amount)
			dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
			quotaToAdd = int(dAmount.Mul(dQuotaPerUnit).IntPart())
		}
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}
		paymentSnapshot, err := manualTopUpPaymentSnapshot(topUp)
		if err != nil {
			return err
		}
		if topUp.ExpectedPaymentAmountMinor <= 0 || strings.TrimSpace(topUp.ExpectedPaymentCurrency) == "" {
			if err := SetTopUpPaymentExpectation(topUp, paymentSnapshot); err != nil {
				return err
			}
		}
		topUp.ActualPaymentAmountMinor = paymentSnapshot.AmountMinor
		topUp.ActualPaymentCurrency = paymentSnapshot.Currency
		topUp.CommissionReconciliationStatus = "manual_completed"
		topUp.CommissionReconciliationReason = "administrator authorized the stored order amount and currency for recharge settlement"

		// 标记完成
		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		topUp.BalanceAfter = snapshotBalanceAfterRecharge(tx, topUp.UserId, quotaToAdd)
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		// 增加用户额度（立即写库，保持一致性）
		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}
		if _, err := RecordTopUpRechargeCreditTx(tx, topUp); err != nil {
			return err
		}
		if err := completeTopUpCouponUseTx(tx, topUp); err != nil {
			return err
		}
		if _, err := RecordLuckyRechargeTx(tx, topUp); err != nil {
			return err
		}
		userId = topUp.UserId
		payMoney = topUp.Money
		paymentMethod = topUp.PaymentMethod
		completedNow = true
		return nil
	})

	if err != nil {
		return err
	}

	if !completedNow {
		return nil
	}

	// 事务外幂等记录日志，避免日志库阻塞主账务事务。
	content := fmt.Sprintf("管理员补单成功，充值额度: %v，支付金额: %.2f；已计入累充、分润和幸运进度", logger.FormatQuota(quotaToAdd), payMoney)
	if err := EnsureTopupPaymentLog(userId, "wallet_topup:manual:"+tradeNo, content, callerIp, paymentMethod, "admin"); err != nil {
		common.SysLog(fmt.Sprintf("failed to record manual topup log for user %d: %s", userId, err.Error()))
	}
	_ = InvalidateUserCache(userId)
	InvalidateRechargeCommissionRecipientCaches(RechargeSourceWalletTopUp, tradeNo)
	return nil
}
func RechargeCreem(referenceId string, customerEmail string, customerName string, callerIp string, actual PaymentSnapshot) (err error) {
	if referenceId == "" {
		return errors.New("未提供支付单号")
	}

	var quota int64
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", referenceId).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		if topUp.PaymentProvider != PaymentProviderCreem {
			return ErrPaymentMethodMismatch
		}
		if err := applyVerifiedTopUpPayment(topUp, actual); err != nil {
			return err
		}

		if topUp.Status == common.TopUpStatusSuccess {
			return reconcileVerifiedSuccessfulTopUpTx(tx, topUp)
		}
		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		// Creem 直接使用 Amount 作为充值额度（整数）
		quota = topUp.Amount
		topUp.BalanceAfter = snapshotBalanceAfterRecharge(tx, topUp.UserId, int(quota))
		err = tx.Save(topUp).Error
		if err != nil {
			return err
		}

		// 构建更新字段，优先使用邮箱，如果邮箱为空则使用用户名
		updateFields := map[string]interface{}{
			"quota": gorm.Expr("quota + ?", quota),
		}

		// 如果有客户邮箱，尝试更新用户邮箱（仅当用户邮箱为空时）
		if customerEmail != "" {
			// 先检查用户当前邮箱是否为空
			var user User
			err = tx.Where("id = ?", topUp.UserId).First(&user).Error
			if err != nil {
				return err
			}

			// 如果用户邮箱为空，则更新为支付时使用的邮箱
			if user.Email == "" {
				updateFields["email"] = customerEmail
			}
		}

		err = tx.Model(&User{}).Where("id = ?", topUp.UserId).Updates(updateFields).Error
		if err != nil {
			return err
		}

		if _, err = RecordTopUpRechargeCreditTx(tx, topUp); err != nil {
			return err
		}
		_, err = RecordLuckyRechargeTx(tx, topUp)
		return err
	})

	if err != nil {
		common.SysError("creem topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}

	if logErr := EnsureTopupPaymentLog(
		topUp.UserId,
		"wallet_topup:creem:"+topUp.TradeNo,
		fmt.Sprintf("使用Creem充值成功，充值额度: %v，支付金额：%.2f", topUp.Amount, topUp.Money),
		callerIp, topUp.PaymentMethod, PaymentMethodCreem,
	); logErr != nil {
		common.SysLog("failed to ensure creem topup log: " + logErr.Error())
	}
	_ = InvalidateUserCache(topUp.UserId)
	InvalidateRechargeCommissionRecipientCaches(RechargeSourceWalletTopUp, topUp.TradeNo)

	return nil
}

func RechargeWaffo(tradeNo string, callerIp string, actual PaymentSnapshot) (err error) {
	if tradeNo == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		if topUp.PaymentProvider != PaymentProviderWaffo {
			return ErrPaymentMethodMismatch
		}
		if err := applyVerifiedTopUpPayment(topUp, actual); err != nil {
			return err
		}

		if topUp.Status == common.TopUpStatusSuccess {
			return reconcileVerifiedSuccessfulTopUpTx(tx, topUp)
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		dAmount := decimal.NewFromInt(topUp.Amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		quotaToAdd = int(dAmount.Mul(dQuotaPerUnit).IntPart())
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		topUp.BalanceAfter = snapshotBalanceAfterRecharge(tx, topUp.UserId, quotaToAdd)
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}

		if _, err = RecordTopUpRechargeCreditTx(tx, topUp); err != nil {
			return err
		}
		_, err = RecordLuckyRechargeTx(tx, topUp)
		return err
	})

	if err != nil {
		common.SysError("waffo topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}

	if quotaToAdd > 0 {
		RecordTopupLog(topUp.UserId, fmt.Sprintf("Waffo充值成功，充值额度: %v，支付金额: %.2f", logger.FormatQuota(quotaToAdd), topUp.Money), callerIp, topUp.PaymentMethod, PaymentMethodWaffo)
		_ = InvalidateUserCache(topUp.UserId)
	}
	InvalidateRechargeCommissionRecipientCaches(RechargeSourceWalletTopUp, topUp.TradeNo)

	return nil
}

func RechargeWaffoPancake(tradeNo string, actual PaymentSnapshot) (err error) {
	if tradeNo == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		if topUp.PaymentProvider != PaymentProviderWaffoPancake {
			return ErrPaymentMethodMismatch
		}
		if err := applyVerifiedTopUpPayment(topUp, actual); err != nil {
			return err
		}

		if topUp.Status == common.TopUpStatusSuccess {
			return reconcileVerifiedSuccessfulTopUpTx(tx, topUp)
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		quotaToAdd = int(decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		topUp.BalanceAfter = snapshotBalanceAfterRecharge(tx, topUp.UserId, quotaToAdd)
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}

		if _, err = RecordTopUpRechargeCreditTx(tx, topUp); err != nil {
			return err
		}
		_, err = RecordLuckyRechargeTx(tx, topUp)
		return err
	})

	if err != nil {
		common.SysError("waffo pancake topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}

	if quotaToAdd > 0 {
		RecordLog(topUp.UserId, LogTypeTopup, fmt.Sprintf("Waffo Pancake充值成功，充值额度: %v，支付金额: %.2f", logger.FormatQuota(quotaToAdd), topUp.Money))
		_ = InvalidateUserCache(topUp.UserId)
	}
	InvalidateRechargeCommissionRecipientCaches(RechargeSourceWalletTopUp, topUp.TradeNo)

	return nil
}
