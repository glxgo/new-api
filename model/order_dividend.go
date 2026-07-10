package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/shopspring/decimal"
)

// SettleOrderDividend 订阅购买订单成功后, 按订单金额一次性分润给推荐人/管理员/超管。
// base = orderQuota(订单金额, quota 单位); 与消费分润(T+1 按 gross)独立, 用 OrderXxxRate 比例。
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

	dDirect := decimal.NewFromFloat(common.OrderAffiliateDirectRate)
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
		if amt := int(dBase.Mul(dDirect).Round(0).IntPart()); amt > 0 {
			accumGift[inv.Id] += amt
			records = append(records, &DividendRecord{BatchId: batchId, UserId: inv.Id, SourceUserId: buyerUserId, Type: DividendTypeDirect, GrossProfit: int(orderQuota), Amount: amt, SourceRef: sourceRef, CreatedAt: now})
		}
	}
	// 间接上级(普通用户)
	if inv2 := getUser(inviter2Id); inv2 != nil && inv2.Role < common.RoleAdminUser {
		if amt := int(dBase.Mul(dIndirect).Round(0).IntPart()); amt > 0 {
			accumGift[inv2.Id] += amt
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

// SettleSubscriptionEndDividend 套餐到期/失效时按实际利润分润。
// 利润 = priceQuota - costQuota; costQuota 从已用额度反推(amountUsed/groupRatio×costRatio)。
// 利润≤0(用户消费超购买价)不分润, 平台承担。sourceRef = sub-end-{subId}, 幂等。
// 复用 SettleOrderDividend 的 Order 比例分发(base=profit)。
func SettleSubscriptionEndDividend(buyerUserId, subId int, priceQuota, amountUsed int64, group string) {
	if buyerUserId <= 0 || priceQuota <= 0 {
		return
	}
	sourceRef := fmt.Sprintf("sub-end-%d", subId)
	// 幂等: 同一 subId 已结算则跳过
	if exists, err := HasDividendRecordBySourceRef(sourceRef); err != nil {
		common.SysError("SettleSubscriptionEndDividend idempotency check failed: " + err.Error())
		return
	} else if exists {
		return
	}
	costQuota := calcSubscriptionCostFromAmountUsed(amountUsed, group)
	profit := priceQuota - costQuota
	if profit <= 0 {
		common.SysLog(fmt.Sprintf("SettleSubscriptionEndDividend skip: subId=%d profit=%d<=0 (price=%d cost=%d)", subId, profit, priceQuota, costQuota))
		return
	}
	// 复用 SettleOrderDividend 按 profit 分发(Order 比例)
	SettleOrderDividend(buyerUserId, profit, sourceRef)
}

// calcSubscriptionCostFromAmountUsed 从已用额度(售价口径)反推平台成本(quota)。
// cost = amountUsed / GroupRatio[group] × GroupCostRatio[group]; AllowedGroup(group恒定)精确。
func calcSubscriptionCostFromAmountUsed(amountUsed int64, group string) int64 {
	if amountUsed <= 0 || group == "" {
		return 0
	}
	groupRatio := ratio_setting.GetGroupRatio(group)
	if groupRatio <= 0 {
		groupRatio = 1
	}
	costRatio := ratio_setting.GetGroupCostRatio(group)
	dAmount := decimal.NewFromInt(amountUsed)
	dGroup := decimal.NewFromFloat(groupRatio)
	dCost := decimal.NewFromFloat(costRatio)
	return dAmount.Div(dGroup).Mul(dCost).Round(0).IntPart()
}
