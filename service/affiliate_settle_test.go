package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDailySettleAggregatesOneInviteeDayBeforeCalculatingRebate(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:daily-affiliate-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Log{}, &model.DividendRecord{},
		&model.AffiliateSettle{}, &model.RechargeCredit{},
	))
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldRedis := common.RedisEnabled
	oldDirect, oldIndirect := common.AffiliateDirectRate, common.AffiliateIndirectRate
	oldRoot := common.RootDividendRate
	oldAdminDirect, oldAdminIndirect := common.AffiliateAdminDirectRate, common.AffiliateAdminIndirectRate
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled = false
	common.AffiliateDirectRate = 0.10
	common.AffiliateIndirectRate = 0
	common.RootDividendRate = 0
	common.AffiliateAdminDirectRate = 0
	common.AffiliateAdminIndirectRate = 0
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.RedisEnabled = oldRedis
		common.AffiliateDirectRate, common.AffiliateIndirectRate = oldDirect, oldIndirect
		common.RootDividendRate = oldRoot
		common.AffiliateAdminDirectRate, common.AffiliateAdminIndirectRate = oldAdminDirect, oldAdminIndirect
	})

	root := model.User{Username: "root", Role: common.RoleRootUser, AffCode: "daily-root"}
	inviter := model.User{Username: "inviter", Role: common.RoleCommonUser, AffCode: "daily-inv"}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&inviter).Error)
	invitee := model.User{
		Username: "invitee", Role: common.RoleCommonUser, AffCode: "daily-source",
		InviterId: inviter.Id,
	}
	require.NoError(t, db.Create(&invitee).Error)

	dayStart := int64(1_800_000_000)
	dayEnd := dayStart + 86_400
	for i := 0; i < 2; i++ {
		require.NoError(t, db.Create(&model.Log{
			UserId: invitee.Id, Type: model.LogTypeConsume, CreatedAt: dayStart + int64(i+1),
			Quota: 6, PaidQuota: 6, Cost: 1, BillingSource: "wallet",
			InviterIdSnap: inviter.Id,
		}).Error)
	}
	require.NoError(t, db.Create(&model.RechargeCredit{
		UserId: invitee.Id, AmountCents: 2_500, SourceType: "topup",
		SourceRef: "daily-recharge", CreatedAt: dayStart + 10,
	}).Error)

	require.NoError(t, RunDailySettle("daily-aggregate-test", dayStart, dayEnd))

	var records []model.DividendRecord
	require.NoError(t, db.Where("user_id = ? AND type = ?", inviter.Id, model.DividendTypeDirect).Find(&records).Error)
	require.Len(t, records, 1)
	require.Equal(t, 10, records[0].GrossProfit)
	require.Equal(t, 1, records[0].Amount, "应先合并 10 毛利再按 10% 计算，而不是逐笔四舍五入为 2")
	require.Equal(t, 12, records[0].SourceUsage)
	require.EqualValues(t, 2_500, records[0].SourceRechargeCents)
	require.Equal(t, 2, records[0].RequestCount)

	var storedInviter model.User
	require.NoError(t, db.First(&storedInviter, inviter.Id).Error)
	require.Equal(t, 1, storedInviter.GiftQuota)
	summaries, err := model.GetAffiliateSourceSummaries(inviter.Id, []int{invitee.Id})
	require.NoError(t, err)
	require.EqualValues(t, 2_500, summaries[invitee.Id].RechargeCents)
	require.EqualValues(t, 12, summaries[invitee.Id].Usage)
	require.EqualValues(t, 10, summaries[invitee.Id].GrossProfit)
	require.EqualValues(t, 1, summaries[invitee.Id].Rebate)

	require.NoError(t, RunDailySettle("daily-aggregate-test", dayStart, dayEnd))
	require.NoError(t, db.First(&storedInviter, inviter.Id).Error)
	require.Equal(t, 1, storedInviter.GiftQuota, "重跑已完成批次不得重复入账")
}
