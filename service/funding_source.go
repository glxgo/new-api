package service

import (
	"time"

	"github.com/QuantumNous/new-api/model"
)

// ---------------------------------------------------------------------------
// FundingSource — 资金来源接口（钱包 or 订阅）
// ---------------------------------------------------------------------------

// FundingSource 抽象了预扣费的资金来源。
type FundingSource interface {
	// Source 返回资金来源标识："wallet" 或 "subscription"
	Source() string
	// PreConsume 从该资金来源预扣 amount 额度
	PreConsume(amount int) error
	// Settle 根据差额调整资金来源（正数补扣，负数退还）
	Settle(delta int) error
	// Refund 退还所有预扣费
	Refund() error
}

// ---------------------------------------------------------------------------
// WalletFunding — 钱包资金来源实现
// ---------------------------------------------------------------------------

type WalletFunding struct {
	userId            int
	consumedGift      int // 实际扣减的赠金额度(退款需原样退回赠金池)
	consumedPrincipal int // 实际扣减的本金额度(退款需原样退回本金池)
}

// GetPaidSplit 返回本资金来源在本计费会话中实际扣减的 (赠金, 本金)。
// 供记账处填 Log.PaidQuota/PaidGiftQuota(阶段2b)。
func (w *WalletFunding) GetPaidSplit() (gift, principal int) {
	return w.consumedGift, w.consumedPrincipal
}

// consumed 返回两池合计(兼容判断, 如 needsRefund)。
func (w *WalletFunding) consumed() int {
	return w.consumedGift + w.consumedPrincipal
}

func (w *WalletFunding) Source() string { return BillingSourceWallet }

func (w *WalletFunding) PreConsume(amount int) error {
	if amount <= 0 {
		return nil
	}
	paidGift, paidPrincipal, err := DecreaseUserQuotaDual(w.userId, amount)
	if err != nil {
		return err
	}
	// PreConsume 是覆盖写(本会话首次预扣), 覆盖之前为零值。
	w.consumedGift = paidGift
	w.consumedPrincipal = paidPrincipal
	return nil
}

func (w *WalletFunding) Settle(delta int) error {
	if delta == 0 {
		return nil
	}
	if delta > 0 {
		// 补扣: 双池拆分累加。
		paidGift, paidPrincipal, err := DecreaseUserQuotaDual(w.userId, delta)
		if err != nil {
			return err
		}
		w.consumedGift += paidGift
		w.consumedPrincipal += paidPrincipal
		return nil
	}
	// delta < 0: 退还多预扣的部分。按本次会话净消费的 gift:principal 比例退,
	// 保证退款拆分与消费日志 PaidGift/PaidPrincipal 比例一致(T+1 对账才平)。
	refund := -delta
	net := w.consumedGift + w.consumedPrincipal
	if net <= 0 {
		// 没有可退记录(如 trusted 旁路且未补扣): 容错退本金。
		if err := model.IncreaseUserQuota(w.userId, refund, false); err != nil {
			return err
		}
		return nil
	}
	// refundGift 向下取整, 余数给 principal, 且不超过已扣赠金(防 consumedGift 成负)。
	refundGift := refund * w.consumedGift / net
	if refundGift > w.consumedGift {
		refundGift = w.consumedGift
	}
	refundPrincipal := refund - refundGift
	if refundPrincipal > w.consumedPrincipal {
		refundPrincipal = w.consumedPrincipal
	}
	if err := RefundDualToPools(w.userId, refundGift, refundPrincipal); err != nil {
		return err
	}
	w.consumedGift -= refundGift
	w.consumedPrincipal -= refundPrincipal
	return nil
}

func (w *WalletFunding) Refund() error {
	if w.consumed() <= 0 {
		return nil
	}
	// 各退全量。非幂等(依赖 billing_session.go 的 refunded 标志保证只调一次)。
	gift, principal := w.consumedGift, w.consumedPrincipal
	w.consumedGift = 0
	w.consumedPrincipal = 0
	return RefundDualToPools(w.userId, gift, principal)
}

// ---------------------------------------------------------------------------
// SubscriptionFunding — 订阅资金来源实现
// ---------------------------------------------------------------------------

type SubscriptionFunding struct {
	requestId      string
	userId         int
	modelName      string
	amount         int64 // 预扣的订阅额度（subConsume）
	subscriptionId int
	preConsumed    int64
	// 以下字段在 PreConsume 成功后填充，供 RelayInfo 同步使用
	AmountTotal     int64
	AmountUsedAfter int64
	PlanId          int
	PlanTitle       string
}

func (s *SubscriptionFunding) Source() string { return BillingSourceSubscription }

func (s *SubscriptionFunding) PreConsume(_ int) error {
	// amount 参数被忽略，使用内部 s.amount（已在构造时根据 preConsumedQuota 计算）
	res, err := model.PreConsumeUserSubscription(s.requestId, s.userId, s.modelName, 0, s.amount)
	if err != nil {
		return err
	}
	s.subscriptionId = res.UserSubscriptionId
	s.preConsumed = res.PreConsumed
	s.AmountTotal = res.AmountTotal
	s.AmountUsedAfter = res.AmountUsedAfter
	// 获取订阅计划信息
	if planInfo, err := model.GetSubscriptionPlanInfoByUserSubscriptionId(res.UserSubscriptionId); err == nil && planInfo != nil {
		s.PlanId = planInfo.PlanId
		s.PlanTitle = planInfo.PlanTitle
	}
	return nil
}

func (s *SubscriptionFunding) Settle(delta int) error {
	if delta == 0 {
		return nil
	}
	return model.PostConsumeUserSubscriptionDelta(s.subscriptionId, int64(delta))
}

func (s *SubscriptionFunding) Refund() error {
	if s.preConsumed <= 0 {
		return nil
	}
	return refundWithRetry(func() error {
		return model.RefundSubscriptionPreConsume(s.requestId)
	})
}

// refundWithRetry 尝试多次执行退款操作以提高成功率，只能用于基于事务的退款函数！！！！！！
// try to refund with retries, only for refund functions based on transactions!!!
func refundWithRetry(fn func() error) error {
	if fn == nil {
		return nil
	}
	const maxAttempts = 3
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i < maxAttempts-1 {
			time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
		}
	}
	return lastErr
}
