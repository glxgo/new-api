package controller

import (
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetProfitSummary 超管利润看板: 汇总指定时间区间的消费/成本/已结算毛利/各项分润/净利。
// 时间范围: query start/end(秒时间戳), 默认本月(本地时区)。
//
// 数据来源:
//   - 总消费/总成本: logs 全量(type=consume, 含未结算)
//   - 已结算毛利/分润/净利: affiliate_settles + dividend_records(已 T+1 发放, 有滞后)
//
// 注: 全站消费成本是「实时」的, 毛利分润是「T+1 已结算」的, 二者口径不同(净利基于已结算)。
func GetProfitSummary(c *gin.Context) {
	start, end := parseProfitTimeRange(c)

	// 1. 全站消费/成本(logs 全量)
	var consume struct {
		TotalQuota int64
		TotalCost  int64
	}
	model.LOG_DB.Table("logs").
		Select("COALESCE(SUM(quota),0) AS total_quota, COALESCE(SUM(cost),0) AS total_cost").
		Where("type = ? AND created_at >= ? AND created_at < ?", model.LogTypeConsume, start, end).
		Scan(&consume)

	// 2. 已结算总毛利(affiliate_settles 已完成批次)
	var settledGross int64
	model.DB.Table("affiliate_settles").
		Select("COALESCE(SUM(total_gross),0)").
		Where("status = ? AND created_at >= ? AND created_at < ?", model.AffiliateSettleStatusDone, start, end).
		Scan(&settledGross)

	// 3. 各项分润(dividend_records 按 type 汇总)
	type typeAmt struct {
		Type   int
		Amount int64
	}
	var amts []typeAmt
	model.DB.Table("dividend_records").
		Select("type, COALESCE(SUM(amount),0) AS amount").
		Where("created_at >= ? AND created_at < ?", start, end).
		Group("type").Scan(&amts)
	var rebate, adminDiv, rootDiv int64
	for _, a := range amts {
		switch a.Type {
		case model.DividendTypeDirect, model.DividendTypeIndirect:
			rebate += a.Amount
		case model.DividendTypeAdmin:
			adminDiv += a.Amount
		case model.DividendTypeRoot:
			rootDiv += a.Amount
		}
	}

	netProfit := settledGross - rebate - adminDiv - rootDiv

	common.ApiSuccess(c, gin.H{
		"start":            start,
		"end":              end,
		"total_consume":    consume.TotalQuota, // 全站消费(quota)
		"total_cost":       consume.TotalCost,  // 全站成本(quota)
		"settled_gross":    settledGross,       // 已结算总毛利
		"affiliate_rebate": rebate,             // 已发拉新返利
		"admin_dividend":   adminDiv,           // 已发管理员分红
		"root_dividend":    rootDiv,            // 已发超管分红
		"net_profit":       netProfit,          // 净利润 = 毛利 - 拉新 - 管理员 - 超管
	})
}

// parseProfitTimeRange 解析看板时间范围: query start/end(秒), 默认本月。
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

// GetDividendRecords 超管/管理员查看分润明细(审计)。支持按 user_id/source_user_id/type 过滤。
func GetDividendRecords(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	tx := model.DB.Table("dividend_records")
	if v := c.Query("user_id"); v != "" {
		if uid, err := strconv.Atoi(v); err == nil {
			tx = tx.Where("user_id = ?", uid)
		}
	}
	if v := c.Query("source_user_id"); v != "" {
		if uid, err := strconv.Atoi(v); err == nil {
			tx = tx.Where("source_user_id = ?", uid)
		}
	}
	if v := c.Query("type"); v != "" {
		if t, err := strconv.Atoi(v); err == nil {
			tx = tx.Where("type = ?", t)
		}
	}
	var total int64
	tx.Count(&total)
	var records []model.DividendRecord
	tx.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records)
	common.ApiSuccess(c, gin.H{"data": records, "total": total})
}
