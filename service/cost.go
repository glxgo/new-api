package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/shopspring/decimal"
)

// ---------------------------------------------------------------------------
// 成本 / 双池 / 毛利 / 分润快照 —— 分润系统计费侧核心计算
// 详见 plan: mellow-growing-waterfall.md 业务规则②③
// ---------------------------------------------------------------------------

// CalcModelCostQuota 计算单次请求的平台成本(quota 单位), 按 billing_mode 分支:
//   - tiered_expr + CostExpr != "": 跑 CostExpr($/1M 输出) / 1e6 × QuotaPerUnit (复用 billingexpr 引擎)
//   - CostPerRequest > 0 (per-request 模式): CostPerRequest × QuotaPerUnit (按次, 不乘 token)
//   - 其余 (per-token): (InputCostPerM×prompt + OutputCostPerM×completion) / 1e6 × QuotaPerUnit
//
// 成本【不乘分组倍率 groupRatio】——成本是平台固定承担的, 与用户分组无关。
// ok=false 表示该模型未配置成本(毛利=收入)。
func CalcModelCostQuota(modelName string, promptTokens, completionTokens int) (int, bool) {
	cost, ok := ratio_setting.GetModelCost(modelName)
	if !ok {
		return 0, false
	}
	if promptTokens < 0 {
		promptTokens = 0
	}
	if completionTokens < 0 {
		completionTokens = 0
	}
	dUnit := decimal.NewFromFloat(common.QuotaPerUnit)

	// tiered_expr 模式: 跑分段成本表达式 (与售价表达式同引擎, v1 系数是 $/1M)
	if billing_setting.GetBillingMode(modelName) == billing_setting.BillingModeTieredExpr {
		if cost.CostExpr == "" {
			return 0, false // tiered_expr 模式未配 CostExpr → 无成本
		}
		params := billingexpr.TokenParams{
			P:   float64(promptTokens),
			C:   float64(completionTokens),
			Len: float64(promptTokens + completionTokens),
		}
		result, _, err := billingexpr.RunExprWithRequest(
			cost.CostExpr, params, billingexpr.RequestInput{},
		)
		if err != nil || result < 0 {
			return 0, false
		}
		costDecimal := decimal.NewFromFloat(result).
			Div(decimal.NewFromInt(1000000)).Mul(dUnit)
		return int(costDecimal.Round(0).IntPart()), true
	}

	// per-request 模式: 按次成本 (不乘 token)
	if cost.CostPerRequest > 0 {
		costDecimal := decimal.NewFromFloat(cost.CostPerRequest).Mul(dUnit)
		return int(costDecimal.Round(0).IntPart()), true
	}

	// per-token 模式: (InputCostPerM × prompt + OutputCostPerM × completion) / 1e6 × QuotaPerUnit
	dInput := decimal.NewFromFloat(cost.InputCostPerM)
	dOutput := decimal.NewFromFloat(cost.OutputCostPerM)
	dPrompt := decimal.NewFromInt(int64(promptTokens))
	dCompletion := decimal.NewFromInt(int64(completionTokens))
	costDecimal := dInput.Mul(dPrompt).Add(dOutput.Mul(dCompletion)).
		Div(decimal.NewFromInt(1000000)).Mul(dUnit)
	return int(costDecimal.Round(0).IntPart()), true
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
