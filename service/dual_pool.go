package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/bytedance/gopkg/util/gopool"
)

// ---------------------------------------------------------------------------
// 双池扣费编排(阶段2b) —— 业务规则③「消费优先扣赠金、不足扣本金」
// ---------------------------------------------------------------------------
//
// DecreaseUserQuotaDual 是双池扣费的唯一业务入口。它不做拆分的原子事务(避免与
// 本金 BatchUpdate 冲突、避免 FOR UPDATE 锁竞争), 而是用「乐观扣减」:
//   1. 强制 DB 读 (giftBalance, principalBalance)
//   2. SplitPayment 算出本次该扣的 (paidGift, paidPrincipal)
//   3. gift/principal 各一条带 WHERE col >= amount 守卫的 UPDATE; RowsAffected==1 才算成功
//   4. gift 扣成功但 principal 被并发抢占 → 补偿把 gift 加回, 重试
//   5. 成功后异步 decr Redis 缓存两个 field
//
// 放在 service 层(而非 model): 拆分+守卫+补偿属于业务编排, 且要调本包 SplitPayment,
// 避免与 model 循环 import。model 层只暴露 Guarded 原语 + 赠金读函数。

// ErrInsufficientQuotaForDual 双池扣减时余额不足(gift+principal 之和小于 amount)。
// 复用 model 包已有的余额不足错误语义; 这里单独定义以便 service 层判别。
type dualInsufficientQuotaError struct {
	userId int
	amount int
}

func (e *dualInsufficientQuotaError) Error() string {
	return fmt.Sprintf("双池扣减失败: 用户 %d 余额(gift+principal)不足以扣减 %d", e.userId, e.amount)
}

// IsDualInsufficientQuotaError 判别 Dual 的余额不足错误。
func IsDualInsufficientQuotaError(err error) bool {
	_, ok := err.(*dualInsufficientQuotaError)
	return ok
}

// DecreaseUserQuotaDual 从用户双池扣减 amount, 优先赠金、不足扣本金。
// 返回本次实际扣减的 (paidGift, paidPrincipal)。余额不足时返回 dualInsufficientQuotaError。
//
// 并发安全: 每段 UPDATE 自带行级锁且带 WHERE 守卫, 单条语句级释放, 与现有裸 UPDATE
// 同级别竞争; 读余额到 UPDATE 之间的 TOCTOU 窗口由 WHERE 守卫 + RowsAffected + 重试兜底。
func DecreaseUserQuotaDual(id int, amount int) (paidGift, paidPrincipal int, err error) {
	if amount <= 0 {
		return 0, 0, nil
	}

	const maxAttempts = 3 // 初次 + 最多 2 次重试
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// 强制从 DB 读当前两池余额(缩小 TOCTOU 窗口, 不信任可能滞后的缓存)。
		giftBalance, gErr := model.GetUserGiftQuota(id, true)
		if gErr != nil {
			return 0, 0, gErr
		}
		prinBalance, pErr := model.GetUserQuota(id, true)
		if pErr != nil {
			return 0, 0, pErr
		}

		paidGift, paidPrincipal = SplitPayment(amount, giftBalance, prinBalance)
		// SplitPayment 在余额不足时会截断(paidGift+paidPrincipal < amount)。
		// 双池扣费要求"要么全额扣够, 要么不扣", 截断即视为余额不足。
		if paidGift+paidPrincipal < amount {
			// 读到的余额可能偏旧(TOCTOU), 重试一次确认。仍不足则报错, 不部分扣减。
			if attempt < maxAttempts-1 {
				continue
			}
			return 0, 0, &dualInsufficientQuotaError{userId: id, amount: amount}
		}

		// ---- 扣赠金 ----
		if paidGift > 0 {
			ok, gErr := model.DecreaseUserGiftQuotaGuarded(id, paidGift)
			if gErr != nil {
				return 0, 0, gErr
			}
			if !ok {
				// 赠金被并发抢占, 重读重试。
				paidGift = 0
				paidPrincipal = 0
				continue
			}
		}

		// ---- 扣本金 ----
		if paidPrincipal > 0 {
			ok, pErr := model.DecreaseUserQuotaGuarded(id, paidPrincipal)
			if pErr != nil {
				// 本金 UPDATE 出错。若赠金已扣需补偿回滚, 避免用户白白损失赠金。
				if paidGift > 0 {
					_ = model.IncreaseUserGiftQuota(id, paidGift, false)
				}
				return 0, 0, pErr
			}
			if !ok {
				// 本金被并发抢占。赠金已扣 → 补偿, 然后整体重试(重读后可能全赠金)。
				if paidGift > 0 {
					_ = model.IncreaseUserGiftQuota(id, paidGift, false)
				}
				paidGift = 0
				paidPrincipal = 0
				continue
			}
		}

		// 两段都成功(或其中一段 amount=0 跳过)。同步 Redis 缓存。
		if paidGift > 0 {
			gopool.Go(func() {
				if e := model.CacheDecrUserGiftQuota(id, int64(paidGift)); e != nil {
					common.SysLog(fmt.Sprintf("failed to decr gift cache (userId=%d, amount=%d): %s", id, paidGift, e.Error()))
				}
			})
		}
		if paidPrincipal > 0 {
			gopool.Go(func() {
				if e := model.CacheDecrUserQuota(id, int64(paidPrincipal)); e != nil {
					common.SysLog(fmt.Sprintf("failed to decr quota cache (userId=%d, amount=%d): %s", id, paidPrincipal, e.Error()))
				}
			})
		}
		return paidGift, paidPrincipal, nil
	}

	// 重试用尽。
	return 0, 0, &dualInsufficientQuotaError{userId: id, amount: amount}
}

// RefundDualToPools 把 (gift, principal) 按原池退还。用于 Settle 退还 / Refund 全退 / rollback。
// 各池单独 Increase, 无并发问题(增不需要守卫)。
func RefundDualToPools(id int, refundGift, refundPrincipal int) error {
	if refundGift < 0 || refundPrincipal < 0 {
		return fmt.Errorf("refund amounts must be non-negative: gift=%d principal=%d", refundGift, refundPrincipal)
	}
	if refundGift > 0 {
		if err := model.IncreaseUserGiftQuota(id, refundGift, false); err != nil {
			return err
		}
	}
	if refundPrincipal > 0 {
		if err := model.IncreaseUserQuota(id, refundPrincipal, false); err != nil {
			return err
		}
	}
	return nil
}

// paidSplitForLog 为消费日志取本次实际扣减的 (赠金, 本金)。
// 有 BillingSession 时委托 GetPaidSplit(); 无(旧路径/任务首笔无 session)时回退全本金。
// 兜底: 拆分之和与 quota 不符(理论上不应发生)时按比例修正, 保证 paidGift+paidPrincipal==quota 入账。
func paidSplitForLog(relayInfo *relaycommon.RelayInfo, quota int) (paidGift, paidPrincipal int) {
	if relayInfo != nil && relayInfo.Billing != nil {
		paidGift, paidPrincipal = relayInfo.Billing.GetPaidSplit()
	}
	sum := paidGift + paidPrincipal
	if sum <= 0 || sum != quota {
		// 回退全本金(Billing 缺失)或拆分与实际消费额不符(如按次计费 delta=0 但 Billing 记的是预扣)。
		// 以 quota 为准, 全记本金, 保证入账不丢失额度。
		return 0, quota
	}
	return paidGift, paidPrincipal
}
