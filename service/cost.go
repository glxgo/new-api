package service

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/shopspring/decimal"
)

// ---------------------------------------------------------------------------
// Cost observations and dual-pool billing helpers. Cost data is analytics only
// and never participates in the fixed recharge commission policy.
// ---------------------------------------------------------------------------

// CalcCostFromSaleQuota 从【售价扣费】反推平台成本(quota 单位)。
// 业务(2026-06-22 重构, plan mellow-growing-waterfall.md): 成本与售价同源 ——
//
//	售价 = 官方基础扣费 × GroupRatio[group]
//	成本 = 官方基础扣费 × GroupCostRatio[group] = (售价 / GroupRatio) × GroupCostRatio
//
// 这样成本自动覆盖与售价完全相同的 token 口径(含 cache/image/audio/tool 附加费等),
// 后台预览与实际结算不会漂移。旧 ModelCost/GroupModelCost 不再参与新日志成本;
// 历史 log.Cost 快照不重算。
//
//   - saleQuota       本次请求实际售价扣费(quota)
//   - saleGroupRatio  售价所用分组倍率(GroupRatio[实际 UsingGroup], 新规则下不再恒 1)
//   - group           实际 UsingGroup(查 GroupCostRatio; 未配置则继承 GroupRatio, 再未配置则 1)
func CalcCostFromSaleQuota(saleQuota int, saleGroupRatio float64, group string) int {
	if saleQuota <= 0 || saleGroupRatio <= 0 {
		return 0
	}
	dBase := decimal.NewFromInt(int64(saleQuota)).Div(decimal.NewFromFloat(saleGroupRatio))
	dCostRatio := decimal.NewFromFloat(ratio_setting.GetGroupCostRatio(group))
	return int(dBase.Mul(dCostRatio).Round(0).IntPart())
}

// CalcCostFromChannelRatio applies the final successful channel's fixed-point
// accounting multiplier. nil deliberately produces no fabricated cost; callers
// must surface the missing configuration and subscription settlement remains pending.
func CalcCostFromChannelRatio(saleQuota int, ratioPPM *int64) int {
	if saleQuota <= 0 || ratioPPM == nil || *ratioPPM <= 0 {
		return 0
	}
	return int(decimal.NewFromInt(int64(saleQuota)).
		Mul(decimal.NewFromInt(*ratioPPM)).
		Div(decimal.NewFromInt(model.ChannelCostRatioScale)).
		Round(0).
		IntPart())
}

// SplitPayment 按业务规则③「消费优先扣赠金、不足扣本金」拆分本次消费额度。
// 返回 (paidGift 赠金扣减量, paidPrincipal 本金扣减量), 二者之和 ≤ totalQuota。
// 用于双池扣费(阶段2b)与消费日志快照。
func SplitPayment(totalQuota, giftBalance, principalBalance int) (paidGift, paidPrincipal int) {
	if totalQuota <= 0 {
		return 0, 0
	}
	paidGift = totalQuota
	if paidGift > giftBalance {
		paidGift = giftBalance
	}
	if paidGift < 0 {
		paidGift = 0
	}
	paidPrincipal = totalQuota - paidGift
	if paidPrincipal > principalBalance {
		// 余额不足: 取剩余本金兜底(预扣阶段理论已拦截不足额, 此处防负数)
		paidPrincipal = principalBalance
	}
	if paidPrincipal < 0 {
		paidPrincipal = 0
	}
	return paidGift, paidPrincipal
}

// GetAffiliateSnapshot preserves relationship metadata in consumption logs for
// historical audit. New commission reads relationships at the paid event.
func GetAffiliateSnapshot(userId int) (affAdminId, inviterId, inviter2Id int) {
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		return 0, 0, 0
	}
	affAdminId = user.AffAdminId
	inviterId = user.InviterId
	if inviterId > 0 {
		inviter, err := model.GetUserById(inviterId, false)
		if err == nil && inviter != nil {
			inviter2Id = inviter.InviterId
		}
	}
	return affAdminId, inviterId, inviter2Id
}
