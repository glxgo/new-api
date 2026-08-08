package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTopUpCouponTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:topup-coupon-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&TopUpCoupon{}, &TopUpCouponUse{}, &TopUp{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })
	return db
}

func TestTopUpCouponDiscountAndPerUserLimit(t *testing.T) {
	db := setupTopUpCouponTestDB(t)
	coupon := TopUpCoupon{
		Code: " summer95 ", Title: "夏日优惠", Discount: 0.95,
		UserLimit: 1, Enabled: true,
	}
	require.NoError(t, SaveTopUpCoupon(&coupon))
	require.Equal(t, "SUMMER95", coupon.Code)

	topup := TopUp{
		UserId: 7, Amount: 20, Money: 20, TradeNo: "coupon-trade-1",
		PaymentMethod: "alipay", PaymentProvider: PaymentProviderEpay,
		CreateTime: common.GetTimestamp(), Status: common.TopUpStatusPending,
	}
	require.NoError(t, CreateTopUpWithCoupon(&topup, "summer95", 20))
	require.InDelta(t, 19, topup.Money, 0.0001)
	require.Equal(t, coupon.Id, topup.CouponId)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return completeTopUpCouponUseTx(tx, &topup)
	}))
	_, err := QuoteTopUpCoupon(7, "SUMMER95", 20)
	require.ErrorContains(t, err, "最多使用 1 次")

	quote, err := QuoteTopUpCoupon(8, "SUMMER95", 20)
	require.NoError(t, err)
	require.InDelta(t, 19, quote.DiscountedMoney, 0.0001)
	require.Equal(t, int64(1), quote.RemainingUses)
}
