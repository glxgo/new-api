package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/shopspring/decimal"
)

// SettleOrderDividend 按上层已计算好的可分润基数，一次性分润给推荐人/管理员/超管。
// 当前唯一调用方是订阅到期结算：base = 套餐实际利润(售价 - 渠道成本, quota 单位)。
// 它与 API 消费分润(T+1 按 gross)独立，使用 OrderXxxRate 比例。
// 幂等: sourceRef(如订单号)去重, 防 webhook 重放/兑换码重试重复发放。
// 兑换码兑换订阅不调本函数(用户要求兑换码不计入分润)。
// 放 model 包(model/subscription.go 直接调), 避免 model→service 循环依赖。
func SettleOrderDividend(buyerUserId int, orderQuota int64, sourceRef string) {
	if orderQuota <= 0 || buyerUserId <= 0 {
		return
	}
	if sourceRef != "" {
		if exists, err := HasDividendRecordBySourceRef(sourceRef); err != nil {
			common.SysError("SettleOrderDividend idempotency check failed: " + err.Error())
			return
		} else if exists {
			common.SysLog(fmt.Sprintf("SettleOrderDividend skip: sourceRef=%s already settled", sourceRef))
			return
		}
	}

	buyer, err := GetUserById(buyerUserId, false)
	if err != nil || buyer == nil {
		return
	}
	if buyer.Role >= common.RoleRootUser {
		return
	}

	// 内联 affiliate snapshot(同包直接读, 不依赖 service.GetAffiliateSnapshot)
	affAdminId := buyer.AffAdminId
	inviterId := buyer.InviterId
	inviter2Id := 0
	if inviterId > 0 {
		if inviter, e := GetUserById(inviterId, false); e == nil && inviter != nil {
			inviter2Id = inviter.InviterId
		}
	}
	root := GetRootUser()

	dIndirect := decimal.NewFromFloat(common.OrderAffiliateIndirectRate)
	dRoot := decimal.NewFromFloat(common.OrderRootDividendRate)
	dMaxDiv := decimal.NewFromFloat(common.MaxOrderDividendRate())
	dAdminDirect := decimal.NewFromFloat(common.OrderAffiliateAdminDirectRate)
	dAdminIndirect := decimal.NewFromFloat(common.OrderAffiliateAdminIndirectRate)
	dBase := decimal.NewFromInt(orderQuota)

	accumGift := map[int]int{}
	accumDividend := map[int]int{}
	var records []*DividendRecord
	now := common.GetTimestamp()
	batchId := "order-" + sourceRef

	getUser := func(id int) *User {
		if id == 0 {
			return nil
		}
		u, e := GetUserById(id, false)
		if e != nil || u == nil {
			return nil
		}
		return u
	}

	// 直接上级(普通用户才发返利)
	if inv := getUser(inviterId); inv != nil && inv.Role < common.RoleAdminUser {
		dDirect := decimal.NewFromFloat(common.OrderAffiliateDirectRateForRole(inv.Role))
		if amt := int(dBase.Mul(dDirect).Round(0).IntPart()); amt > 0 {
			if common.AffiliateRewardIsWithdrawable(inv.Role) {
				accumDividend[inv.Id] += amt
			} else {
				accumGift[inv.Id] += amt
			}
			records = append(records, &DividendRecord{BatchId: batchId, UserId: inv.Id, SourceUserId: buyerUserId, Type: DividendTypeDirect, GrossProfit: int(orderQuota), Amount: amt, SourceRef: sourceRef, CreatedAt: now})
		}
	}
	// 间接上级(普通用户)
	if inv2 := getUser(inviter2Id); inv2 != nil && inv2.Role < common.RoleAdminUser {
		if amt := int(dBase.Mul(dIndirect).Round(0).IntPart()); amt > 0 {
			if common.AffiliateRewardIsWithdrawable(inv2.Role) {
				accumDividend[inv2.Id] += amt
			} else {
				accumGift[inv2.Id] += amt
			}
			records = append(records, &DividendRecord{BatchId: batchId, UserId: inv2.Id, SourceUserId: buyerUserId, Type: DividendTypeIndirect, GrossProfit: int(orderQuota), Amount: amt, SourceRef: sourceRef, CreatedAt: now})
		}
	}
	// 管理员分红(树顶管理员, 按层级距离选比例, 上限 MaxOrderDividendRate)
	if admin := getUser(affAdminId); admin != nil && admin.Role >= common.RoleAdminUser {
		var rate decimal.Decimal
		if inviterId == admin.Id {
			rate = dAdminDirect
			if rate.GreaterThan(dMaxDiv) {
				rate = dMaxDiv
			}
		} else {
			rate = dAdminIndirect
		}
		if rate.GreaterThan(decimal.Zero) {
			if amt := int(dBase.Mul(rate).Round(0).IntPart()); amt > 0 {
				accumDividend[admin.Id] += amt
				records = append(records, &DividendRecord{BatchId: batchId, UserId: admin.Id, SourceUserId: buyerUserId, Type: DividendTypeAdmin, GrossProfit: int(orderQuota), Amount: amt, SourceRef: sourceRef, CreatedAt: now})
			}
		}
	}
	// 超管分红(所有订单金额 × OrderRootDividendRate)
	if root != nil {
		if amt := int(dBase.Mul(dRoot).Round(0).IntPart()); amt > 0 {
			accumDividend[root.Id] += amt
			records = append(records, &DividendRecord{BatchId: batchId, UserId: root.Id, SourceUserId: buyerUserId, Type: DividendTypeRoot, GrossProfit: int(orderQuota), Amount: amt, SourceRef: sourceRef, CreatedAt: now})
		}
	}

	if len(records) == 0 {
		return
	}
	// 先写明细(含 source_ref 幂等), 再 post-commit 发放(沿用 RunDailySettle 崩溃只漏发不多发范式)
	if err := BatchInsertDividendRecords(records); err != nil {
		common.SysError("SettleOrderDividend insert records failed: " + err.Error())
		return
	}
	for uid, amt := range accumGift {
		if err := IncreaseUserGiftQuota(uid, amt, true); err != nil {
			common.SysError(fmt.Sprintf("SettleOrderDividend increase gift failed uid=%d: %v", uid, err))
		}
	}
	for uid, amt := range accumDividend {
		if err := IncreaseUserDividend(uid, amt); err != nil {
			common.SysError(fmt.Sprintf("SettleOrderDividend increase dividend failed uid=%d: %v", uid, err))
		}
	}
}

// SettleSubscriptionEndDividend 套餐到期/失效时按实际利润分润(v2 简化版)。
//
//	利润 = 套餐售价 − 成本
//	  套餐售价 = plan.PriceAmount × QuotaPerUnit (用户实付金额)
//	  成本     = 用户消费全部售价 × 分组成本倍率(GroupCostRatio)
//
// 依据(2026-07-14): 售价=官方价(套餐专用分组 GR=1 平价), log.quota 即官方售价;
// log.quota 由分段公式算出已含缓存折扣(cache 自动包含), 无需 ModelCost/单独算 cache。
// 不参与: Source=redeem(兑换码) / buyer.Role>=root(超管)。
// GroupCostRatio 未配(default) → 整个套餐不分润。profit<=0 → 不分润。
// 幂等: sourceRef=sub-end-{subId}。
func SettleSubscriptionEndDividend(buyerUserId, subId int) {
	if buyerUserId <= 0 || subId <= 0 {
		return
	}
	sourceRef := fmt.Sprintf("sub-end-%d", subId)
	if exists, err := HasDividendRecordBySourceRef(sourceRef); err != nil {
		common.SysError("SettleSubscriptionEndDividend idempotency check failed: " + err.Error())
		return
	} else if exists {
		return
	}

	var sub UserSubscription
	if err := DB.Where("id = ?", subId).First(&sub).Error; err != nil {
		common.SysError(fmt.Sprintf("SettleSubscriptionEndDividend sub %d not found: %v", subId, err))
		return
	}

	// 兑换码兑换不分润
	if strings.TrimSpace(sub.Source) == "redeem" {
		common.SysLog(fmt.Sprintf("SettleSubscriptionEndDividend skip: subId=%d source=redeem(兑换码不分润)", subId))
		return
	}

	// 超管不分润
	buyer, err := GetUserById(buyerUserId, false)
	if err != nil || buyer == nil {
		return
	}
	if buyer.Role >= common.RoleRootUser {
		return
	}

	// 分组成本倍率: 未配(default) → 整个套餐不分润
	group := strings.TrimSpace(sub.AllowedGroup)
	if group == "" {
		group = strings.TrimSpace(sub.UpgradeGroup)
	}
	gcr, costSource := ratio_setting.GetGroupCostRatioWithSource(group)
	if costSource == ratio_setting.CostRatioSourceDefault {
		common.SysError(fmt.Sprintf("SettleSubscriptionEndDividend skip: subId=%d group=%s GroupCostRatio未配置→整个套餐不分润", subId, group))
		return
	}
	// 守护(方案A): 套餐分组销售倍率(GroupRatio)必须=1(平价)。
	// 成本 = log.quota × GCR, 而 log.quota 含销售倍率; 倍率=1 时 log.quota=官方价口径才正确。
	// 若被改成≠1(加价卖), 成本会放大→利润算错, 故直接拦下告警, 不分润。
	if gr := ratio_setting.GetGroupRatio(group); gr != 1 {
		common.SysError(fmt.Sprintf("SettleSubscriptionEndDividend skip: subId=%d group=%s GroupRatio=%.4f≠1, 套餐分组必须平价(否则成本口径错误)→整个套餐不分润", subId, group, gr))
		return
	}

	// 套餐售价(用户实付)
	plan, err := GetSubscriptionPlanById(sub.PlanId)
	if err != nil || plan == nil {
		return
	}
	priceQuota, err := calcSubscriptionBalanceQuota(plan.PriceAmount)
	if err != nil || priceQuota <= 0 {
		return
	}

	// 成本 = 用户消费全部售价 × GCR
	totalSaleQuota, err := GetSubscriptionConsumedQuota(subId)
	if err != nil {
		common.SysError(fmt.Sprintf("SettleSubscriptionEndDividend get consumed quota failed subId=%d: %v", subId, err))
		return
	}
	costQuota := decimal.NewFromInt(totalSaleQuota).Mul(decimal.NewFromFloat(gcr)).Round(0).IntPart()

	profit := int64(priceQuota) - costQuota
	if profit <= 0 {
		common.SysLog(fmt.Sprintf("SettleSubscriptionEndDividend skip: subId=%d profit=%d<=0 (price=%d cost=%d saleQuota=%d gcr=%.4f)",
			subId, profit, priceQuota, costQuota, totalSaleQuota, gcr))
		return
	}

	common.SysLog(fmt.Sprintf("SettleSubscriptionEndDividend settle: subId=%d price=%d saleQuota=%d gcr=%.4f cost=%d profit=%d",
		subId, priceQuota, totalSaleQuota, gcr, costQuota, profit))
	SettleOrderDividend(buyerUserId, profit, sourceRef)
}

// GetSubscriptionConsumedQuota 返回某订阅所有消费日志的售价额度合计(quota)。
// 用于套餐结束分润成本反推: 成本 = 该合计 × GroupCostRatio。
// log.quota 已含缓存折扣(分段公式算), 无需单独处理 cache。
func GetSubscriptionConsumedQuota(subId int) (int64, error) {
	var sum int64
	err := LOG_DB.Model(&Log{}).
		Where("type = ? AND billing_source = ? AND JSON_EXTRACT(other, '$.subscription_id') = ?",
			LogTypeConsume, "subscription", subId).
		Select("COALESCE(SUM(quota),0)").Scan(&sum).Error
	return sum, err
}
