package service

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/shopspring/decimal"
)

// ---------------------------------------------------------------------------
// 成本 / 双池 / 毛利 / 分润快照 —— 分润系统计费侧核心计算
// 详见 plan: mellow-growing-waterfall.md 业务规则②③
// ---------------------------------------------------------------------------

// CalcCostFromSaleQuota 从【售价扣费】反推平台成本(quota 单位)。
// 业务(2026-06-22 重构, plan mellow-growing-waterfall.md): 成本与售价同源 ——
//
//	售价 = 官方基础扣费 × GroupRatio[group]
//	成本 = 官方基础扣费 × GroupCostRatio[group] = (售价 / GroupRatio) × GroupCostRatio
//
// 这样成本自动覆盖与售价完全相同的 token 口径(含 cache/image/audio/tool 附加费等),
// 后台预览与实际结算不会漂移。旧 ModelCost/GroupModelCost 不再参与新日志成本;
// 历史 log.Cost 快照不重算(T+1 结算只读快照)。
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

// CalcGrossProfit 计算单笔毛利(quota 单位)。
// 跨池分摊规则: 成本只按「本金占比」计入利润, 赠金部分对应的成本由平台自负(不进分润)。
//
//	grossProfit = paidPrincipal − cost × paidPrincipal / (paidPrincipal + paidGift)
func CalcGrossProfit(paidPrincipal, paidGift, cost int) int {
	total := paidPrincipal + paidGift
	if total <= 0 {
		return 0
	}
	dCost := decimal.NewFromInt(int64(cost))
	dPrincipal := decimal.NewFromInt(int64(paidPrincipal))
	dTotal := decimal.NewFromInt(int64(total))
	gross := dPrincipal.Sub(dCost.Mul(dPrincipal).Div(dTotal))
	return int(gross.Round(0).IntPart())
}

// GetAffiliateSnapshot 读取消费用户的分润快照。
// 这些关系在注册时已固化(见 model/user.go calcAffAdminId), 消费时直接读当前值写日志, 不事后回溯。
//   - affAdminId: 树顶管理员(管理员分红归属, 0 表示无主用户)
//   - inviterId:  直接上级(拉新返利直接率 10%; 若该级是管理员则 T+1 不发返利)
//   - inviter2Id: 间接上级/上上级(拉新返利间接率 5%)
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
