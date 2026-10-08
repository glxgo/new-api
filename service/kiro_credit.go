package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/kiro"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

// BeginKiroUpstream runs after all configurable header overrides. Client IDs
// cannot become billing correlation IDs. Each actual HTTP attempt gets a UUID.
func BeginKiroUpstream(c *gin.Context, info *relaycommon.RelayInfo, req *http.Request) error {
	if info.Kiro == nil {
		return nil
	}
	FinishKiroAttempt(c, info, true)
	r := info.Kiro.Begin(info.ChannelId)
	c.Set(kiro.ContextKey, info.Kiro)
	req.Header.Set("X-Hlool-Request-Id", r.RequestID)
	req.Header.Set("X-Hlool-Attempt-Id", r.AttemptID)
	if info.Kiro.Charge() && (!r.Trusted || info.UsingGroup != info.Kiro.Snapshot.Group) {
		return errors.New("Kiro channel or service group does not match the frozen billing configuration")
	}
	if info.Kiro.Snapshot.Mode != "off" {
		info.ShouldIncludeUsage = true
		// Apply after all body overrides, including raw passthrough channels.
		if info.IsStream && strings.HasSuffix(req.URL.Path, "/chat/completions") && req.Body != nil {
			data, err := io.ReadAll(req.Body)
			if err != nil {
				return err
			}
			_ = req.Body.Close()
			var body map[string]json.RawMessage
			if err := common.Unmarshal(data, &body); err != nil || body == nil {
				return errors.New("invalid Kiro Chat request body")
			}
			var options map[string]json.RawMessage
			_ = common.Unmarshal(body["stream_options"], &options)
			if options == nil {
				options = make(map[string]json.RawMessage)
			}
			options["include_usage"] = json.RawMessage("true")
			body["stream_options"], _ = common.Marshal(options)
			data, err = common.Marshal(body)
			if err != nil {
				return err
			}
			req.Body = io.NopCloser(bytes.NewReader(data))
			req.ContentLength = int64(len(data))
			req.Header.Del("Content-Length")
			req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
		}
	}
	return nil
}

func kiroKnownCost(r kiro.Result) (*string, string) {
	if r.MeteringInvalid || !r.Trusted || r.Receipt == nil || r.Receipt.Consumed == nil {
		return nil, "missing"
	}
	credits, err := kiro.Decimal(*r.Receipt.Consumed)
	if err != nil {
		return nil, "missing"
	}
	price, err := kiro.Decimal(r.CostPrice)
	if err != nil {
		return nil, "missing"
	}
	cost := credits.Mul(price).String()
	return &cost, r.Receipt.Coverage
}

func FinishKiroAttempt(c *gin.Context, info *relaycommon.RelayInfo, failed bool) {
	if info == nil || info.Kiro == nil || info.Kiro.Snapshot.Mode == "off" {
		return
	}
	r := info.Kiro.Result()
	if r.AttemptID == "" {
		return
	}
	cost, coverage := kiroKnownCost(r)
	evidence, _ := common.Marshal(r.Evidence)
	status := "complete"
	if failed || !r.Complete {
		status = "failed"
	}
	if r.Error != "" {
		status = "metering_invalid"
	}
	row := &model.KiroCreditAttempt{AttemptID: r.AttemptID, RequestID: r.RequestID,
		UserID: info.UserId, ChannelID: r.ChannelID, Mode: info.Kiro.Snapshot.Mode,
		Status: status, RawReceipt: string(r.Raw), Error: r.Error, CostPerCredit: r.CostPrice,
		KnownCostCNY: cost, ConsumedCoverage: coverage}
	row.ReceiptEvidence = string(evidence)
	if info.Kiro.Snapshot.Mode == "shadow" && !failed && r.Complete && r.Error == "" && r.Receipt != nil && r.Receipt.Status == "complete" {
		if q, err := info.Kiro.Snapshot.Quota(*r.Receipt.Billable); err == nil {
			value := int64(q)
			row.ShadowQuota = &value
		}
	}
	if err := model.RecordKiroCreditAttempt(row); err != nil {
		logger.LogError(c, "Kiro attempt evidence write failed: "+err.Error())
	}
}

// PostKiroConsume owns the entire charge-mode settlement. No token, tool,
// cache, priority or ingress surcharge is added to this path. Shadow/off return
// false and the established billing path continues unchanged.
func PostKiroConsume(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage) bool {
	if info == nil || !info.Kiro.Charge() {
		return false
	}
	if info.KiroFinalized {
		return true
	}
	info.KiroFinalized = true
	var quota int
	var settleErr error
	r := info.Kiro.Result()
	if c.Request != nil && c.Request.Context().Err() != nil {
		settleErr = c.Request.Context().Err()
	}
	if err := info.KiroCompletionError(); err != nil {
		settleErr = err
	}
	if info.Billing == nil {
		settleErr = errors.New("Kiro billing reservation is missing")
	}
	if settleErr == nil {
		quota, settleErr = info.Kiro.Snapshot.Quota(*r.Receipt.Billable)
	}
	cost, coverage := kiroKnownCost(r)
	claimed := false
	row := &model.KiroCreditSettlement{}
	if settleErr == nil {
		price, err := common.Marshal(info.Kiro.Snapshot)
		if err != nil {
			settleErr = err
		} else {
			row = &model.KiroCreditSettlement{MeteringID: r.Receipt.MeteringID, RequestID: r.RequestID,
				AttemptID: r.AttemptID, UserID: info.UserId, TokenID: info.TokenId, ChannelID: r.ChannelID,
				Quota: int64(quota), RawReceipt: string(r.Raw), PriceSnapshot: string(price), KnownCostCNY: cost, ConsumedCoverage: coverage}
			claimed, settleErr = model.ClaimKiroSettlement(row)
			if settleErr == nil && !claimed {
				settleErr = errors.New("duplicate Kiro metering receipt; settlement not replayed")
			}
		}
	}
	if settleErr != nil {
		quota = 0
		if info.Billing != nil {
			if err := info.Billing.Settle(0); err != nil {
				logger.LogError(c, "Kiro reservation refund needs review: "+err.Error())
				info.Billing.Refund(c)
			}
		}
		logger.LogError(c, "Kiro credit charge refused: "+settleErr.Error())
	} else {
		settleErr = SettleBilling(c, info, quota)
		status := "settled"
		if settleErr != nil {
			status = "needs_review"
		}
		if err := model.FinishKiroSettlement(row.MeteringKey, status); err != nil {
			logger.LogError(c, "Kiro settlement evidence needs review: "+err.Error())
		}
		if settleErr != nil {
			// Funding may already be committed. Never automatically debit or refund
			// again: pending/needs_review is an operator reconciliation obligation.
			logger.LogError(c, "Kiro settlement needs review: "+settleErr.Error())
			return true
		}
	}
	info.FinalConsumedQuota = quota
	if usage == nil {
		usage = &dto.Usage{}
	}
	other := map[string]any{"billing_unit": kiro.Unit, "kiro_price": info.Kiro.Snapshot.PublicPrice()}
	if receipt := info.Kiro.Forward(); len(receipt) > 0 {
		other["kiro_metering"] = receipt
	}
	if settleErr != nil {
		other["kiro_billing_status"] = "not_charged"
	} else {
		other["kiro_billing_status"] = "settled"
	}
	// Credit cost lives exclusively in the attempt ledger (including failures).
	// Legacy log cost is NULL, never a fictitious zero or a duplicated receipt.
	other["cost_rule_version"] = 3
	other["kiro_cost_ledger"] = true
	if quota > 0 {
		model.UpdateUserUsedQuotaAndRequestCount(info.UserId, quota)
		model.UpdateChannelUsedQuota(info.ChannelId, quota)
	}
	paidGift, paidPrincipal := paidSplitForLog(info, quota)
	model.RecordConsumeLog(c, info.UserId, model.RecordConsumeLogParams{
		ChannelId: info.ChannelId, ModelName: info.OriginModelName, TokenId: info.TokenId, TokenName: c.GetString("token_name"),
		PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
		CacheTokens: usage.PromptTokensDetails.CachedTokens, Quota: quota,
		PaidGiftQuota: paidGift, PaidQuota: paidPrincipal, Group: info.UsingGroup, IsStream: info.IsStream,
		UseTimeSeconds: int(time.Since(info.StartTime).Seconds()), Other: other,
		Content:       fmt.Sprintf("Kiro credit (%s)", other["kiro_billing_status"]),
		BillingSource: info.BillingSource, SubscriptionId: info.SubscriptionId, CostRuleVersion: 3,
	})
	return true
}
