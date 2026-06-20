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
// 流程: 幂等检查 → 分批扫未结算消费日志 → 算每笔毛利(CalcGrossProfit)
//   → 按规则累加分润(拉新进赠金池 / 管理员+超管进分红账户) → 批量标记日志 settled
//   → 发放 → 写 DividendRecord 明细 → 标记批次完成。
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

	dDirect := decimal.NewFromFloat(common.AffiliateDirectRate)
	dIndirect := decimal.NewFromFloat(common.AffiliateIndirectRate)
	dRoot := decimal.NewFromFloat(common.RootDividendRate)
	dMaxDiv := decimal.NewFromFloat(common.MaxDividendRate())
	dAdminIndirect := decimal.NewFromFloat(common.AffiliateAdminIndirectRate) // 管理员间接/三层+拉新分红
	root := model.GetRootUser()

	accumGift := map[int]int{}     // userId -> 赠金累加(拉新返利)
	accumDividend := map[int]int{} // userId -> 分红累加(管理员/超管)
	var records []*model.DividendRecord
	totalGross, logCount := 0, 0

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
		gross := CalcGrossProfit(log.PaidQuota, log.PaidGiftQuota, log.Cost)
		if gross <= 0 {
			return
		}
		totalGross += gross
		logCount++
		dGross := decimal.NewFromInt(int64(gross))

		// 拉新返利 - 直接上级(普通用户才发; 上级是管理员则走分红, 不发返利)
		if inv := getUser(log.InviterIdSnap); inv != nil && inv.Role < common.RoleAdminUser {
			if amt := int(dGross.Mul(dDirect).Round(0).IntPart()); amt > 0 {
				accumGift[inv.Id] += amt
				records = append(records, &model.DividendRecord{BatchId: batchId, UserId: inv.Id, SourceUserId: log.UserId, LogId: log.Id, Type: model.DividendTypeDirect, GrossProfit: gross, Amount: amt, CreatedAt: common.GetTimestamp()})
			}
		}
		// 拉新返利 - 间接上级
		if inv2 := getUser(log.Inviter2IdSnap); inv2 != nil && inv2.Role < common.RoleAdminUser {
			if amt := int(dGross.Mul(dIndirect).Round(0).IntPart()); amt > 0 {
				accumGift[inv2.Id] += amt
				records = append(records, &model.DividendRecord{BatchId: batchId, UserId: inv2.Id, SourceUserId: log.UserId, LogId: log.Id, Type: model.DividendTypeIndirect, GrossProfit: gross, Amount: amt, CreatedAt: common.GetTimestamp()})
			}
		}
		// 管理员分红(树顶管理员, 按层级距离):
		//   - 直接上级是管理员(InviterIdSnap==admin): 用管理员个人 DividendRate(上限 MaxDividendRate)
		//   - 间接/三层+(树顶但非直接上级): 用全局 AffiliateAdminIndirectRate(默认 22%)
		if admin := getUser(log.AffAdminIdSnap); admin != nil && admin.Role >= common.RoleAdminUser {
			var rate decimal.Decimal
			if log.InviterIdSnap == admin.Id {
				rate = decimal.NewFromFloat(admin.DividendRate)
				if rate.GreaterThan(dMaxDiv) {
					rate = dMaxDiv
				}
			} else {
				rate = dAdminIndirect
			}
			if rate.GreaterThan(decimal.Zero) {
				if amt := int(dGross.Mul(rate).Round(0).IntPart()); amt > 0 {
					accumDividend[admin.Id] += amt
					records = append(records, &model.DividendRecord{BatchId: batchId, UserId: admin.Id, SourceUserId: log.UserId, LogId: log.Id, Type: model.DividendTypeAdmin, GrossProfit: gross, Amount: amt, CreatedAt: common.GetTimestamp()})
				}
			}
		}
		// 超管分红(所有毛利 × RootDividendRate)
		if root != nil {
			if amt := int(dGross.Mul(dRoot).Round(0).IntPart()); amt > 0 {
				accumDividend[root.Id] += amt
				records = append(records, &model.DividendRecord{BatchId: batchId, UserId: root.Id, SourceUserId: log.UserId, LogId: log.Id, Type: model.DividendTypeRoot, GrossProfit: gross, Amount: amt, CreatedAt: common.GetTimestamp()})
			}
		}
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

	// 3. 发放(赠金进 gift_quota, 分红进 dividend_balance + dividend_total)
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

	// 4. 写明细
	if err := model.BatchInsertDividendRecords(records); err != nil {
		logger.LogError(ctx, fmt.Sprintf("daily settle %s insert records failed: %v", batchId, err))
	}

	// 5. 标记批次完成
	if err := model.FinishAffiliateSettle(batchId, logCount, totalGross); err != nil {
		logger.LogError(ctx, fmt.Sprintf("daily settle %s finish batch failed: %v", batchId, err))
	}

	logger.LogInfo(ctx, fmt.Sprintf("daily settle %s done: logs=%d gross=%d giftUsers=%d divUsers=%d records=%d",
		batchId, logCount, totalGross, len(accumGift), len(accumDividend), len(records)))
	return nil
}
