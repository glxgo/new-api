package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProfitReadersIncludeAllFixedPoliciesAndExcludeLegacy(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:profit-policy-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.RechargeCredit{}, &model.DividendRecord{},
		&model.TopUp{}, &model.SubscriptionOrder{},
	))
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })
	buyer := model.User{Username: "profit-policy-buyer", Role: common.RoleCommonUser, AffCode: "profit-policy-buyer"}
	require.NoError(t, db.Create(&buyer).Error)
	require.NoError(t, db.Create(&[]model.RechargeCredit{
		{UserId: buyer.Id, AmountCents: 1_000, SourceType: model.RechargeSourceSubscription, SourceRef: "profit-v1", CreatedAt: 100, CommissionBaseQuota: 10_000, PaymentCurrency: "CNY", CommissionState: model.RechargeCommissionDone, CommissionPolicyVersion: model.RechargeCommissionPolicyV1},
		{UserId: buyer.Id, AmountCents: 2_000, SourceType: model.RechargeSourceSubscription, SourceRef: "profit-v2", CreatedAt: 200, CommissionBaseQuota: 20_000, PaymentCurrency: "CNY", CommissionState: model.RechargeCommissionDone, CommissionPolicyVersion: model.RechargeCommissionPolicyV2},
	}).Error)
	require.NoError(t, db.Create(&[]model.DividendRecord{
		{BatchId: "profit-v1", UserId: 10, SourceUserId: buyer.Id, Type: model.DividendTypeDirect, Amount: 100, SourceRechargeCents: 1_000, SourceRef: "profit-v1", PolicyVersion: model.RechargeCommissionPolicyV1, CreatedAt: 100},
		{BatchId: "profit-v2", UserId: 20, SourceUserId: buyer.Id, Type: model.DividendTypeRoot, Amount: 200, SourceRechargeCents: 2_000, SourceRef: "profit-v2", PolicyVersion: model.RechargeCommissionPolicyV2, CreatedAt: 200},
		{BatchId: "profit-legacy", UserId: 30, SourceUserId: buyer.Id, Type: model.DividendTypeAdmin, Amount: 900, SourceRechargeCents: 9_000, SourceRef: "profit-legacy", PolicyVersion: 0, CreatedAt: 50},
	}).Error)

	gin.SetMode(gin.TestMode)
	summaryRecorder := httptest.NewRecorder()
	summaryContext, _ := gin.CreateTestContext(summaryRecorder)
	summaryContext.Request = httptest.NewRequest(http.MethodGet, "/api/profit/summary?start=0&end=300", nil)
	GetProfitSummary(summaryContext)
	require.Equal(t, http.StatusOK, summaryRecorder.Code)
	var summary struct {
		Success bool `json:"success"`
		Data    struct {
			PaidRechargeCents int64 `json:"paid_recharge_cents"`
			AffiliateRebate   int64 `json:"affiliate_rebate"`
			AdminDividend     int64 `json:"admin_dividend"`
			RootDividend      int64 `json:"root_dividend"`
			TotalCommission   int64 `json:"total_commission"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(summaryRecorder.Body.Bytes(), &summary))
	require.True(t, summary.Success)
	require.EqualValues(t, 3_000, summary.Data.PaidRechargeCents)
	require.EqualValues(t, 100, summary.Data.AffiliateRebate)
	require.Zero(t, summary.Data.AdminDividend)
	require.EqualValues(t, 200, summary.Data.RootDividend)
	require.EqualValues(t, 300, summary.Data.TotalCommission)

	recordsRecorder := httptest.NewRecorder()
	recordsContext, _ := gin.CreateTestContext(recordsRecorder)
	recordsContext.Request = httptest.NewRequest(http.MethodGet, "/api/profit/dividend_records?page=1&page_size=20", nil)
	GetDividendRecords(recordsContext)
	require.Equal(t, http.StatusOK, recordsRecorder.Code)
	var records struct {
		Success bool `json:"success"`
		Data    struct {
			Data []struct {
				PolicyVersion int `json:"policy_version"`
			} `json:"data"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recordsRecorder.Body.Bytes(), &records))
	require.True(t, records.Success)
	require.EqualValues(t, 2, records.Data.Total)
	require.Len(t, records.Data.Data, 2)
	versions := map[int]bool{}
	for _, record := range records.Data.Data {
		versions[record.PolicyVersion] = true
	}
	require.Equal(t, map[int]bool{model.RechargeCommissionPolicyV1: true, model.RechargeCommissionPolicyV2: true}, versions)
}
