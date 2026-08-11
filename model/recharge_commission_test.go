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

func TestRechargeCommissionMigrationLeavesLegacyProfitQueueUntouched(t *testing.T) {
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
	lateLegacy := Log{UserId: 1, Type: LogTypeConsume, Settled: false, CreatedAt: 12}
	require.NoError(t, db.Create(&lateLegacy).Error)
	require.NoError(t, db.First(&lateLegacy, lateLegacy.Id).Error)
	require.False(t, lateLegacy.Settled, "old-version logs written during blue-green drain remain untouched")
	require.NoError(t, db.First(&historicalDone, historicalDone.Id).Error)
	require.Equal(t, "2026-08-01", historicalDone.SettleBatchId)
	var running, done AffiliateSettle
	require.NoError(t, db.Where("batch_id = ?", "running").First(&running).Error)
	require.NoError(t, db.Where("batch_id = ?", "done").First(&done).Error)
	require.Equal(t, AffiliateSettleStatusFailed, running.Status)
	require.Equal(t, AffiliateSettleStatusDone, done.Status)
	require.Equal(t, 123, done.TotalGross, "settled historical profit must remain immutable")
	cutoverAt, err := RechargeCommissionCutoverAt()
	require.NoError(t, err)
	require.Positive(t, cutoverAt)
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

func TestRechargeCommissionMigrationNeverBackfillsHistoricalOrders(t *testing.T) {
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
	require.Zero(t, gotInviter.GiftQuota)
	require.Zero(t, gotRoot.DividendBalance)
	var count int64
	require.NoError(t, db.Model(&DividendRecord{}).Where("policy_version = ?", RechargeCommissionPolicyV1).Count(&count).Error)
	require.Zero(t, count)
	var historicalCredit RechargeCredit
	require.NoError(t, db.Where("source_ref = ?", order.TradeNo).First(&historicalCredit).Error)
	require.Equal(t, RechargeCommissionLegacy, historicalCredit.CommissionState)
	require.Zero(t, historicalCredit.CommissionPolicyVersion)
	require.NoError(t, db.First(&order, order.Id).Error)
	require.Equal(t, SubscriptionDividendPending, order.DividendState)
	require.NoError(t, db.First(&ambiguous, ambiguous.Id).Error)
	require.Equal(t, SubscriptionDividendPending, ambiguous.DividendState)
	require.NoError(t, db.Model(&RechargeCredit{}).Where("source_ref = ?", ambiguous.TradeNo).Count(&count).Error)
	require.Zero(t, count)
}

func TestRechargeCommissionMigrationDoesNotInspectDeletedHistoricalOwners(t *testing.T) {
	db := newRechargeCommissionTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&Log{}, &AffiliateSettle{}, &Option{}, &TopUp{},
		&SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{},
		&VirtualMembershipPlan{}, &VirtualMembershipOrder{}, &UserVirtualMembership{},
	))
	oldDB, oldLogDB, oldMaster, oldPrice, oldQuota := DB, LOG_DB, common.IsMasterNode, operation_setting.Price, common.QuotaPerUnit
	DB, LOG_DB, common.IsMasterNode, operation_setting.Price, common.QuotaPerUnit = db, db, true, 7.3, 500_000
	t.Cleanup(func() {
		DB, LOG_DB, common.IsMasterNode, operation_setting.Price, common.QuotaPerUnit = oldDB, oldLogDB, oldMaster, oldPrice, oldQuota
	})

	deleted := User{Username: "deleted-payment-owner", Password: "hashed-password", AffCode: "deleted-payment-owner"}
	require.NoError(t, db.Create(&deleted).Error)

	subPlan := SubscriptionPlan{Title: "deleted-owner-sub", PriceAmount: 73, DurationValue: 30, DurationUnit: "day"}
	require.NoError(t, db.Create(&subPlan).Error)
	subOrder := SubscriptionOrder{
		UserId: deleted.Id, PlanId: subPlan.Id, Money: 73, TradeNo: "deleted-owner-sub-order",
		PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusSuccess, CompleteTime: 1_800_001_000,
	}
	require.NoError(t, db.Create(&subOrder).Error)
	subscription := UserSubscription{
		UserId: deleted.Id, PlanId: subPlan.Id, Source: "order", CreatedAt: subOrder.CompleteTime,
		Status: "active", DividendState: SubscriptionDividendPending,
	}
	require.NoError(t, db.Create(&subscription).Error)

	vmPlan := VirtualMembershipPlan{Code: "deleted-owner-vm", Title: "deleted-owner-vm", PriceAmount: 73, DurationDays: 30, Enabled: true}
	require.NoError(t, db.Create(&vmPlan).Error)
	vmOrder := VirtualMembershipOrder{
		UserId: deleted.Id, PlanId: vmPlan.Id, GroupSize: 1, Money: 73, TradeNo: "deleted-owner-vm-order",
		PaymentProvider: PaymentProviderEpay, Status: VirtualMembershipOrderSuccess,
		DividendState: SubscriptionDividendPending, CompleteTime: 1_800_001_001,
	}
	require.NoError(t, db.Create(&vmOrder).Error)
	vmOrderID := vmOrder.Id
	require.NoError(t, db.Create(&UserVirtualMembership{
		UserId: deleted.Id, PlanId: vmPlan.Id, OrderId: vmOrder.Id, OrderUniqueId: &vmOrderID,
		Status: VirtualMembershipStatusActive,
	}).Error)

	topup := TopUp{
		UserId: deleted.Id, Amount: 10, Money: 73, TradeNo: "deleted-owner-topup",
		PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusSuccess, CompleteTime: 1_800_001_002,
	}
	snapshot, err := NewPaymentSnapshotFromMoney(topup.Money, "CNY")
	require.NoError(t, err)
	require.NoError(t, SetTopUpPaymentExpectation(&topup, snapshot))
	topup.ActualPaymentAmountMinor = snapshot.AmountMinor
	topup.ActualPaymentCurrency = snapshot.Currency
	require.NoError(t, db.Create(&topup).Error)

	require.NoError(t, db.Delete(&deleted).Error)
	require.NoError(t, MigrateRechargeCommissionPolicyV1())

	require.NoError(t, db.First(&subOrder, subOrder.Id).Error)
	require.Empty(t, subOrder.CommissionReconciliationStatus)
	require.NoError(t, db.First(&subscription, subscription.Id).Error)
	require.Equal(t, SubscriptionDividendPending, subscription.DividendState)
	require.NoError(t, db.First(&vmOrder, vmOrder.Id).Error)
	require.Equal(t, SubscriptionDividendPending, vmOrder.DividendState)
	require.NoError(t, db.First(&topup, topup.Id).Error)
	require.Empty(t, topup.CommissionReconciliationStatus)
	var creditCount int64
	require.NoError(t, db.Model(&RechargeCredit{}).Where("source_ref IN ?", []string{subOrder.TradeNo, vmOrder.TradeNo, topup.TradeNo}).Count(&creditCount).Error)
	require.Zero(t, creditCount)
}

func TestRechargeCommissionCutoverRejectsPreCutoverCreditAndPaysNewCredit(t *testing.T) {
	db := newRechargeCommissionTestDB(t)
	require.NoError(t, db.AutoMigrate(&Option{}))
	oldDB, oldRedis := DB, common.RedisEnabled
	DB, common.RedisEnabled = db, false
	t.Cleanup(func() { DB, common.RedisEnabled = oldDB, oldRedis })

	require.NoError(t, db.Create(&Option{Key: rechargeCommissionCutoverAtKey, Value: "1000"}).Error)
	root := User{Username: "cutover-root", Role: common.RoleRootUser, AffCode: "cutover-root"}
	inviter := User{Username: "cutover-inviter", Role: common.RoleCommonUser, AffCode: "cutover-inviter"}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&inviter).Error)
	buyer := User{Username: "cutover-buyer", Role: common.RoleCommonUser, AffCode: "cutover-buyer", InviterId: inviter.Id}
	require.NoError(t, db.Create(&buyer).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		_, err := RecordPaidRechargeCreditTx(tx, buyer.Id, 10_000, 10_000, "CNY", RechargeSourceWalletTopUp, "before-cutover", 999)
		return err
	}))
	var before RechargeCredit
	require.NoError(t, db.Where("source_ref = ?", "before-cutover").First(&before).Error)
	require.Equal(t, RechargeCommissionLegacy, before.CommissionState)
	require.Zero(t, before.CommissionPolicyVersion)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		_, err := RecordPaidRechargeCreditTx(tx, buyer.Id, 10_000, 10_000, "CNY", RechargeSourceWalletTopUp, "at-cutover", 1000)
		return err
	}))
	var after RechargeCredit
	require.NoError(t, db.Where("source_ref = ?", "at-cutover").First(&after).Error)
	require.Equal(t, RechargeCommissionDone, after.CommissionState)
	require.Equal(t, RechargeCommissionPolicyV1, after.CommissionPolicyVersion)

	var gotRoot, gotInviter User
	require.NoError(t, db.First(&gotRoot, root.Id).Error)
	require.NoError(t, db.First(&gotInviter, inviter.Id).Error)
	require.Equal(t, 500, gotRoot.DividendBalance)
	require.Equal(t, 500, gotInviter.GiftQuota)
	var dividendCount int64
	require.NoError(t, db.Model(&DividendRecord{}).Count(&dividendCount).Error)
	require.EqualValues(t, 2, dividendCount)
}
