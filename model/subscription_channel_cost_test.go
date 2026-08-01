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
		&Token{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&SubscriptionPreConsumeRecord{},
		&TokenSubscriptionBindingHistory{},
		&DividendRecord{},
	))
	oldDB := DB
	DB = db
	t.Cleanup(func() {
		DB = oldDB
	})
	return db
}

func useWalletDividendTestRates(t *testing.T) {
	t.Helper()
	oldDirect := common.AffiliateDirectRate
	oldIndirect := common.AffiliateIndirectRate
	oldAgentDirect := common.AgentAffiliateDirectRate
	oldRoot := common.RootDividendRate
	oldAdminDirect := common.AffiliateAdminDirectRate
	oldAdminIndirect := common.AffiliateAdminIndirectRate
	common.AffiliateDirectRate = 0.10
	common.AffiliateIndirectRate = 0.05
	common.AgentAffiliateDirectRate = 0.20
	common.RootDividendRate = 0.15
	common.AffiliateAdminDirectRate = 0.40
	common.AffiliateAdminIndirectRate = 0.22
	t.Cleanup(func() {
		common.AffiliateDirectRate = oldDirect
		common.AffiliateIndirectRate = oldIndirect
		common.AgentAffiliateDirectRate = oldAgentDirect
		common.RootDividendRate = oldRoot
		common.AffiliateAdminDirectRate = oldAdminDirect
		common.AffiliateAdminIndirectRate = oldAdminIndirect
	})
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

func TestAdminDeleteSubscriptionWithCostRecordIsBlocked(t *testing.T) {
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
	require.Error(t, err)
	require.NoError(t, db.First(&UserSubscription{}, sub.Id).Error)
	var requestCount int64
	require.NoError(t, db.Model(&SubscriptionPreConsumeRecord{}).
		Where("user_subscription_id = ?", sub.Id).Count(&requestCount).Error)
	require.Equal(t, int64(1), requestCount)
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
	useWalletDividendTestRates(t)
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
	require.Equal(t, int(expectedProfit*10/100), gotInviter.GiftQuota)
	require.Equal(t, int(expectedProfit*15/100), gotRoot.DividendBalance)
	require.Equal(t, gotRoot.DividendBalance, gotRoot.DividendTotal)
}

func TestSubscriptionDividendUsesWalletReferralAndAdminIndirectRates(t *testing.T) {
	db := setupSubscriptionChannelCostTestDB(t)
	useWalletDividendTestRates(t)

	root := User{Username: "wallet-rate-root", Role: common.RoleRootUser, AffCode: "wrr01"}
	admin := User{Username: "wallet-rate-admin", Role: common.RoleAdminUser, AffCode: "wra01"}
	indirect := User{Username: "wallet-rate-indirect", Role: common.RoleCommonUser, AffCode: "wri01"}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&admin).Error)
	require.NoError(t, db.Create(&indirect).Error)
	direct := User{
		Username:  "wallet-rate-direct",
		Role:      common.RoleCommonUser,
		AffCode:   "wrd01",
		InviterId: indirect.Id,
	}
	require.NoError(t, db.Create(&direct).Error)
	buyer := User{
		Username:   "wallet-rate-buyer",
		Role:       common.RoleCommonUser,
		AffCode:    "wrb01",
		InviterId:  direct.Id,
		AffAdminId: admin.Id,
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

	var gotDirect, gotIndirect, gotAdmin, gotRoot User
	require.NoError(t, db.First(&gotDirect, direct.Id).Error)
	require.NoError(t, db.First(&gotIndirect, indirect.Id).Error)
	require.NoError(t, db.First(&gotAdmin, admin.Id).Error)
	require.NoError(t, db.First(&gotRoot, root.Id).Error)
	require.Equal(t, 80, gotDirect.GiftQuota)
	require.Equal(t, 40, gotIndirect.GiftQuota)
	require.Equal(t, 176, gotAdmin.DividendBalance)
	require.Equal(t, 120, gotRoot.DividendBalance)

	var records []DividendRecord
	require.NoError(t, db.Where("source_ref = ?", fmt.Sprintf("sub-end-%d", sub.Id)).Find(&records).Error)
	require.Len(t, records, 4)
}

func TestSubscriptionDividendUsesWalletAgentAndAdminDirectRates(t *testing.T) {
	db := setupSubscriptionChannelCostTestDB(t)
	useWalletDividendTestRates(t)

	root := User{Username: "wallet-special-root", Role: common.RoleRootUser, AffCode: "wsr01"}
	admin := User{Username: "wallet-special-admin", Role: common.RoleAdminUser, AffCode: "wsa01"}
	agent := User{Username: "wallet-special-agent", Role: common.RoleAgentUser, AffCode: "wsg01"}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&admin).Error)
	require.NoError(t, db.Create(&agent).Error)

	agentBuyer := User{
		Username:   "wallet-agent-buyer",
		Role:       common.RoleCommonUser,
		AffCode:    "wab01",
		InviterId:  agent.Id,
		AffAdminId: admin.Id,
	}
	require.NoError(t, db.Create(&agentBuyer).Error)
	agentSub := UserSubscription{
		UserId:           agentBuyer.Id,
		Status:           "expired",
		EndTime:          common.GetTimestamp() - 3600,
		PaidRevenueQuota: 1000,
		DividendState:    SubscriptionDividendPending,
	}
	require.NoError(t, db.Create(&agentSub).Error)
	SettleSubscriptionEndDividend(agentBuyer.Id, agentSub.Id)

	adminBuyer := User{
		Username:   "wallet-admin-buyer",
		Role:       common.RoleCommonUser,
		AffCode:    "wdb01",
		InviterId:  admin.Id,
		AffAdminId: admin.Id,
	}
	require.NoError(t, db.Create(&adminBuyer).Error)
	adminSub := UserSubscription{
		UserId:           adminBuyer.Id,
		Status:           "expired",
		EndTime:          common.GetTimestamp() - 3600,
		PaidRevenueQuota: 1000,
		DividendState:    SubscriptionDividendPending,
	}
	require.NoError(t, db.Create(&adminSub).Error)
	SettleSubscriptionEndDividend(adminBuyer.Id, adminSub.Id)

	var gotAgent, gotAdmin, gotRoot User
	require.NoError(t, db.First(&gotAgent, agent.Id).Error)
	require.NoError(t, db.First(&gotAdmin, admin.Id).Error)
	require.NoError(t, db.First(&gotRoot, root.Id).Error)
	require.Equal(t, 200, gotAgent.DividendBalance)
	require.Zero(t, gotAgent.GiftQuota)
	require.Equal(t, 620, gotAdmin.DividendBalance)
	require.Equal(t, 300, gotRoot.DividendBalance)
}

func TestChannelCostNumeratorRejectsOverflow(t *testing.T) {
	_, err := channelCostNumerator(1<<62, MaxChannelCostRatioPPM)
	require.Error(t, err)
}
