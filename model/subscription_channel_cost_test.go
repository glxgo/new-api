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

func setupSubscriptionChannelCostTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:subscription-channel-cost-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&User{},
		&UserSubscription{},
		&SubscriptionPreConsumeRecord{},
		&DividendRecord{},
	))
	oldDB := DB
	DB = db
	t.Cleanup(func() {
		DB = oldDB
	})
	return db
}

func TestFinalizeSubscriptionPreConsumeUsesOldToNewCostDelta(t *testing.T) {
	db := setupSubscriptionChannelCostTestDB(t)
	sub := UserSubscription{
		UserId:           101,
		Status:           "active",
		EndTime:          common.GetTimestamp() + 3600,
		PaidRevenueQuota: 1000,
		DividendState:    SubscriptionDividendPending,
	}
	require.NoError(t, db.Create(&sub).Error)
	record := SubscriptionPreConsumeRecord{
		RequestId:          "req-channel-cost-delta",
		UserId:             sub.UserId,
		UserSubscriptionId: sub.Id,
		PreConsumed:        100,
		Status:             SubscriptionCostStatusReserved,
	}
	require.NoError(t, db.Create(&record).Error)

	ratio := int64(850_000)
	require.NoError(t, FinalizeSubscriptionPreConsume(record.RequestId, 101, 8, &ratio, false))
	require.NoError(t, db.First(&sub, sub.Id).Error)
	require.Equal(t, int64(85_850_000), sub.CostAccumulator)

	require.NoError(t, FinalizeSubscriptionPreConsume(record.RequestId, 201, 31, &ratio, false))
	require.NoError(t, db.First(&sub, sub.Id).Error)
	require.Equal(t, int64(170_850_000), sub.CostAccumulator)
	require.Equal(t, int64(171), subscriptionCostQuota(sub.CostAccumulator))

	require.NoError(t, db.Where("request_id = ?", record.RequestId).First(&record).Error)
	require.Equal(t, int64(201), record.FinalSaleQuota)
	require.Equal(t, 31, record.ChannelId)
	require.Equal(t, SubscriptionCostStatusFinal, record.Status)
	require.NotNil(t, record.ChannelCostRatioPPM)
	require.Equal(t, ratio, *record.ChannelCostRatioPPM)
}

func TestFinalizeSubscriptionPreConsumeMissingRatioStaysReserved(t *testing.T) {
	db := setupSubscriptionChannelCostTestDB(t)
	sub := UserSubscription{
		UserId:     102,
		Status:     "active",
		EndTime:    common.GetTimestamp() + 3600,
		AmountUsed: 10,
	}
	require.NoError(t, db.Create(&sub).Error)
	record := SubscriptionPreConsumeRecord{
		RequestId:          "req-missing-channel-ratio",
		UserId:             sub.UserId,
		UserSubscriptionId: sub.Id,
		PreConsumed:        10,
		Status:             SubscriptionCostStatusReserved,
	}
	require.NoError(t, db.Create(&record).Error)

	err := SettleSubscriptionPreConsume(record.RequestId, 2, 12, 8, nil, false)
	require.ErrorIs(t, err, ErrChannelCostRatioMissing)
	require.NoError(t, db.First(&sub, sub.Id).Error)
	require.Zero(t, sub.CostAccumulator)
	require.Equal(t, int64(12), sub.AmountUsed)
	require.NoError(t, db.First(&record, record.Id).Error)
	require.Equal(t, SubscriptionCostStatusReserved, record.Status)
	require.Equal(t, int64(12), record.FinalSaleQuota)
	require.Equal(t, 8, record.ChannelId)
}

func TestZeroFinalSaleClearsCostWithoutConfiguredRatio(t *testing.T) {
	db := setupSubscriptionChannelCostTestDB(t)
	ratio := int64(900_000)
	sub := UserSubscription{
		UserId:          104,
		Status:          "active",
		EndTime:         common.GetTimestamp() + 3600,
		AmountUsed:      50,
		CostAccumulator: 50 * ratio,
	}
	require.NoError(t, db.Create(&sub).Error)
	record := SubscriptionPreConsumeRecord{
		RequestId:           "req-zero-final-cost",
		UserId:              sub.UserId,
		UserSubscriptionId:  sub.Id,
		PreConsumed:         50,
		FinalSaleQuota:      50,
		ChannelId:           8,
		ChannelCostRatioPPM: &ratio,
		CostNumerator:       50 * ratio,
		Status:              SubscriptionCostStatusProvisional,
	}
	require.NoError(t, db.Create(&record).Error)

	require.NoError(t, SettleSubscriptionPreConsume(record.RequestId, -50, 0, 8, nil, false))
	require.NoError(t, db.First(&sub, sub.Id).Error)
	require.Zero(t, sub.AmountUsed)
	require.Zero(t, sub.CostAccumulator)
	require.NoError(t, db.First(&record, record.Id).Error)
	require.Equal(t, SubscriptionCostStatusFinal, record.Status)
	require.Zero(t, record.FinalSaleQuota)
	require.Zero(t, record.CostNumerator)
}

func TestCleanupSubscriptionPreConsumeRecordsKeepsPendingRows(t *testing.T) {
	db := setupSubscriptionChannelCostTestDB(t)
	old := common.GetTimestamp() - 10*24*3600
	for i, status := range []string{
		SubscriptionCostStatusReserved,
		SubscriptionCostStatusProvisional,
		SubscriptionCostStatusFinal,
		SubscriptionCostStatusRefunded,
	} {
		record := SubscriptionPreConsumeRecord{
			RequestId: fmt.Sprintf("cleanup-%d", i),
			Status:    status,
			CreatedAt: old,
			UpdatedAt: old,
		}
		require.NoError(t, db.Create(&record).Error)
		require.NoError(t, db.Model(&record).Update("updated_at", old).Error)
	}

	deleted, err := CleanupSubscriptionPreConsumeRecords(7 * 24 * 3600)
	require.NoError(t, err)
	require.Equal(t, int64(2), deleted)
	var statuses []string
	require.NoError(t, db.Model(&SubscriptionPreConsumeRecord{}).Order("id").Pluck("status", &statuses).Error)
	require.Equal(t, []string{SubscriptionCostStatusReserved, SubscriptionCostStatusProvisional}, statuses)
}

func TestAdminDeleteSubscriptionVoidsCostWithoutDividend(t *testing.T) {
	db := setupSubscriptionChannelCostTestDB(t)
	sub := UserSubscription{
		UserId:           103,
		Status:           "expired",
		PaidRevenueQuota: 1000,
		CostAccumulator:  100 * ChannelCostRatioScale,
		DividendState:    SubscriptionDividendPending,
	}
	require.NoError(t, db.Create(&sub).Error)
	require.NoError(t, db.Create(&SubscriptionPreConsumeRecord{
		RequestId:          "req-deleted-subscription",
		UserId:             sub.UserId,
		UserSubscriptionId: sub.Id,
		Status:             SubscriptionCostStatusFinal,
	}).Error)

	_, err := AdminDeleteUserSubscription(sub.Id)
	require.NoError(t, err)
	require.ErrorIs(t, db.First(&UserSubscription{}, sub.Id).Error, gorm.ErrRecordNotFound)
	var requestCount int64
	require.NoError(t, db.Model(&SubscriptionPreConsumeRecord{}).
		Where("user_subscription_id = ?", sub.Id).Count(&requestCount).Error)
	require.Zero(t, requestCount)
	var dividendCount int64
	require.NoError(t, db.Model(&DividendRecord{}).
		Where("source_ref = ?", fmt.Sprintf("sub-end-%d", sub.Id)).Count(&dividendCount).Error)
	require.Zero(t, dividendCount)
}

func TestSubscriptionDividendNeverPaysNegativeProfit(t *testing.T) {
	db := setupSubscriptionChannelCostTestDB(t)
	buyer := User{
		Username: "subscription-loss-buyer",
		Role:     common.RoleCommonUser,
		AffCode:  "loss01",
	}
	require.NoError(t, db.Create(&buyer).Error)
	sub := UserSubscription{
		UserId:           buyer.Id,
		Status:           "expired",
		EndTime:          common.GetTimestamp() - 3600,
		PaidRevenueQuota: 100,
		CostAccumulator:  150 * ChannelCostRatioScale,
		DividendState:    SubscriptionDividendPending,
	}
	require.NoError(t, db.Create(&sub).Error)

	SettleSubscriptionEndDividend(buyer.Id, sub.Id)
	require.NoError(t, db.First(&sub, sub.Id).Error)
	require.Equal(t, SubscriptionDividendSkippedNoProfit, sub.DividendState)
	var count int64
	require.NoError(t, db.Model(&DividendRecord{}).
		Where("source_ref = ?", fmt.Sprintf("sub-end-%d", sub.Id)).Count(&count).Error)
	require.Zero(t, count)
}

func TestSubscriptionDividendWaitsForProvisionalCost(t *testing.T) {
	db := setupSubscriptionChannelCostTestDB(t)
	buyer := User{
		Username: "subscription-pending-buyer",
		Role:     common.RoleCommonUser,
		AffCode:  "pending01",
	}
	require.NoError(t, db.Create(&buyer).Error)
	sub := UserSubscription{
		UserId:           buyer.Id,
		Status:           "expired",
		EndTime:          common.GetTimestamp() - 3600,
		PaidRevenueQuota: 1000,
		DividendState:    SubscriptionDividendPending,
	}
	require.NoError(t, db.Create(&sub).Error)
	require.NoError(t, db.Create(&SubscriptionPreConsumeRecord{
		RequestId:          "req-provisional-cost",
		UserId:             buyer.Id,
		UserSubscriptionId: sub.Id,
		Status:             SubscriptionCostStatusProvisional,
	}).Error)

	SettleSubscriptionEndDividend(buyer.Id, sub.Id)
	require.NoError(t, db.First(&sub, sub.Id).Error)
	require.Equal(t, SubscriptionDividendPending, sub.DividendState)
}

func TestSubscriptionDividendCreditsAndRecordsExactlyOnce(t *testing.T) {
	db := setupSubscriptionChannelCostTestDB(t)
	root := User{
		Username: "subscription-root",
		Role:     common.RoleRootUser,
		AffCode:  "root01",
	}
	inviter := User{
		Username: "subscription-inviter",
		Role:     common.RoleCommonUser,
		AffCode:  "invite01",
	}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&inviter).Error)
	buyer := User{
		Username:  "subscription-profit-buyer",
		Role:      common.RoleCommonUser,
		AffCode:   "profit01",
		InviterId: inviter.Id,
	}
	require.NoError(t, db.Create(&buyer).Error)
	sub := UserSubscription{
		UserId:           buyer.Id,
		Status:           "expired",
		EndTime:          common.GetTimestamp() - 3600,
		PaidRevenueQuota: 1000,
		CostAccumulator:  200 * ChannelCostRatioScale,
		DividendState:    SubscriptionDividendPending,
	}
	require.NoError(t, db.Create(&sub).Error)

	SettleSubscriptionEndDividend(buyer.Id, sub.Id)
	SettleSubscriptionEndDividend(buyer.Id, sub.Id)

	require.NoError(t, db.First(&sub, sub.Id).Error)
	require.Equal(t, SubscriptionDividendDone, sub.DividendState)
	var records []DividendRecord
	require.NoError(t, db.Where("source_ref = ?", fmt.Sprintf("sub-end-%d", sub.Id)).
		Order("id").Find(&records).Error)
	require.Len(t, records, 2)

	var gotInviter User
	var gotRoot User
	require.NoError(t, db.First(&gotInviter, inviter.Id).Error)
	require.NoError(t, db.First(&gotRoot, root.Id).Error)
	expectedProfit := int64(800)
	require.Equal(t, int(expectedProfit*5/100), gotInviter.GiftQuota)
	require.Equal(t, int(expectedProfit*5/100), gotRoot.DividendBalance)
	require.Equal(t, gotRoot.DividendBalance, gotRoot.DividendTotal)
}

func TestChannelCostNumeratorRejectsOverflow(t *testing.T) {
	_, err := channelCostNumerator(1<<62, MaxChannelCostRatioPPM)
	require.Error(t, err)
}
