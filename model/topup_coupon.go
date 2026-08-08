package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	TopUpCouponUsePending = "pending"
	TopUpCouponUseSuccess = "success"
)

type TopUpCoupon struct {
	Id          int     `json:"id" gorm:"primaryKey;autoIncrement"`
	Code        string  `json:"code" gorm:"uniqueIndex;type:varchar(64)"`
	Title       string  `json:"title" gorm:"type:varchar(128)"`
	Description string  `json:"description" gorm:"type:text"`
	Discount    float64 `json:"discount" gorm:"not null;default:1"`
	UserLimit   int     `json:"user_limit" gorm:"not null;default:1"`
	Enabled     bool    `json:"enabled" gorm:"not null;default:true;index"`
	CreatedAt   int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64   `json:"updated_at" gorm:"bigint"`
}

type TopUpCouponUse struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	CouponId   int    `json:"coupon_id" gorm:"index;uniqueIndex:idx_topup_coupon_trade,priority:1"`
	UserId     int    `json:"user_id" gorm:"index"`
	TradeNo    string `json:"trade_no" gorm:"type:varchar(255);uniqueIndex:idx_topup_coupon_trade,priority:2"`
	Status     string `json:"status" gorm:"type:varchar(20);index"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint"`
	CompleteAt int64  `json:"complete_at" gorm:"bigint"`
}

type TopUpCouponQuote struct {
	Coupon          TopUpCoupon `json:"coupon"`
	OriginalMoney   float64     `json:"original_money"`
	DiscountedMoney float64     `json:"discounted_money"`
	UsedCount       int64       `json:"used_count"`
	RemainingUses   int64       `json:"remaining_uses"`
}

func (coupon *TopUpCoupon) Normalize() {
	coupon.Code = strings.ToUpper(strings.TrimSpace(coupon.Code))
	coupon.Title = strings.TrimSpace(coupon.Title)
	coupon.Description = strings.TrimSpace(coupon.Description)
	if coupon.CreatedAt == 0 {
		coupon.CreatedAt = common.GetTimestamp()
	}
	coupon.UpdatedAt = common.GetTimestamp()
}

func (coupon *TopUpCoupon) Validate() error {
	if coupon == nil || coupon.Code == "" || coupon.Title == "" {
		return errors.New("优惠码和名称不能为空")
	}
	if strings.ContainsAny(coupon.Code, " /\\") {
		return errors.New("优惠码不能包含空格或路径分隔符")
	}
	if coupon.Discount <= 0 || coupon.Discount >= 1 {
		return errors.New("优惠折扣必须大于 0 且小于 1，例如 0.95")
	}
	if coupon.UserLimit <= 0 {
		return errors.New("每个用户可用次数必须大于 0")
	}
	return nil
}

func SaveTopUpCoupon(coupon *TopUpCoupon) error {
	if coupon == nil {
		return errors.New("优惠码不存在")
	}
	coupon.Normalize()
	if err := coupon.Validate(); err != nil {
		return err
	}
	return DB.Save(coupon).Error
}

func ListTopUpCoupons() ([]TopUpCoupon, error) {
	var coupons []TopUpCoupon
	return coupons, DB.Order("id desc").Find(&coupons).Error
}

func DeleteTopUpCoupon(couponId int) error {
	if couponId <= 0 {
		return errors.New("优惠码不存在")
	}
	var uses int64
	if err := DB.Model(&TopUpCouponUse{}).Where("coupon_id = ?", couponId).Count(&uses).Error; err != nil {
		return err
	}
	if uses > 0 {
		return errors.New("该优惠码已有使用记录，请停用而不是删除")
	}
	result := DB.Delete(&TopUpCoupon{}, couponId)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("优惠码不存在")
	}
	return nil
}

func quoteTopUpCouponTx(tx *gorm.DB, userId int, code string, originalMoney float64, lock bool) (*TopUpCouponQuote, error) {
	if tx == nil || userId <= 0 || originalMoney <= 0 {
		return nil, errors.New("优惠码校验参数无效")
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, nil
	}
	query := tx.Where("code = ? AND enabled = ?", code, true)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var coupon TopUpCoupon
	if err := query.First(&coupon).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("优惠码不存在或已停用")
		}
		return nil, err
	}
	if err := coupon.Validate(); err != nil {
		return nil, err
	}
	var used int64
	if err := tx.Model(&TopUpCouponUse{}).
		Where("coupon_id = ? AND user_id = ? AND (status = ? OR (status = ? AND created_at >= ?))",
			coupon.Id, userId, TopUpCouponUseSuccess, TopUpCouponUsePending, common.GetTimestamp()-24*3600).
		Count(&used).Error; err != nil {
		return nil, err
	}
	if used >= int64(coupon.UserLimit) {
		return nil, fmt.Errorf("该优惠码每个用户最多使用 %d 次", coupon.UserLimit)
	}
	discounted := decimal.NewFromFloat(originalMoney).
		Mul(decimal.NewFromFloat(coupon.Discount)).Round(2).InexactFloat64()
	if discounted < 0.01 {
		return nil, errors.New("使用优惠码后的支付金额过低")
	}
	return &TopUpCouponQuote{
		Coupon: coupon, OriginalMoney: originalMoney, DiscountedMoney: discounted,
		UsedCount: used, RemainingUses: int64(coupon.UserLimit) - used,
	}, nil
}

func HasEnabledTopUpCoupons() bool {
	if DB == nil {
		return false
	}
	var count int64
	return DB.Model(&TopUpCoupon{}).Where("enabled = ?", true).Count(&count).Error == nil && count > 0
}

func QuoteTopUpCoupon(userId int, code string, originalMoney float64) (*TopUpCouponQuote, error) {
	return quoteTopUpCouponTx(DB, userId, code, originalMoney, false)
}

func CreateTopUpWithCoupon(topUp *TopUp, couponCode string, originalMoney float64) error {
	if topUp == nil {
		return errors.New("充值订单不存在")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		quote, err := quoteTopUpCouponTx(tx, topUp.UserId, couponCode, originalMoney, true)
		if err != nil {
			return err
		}
		if quote != nil {
			topUp.CouponId = quote.Coupon.Id
			topUp.CouponCode = quote.Coupon.Code
			topUp.CouponDiscount = quote.Coupon.Discount
			topUp.Money = quote.DiscountedMoney
		}
		if err := tx.Create(topUp).Error; err != nil {
			return err
		}
		if quote == nil {
			return nil
		}
		return tx.Create(&TopUpCouponUse{
			CouponId: quote.Coupon.Id, UserId: topUp.UserId, TradeNo: topUp.TradeNo,
			Status: TopUpCouponUsePending, CreatedAt: common.GetTimestamp(),
		}).Error
	})
}

func completeTopUpCouponUseTx(tx *gorm.DB, topUp *TopUp) error {
	if tx == nil || topUp == nil || topUp.CouponId <= 0 {
		return nil
	}
	result := tx.Model(&TopUpCouponUse{}).
		Where("coupon_id = ? AND trade_no = ? AND status = ?", topUp.CouponId, topUp.TradeNo, TopUpCouponUsePending).
		Updates(map[string]interface{}{"status": TopUpCouponUseSuccess, "complete_at": common.GetTimestamp()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("优惠码使用记录不存在或状态无效")
	}
	return nil
}
