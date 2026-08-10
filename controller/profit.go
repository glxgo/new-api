package controller

import (
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetProfitSummary keeps the historical route name for API compatibility, but
// now exposes a recharge-commission audit. It deliberately does not calculate
// sale price, channel cost, gross profit or platform net profit.
func GetProfitSummary(c *gin.Context) {
	start, end := parseProfitTimeRange(c)

	var paid struct {
		Amount int64
		Count  int64
	}
	model.DB.Model(&model.RechargeCredit{}).
		Select("COALESCE(SUM(amount_cents),0) AS amount, COUNT(*) AS count").
		Where("commission_state = ? AND created_at >= ? AND created_at < ?", model.RechargeCommissionDone, start, end).
		Scan(&paid)

	type typeAmount struct {
		Type   int
		Amount int64
	}
	var amounts []typeAmount
	model.DB.Model(&model.DividendRecord{}).
		Select("type, COALESCE(SUM(amount),0) AS amount").
		Where("policy_version = ? AND created_at >= ? AND created_at < ?", model.RechargeCommissionPolicyV1, start, end).
		Group("type").Scan(&amounts)
	var affiliate, admin, root int64
	for _, item := range amounts {
		switch item.Type {
		case model.DividendTypeDirect, model.DividendTypeIndirect:
			affiliate += item.Amount
		case model.DividendTypeAdmin:
			admin += item.Amount
		case model.DividendTypeRoot:
			root += item.Amount
		}
	}

	// Historical rows remain immutable and visible as a separately labelled
	// audit total. They are never mixed into the current recharge policy.
	var legacyCommission int64
	model.DB.Model(&model.DividendRecord{}).
		Where("policy_version = 0 AND created_at >= ? AND created_at < ?", start, end).
		Select("COALESCE(SUM(amount),0)").Scan(&legacyCommission)

	var topUpReview, subscriptionReview, legacyLogReview int64
	model.DB.Model(&model.TopUp{}).Where("commission_reconciliation_status = ?", "manual_review").Count(&topUpReview)
	model.DB.Model(&model.SubscriptionOrder{}).Where("commission_reconciliation_status = ?", "manual_review").Count(&subscriptionReview)
	if count, err := model.CountPendingLegacyProfitReconciliations(); err == nil {
		legacyLogReview = count
	}

	common.ApiSuccess(c, gin.H{
		"start":                        start,
		"end":                          end,
		"paid_recharge_cents":          paid.Amount,
		"paid_order_count":             paid.Count,
		"affiliate_rebate":             affiliate,
		"admin_dividend":               admin,
		"root_dividend":                root,
		"total_commission":             affiliate + admin + root,
		"legacy_commission_paid":       legacyCommission,
		"pending_reconciliation_count": topUpReview + subscriptionReview + legacyLogReview,
	})
}

func parseProfitTimeRange(c *gin.Context) (start, end int64) {
	now := time.Now()
	start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
	end = now.Unix()
	if s := c.Query("start"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			start = v
		}
	}
	if e := c.Query("end"); e != "" {
		if v, err := strconv.ParseInt(e, 10, 64); err == nil {
			end = v
		}
	}
	return start, end
}

// GetDividendRecords returns current recharge commissions and immutable legacy
// settlement rows through the existing endpoint. policy_version makes the
// historical boundary explicit to administrators.
func GetDividendRecords(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	applyFilters := func() *gorm.DB {
		tx := model.DB.Table("dividend_records")
		if v := c.Query("source_user_id"); v != "" {
			if uid, err := strconv.Atoi(v); err == nil {
				tx = tx.Where("source_user_id = ?", uid)
			}
		}
		if v := c.Query("type"); v != "" {
			if recordType, err := strconv.Atoi(v); err == nil {
				tx = tx.Where("type = ?", recordType)
			}
		}
		return tx
	}
	const groupBy = "dividend_records.source_user_id, dividend_records.batch_id, dividend_records.source_ref, dividend_records.policy_version"
	const selectFields = "dividend_records.source_user_id, dividend_records.batch_id, dividend_records.source_ref, dividend_records.policy_version, MAX(users.username) AS source_username, MAX(dividend_records.source_recharge_cents) AS source_recharge_cents, SUM(dividend_records.amount) AS amount, COUNT(*) AS record_count, MIN(dividend_records.created_at) AS created_at"
	var total int64
	applyFilters().Joins("LEFT JOIN users ON users.id = dividend_records.source_user_id").
		Select(selectFields).Group(groupBy).Count(&total)
	var records []dividendRecordAggregate
	applyFilters().Joins("LEFT JOIN users ON users.id = dividend_records.source_user_id").
		Select(selectFields).Group(groupBy).
		Order("MIN(dividend_records.created_at) desc").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&records)
	common.ApiSuccess(c, gin.H{"data": records, "total": total})
}

type dividendRecordAggregate struct {
	SourceUserId        int    `json:"source_user_id"`
	SourceUsername      string `json:"source_username"`
	BatchId             string `json:"batch_id"`
	SourceRef           string `json:"source_ref"`
	PolicyVersion       int    `json:"policy_version"`
	SourceRechargeCents int64  `json:"source_recharge_cents"`
	Amount              int64  `json:"amount"`
	RecordCount         int    `json:"record_count"`
	CreatedAt           int64  `json:"created_at"`
}
