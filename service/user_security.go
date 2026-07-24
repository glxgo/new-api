package service

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

const cyberPolicyMessageSignature = "flagged for possible cybersecurity risk"

var chinaStandardTime = time.FixedZone("CST", 8*60*60)

func IsCyberPolicyError(apiErr *types.NewAPIError) bool {
	if apiErr == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(string(apiErr.GetErrorCode())), model.UserSecurityRuleCyberPolicy) {
		return true
	}
	return strings.Contains(strings.ToLower(apiErr.Error()), cyberPolicyMessageSignature)
}

func EnforceCyberPolicyViolation(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) (model.UserSecurityEnforcementResult, error) {
	result := model.UserSecurityEnforcementResult{}
	if c == nil || info == nil || !IsCyberPolicyError(apiErr) {
		return result, nil
	}
	requestId := c.GetString(common.RequestIdKey)
	if requestId == "" {
		requestId = info.RequestId
	}
	result, err := model.ApplyCyberPolicyViolation(
		info.UserId,
		info.TokenId,
		requestId,
		info.OriginModelName,
		string(apiErr.GetErrorCode()),
		common.GetTimestamp(),
	)
	if err != nil || !result.Counted {
		return result, err
	}
	gopool.Go(func() {
		sendCyberPolicyWarningEmail(info.UserId, requestId, result)
	})
	return result, nil
}

func securityActionText(result model.UserSecurityEnforcementResult) string {
	if result.Permanent {
		return "API 服务已被永久封禁"
	}
	switch result.Action {
	case model.UserSecurityActionSuspend10Minutes:
		return "API 服务已暂停 10 分钟"
	case model.UserSecurityActionSuspend2Hours:
		return "API 服务已暂停 2 小时"
	case model.UserSecurityActionSuspend24Hours:
		return "API 服务已暂停 24 小时"
	default:
		return "API 服务已被限制"
	}
}

func CyberPolicyUserMessage(result model.UserSecurityEnforcementResult) string {
	action := securityActionText(result)
	if result.Permanent {
		return fmt.Sprintf("检测到违反平台安全使用规则的请求，这是第 %d 次有效警告，%s。如需申诉请联系管理员。", result.StrikeNumber, action)
	}
	until := time.Unix(result.SuspendedUntil, 0).In(chinaStandardTime).Format("2006-01-02 15:04:05")
	return fmt.Sprintf("检测到违反平台安全使用规则的请求，这是第 %d 次有效警告，%s，预计恢复时间：%s。", result.StrikeNumber, action, until)
}

func sendCyberPolicyWarningEmail(userId int, requestId string, result model.UserSecurityEnforcementResult) {
	email, err := model.GetUserEmail(userId)
	if err != nil || strings.TrimSpace(email) == "" {
		return
	}
	title := fmt.Sprintf("API 安全使用警告（第 %d 次）", result.StrikeNumber)
	expiry := "需联系管理员申诉解封"
	if !result.Permanent {
		expiry = time.Unix(result.SuspendedUntil, 0).In(chinaStandardTime).Format("2006-01-02 15:04:05")
	}
	content := fmt.Sprintf(
		`<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;line-height:1.7;color:#18181b">
<h2 style="margin:0 0 16px">API 安全使用警告</h2>
<p>系统检测到违反平台安全使用规则的请求。为保护平台及上游账号安全，已自动执行限制。</p>
<table style="border-collapse:collapse;width:100%%;max-width:560px">
<tr><td style="padding:8px;border-bottom:1px solid #e4e4e7">有效警告次数</td><td style="padding:8px;border-bottom:1px solid #e4e4e7">%d</td></tr>
<tr><td style="padding:8px;border-bottom:1px solid #e4e4e7">处理结果</td><td style="padding:8px;border-bottom:1px solid #e4e4e7">%s</td></tr>
<tr><td style="padding:8px;border-bottom:1px solid #e4e4e7">恢复时间</td><td style="padding:8px;border-bottom:1px solid #e4e4e7">%s</td></tr>
<tr><td style="padding:8px;border-bottom:1px solid #e4e4e7">请求编号</td><td style="padding:8px;border-bottom:1px solid #e4e4e7">%s</td></tr>
</table>
<p style="color:#52525b">请勿尝试绕过模型安全限制或提交可能危害网络与系统安全的内容。邮件不包含您的原始请求内容。</p>
</div>`,
		result.StrikeNumber,
		html.EscapeString(securityActionText(result)),
		html.EscapeString(expiry),
		html.EscapeString(requestId),
	)
	if err := common.SendEmail(title, email, content); err != nil {
		common.SysLog(fmt.Sprintf("failed to send cyber policy warning to user %d: %s", userId, err.Error()))
	}
}
