package model

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCNYCommissionBaseUsesPaymentPriceSnapshotConversion(t *testing.T) {
	oldPrice, oldQuota := operation_setting.Price, common.QuotaPerUnit
	operation_setting.Price, common.QuotaPerUnit = 7.3, 500_000
	t.Cleanup(func() { operation_setting.Price, common.QuotaPerUnit = oldPrice, oldQuota })
	base, err := CNYCentsToCommissionBaseQuota(7_300) // ¥73 = $10 at the configured sale price.
	require.NoError(t, err)
	require.EqualValues(t, 5_000_000, base)
	require.Equal(t, 250_000, rechargeCommissionAmount(base, rechargeCommissionOrdinaryDirectBP))
}

func newRechargeCommissionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:recharge-commission-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &RechargeCredit{}, &DividendRecord{}))
	return db
}

func TestRecordRechargeCreditSettlesFixedRechargeRatesAtomically(t *testing.T) {
	db := newRechargeCommissionTestDB(t)
	oldDB, oldRedis, oldQuotaPerUnit := DB, common.RedisEnabled, common.QuotaPerUnit
	DB, common.RedisEnabled, common.QuotaPerUnit = db, false, 100
	t.Cleanup(func() {
		DB, common.RedisEnabled, common.QuotaPerUnit = oldDB, oldRedis, oldQuotaPerUnit
	})

	root := User{Username: "root", Role: common.RoleRootUser, AffCode: "root-fixed"}
	admin := User{Username: "admin", Role: common.RoleAdminUser, AffCode: "admin-fixed"}
	indirect := User{Username: "indirect", Role: common.RoleCommonUser, AffCode: "indirect-fixed"}
	direct := User{Username: "direct", Role: common.RoleCommonUser, AffCode: "direct-fixed"}
	for _, user := range []*User{&root, &admin} {
		require.NoError(t, db.Create(user).Error)
	}
	indirect.InviterId, indirect.AffAdminId = admin.Id, admin.Id
	require.NoError(t, db.Create(&indirect).Error)
	direct.InviterId, direct.AffAdminId = indirect.Id, admin.Id
	require.NoError(t, db.Create(&direct).Error)
	buyer := User{
		Username: "buyer", Role: common.RoleCommonUser, AffCode: "buyer-fixed",
		InviterId: direct.Id, AffAdminId: admin.Id,
	}
	require.NoError(t, db.Create(&buyer).Error)

	err := db.Transaction(func(tx *gorm.DB) error {
		created, err := RecordPaidRechargeCreditTx(tx, buyer.Id, 10_000, 10_000, "CNY", RechargeSourceWalletTopUp, "cash-001", 1_800_000_000)
		require.True(t, created)
		return err
	})
	require.NoError(t, err)

	assertUser := func(id int, gift, dividend int) {
		var user User
		require.NoError(t, db.First(&user, id).Error)
		require.Equal(t, gift, user.GiftQuota)
		require.Equal(t, dividend, user.DividendBalance)
		require.Equal(t, dividend, user.DividendTotal)
	}
	// 10,000 cents with QuotaPerUnit=100 is a 10,000 quota settlement base.
	assertUser(direct.Id, 500, 0)   // ordinary direct: 5%
	assertUser(indirect.Id, 200, 0) // ordinary second level: 2%
	assertUser(admin.Id, 0, 0)      // owning admin is third level here: no commission
	assertUser(root.Id, 0, 500)     // root always: 5%

	var credit RechargeCredit
	require.NoError(t, db.Where("source_type = ? AND source_ref = ?", RechargeSourceWalletTopUp, "cash-001").First(&credit).Error)
	require.Equal(t, RechargeCommissionDone, credit.CommissionState)
	require.Equal(t, RechargeCommissionPolicyV1, credit.CommissionPolicyVersion)
	require.NotZero(t, credit.CommissionSettledAt)

	var records []DividendRecord
	require.NoError(t, db.Where("source_ref = ?", RechargeCommissionSourceRef(RechargeSourceWalletTopUp, "cash-001")).Order("type, user_id").Find(&records).Error)
	require.Len(t, records, 3)
	for _, record := range records {
		require.Zero(t, record.GrossProfit, "new commission records must not claim a profit base")
		require.EqualValues(t, 10_000, record.SourceRechargeCents)
		require.Equal(t, RechargeCommissionPolicyV1, record.PolicyVersion)
	}

	// Callback replay must neither duplicate audit rows nor move balances again.
	err = db.Transaction(func(tx *gorm.DB) error {
		created, err := RecordPaidRechargeCreditTx(tx, buyer.Id, 10_000, 10_000, "CNY", RechargeSourceWalletTopUp, "cash-001", 1_800_000_000)
		require.False(t, created)
		return err
	})
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&DividendRecord{}).Where("source_ref = ?", RechargeCommissionSourceRef(RechargeSourceWalletTopUp, "cash-001")).Count(&count).Error)
	require.EqualValues(t, 3, count)
	assertUser(direct.Id, 500, 0)
}

func TestRechargeCommissionUsesAgentAndAdminDirectRates(t *testing.T) {
	db := newRechargeCommissionTestDB(t)
	oldDB, oldRedis, oldQuotaPerUnit := DB, common.RedisEnabled, common.QuotaPerUnit
	DB, common.RedisEnabled, common.QuotaPerUnit = db, false, 100
	t.Cleanup(func() {
		DB, common.RedisEnabled, common.QuotaPerUnit = oldDB, oldRedis, oldQuotaPerUnit
	})

	root := User{Username: "root", Role: common.RoleRootUser, AffCode: "root-rates"}
	admin := User{Username: "admin", Role: common.RoleAdminUser, AffCode: "admin-rates"}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&admin).Error)

	// An administrator who directly invited the payer receives 15%.
	adminBuyer := User{Username: "admin-buyer", Role: common.RoleCommonUser, AffCode: "admin-buyer", InviterId: admin.Id, AffAdminId: admin.Id}
	require.NoError(t, db.Create(&adminBuyer).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		_, err := RecordPaidRechargeCreditTx(tx, adminBuyer.Id, 10_000, 10_000, "CNY", RechargeSourceSubscription, "sub-001", 1_800_000_001)
		return err
	}))

	// Agents receive 8% from direct payers and 4% from second-level payers.
	agent := User{Username: "agent", Role: common.RoleAgentUser, AffCode: "agent-rates", InviterId: admin.Id, AffAdminId: admin.Id}
	require.NoError(t, db.Create(&agent).Error)
	agentDirect := User{Username: "agent-direct", Role: common.RoleCommonUser, AffCode: "agent-direct", InviterId: agent.Id, AffAdminId: admin.Id}
	require.NoError(t, db.Create(&agentDirect).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		_, err := RecordPaidRechargeCreditTx(tx, agentDirect.Id, 10_000, 10_000, "CNY", RechargeSourceVirtualMembership, "vm-001", 1_800_000_002)
		return err
	}))
	agentSecond := User{Username: "agent-second", Role: common.RoleCommonUser, AffCode: "agent-second", InviterId: agentDirect.Id, AffAdminId: admin.Id}
	require.NoError(t, db.Create(&agentSecond).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		_, err := RecordPaidRechargeCreditTx(tx, agentSecond.Id, 10_000, 10_000, "CNY", RechargeSourceWalletTopUp, "cash-002", 1_800_000_003)
		return err
	}))

	var refreshedAdmin, refreshedAgent User
	require.NoError(t, db.First(&refreshedAdmin, admin.Id).Error)
	require.NoError(t, db.First(&refreshedAgent, agent.Id).Error)
	require.Equal(t, 2_000, refreshedAdmin.DividendBalance, "15% direct plus one 5% second-level payment; third level pays nothing")
	require.Equal(t, 1_200, refreshedAgent.DividendBalance, "8% direct plus 4% second level")
	require.Zero(t, refreshedAgent.GiftQuota)
}

func TestAdministratorRechargeCreditNeverPaysCashCommission(t *testing.T) {
	db := newRechargeCommissionTestDB(t)
	oldDB, oldRedis, oldQuotaPerUnit := DB, common.RedisEnabled, common.QuotaPerUnit
	DB, common.RedisEnabled, common.QuotaPerUnit = db, false, 100
	t.Cleanup(func() {
		DB, common.RedisEnabled, common.QuotaPerUnit = oldDB, oldRedis, oldQuotaPerUnit
	})

	root := User{Username: "root", Role: common.RoleRootUser, AffCode: "root-admin-skip"}
	inviter := User{Username: "inviter", Role: common.RoleCommonUser, AffCode: "inviter-admin-skip"}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&inviter).Error)
	buyer := User{Username: "buyer", Role: common.RoleCommonUser, AffCode: "buyer-admin-skip", InviterId: inviter.Id}
	require.NoError(t, db.Create(&buyer).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		_, err := RecordRechargeCreditTx(tx, buyer.Id, 10_000, RechargeSourceAdmin, "grant-001", 1_800_000_004)
		return err
	}))
	var credit RechargeCredit
	require.NoError(t, db.Where("source_type = ? AND source_ref = ?", RechargeSourceAdmin, "grant-001").First(&credit).Error)
	require.Equal(t, RechargeCommissionSkippedSource, credit.CommissionState)
	var recordCount int64
	require.NoError(t, db.Model(&DividendRecord{}).Count(&recordCount).Error)
	require.Zero(t, recordCount)
	require.NoError(t, db.First(&buyer, buyer.Id).Error)
	require.Zero(t, buyer.RechargeTotalCents, "an administrator grant is not cumulative recharge")
}

func TestRootDirectInvitationReceivesOnlyFixedFivePercent(t *testing.T) {
	db := newRechargeCommissionTestDB(t)
	oldDB, oldRedis, oldQuotaPerUnit := DB, common.RedisEnabled, common.QuotaPerUnit
	DB, common.RedisEnabled, common.QuotaPerUnit = db, false, 100
	t.Cleanup(func() {
		DB, common.RedisEnabled, common.QuotaPerUnit = oldDB, oldRedis, oldQuotaPerUnit
	})
	root := User{Username: "root", Role: common.RoleRootUser, AffCode: "root-only-five"}
	require.NoError(t, db.Create(&root).Error)
	buyer := User{
		Username: "root-direct-buyer", Role: common.RoleCommonUser, AffCode: "root-direct-buyer",
		InviterId: root.Id, AffAdminId: root.Id,
	}
	require.NoError(t, db.Create(&buyer).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		_, err := RecordPaidRechargeCreditTx(tx, buyer.Id, 10_000, 10_000, "CNY", RechargeSourceWalletTopUp, "root-direct", 1_800_000_005)
		return err
	}))
	var refreshed RootUserAliasForRechargeTest
	require.NoError(t, db.Table("users").Select("id, dividend_balance, dividend_total").Where("id = ?", root.Id).Scan(&refreshed).Error)
	require.Equal(t, 500, refreshed.DividendBalance)
	require.Equal(t, 500, refreshed.DividendTotal)
	var records []DividendRecord
	require.NoError(t, db.Where("source_ref = ?", RechargeCommissionSourceRef(RechargeSourceWalletTopUp, "root-direct")).Find(&records).Error)
	require.Len(t, records, 1)
	require.Equal(t, DividendTypeRoot, records[0].Type)
}

type RootUserAliasForRechargeTest struct {
	Id              int
	DividendBalance int
	DividendTotal   int
}

func TestRechargeCommissionMigrationClosesOnlyUnsettledLegacyProfitQueue(t *testing.T) {
	db := newRechargeCommissionTestDB(t)
	require.NoError(t, db.AutoMigrate(&Log{}, &AffiliateSettle{}, &Option{}))
	oldDB, oldLogDB, oldMaster := DB, LOG_DB, common.IsMasterNode
	DB, LOG_DB, common.IsMasterNode = db, db, true
	t.Cleanup(func() {
		DB, LOG_DB, common.IsMasterNode = oldDB, oldLogDB, oldMaster
	})
	legacyPending := Log{UserId: 1, Type: LogTypeConsume, Settled: false, CreatedAt: 10}
	historicalDone := Log{UserId: 1, Type: LogTypeConsume, Settled: true, SettleBatchId: "2026-08-01", CreatedAt: 11}
	require.NoError(t, db.Create(&legacyPending).Error)
	require.NoError(t, db.Create(&historicalDone).Error)
	require.NoError(t, db.Create(&AffiliateSettle{BatchId: "running", Status: AffiliateSettleStatusRunning}).Error)
	require.NoError(t, db.Create(&AffiliateSettle{BatchId: "done", Status: AffiliateSettleStatusDone, TotalGross: 123}).Error)

	require.NoError(t, MigrateRechargeCommissionPolicyV1())
	require.NoError(t, MigrateRechargeCommissionPolicyV1())
	require.NoError(t, db.First(&legacyPending, legacyPending.Id).Error)
	require.False(t, legacyPending.Settled, "unmapped consumption must not be disguised as settled")
	require.Empty(t, legacyPending.SettleBatchId)
	require.Empty(t, legacyPending.ProfitReconciliationStatus, "cutover must not rewrite the legacy log table")
	require.Empty(t, legacyPending.ProfitReconciliationReason)
	pendingCount, err := CountPendingLegacyProfitReconciliations()
	require.NoError(t, err)
	require.EqualValues(t, 1, pendingCount)

	lateLegacy := Log{UserId: 1, Type: LogTypeConsume, Settled: false, CreatedAt: 12}
	require.NoError(t, db.Create(&lateLegacy).Error)
	pendingCount, err = CountPendingLegacyProfitReconciliations()
	require.NoError(t, err)
	require.EqualValues(t, 2, pendingCount, "old-version logs written during blue-green drain remain auditable")
	require.NoError(t, db.First(&historicalDone, historicalDone.Id).Error)
	require.Equal(t, "2026-08-01", historicalDone.SettleBatchId)
	var running, done AffiliateSettle
	require.NoError(t, db.Where("batch_id = ?", "running").First(&running).Error)
	require.NoError(t, db.Where("batch_id = ?", "done").First(&done).Error)
	require.Equal(t, AffiliateSettleStatusFailed, running.Status)
	require.Equal(t, AffiliateSettleStatusDone, done.Status)
	require.Equal(t, 123, done.TotalGross, "settled historical profit must remain immutable")
}

func TestRecordConsumeLogKeepsNewRequestsOutOfLegacyProfitQueue(t *testing.T) {
	db := newRechargeCommissionTestDB(t)
	require.NoError(t, db.AutoMigrate(&Log{}))
	oldDB, oldLogDB := DB, LOG_DB
	oldEnabled, oldExport := common.LogConsumeEnabled, common.DataExportEnabled
	DB, LOG_DB = db, db
	common.LogConsumeEnabled, common.DataExportEnabled = true, false
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		common.LogConsumeEnabled, common.DataExportEnabled = oldEnabled, oldExport
	})

	user := User{Username: "new-policy-log", Password: "hashed-password", AffCode: "new-policy-log"}
	require.NoError(t, db.Create(&user).Error)
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Set("username", user.Username)
	RecordConsumeLog(c, user.Id, RecordConsumeLogParams{ModelName: "gpt-test", Quota: 100})

	var log Log
	require.NoError(t, db.Where("user_id = ?", user.Id).First(&log).Error)
	require.True(t, log.Settled)
	require.Equal(t, rechargeCommissionLogBatchV1, log.SettleBatchId)
}

func TestRechargeCommissionMigrationPaysOnlyIdentifiablePendingEpayOrderOnce(t *testing.T) {
	db := newRechargeCommissionTestDB(t)
	require.NoError(t, db.AutoMigrate(&Log{}, &AffiliateSettle{}, &Option{}, &VirtualMembershipPlan{}, &VirtualMembershipOrder{}, &UserVirtualMembership{}))
	oldDB, oldLogDB, oldMaster, oldPrice, oldQuota := DB, LOG_DB, common.IsMasterNode, operation_setting.Price, common.QuotaPerUnit
	DB, LOG_DB, common.IsMasterNode, operation_setting.Price, common.QuotaPerUnit = db, db, true, 7.3, 500_000
	t.Cleanup(func() {
		DB, LOG_DB, common.IsMasterNode, operation_setting.Price, common.QuotaPerUnit = oldDB, oldLogDB, oldMaster, oldPrice, oldQuota
	})
	root := User{Username: "migration-root", Role: common.RoleRootUser, AffCode: "migration-root"}
	inviter := User{Username: "migration-inviter", Role: common.RoleCommonUser, AffCode: "migration-inviter"}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&inviter).Error)
	buyer := User{Username: "migration-buyer", Role: common.RoleCommonUser, AffCode: "migration-buyer", InviterId: inviter.Id}
	require.NoError(t, db.Create(&buyer).Error)
	plan := VirtualMembershipPlan{Code: "migration-plan", Title: "migration", PriceAmount: 73, DurationDays: 30, Enabled: true}
	require.NoError(t, db.Create(&plan).Error)
	order := VirtualMembershipOrder{UserId: buyer.Id, PlanId: plan.Id, GroupSize: 1, Money: 73, TradeNo: "migration-vm-order", PaymentProvider: PaymentProviderEpay, Status: VirtualMembershipOrderSuccess, DividendState: SubscriptionDividendPending, CompleteTime: 1_800_000_100}
	require.NoError(t, db.Create(&order).Error)
	ambiguous := VirtualMembershipOrder{UserId: buyer.Id, PlanId: plan.Id, GroupSize: 1, Money: 73, TradeNo: "migration-vm-unresolved", PaymentProvider: PaymentProviderEpay, Status: VirtualMembershipOrderSuccess, DividendState: SubscriptionDividendPending, CompleteTime: 1_800_000_101}
	require.NoError(t, db.Create(&ambiguous).Error)
	uniqueOrder := order.Id
	require.NoError(t, db.Create(&UserVirtualMembership{UserId: buyer.Id, PlanId: plan.Id, OrderId: order.Id, OrderUniqueId: &uniqueOrder, Status: VirtualMembershipStatusActive}).Error)
	require.NoError(t, db.Create(&RechargeCredit{UserId: buyer.Id, AmountCents: 7_300, SourceType: RechargeSourceWalletTopUp, SourceRef: order.TradeNo, CommissionState: RechargeCommissionLegacy, CreatedAt: order.CompleteTime}).Error)

	require.NoError(t, MigrateRechargeCommissionPolicyV1())
	require.NoError(t, MigrateRechargeCommissionPolicyV1())
	var gotInviter, gotRoot User
	require.NoError(t, db.First(&gotInviter, inviter.Id).Error)
	require.NoError(t, db.First(&gotRoot, root.Id).Error)
	require.Equal(t, 250_000, gotInviter.GiftQuota)
	require.Equal(t, 250_000, gotRoot.DividendBalance)
	var count int64
	require.NoError(t, db.Model(&DividendRecord{}).Where("policy_version = ?", RechargeCommissionPolicyV1).Count(&count).Error)
	require.EqualValues(t, 2, count)
	require.NoError(t, db.First(&ambiguous, ambiguous.Id).Error)
	require.Equal(t, SubscriptionDividendPending, ambiguous.DividendState, "unmapped historical payment remains visibly unresolved")
	require.NoError(t, db.Model(&RechargeCredit{}).Where("source_ref = ?", ambiguous.TradeNo).Count(&count).Error)
	require.Zero(t, count)
}
