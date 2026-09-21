package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInvoiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:invoice-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &InvoiceApplication{}, &InvoiceApplicationOrder{}, &LuckyCampaign{}))
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })
	return db
}

func TestListEligibleInvoiceTopUpsFiltersPaidWalletOrders(t *testing.T) {
	db := setupInvoiceTestDB(t)
	user := User{Username: "invoice-eligible", Password: "hashed-password"}
	require.NoError(t, db.Create(&user).Error)

	actual := int64(1_250)
	require.NoError(t, db.Create(&TopUp{
		UserId: user.Id, Amount: 1, Money: 10, TradeNo: "invoice-paid",
		PaymentMethod: "epay", PaymentProvider: PaymentProviderEpay,
		ExpectedPaymentAmountMinor: actual, ExpectedPaymentCurrency: "CNY",
		ActualPaymentAmountMinor: actual, ActualPaymentCurrency: "CNY",
		Status: common.TopUpStatusSuccess,
	}).Error)
	require.NoError(t, db.Create(&TopUp{
		UserId: user.Id, Amount: 0, Money: 20, TradeNo: "invoice-subscription",
		PaymentMethod: "epay", PaymentProvider: PaymentProviderEpay,
		Status: common.TopUpStatusSuccess,
	}).Error)
	require.NoError(t, db.Create(&TopUp{
		UserId: user.Id, Amount: 1, Money: 30, TradeNo: "invoice-balance",
		PaymentMethod: PaymentMethodBalance, PaymentProvider: PaymentProviderBalance,
		Status: common.TopUpStatusSuccess,
	}).Error)
	require.NoError(t, db.Create(&TopUp{
		UserId: user.Id, Amount: 1, Money: 40, TradeNo: "invoice-failed",
		PaymentMethod: "epay", PaymentProvider: PaymentProviderEpay,
		Status: common.TopUpStatusFailed,
	}).Error)
	require.NoError(t, db.Create(&TopUp{
		UserId: user.Id, Amount: 1, Money: 50, TradeNo: "invoice-legacy",
		Status: common.TopUpStatusSuccess,
	}).Error)

	orders, err := ListEligibleInvoiceTopUps(user.Id)
	require.NoError(t, err)
	require.Len(t, orders, 2)
	byTradeNo := make(map[string]*InvoiceEligibleTopUp, len(orders))
	for _, order := range orders {
		byTradeNo[order.TradeNo] = order
	}
	require.Contains(t, byTradeNo, "invoice-paid")
	require.Contains(t, byTradeNo, "invoice-legacy")
	require.InDelta(t, 12.5, byTradeNo["invoice-paid"].Amount, 0.0001)
	require.Equal(t, "CNY", byTradeNo["invoice-paid"].Currency)
}

func TestCreateInvoiceApplicationClaimsOrdersAndRejectsInvalidCombinations(t *testing.T) {
	db := setupInvoiceTestDB(t)
	user := User{Username: "invoice-create", Password: "hashed-password", AffCode: "invoice-create-aff"}
	other := User{Username: "invoice-other", Password: "hashed-password", AffCode: "invoice-other-aff"}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&other).Error)

	makeTopUp := func(owner int, id string, currency string, amount int64) TopUp {
		return TopUp{
			UserId: owner, Amount: 1, Money: float64(amount) / 100, TradeNo: id,
			PaymentMethod: "epay", PaymentProvider: PaymentProviderEpay,
			ExpectedPaymentAmountMinor: amount, ExpectedPaymentCurrency: currency,
			ActualPaymentAmountMinor: amount, ActualPaymentCurrency: currency,
			Status: common.TopUpStatusSuccess,
		}
	}
	cnyOne := makeTopUp(user.Id, "invoice-cny-1", "CNY", 1_000)
	cnyTwo := makeTopUp(user.Id, "invoice-cny-2", "CNY", 2_500)
	usd := makeTopUp(user.Id, "invoice-usd", "USD", 500)
	otherOrder := makeTopUp(other.Id, "invoice-other-user", "CNY", 700)
	for _, topUp := range []*TopUp{&cnyOne, &cnyTwo, &usd, &otherOrder} {
		require.NoError(t, db.Create(topUp).Error)
	}

	_, err := CreateInvoiceApplication(user.Id, "company", "星岛科技", "9132", "billing@example.com", []int{usd.Id, cnyOne.Id})
	require.EqualError(t, err, "不同币种的充值订单不能合并开票")

	application, err := CreateInvoiceApplication(user.Id, "company", "星岛科技", "9132", "billing@example.com", []int{cnyOne.Id, cnyTwo.Id})
	require.NoError(t, err)
	require.Equal(t, 35.0, application.TotalAmount)
	require.Equal(t, "CNY", application.Currency)
	require.Len(t, application.Orders, 2)

	_, err = CreateInvoiceApplication(user.Id, "company", "星岛科技", "9132", "billing@example.com", []int{cnyOne.Id})
	require.EqualError(t, err, "所选充值订单已有发票申请")

	reviewed, err := ReviewInvoiceApplication(application.Id, InvoiceStatusApproved, 99, "root", "已完成线下开票")
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusApproved, reviewed.Status)
	require.Equal(t, "已完成线下开票", reviewed.Remark)
	_, err = ReviewInvoiceApplication(application.Id, InvoiceStatusRejected, 100, "other-root", "")
	require.EqualError(t, err, "该发票申请已处理")

	_, err = CreateInvoiceApplication(user.Id, "company", "星岛科技", "9132", "billing@example.com", []int{otherOrder.Id})
	require.EqualError(t, err, "存在不可开票或不属于当前用户的充值订单")
}
