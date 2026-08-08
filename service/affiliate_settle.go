package service

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/shopspring/decimal"
)

const dailySettleLogBatchSize = 1000

// RunDailySettle 执行某日的 T+1 分润结算(幂等, 可重跑)。
// batchId 如 "2026-06-16", dayStart/dayEnd 为该日 [起, 止) 时间戳(秒)。
//
// 流程: 幂等检查 → 分批扫未结算消费日志 → 按消费用户汇总整日毛利(CalcGrossProfit)
//
//	→ 每个用户每天只计算一次分润(拉新进赠金池 / 管理员+超管进分红账户) → 批量标记日志 settled
//	→ 发放 → 写 DividendRecord 明细 → 标记批次完成。
//
// 安全性: 「先标记 settled 再发放」, 崩溃只会漏发(不凭空多发), 可用 AffiliateSettle.TotalGross
// 与 DividendRecord 汇总对账补齐。重跑只处理 settled=false 的日志。
func RunDailySettle(batchId string, dayStart, dayEnd int64) error {
	ctx := context.Background()

	// 1. 幂等: 批次已完成则跳过
	existing, getErr := model.GetAffiliateSettle(batchId)
	if getErr == nil && existing != nil && existing.Status == model.AffiliateSettleStatusDone {
		logger.LogInfo(ctx, fmt.Sprintf("daily settle %s already done, skip", batchId))
		return nil
	}
	if existing == nil {
		if err := model.CreateAffiliateSettle(&model.AffiliateSettle{
			BatchId:   batchId,
			DayStart:  dayStart,
			DayEnd:    dayEnd,
			Status:    model.AffiliateSettleStatusRunning,
			CreatedAt: common.GetTimestamp(),
		}); err != nil {
			// 并发下另一节点已创建, 忽略主键冲突继续
			logger.LogInfo(ctx, fmt.Sprintf("daily settle %s batch record exists: %v", batchId, err))
		}
	}

	dIndirect := decimal.NewFromFloat(common.AffiliateIndirectRate)
	dRoot := decimal.NewFromFloat(common.RootDividendRate)
	dMaxDiv := decimal.NewFromFloat(common.MaxDividendRate())
	dAdminDirect := decimal.NewFromFloat(common.AffiliateAdminDirectRate)     // 管理员直接拉新分红
	dAdminIndirect := decimal.NewFromFloat(common.AffiliateAdminIndirectRate) // 管理员间接/三层+拉新分红
	root := model.GetRootUser()
	if unresolved, err := model.CountUnresolvedWalletCostLogs(dayStart, dayEnd); err != nil {
		return err
	} else if unresolved > 0 {
		return fmt.Errorf("daily settle %s blocked: %d logs have unresolved channel cost ratio", batchId, unresolved)
	}

	accumGift := map[int]int{}     // userId -> 普通用户赠金返利
	accumDividend := map[int]int{} // userId -> 代理可提佣金 / 管理员与超管分红
	totalGross, logCount := 0, 0
	type sourceDailyAggregate struct {
		UserId         int
		GrossProfit    int
		Usage          int
		RequestCount   int
		InviterIdSnap  int
		Inviter2IdSnap int
		AffAdminIdSnap int
	}
	sources := map[int]*sourceDailyAggregate{}

	userCache := map[int]*model.User{}
	getUser := func(id int) *model.User {
		if id == 0 {
			return nil
		}
		if u, ok := userCache[id]; ok {
			return u
		}
		u, err := model.GetUserById(id, false)
		if err != nil || u == nil {
			return nil
		}
		userCache[id] = u
		return u
	}

	processLog := func(log *model.Log) {
		// 超管自己的消费不计入分润/利润(超管使用站内 API 不产生任何分润; log 仍会被批量标记 settled)
		if u := getUser(log.UserId); u != nil && u.Role >= common.RoleRootUser {
			return
		}
		source := sources[log.UserId]
		if source == nil {
			source = &sourceDailyAggregate{
				UserId: log.UserId, InviterIdSnap: log.InviterIdSnap,
				Inviter2IdSnap: log.Inviter2IdSnap, AffAdminIdSnap: log.AffAdminIdSnap,
			}
			sources[log.UserId] = source
		} else if source.InviterIdSnap != log.InviterIdSnap ||
			source.Inviter2IdSnap != log.Inviter2IdSnap || source.AffAdminIdSnap != log.AffAdminIdSnap {
			logger.LogInfo(ctx, fmt.Sprintf("daily settle %s source user %d has changed affiliate snapshot; using first snapshot", batchId, log.UserId))
		}
		source.Usage += log.Quota
		source.RequestCount++

		gross := CalcGrossProfit(log.PaidQuota, log.PaidGiftQuota, log.Cost)
		if gross <= 0 {
			return
		}
		source.GrossProfit += gross
		totalGross += gross
		logCount++
	}

	// 2. 分批扫描未结算日志 → 算分润 → 标记 settled
	lastId := 0
	for {
		logs, err := model.GetUnsettledConsumeLogs(dayStart, dayEnd, lastId, dailySettleLogBatchSize)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("daily settle %s scan logs failed: %v", batchId, err))
			return err
		}
		if len(logs) == 0 {
			break
		}
		ids := make([]int, 0, len(logs))
		for _, log := range logs {
			processLog(log)
			lastId = log.Id
			ids = append(ids, log.Id)
		}
		// 先标记 settled(防重跑重复算), 再发放; 崩溃只会漏发不会多发
		if err := model.MarkLogsSettled(ids, batchId); err != nil {
			logger.LogError(ctx, fmt.Sprintf("daily settle %s mark settled failed: %v", batchId, err))
			return err
		}
		if len(logs) < dailySettleLogBatchSize {
			break
		}
	}

	// 3. 每个消费用户先汇总整日毛利，再计算一次返利。这样小额请求不会因
	// 逐笔四舍五入被拆成大量明细，审计行与实际结算口径保持一致。
	sourceIds := make([]int, 0, len(sources))
	for sourceUserId := range sources {
		sourceIds = append(sourceIds, sourceUserId)
	}
	rechargeByUser, err := model.GetRechargeCentsByUsersBetween(sourceIds, dayStart, dayEnd)
	if err != nil {
		return err
	}
	records := make([]*model.DividendRecord, 0, len(sources)*2)
	now := common.GetTimestamp()
	appendRecord := func(source *sourceDailyAggregate, recipientId, dividendType, amount int) {
		if source == nil || recipientId <= 0 || amount <= 0 {
			return
		}
		records = append(records, &model.DividendRecord{
			BatchId: batchId, UserId: recipientId, SourceUserId: source.UserId,
			Type: dividendType, GrossProfit: source.GrossProfit, Amount: amount,
			SourceUsage: source.Usage, SourceRechargeCents: rechargeByUser[source.UserId],
			RequestCount: source.RequestCount,
			SourceRef:    fmt.Sprintf("daily:%s:%d:%d:%d", batchId, recipientId, source.UserId, dividendType),
			CreatedAt:    now,
		})
	}
	for _, source := range sources {
		if source.GrossProfit <= 0 {
			continue
		}
		dGross := decimal.NewFromInt(int64(source.GrossProfit))
		if inv := getUser(source.InviterIdSnap); inv != nil && inv.Role < common.RoleAdminUser {
			dDirect := decimal.NewFromFloat(common.AffiliateDirectRateForRole(inv.Role))
			if amt := int(dGross.Mul(dDirect).Round(0).IntPart()); amt > 0 {
				if common.AffiliateRewardIsWithdrawable(inv.Role) {
					accumDividend[inv.Id] += amt
				} else {
					accumGift[inv.Id] += amt
				}
				appendRecord(source, inv.Id, model.DividendTypeDirect, amt)
			}
		}
		if inv2 := getUser(source.Inviter2IdSnap); inv2 != nil && inv2.Role < common.RoleAdminUser {
			if amt := int(dGross.Mul(dIndirect).Round(0).IntPart()); amt > 0 {
				if common.AffiliateRewardIsWithdrawable(inv2.Role) {
					accumDividend[inv2.Id] += amt
				} else {
					accumGift[inv2.Id] += amt
				}
				appendRecord(source, inv2.Id, model.DividendTypeIndirect, amt)
			}
		}
		if admin := getUser(source.AffAdminIdSnap); admin != nil && admin.Role >= common.RoleAdminUser {
			var rate decimal.Decimal
			if source.InviterIdSnap == admin.Id {
				rate = dAdminDirect
				if rate.GreaterThan(dMaxDiv) {
					rate = dMaxDiv
				}
			} else {
				rate = dAdminIndirect
			}
			if rate.GreaterThan(decimal.Zero) {
				if amt := int(dGross.Mul(rate).Round(0).IntPart()); amt > 0 {
					accumDividend[admin.Id] += amt
					appendRecord(source, admin.Id, model.DividendTypeAdmin, amt)
				}
			}
		}
		if root != nil {
			if amt := int(dGross.Mul(dRoot).Round(0).IntPart()); amt > 0 {
				accumDividend[root.Id] += amt
				appendRecord(source, root.Id, model.DividendTypeRoot, amt)
			}
		}
	}

	// 4. 发放(普通返利进 gift_quota; 代理佣金和管理分红进 dividend_balance + dividend_total)
	for uid, amt := range accumGift {
		if amt > 0 {
			if err := model.IncreaseUserGiftQuota(uid, amt, false); err != nil {
				logger.LogError(ctx, fmt.Sprintf("daily settle %s increase gift for user %d failed: %v", batchId, uid, err))
			}
		}
	}
	for uid, amt := range accumDividend {
		if amt > 0 {
			if err := model.IncreaseUserDividend(uid, amt); err != nil {
				logger.LogError(ctx, fmt.Sprintf("daily settle %s increase dividend for user %d failed: %v", batchId, uid, err))
			}
		}
	}

	// 5. 写按用户、按日聚合后的明细
	if err := model.BatchInsertDividendRecords(records); err != nil {
		logger.LogError(ctx, fmt.Sprintf("daily settle %s insert records failed: %v", batchId, err))
	}

	// 6. 标记批次完成
	if err := model.FinishAffiliateSettle(batchId, logCount, totalGross); err != nil {
		logger.LogError(ctx, fmt.Sprintf("daily settle %s finish batch failed: %v", batchId, err))
	}

	logger.LogInfo(ctx, fmt.Sprintf("daily settle %s done: logs=%d gross=%d giftUsers=%d divUsers=%d records=%d",
		batchId, logCount, totalGross, len(accumGift), len(accumDividend), len(records)))
	return nil
}
