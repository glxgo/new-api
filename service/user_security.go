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
		return "API 调用权限已被永久封禁"
	}
	switch result.Action {
	case model.UserSecurityActionSuspend10Minutes:
		return "API 调用权限已封禁 10 分钟"
	case model.UserSecurityActionSuspend2Hours:
		return "API 调用权限已封禁 2 小时"
	case model.UserSecurityActionSuspend24Hours:
		return "API 调用权限已封禁 24 小时"
	default:
		return "API 调用权限已被封禁"
	}
}

func cyberPolicyBanDuration(strikeNumber int, permanent bool) string {
	if permanent || strikeNumber >= 4 {
		return "永久"
	}
	switch strikeNumber {
	case 1:
		return "10 分钟"
	case 2:
		return "2 小时"
	case 3:
		return "24 小时"
	default:
		return "未知"
	}
}

func cyberPolicyNextBanDuration(strikeNumber int, permanent bool) string {
	if permanent || strikeNumber >= 4 {
		return "无（当前已是永久封禁）"
	}
	return cyberPolicyBanDuration(strikeNumber+1, strikeNumber+1 >= 4)
}

func CyberPolicyRestrictionMessage(strikeNumber int, suspendedUntil int64, permanent bool) string {
	currentDuration := cyberPolicyBanDuration(strikeNumber, permanent)
	nextDuration := cyberPolicyNextBanDuration(strikeNumber, permanent)
	if permanent {
		return fmt.Sprintf(
			"API 调用权限已被封禁。本次封禁时长：%s（第 %d 次有效警告）；下一次封禁时长：%s。如需申诉请联系管理员。",
			currentDuration,
			strikeNumber,
			nextDuration,
		)
	}
	recoverAt := time.Unix(suspendedUntil, 0).In(chinaStandardTime).Format("2006-01-02 15:04:05")
	return fmt.Sprintf(
		"API 调用权限已被封禁。本次封禁时长：%s（第 %d 次有效警告），预计恢复时间：%s；封禁解除后如再次触发，下一次封禁时长：%s。",
		currentDuration,
		strikeNumber,
		recoverAt,
		nextDuration,
	)
}

func CyberPolicyUserMessage(result model.UserSecurityEnforcementResult) string {
	return "本次请求触发了平台安全使用规则。" + CyberPolicyRestrictionMessage(
		result.StrikeNumber,
		result.SuspendedUntil,
		result.Permanent,
	)
}

func cyberPolicyWarningEmail(requestId string, result model.UserSecurityEnforcementResult) (string, string) {
	currentDuration := cyberPolicyBanDuration(result.StrikeNumber, result.Permanent)
	nextDuration := cyberPolicyNextBanDuration(result.StrikeNumber, result.Permanent)
	title := fmt.Sprintf("API 已封禁：第 %d 次安全规则警告（%s）", result.StrikeNumber, currentDuration)
	expiry := "需联系管理员申诉解封"
	if !result.Permanent {
		expiry = time.Unix(result.SuspendedUntil, 0).In(chinaStandardTime).Format("2006-01-02 15:04:05")
	}
	content := fmt.Sprintf(
		`<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;line-height:1.7;color:#18181b">
<h2 style="margin:0 0 16px">API 调用权限已被封禁</h2>
<p>系统检测到违反平台安全使用规则的请求。为保护平台及上游账号安全，您的 API 调用权限已自动封禁。</p>
<table style="border-collapse:collapse;width:100%%;max-width:560px">
<tr><td style="padding:8px;border-bottom:1px solid #e4e4e7">有效警告次数</td><td style="padding:8px;border-bottom:1px solid #e4e4e7">%d</td></tr>
<tr><td style="padding:8px;border-bottom:1px solid #e4e4e7">当前状态</td><td style="padding:8px;border-bottom:1px solid #e4e4e7">%s</td></tr>
<tr><td style="padding:8px;border-bottom:1px solid #e4e4e7">本次封禁时长</td><td style="padding:8px;border-bottom:1px solid #e4e4e7">%s</td></tr>
<tr><td style="padding:8px;border-bottom:1px solid #e4e4e7">恢复时间</td><td style="padding:8px;border-bottom:1px solid #e4e4e7">%s</td></tr>
<tr><td style="padding:8px;border-bottom:1px solid #e4e4e7">下一次封禁时长</td><td style="padding:8px;border-bottom:1px solid #e4e4e7">%s</td></tr>
<tr><td style="padding:8px;border-bottom:1px solid #e4e4e7">请求编号</td><td style="padding:8px;border-bottom:1px solid #e4e4e7">%s</td></tr>
</table>
<p style="color:#52525b">请勿尝试绕过模型安全限制或提交可能危害网络与系统安全的内容。邮件不包含您的原始请求内容。</p>
</div>`,
		result.StrikeNumber,
		html.EscapeString(securityActionText(result)),
		html.EscapeString(currentDuration),
		html.EscapeString(expiry),
		html.EscapeString(nextDuration),
		html.EscapeString(requestId),
	)
	return title, content
}

func sendCyberPolicyWarningEmail(userId int, requestId string, result model.UserSecurityEnforcementResult) {
	email, err := model.GetUserEmail(userId)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to load cyber policy notification email for user %d: %s", userId, err.Error()))
		return
	}
	if userSetting, settingErr := model.GetUserSetting(userId, true); settingErr == nil {
		if notificationEmail := strings.TrimSpace(userSetting.NotificationEmail); notificationEmail != "" {
			email = notificationEmail
		}
	} else {
		common.SysLog(fmt.Sprintf("failed to load cyber policy notification setting for user %d: %s", userId, settingErr.Error()))
	}
	email = strings.TrimSpace(email)
	if email == "" {
		common.SysLog(fmt.Sprintf("user %d has no email for cyber policy ban notification", userId))
		return
	}
	title, content := cyberPolicyWarningEmail(requestId, result)
	if err := common.SendEmail(title, email, content); err != nil {
		common.SysLog(fmt.Sprintf("failed to send cyber policy warning to user %d: %s", userId, err.Error()))
	}
}
