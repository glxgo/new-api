package controller

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/commission_setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetProfitSummary keeps the historical route name for API compatibility, but
// now exposes a recharge-commission audit. It deliberately does not calculate
// sale price, channel cost, gross profit or platform net profit.
func GetProfitSummary(c *gin.Context) {
	start, end := parseProfitTimeRange(c)
	cutoverAt, _ := model.RechargeCommissionCutoverAt()
	if cutoverAt > start {
		start = cutoverAt
	}

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
		Where("policy_version >= ? AND created_at >= ? AND created_at < ?", model.RechargeCommissionPolicyV1, start, end).
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

	var topUpReview, subscriptionReview int64
	model.DB.Model(&model.TopUp{}).Where("commission_reconciliation_status = ? AND complete_time >= ?", "manual_review", cutoverAt).Count(&topUpReview)
	model.DB.Model(&model.SubscriptionOrder{}).Where("commission_reconciliation_status = ? AND complete_time >= ?", "manual_review", cutoverAt).Count(&subscriptionReview)

	common.ApiSuccess(c, gin.H{
		"start":                        start,
		"end":                          end,
		"paid_recharge_cents":          paid.Amount,
		"paid_order_count":             paid.Count,
		"affiliate_rebate":             affiliate,
		"admin_dividend":               admin,
		"root_dividend":                root,
		"total_commission":             affiliate + admin + root,
		"pending_reconciliation_count": topUpReview + subscriptionReview,
	})
}

type commissionSettingsResponse struct {
	OrdinaryDirect   int64 `json:"ordinary_direct_bp"`
	OrdinaryIndirect int64 `json:"ordinary_indirect_bp"`
	AgentDirect      int64 `json:"agent_direct_bp"`
	AgentIndirect    int64 `json:"agent_indirect_bp"`
	AdminDirect      int64 `json:"admin_direct_bp"`
	AdminIndirect    int64 `json:"admin_indirect_bp"`
	Root             int64 `json:"root_bp"`
}

func currentCommissionSettings() commissionSettingsResponse {
	values := commission_setting.Values()
	return commissionSettingsResponse{
		OrdinaryDirect: values[commission_setting.KeyOrdinaryDirect], OrdinaryIndirect: values[commission_setting.KeyOrdinaryIndirect],
		AgentDirect: values[commission_setting.KeyAgentDirect], AgentIndirect: values[commission_setting.KeyAgentIndirect],
		AdminDirect: values[commission_setting.KeyAdminDirect], AdminIndirect: values[commission_setting.KeyAdminIndirect],
		Root: values[commission_setting.KeyRoot],
	}
}

// GetCommissionSettings exposes the live future-settlement rates to the root
// administrator. Values are basis points (100 = 1%).
func GetCommissionSettings(c *gin.Context) {
	common.ApiSuccess(c, currentCommissionSettings())
}

type updateCommissionSettingsRequest struct {
	OrdinaryDirect   *int64 `json:"ordinary_direct_bp"`
	OrdinaryIndirect *int64 `json:"ordinary_indirect_bp"`
	AgentDirect      *int64 `json:"agent_direct_bp"`
	AgentIndirect    *int64 `json:"agent_indirect_bp"`
	AdminDirect      *int64 `json:"admin_direct_bp"`
	AdminIndirect    *int64 `json:"admin_indirect_bp"`
	Root             *int64 `json:"root_bp"`
}

func GetCommissionRateValue(value *int64) (string, error) {
	if value == nil {
		return "", nil
	}
	if *value < 0 || *value > 10000 {
		return "", fmt.Errorf("分润比例必须在 0%% 到 100%% 之间")
	}
	return strconv.FormatInt(*value, 10), nil
}

// UpdateCommissionSettings persists all supplied rates atomically. Existing
// settled records are immutable; the new values apply to future payments.
func UpdateCommissionSettings(c *gin.Context) {
	var req updateCommissionSettingsRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	values := map[string]string{}
	items := []struct {
		key   string
		value *int64
	}{
		{commission_setting.KeyOrdinaryDirect, req.OrdinaryDirect}, {commission_setting.KeyOrdinaryIndirect, req.OrdinaryIndirect},
		{commission_setting.KeyAgentDirect, req.AgentDirect}, {commission_setting.KeyAgentIndirect, req.AgentIndirect},
		{commission_setting.KeyAdminDirect, req.AdminDirect}, {commission_setting.KeyAdminIndirect, req.AdminIndirect},
		{commission_setting.KeyRoot, req.Root},
	}
	for _, item := range items {
		if item.value == nil {
			continue
		}
		value, err := GetCommissionRateValue(item.value)
		if err != nil {
			common.ApiErrorMsg(c, err.Error())
			return
		}
		values[item.key] = strings.TrimSpace(value)
	}
	if len(values) == 0 {
		common.ApiErrorMsg(c, "至少修改一项比例")
		return
	}
	if err := model.UpdateOptionsBulk(values); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, currentCommissionSettings())
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
		tx := model.DB.Table("dividend_records").Where("policy_version >= ?", model.RechargeCommissionPolicyV1)
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
