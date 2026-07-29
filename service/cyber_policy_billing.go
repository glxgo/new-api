package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

// SettleCyberPolicyOrder settles an order that was already sent upstream but
// ended with OpenAI's exact cyber_policy rejection. Since upstream usage is not
// available, the input token estimate and frozen pricing snapshot are used.
// A pre-upstream local filter never reaches this function.
func SettleCyberPolicyOrder(ctx *gin.Context, relayInfo *relaycommon.RelayInfo) error {
	if ctx == nil || relayInfo == nil || relayInfo.PriceData.FreeModel {
		return nil
	}

	summary, tieredBillingApplied, tieredResult := calculateCyberPolicyOrderSummary(ctx, relayInfo)

	if err := SettleBilling(ctx, relayInfo, summary.Quota); err != nil {
		return err
	}

	model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, summary.Quota)
	model.UpdateChannelUsedQuota(relayInfo.ChannelId, summary.Quota)

	other := GenerateTextOtherInfo(
		ctx,
		relayInfo,
		summary.ModelRatio,
		summary.GroupRatio,
		summary.CompletionRatio,
		0,
		summary.CacheRatio,
		summary.ModelPrice,
		relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio,
	)
	other["cyber_policy_billed"] = true
	other["upstream_sent"] = true
	other["upstream_error_code"] = model.UserSecurityRuleCyberPolicy
	other["billing_basis"] = "estimated_input_tokens"
	if tieredBillingApplied {
		InjectTieredBillingInfo(other, relayInfo, tieredResult)
	}

	logModel := summary.ModelName
	if strings.HasPrefix(logModel, "gpt-4-gizmo") {
		logModel = "gpt-4-gizmo-*"
	}
	if strings.HasPrefix(logModel, "gpt-4o-gizmo") {
		logModel = "gpt-4o-gizmo-*"
	}
	affAdminID, inviterID, inviter2ID := GetAffiliateSnapshot(relayInfo.UserId)
	paidGift, paidPrincipal := paidSplitForLog(relayInfo, summary.Quota)
	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:           relayInfo.ChannelId,
		PromptTokens:        summary.PromptTokens,
		CompletionTokens:    0,
		ModelName:           logModel,
		TokenName:           summary.TokenName,
		Quota:               summary.Quota,
		Cost:                summary.Cost,
		PaidQuota:           paidPrincipal,
		PaidGiftQuota:       paidGift,
		AffAdminIdSnap:      affAdminID,
		InviterIdSnap:       inviterID,
		Inviter2IdSnap:      inviter2ID,
		Content:             fmt.Sprintf("上游已接收请求后返回 %s，按正常订单结算", model.UserSecurityRuleCyberPolicy),
		TokenId:             relayInfo.TokenId,
		UseTimeSeconds:      int(time.Now().Unix() - relayInfo.StartTime.Unix()),
		IsStream:            relayInfo.IsStream,
		Group:               relayInfo.UsingGroup,
		Other:               other,
		BillingSource:       relayInfo.BillingSource,
		SubscriptionId:      relayInfo.SubscriptionId,
		CostRuleVersion:     2,
		ChannelCostRatioPPM: relayInfo.ChannelCostRatioPPM,
	})
	return nil
}

func calculateCyberPolicyOrderSummary(ctx *gin.Context, relayInfo *relaycommon.RelayInfo) (textQuotaSummary, bool, *billingexpr.TieredResult) {
	usage := &dto.Usage{
		PromptTokens:  relayInfo.GetEstimatePromptTokens(),
		TotalTokens:   relayInfo.GetEstimatePromptTokens(),
		UsageSemantic: "openai",
	}
	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	var tieredResult *billingexpr.TieredResult
	tieredBillingApplied := false
	var tieredUsedVars map[string]bool
	if snap := relayInfo.TieredBillingSnapshot; snap != nil {
		tieredUsedVars = billingexpr.UsedVars(snap.ExprString)
	}
	if tieredOK, tieredQuota, result := TryTieredSettle(
		relayInfo,
		BuildTieredTokenParams(usage, false, tieredUsedVars),
	); tieredOK {
		tieredBillingApplied = true
		tieredResult = result
		summary.Quota = composeTieredTextQuota(relayInfo, summary, tieredQuota, result)
	}
	summary.Quota = relayInfo.ApplyPrioritySurcharge(summary.Quota)
	summary.Cost = CalcCostFromChannelRatio(summary.Quota, relayInfo.ChannelCostRatioPPM)
	return summary, tieredBillingApplied, tieredResult
}
