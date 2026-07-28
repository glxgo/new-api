package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// settleOrderDividendTx writes order/subscription dividend records and recipient
// balances in the caller's transaction. This keeps the audit rows and money
// movement atomic: a crash can no longer leave "recorded but not credited".
func settleOrderDividendTx(tx *gorm.DB, buyer *User, orderQuota int64, sourceRef string) ([]int, error) {
	if tx == nil {
		return nil, errors.New("dividend transaction is nil")
	}
	if buyer == nil || buyer.Id <= 0 || orderQuota <= 0 || buyer.Role >= common.RoleRootUser {
		return nil, nil
	}
	if sourceRef != "" {
		var existing int64
		if err := tx.Model(&DividendRecord{}).Where("source_ref = ?", sourceRef).Count(&existing).Error; err != nil {
			return nil, err
		}
		if existing > 0 {
			return nil, nil
		}
	}

	buyerUserId := buyer.Id
	affAdminId := buyer.AffAdminId
	inviterId := buyer.InviterId
	inviter2Id := 0
	getUser := func(id int) *User {
		if id <= 0 {
			return nil
		}
		var user User
		if err := tx.Omit("password").Where("id = ?", id).First(&user).Error; err != nil {
			return nil
		}
		return &user
	}
	if inviter := getUser(inviterId); inviter != nil {
		inviter2Id = inviter.InviterId
	}
	var root *User
	var rootUser User
	if err := tx.Omit("password").Where("role = ?", common.RoleRootUser).First(&rootUser).Error; err == nil {
		root = &rootUser
	}

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

	if len(records) > 0 {
		if err := tx.CreateInBatches(records, 500).Error; err != nil {
			return nil, err
		}
	}
	giftRecipients := make([]int, 0, len(accumGift))
	for uid, amt := range accumGift {
		if amt <= 0 {
			continue
		}
		res := tx.Model(&User{}).Where("id = ?", uid).
			Update("gift_quota", gorm.Expr("gift_quota + ?", amt))
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected != 1 {
			return nil, fmt.Errorf("dividend gift recipient %d not found", uid)
		}
		giftRecipients = append(giftRecipients, uid)
	}
	for uid, amt := range accumDividend {
		if amt <= 0 {
			continue
		}
		res := tx.Model(&User{}).Where("id = ?", uid).
			Updates(map[string]interface{}{
				"dividend_balance": gorm.Expr("dividend_balance + ?", amt),
				"dividend_total":   gorm.Expr("dividend_total + ?", amt),
			})
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected != 1 {
			return nil, fmt.Errorf("dividend recipient %d not found", uid)
		}
	}
	return giftRecipients, nil
}

func invalidateDividendGiftCaches(userIds []int) {
	for _, userId := range userIds {
		if err := invalidateUserCache(userId); err != nil {
			common.SysError(fmt.Sprintf("invalidate dividend gift cache failed uid=%d: %v", userId, err))
		}
	}
}

// SettleOrderDividend 按上层已计算好的可分润基数，一次性分润给推荐人/管理员/超管。
// 明细和余额在同一事务中提交，sourceRef 用于幂等。
func SettleOrderDividend(buyerUserId int, orderQuota int64, sourceRef string) {
	if orderQuota <= 0 || buyerUserId <= 0 {
		return
	}
	var giftRecipients []int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var buyer User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Omit("password").Where("id = ?", buyerUserId).First(&buyer).Error; err != nil {
			return err
		}
		var err error
		giftRecipients, err = settleOrderDividendTx(tx, &buyer, orderQuota, sourceRef)
		return err
	})
	if err != nil {
		common.SysError("SettleOrderDividend failed: " + err.Error())
		return
	}
	invalidateDividendGiftCaches(giftRecipients)
}

func subscriptionCostQuota(costNumerator int64) int64 {
	if costNumerator <= 0 {
		return 0
	}
	return (costNumerator-1)/ChannelCostRatioScale + 1
}

// SettleSubscriptionEndDividend settles from the subscription's immutable paid
// revenue snapshot and O(1) channel-cost accumulator. It never scans logs.
func SettleSubscriptionEndDividend(buyerUserId, subId int) {
	if buyerUserId <= 0 || subId <= 0 {
		return
	}
	sourceRef := fmt.Sprintf("sub-end-%d", subId)
	var profit int64
	var priceQuota int64
	var costQuota int64
	var giftRecipients []int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", subId).First(&sub).Error; err != nil {
			return err
		}
		if sub.UserId != buyerUserId {
			return errors.New("subscription buyer mismatch")
		}
		var buyer User
		if err := tx.Omit("password").Where("id = ?", buyerUserId).First(&buyer).Error; err != nil {
			return err
		}
		if sub.Status == "active" {
			return nil
		}
		switch sub.DividendState {
		case SubscriptionDividendDone, SubscriptionDividendSkippedNoProfit, SubscriptionDividendSkippedSource:
			return nil
		}
		var pending int64
		if err := tx.Model(&SubscriptionPreConsumeRecord{}).
			Where("user_subscription_id = ? AND status IN ?", subId, []string{
				SubscriptionCostStatusReserved,
				SubscriptionCostStatusProvisional,
			}).Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return nil
		}
		if sub.Source == "redeem" || sub.Source == "admin" || buyer.Role >= common.RoleRootUser {
			return tx.Model(&sub).Updates(map[string]interface{}{
				"dividend_state": SubscriptionDividendSkippedSource,
				"updated_at":     common.GetTimestamp(),
			}).Error
		}
		priceQuota = sub.PaidRevenueQuota
		costQuota = subscriptionCostQuota(sub.CostAccumulator)
		profit = priceQuota - costQuota
		if profit <= 0 {
			return tx.Model(&sub).Updates(map[string]interface{}{
				"dividend_state": SubscriptionDividendSkippedNoProfit,
				"updated_at":     common.GetTimestamp(),
			}).Error
		}
		if err := tx.Model(&sub).Updates(map[string]interface{}{
			"dividend_state": SubscriptionDividendProcessing,
			"updated_at":     common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		var err error
		giftRecipients, err = settleOrderDividendTx(tx, &buyer, profit, sourceRef)
		if err != nil {
			return err
		}
		return tx.Model(&sub).Updates(map[string]interface{}{
			"dividend_state": SubscriptionDividendDone,
			"updated_at":     common.GetTimestamp(),
		}).Error
	})
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			common.SysError(fmt.Sprintf("SettleSubscriptionEndDividend prepare failed subId=%d: %v", subId, err))
		}
		return
	}
	invalidateDividendGiftCaches(giftRecipients)
	if profit > 0 {
		common.SysLog(fmt.Sprintf("SettleSubscriptionEndDividend settled atomically: subId=%d price=%d cost=%d profit=%d",
			subId, priceQuota, costQuota, profit))
	}
}
