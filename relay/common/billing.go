package common

import "github.com/gin-gonic/gin"

// BillingSettler 抽象计费会话的生命周期操作。
// 由 service.BillingSession 实现，存储在 RelayInfo 上以避免循环引用。
type BillingSettler interface {
	// Settle 根据实际消耗额度进行结算，计算 delta = actualQuota - preConsumedQuota，
	// 同时调整资金来源（钱包/订阅）和令牌额度。
	Settle(actualQuota int) error

	// Refund 退还所有预扣费额度（资金来源 + 令牌），幂等安全。
	// 通过 gopool 异步执行。如果已经结算或退款则不做任何操作。
	Refund(c *gin.Context)

	// NeedsRefund 返回会话是否存在需要退还的预扣状态（未结算且未退款）。
	NeedsRefund() bool

	// GetPreConsumedQuota 返回实际预扣的额度值（信任用户可能为 0）。
	GetPreConsumedQuota() int

	// GetPaidSplit 返回本会话实际扣减的 (赠金, 本金)，供记账填 Log.PaidQuota/PaidGiftQuota(阶段2b)。
	// 必须在 Settle 之后调用。订阅资金来源不分池，返回 (0, 实际消费本金)。
	GetPaidSplit() (gift, principal int)

	// Reserve 将预扣额度补到目标值；若目标值不高于当前预扣额度则不做任何事。
	Reserve(targetQuota int) error
}
