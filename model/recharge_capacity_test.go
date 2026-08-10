package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRechargeCapacityTierBoundaries(t *testing.T) {
	tests := []struct {
		cents       int64
		concurrency int
		rpm         int
	}{
		{0, 8, 15},
		{999, 8, 15},
		{1000, 15, 30},
		{4999, 15, 30},
		{5000, 20, 50},
		{19999, 20, 50},
		{20000, 30, 80},
		{50000, 50, 100},
		{100000, 70, 150},
		{199999, 70, 150},
		{200000, 70, 150},
		{999999, 70, 150},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d", test.cents), func(t *testing.T) {
			tier := RechargeCapacityForCents(test.cents)
			require.Equal(t, test.concurrency, tier.ConcurrencyLimit)
			require.Equal(t, test.rpm, tier.RPMLimit)
		})
	}
}

func TestRechargeCreditIsIdempotentAndUpdatesCapacity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &RechargeCredit{}))
	oldDB := DB
	oldEnabled := common.RechargeCapacityEnabled
	DB = db
	common.RechargeCapacityEnabled = true
	t.Cleanup(func() {
		DB = oldDB
		common.RechargeCapacityEnabled = oldEnabled
	})

	user := User{Username: "capacity-credit", Password: "hashed-password", AffCode: "rcc1"}
	require.NoError(t, db.Create(&user).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		created, createErr := RecordRechargeCreditTx(tx, user.Id, 5000, "topup", "order-1", 100)
		require.True(t, created)
		return createErr
	}))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		created, createErr := RecordRechargeCreditTx(tx, user.Id, 5000, "topup", "order-1", 100)
		require.False(t, created)
		return createErr
	}))

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.EqualValues(t, 5000, stored.RechargeTotalCents)
	require.Equal(t, 20, stored.EffectiveConcurrencyLimit())
	require.Equal(t, 50, stored.EffectiveRPMLimit())

	var count int64
	require.NoError(t, db.Model(&RechargeCredit{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestRechargeCapacityProgress(t *testing.T) {
	progress := BuildRechargeCapacityProgress(2750, 4, 20)
	require.EqualValues(t, 2250, progress.RemainingCents)
	require.InDelta(t, 0.4375, progress.Progress, 0.0001)
	require.NotNil(t, progress.NextTier)
	require.EqualValues(t, 5000, progress.NextTier.MinimumCents)
	require.Len(t, progress.Tiers, 6)
}

func TestAdministratorGrantIsAtomicIdempotentAndNotCumulativeRecharge(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &RechargeCredit{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	user := User{Username: "admin-recharge", Password: "hashed-password", AffCode: "rcc2"}
	require.NoError(t, db.Create(&user).Error)

	require.NoError(t, IncreaseUserQuotaWithRechargeCredit(user.Id, 500_000, 100, "admin-request-1"))
	require.NoError(t, IncreaseUserQuotaWithRechargeCredit(user.Id, 500_000, 100, "admin-request-1"))

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Equal(t, 500_000, stored.Quota)
	require.Zero(t, stored.RechargeTotalCents)
}

func TestManualCompleteTopUpWithholdsUnverifiedStripeCommissionButCompletesQuota(t *testing.T) {
	db := newRechargeCommissionTestDB(t)
	require.NoError(t, db.AutoMigrate(&TopUp{}, &Log{}))
	oldDB, oldQuota, oldPrice := DB, common.QuotaPerUnit, operation_setting.Price
	oldLogDB := LOG_DB
	DB, common.QuotaPerUnit, operation_setting.Price = db, 100, 7.3
	LOG_DB = db
	t.Cleanup(func() {
		DB, LOG_DB, common.QuotaPerUnit, operation_setting.Price = oldDB, oldLogDB, oldQuota, oldPrice
	})

	user := User{Username: "manual-stripe", Password: "hashed-password", AffCode: "manual-stripe"}
	require.NoError(t, db.Create(&user).Error)
	topup := TopUp{UserId: user.Id, Amount: 10, Money: 10, TradeNo: "manual-stripe-review", PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusPending}
	require.NoError(t, db.Create(&topup).Error)

	require.NoError(t, ManualCompleteTopUp(topup.TradeNo, "127.0.0.1"))
	require.NoError(t, db.First(&topup, topup.Id).Error)
	require.Equal(t, common.TopUpStatusSuccess, topup.Status)
	require.Equal(t, "manual_review", topup.CommissionReconciliationStatus)
	var creditCount int64
	require.NoError(t, db.Model(&RechargeCredit{}).Where("source_ref = ?", topup.TradeNo).Count(&creditCount).Error)
	require.Zero(t, creditCount)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.Equal(t, 1_000, user.Quota)
	require.Zero(t, user.RechargeTotalCents)

	actual := PaymentSnapshot{AmountMinor: 1_000, Currency: "USD"}
	require.NoError(t, Recharge(topup.TradeNo, "cus_verified", "127.0.0.1", actual))
	require.NoError(t, Recharge(topup.TradeNo, "cus_verified", "127.0.0.1", actual))
	require.NoError(t, db.First(&user, user.Id).Error)
	require.Equal(t, 1_000, user.Quota, "verified callback after manual completion must not credit quota twice")
	require.EqualValues(t, 7_300, user.RechargeTotalCents)
	require.NoError(t, db.First(&topup, topup.Id).Error)
	require.Empty(t, topup.CommissionReconciliationStatus)
	require.NoError(t, db.Model(&RechargeCredit{}).Where("source_ref = ?", topup.TradeNo).Count(&creditCount).Error)
	require.EqualValues(t, 1, creditCount)
	var providerLogCount int64
	require.NoError(t, db.Model(&Log{}).Where("financial_event_key = ?", "topup:wallet_topup:stripe:"+topup.TradeNo).Count(&providerLogCount).Error)
	require.EqualValues(t, 1, providerLogCount, "provider callback replay must not duplicate the financial log")
}

func TestManualCompleteTopUpWithholdsExpectedSnapshotUntilVerifiedCallback(t *testing.T) {
	db := newRechargeCommissionTestDB(t)
	require.NoError(t, db.AutoMigrate(&TopUp{}, &LuckyRechargeEvent{}, &LuckyRechargeProgress{}))
	oldDB, oldPrice, oldQuota := DB, operation_setting.Price, common.QuotaPerUnit
	DB, operation_setting.Price, common.QuotaPerUnit = db, 7.3, 100
	t.Cleanup(func() { DB, operation_setting.Price, common.QuotaPerUnit = oldDB, oldPrice, oldQuota })

	user := User{Username: "manual-waffo", Password: "hashed-password", AffCode: "manual-waffo"}
	require.NoError(t, db.Create(&user).Error)
	topup := TopUp{
		UserId: user.Id, Amount: 10, Money: 10, TradeNo: "manual-waffo-trusted",
		PaymentProvider: PaymentProviderWaffo, Status: common.TopUpStatusPending,
		LuckyRechargeEligible: true, LuckyRuleSetId: 99,
	}
	expected := PaymentSnapshot{AmountMinor: 1_000, Currency: "USD"}
	require.NoError(t, SetTopUpPaymentExpectation(&topup, expected))
	require.NoError(t, db.Create(&topup).Error)

	require.NoError(t, ManualCompleteTopUp(topup.TradeNo, "127.0.0.1"))
	require.NoError(t, db.First(&topup, topup.Id).Error)
	require.Equal(t, common.TopUpStatusSuccess, topup.Status)
	require.Equal(t, "manual_review", topup.CommissionReconciliationStatus)
	require.Zero(t, topup.ActualPaymentAmountMinor)
	var creditCount, luckyEventCount, luckyProgressCount int64
	require.NoError(t, db.Model(&RechargeCredit{}).Where("source_ref = ?", topup.TradeNo).Count(&creditCount).Error)
	require.NoError(t, db.Model(&LuckyRechargeEvent{}).Where("source_ref = ?", topup.TradeNo).Count(&luckyEventCount).Error)
	require.NoError(t, db.Model(&LuckyRechargeProgress{}).Where("user_id = ?", user.Id).Count(&luckyProgressCount).Error)
	require.Zero(t, creditCount)
	require.Zero(t, luckyEventCount)
	require.Zero(t, luckyProgressCount)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.Equal(t, 1_000, user.Quota)
	require.Zero(t, user.RechargeTotalCents)
}

func TestManualCompleteCreemUsesStoredQuotaWithoutMultiplyingAgain(t *testing.T) {
	db := newRechargeCommissionTestDB(t)
	require.NoError(t, db.AutoMigrate(&TopUp{}, &Log{}))
	oldDB, oldLogDB, oldQuota, oldPrice := DB, LOG_DB, common.QuotaPerUnit, operation_setting.Price
	DB, LOG_DB, common.QuotaPerUnit, operation_setting.Price = db, db, 100, 7.3
	t.Cleanup(func() {
		DB, LOG_DB, common.QuotaPerUnit, operation_setting.Price = oldDB, oldLogDB, oldQuota, oldPrice
	})

	user := User{Username: "manual-creem", Password: "hashed-password", AffCode: "manual-creem"}
	require.NoError(t, db.Create(&user).Error)
	topup := TopUp{
		UserId: user.Id, Amount: 1_000, Money: 10, TradeNo: "manual-creem-quota",
		PaymentProvider: PaymentProviderCreem, Status: common.TopUpStatusPending,
	}
	require.NoError(t, db.Create(&topup).Error)

	require.NoError(t, ManualCompleteTopUp(topup.TradeNo, "127.0.0.1"))
	require.NoError(t, db.First(&user, user.Id).Error)
	require.Equal(t, 1_000, user.Quota, "Creem Amount is already internal quota")
	require.Zero(t, user.RechargeTotalCents)
	require.NoError(t, db.First(&topup, topup.Id).Error)
	require.Equal(t, "manual_review", topup.CommissionReconciliationStatus)
	var creditCount int64
	require.NoError(t, db.Model(&RechargeCredit{}).Where("source_ref = ?", topup.TradeNo).Count(&creditCount).Error)
	require.Zero(t, creditCount)

	actual := PaymentSnapshot{AmountMinor: 1_000, Currency: "USD"}
	require.NoError(t, RechargeCreem(topup.TradeNo, "payer@example.com", "Payer", "127.0.0.1", actual))
	require.NoError(t, RechargeCreem(topup.TradeNo, "payer@example.com", "Payer", "127.0.0.1", actual))
	require.NoError(t, db.First(&user, user.Id).Error)
	require.Equal(t, 1_000, user.Quota, "verified callback after manual completion must not credit Creem quota twice")
	require.NoError(t, db.Model(&RechargeCredit{}).Where("source_ref = ?", topup.TradeNo).Count(&creditCount).Error)
	require.EqualValues(t, 1, creditCount)
	var providerLogCount int64
	require.NoError(t, db.Model(&Log{}).Where("financial_event_key = ?", "topup:wallet_topup:creem:"+topup.TradeNo).Count(&providerLogCount).Error)
	require.EqualValues(t, 1, providerLogCount, "Creem callback replay must not duplicate the financial log")
}

func TestEpayCompletionCreditsQuotaAndCapacityOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &RechargeCredit{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	user := User{Username: "epay-recharge", Password: "hashed-password", AffCode: "rcc3"}
	require.NoError(t, db.Create(&user).Error)
	topUp := TopUp{
		UserId:          user.Id,
		Amount:          10,
		Money:           9.5,
		TradeNo:         "epay-order-1",
		PaymentMethod:   "alipay",
		PaymentProvider: PaymentProviderEpay,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, db.Create(&topUp).Error)

	actual, err := NewPaymentSnapshotFromMoney(topUp.Money, "CNY")
	require.NoError(t, err)
	_, firstQuota, err := CompleteEpayTopUp(topUp.TradeNo, "alipay", actual)
	require.NoError(t, err)
	require.Equal(t, int(10*common.QuotaPerUnit), firstQuota)
	_, duplicateQuota, err := CompleteEpayTopUp(topUp.TradeNo, "alipay", actual)
	require.NoError(t, err)
	require.Zero(t, duplicateQuota)

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Equal(t, firstQuota, stored.Quota)
	require.EqualValues(t, 950, stored.RechargeTotalCents)
}

func TestRechargeCapacityMigrationBackfillsExternalSubscriptionsOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&User{},
		&TopUp{},
		&SubscriptionOrder{},
		&RechargeCredit{},
		&Log{},
		&Option{},
	))
	oldDB := DB
	oldLogDB := LOG_DB
	oldMaster := common.IsMasterNode
	DB = db
	LOG_DB = db
	common.IsMasterNode = true
	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
		common.IsMasterNode = oldMaster
	})

	user := User{Username: "migration-user", Password: "hashed-password", AffCode: "rcc4"}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&TopUp{
		UserId:  user.Id,
		Money:   10,
		TradeNo: "shared-order",
		Status:  common.TopUpStatusSuccess,
	}).Error)
	require.NoError(t, db.Create(&SubscriptionOrder{
		UserId:          user.Id,
		Money:           10,
		TradeNo:         "shared-order",
		PaymentMethod:   "alipay",
		PaymentProvider: PaymentProviderEpay,
		Status:          common.TopUpStatusSuccess,
	}).Error)
	require.NoError(t, db.Create(&SubscriptionOrder{
		UserId:          user.Id,
		Money:           20,
		TradeNo:         "subscription-only",
		PaymentMethod:   "alipay",
		PaymentProvider: PaymentProviderEpay,
		Status:          common.TopUpStatusSuccess,
	}).Error)
	require.NoError(t, db.Create(&SubscriptionOrder{
		UserId:          user.Id,
		Money:           30,
		TradeNo:         "balance-order",
		PaymentMethod:   PaymentMethodBalance,
		PaymentProvider: PaymentProviderBalance,
		Status:          common.TopUpStatusSuccess,
	}).Error)

	require.NoError(t, MigrateRechargeCapacityCreditsV3())
	require.NoError(t, MigrateRechargeCapacityCreditsV3())

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.EqualValues(t, 3000, stored.RechargeTotalCents)

	var count int64
	require.NoError(t, db.Model(&RechargeCredit{}).Count(&count).Error)
	require.EqualValues(t, 2, count)
	var nonLegacy int64
	require.NoError(t, db.Model(&RechargeCredit{}).Where("commission_state <> ?", RechargeCommissionLegacy).Count(&nonLegacy).Error)
	require.Zero(t, nonLegacy, "historical backfill must never retroactively pay recharge commission")
}
