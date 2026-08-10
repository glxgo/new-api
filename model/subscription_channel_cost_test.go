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
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:subscription-channel-cost-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &UserSubscription{}, &SubscriptionPreConsumeRecord{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })
	return db
}

func createCostObservation(t *testing.T, db *gorm.DB, requestID string) (UserSubscription, SubscriptionPreConsumeRecord) {
	t.Helper()
	sub := UserSubscription{UserId: 101, Status: "active", EndTime: common.GetTimestamp() + 3600, PaidRevenueQuota: 1000, DividendState: SubscriptionDividendPending}
	require.NoError(t, db.Create(&sub).Error)
	record := SubscriptionPreConsumeRecord{RequestId: requestID, UserId: sub.UserId, UserSubscriptionId: sub.Id, PreConsumed: 100, Status: SubscriptionCostStatusReserved}
	require.NoError(t, db.Create(&record).Error)
	return sub, record
}

func TestFinalizeSubscriptionPreConsumeRecordsOptionalCostObservation(t *testing.T) {
	db := setupSubscriptionChannelCostTestDB(t)
	sub, record := createCostObservation(t, db, "req-cost-observation")
	ratio := int64(850_000)
	require.NoError(t, FinalizeSubscriptionPreConsume(record.RequestId, 201, 31, &ratio, false))
	require.NoError(t, db.First(&sub, sub.Id).Error)
	require.Equal(t, int64(170_850_000), sub.CostAccumulator)
	require.NoError(t, db.Where("request_id = ?", record.RequestId).First(&record).Error)
	require.Equal(t, SubscriptionCostStatusFinal, record.Status)
	require.NotNil(t, record.ChannelCostRatioPPM)
}

func TestMissingChannelCostRatioNeverBlocksOrLeavesReserved(t *testing.T) {
	db := setupSubscriptionChannelCostTestDB(t)
	sub, record := createCostObservation(t, db, "req-no-cost-ratio")
	require.NoError(t, SettleSubscriptionPreConsume(record.RequestId, 12, 120, 8, nil, false))
	require.NoError(t, db.First(&sub, sub.Id).Error)
	require.Equal(t, int64(12), sub.AmountUsed)
	require.Zero(t, sub.CostAccumulator)
	require.NoError(t, db.Where("request_id = ?", record.RequestId).First(&record).Error)
	require.Equal(t, SubscriptionCostStatusFinal, record.Status)
	require.Nil(t, record.ChannelCostRatioPPM)
	require.Equal(t, int64(120), record.FinalSaleQuota)
}

func TestChannelCostNumeratorRejectsOverflow(t *testing.T) {
	_, err := channelCostNumerator(1<<62, MaxChannelCostRatioPPM)
	require.Error(t, err)
}
