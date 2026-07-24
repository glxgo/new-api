package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
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
		{0, 2, 10},
		{999, 2, 10},
		{1000, 4, 20},
		{4999, 4, 20},
		{5000, 8, 40},
		{19999, 8, 40},
		{20000, 15, 60},
		{50000, 20, 100},
		{100000, 30, 150},
		{199999, 30, 150},
		{200000, 50, 200},
		{999999, 50, 200},
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
	require.Equal(t, 8, stored.EffectiveConcurrencyLimit())
	require.Equal(t, 40, stored.EffectiveRPMLimit())

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
	require.Len(t, progress.Tiers, 7)
}

func TestAdministratorRechargeIsAtomicAndIdempotent(t *testing.T) {
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
	require.EqualValues(t, 100, stored.RechargeTotalCents)
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

	_, firstQuota, err := CompleteEpayTopUp(topUp.TradeNo, "alipay")
	require.NoError(t, err)
	require.Equal(t, int(10*common.QuotaPerUnit), firstQuota)
	_, duplicateQuota, err := CompleteEpayTopUp(topUp.TradeNo, "alipay")
	require.NoError(t, err)
	require.Zero(t, duplicateQuota)

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Equal(t, firstQuota, stored.Quota)
	require.EqualValues(t, 950, stored.RechargeTotalCents)
}
