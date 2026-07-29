package service

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestIsCyberPolicyError(t *testing.T) {
	require.True(t, IsCyberPolicyError(types.WithOpenAIError(types.OpenAIError{
		Code:    "cyber_policy",
		Message: "blocked",
	}, http.StatusBadGateway)))
	require.True(t, IsCyberPolicyError(types.NewOpenAIError(
		errors.New("This content was flagged for possible cybersecurity risk."),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)))
	require.False(t, IsCyberPolicyError(types.NewOpenAIError(
		errors.New("ordinary upstream failure"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)))
}

func TestCyberPolicyRestrictionMessageShowsCurrentAndNextBan(t *testing.T) {
	tests := []struct {
		name          string
		strike        int
		permanent     bool
		current       string
		next          string
		expectedUntil bool
	}{
		{name: "first", strike: 1, current: "10 分钟", next: "2 小时", expectedUntil: true},
		{name: "second", strike: 2, current: "2 小时", next: "24 小时", expectedUntil: true},
		{name: "third", strike: 3, current: "24 小时", next: "永久", expectedUntil: true},
		{name: "permanent", strike: 4, permanent: true, current: "永久", next: "无（当前已是永久封禁）"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message := CyberPolicyRestrictionMessage(test.strike, 1_700_000_000, test.permanent)

			require.Contains(t, message, "API 调用权限已被封禁")
			require.Contains(t, message, "本次封禁时长："+test.current)
			require.Contains(t, message, "下一次封禁时长："+test.next)
			require.Equal(t, test.expectedUntil, strings.Contains(message, "预计恢复时间"))
		})
	}
}

func TestCyberPolicyWarningEmailStatesBanAndEscalation(t *testing.T) {
	result := model.UserSecurityEnforcementResult{
		Counted:        true,
		StrikeNumber:   2,
		Action:         model.UserSecurityActionSuspend2Hours,
		SuspendedUntil: 1_700_000_000,
	}

	title, content := cyberPolicyWarningEmail("<request&42>", result)

	require.Contains(t, title, "API 已封禁")
	require.Contains(t, title, "2 小时")
	require.Contains(t, content, "API 调用权限已被封禁")
	require.Contains(t, content, "本次封禁时长")
	require.Contains(t, content, "2 小时")
	require.Contains(t, content, "下一次封禁时长")
	require.Contains(t, content, "24 小时")
	require.Contains(t, content, "&lt;request&amp;42&gt;")
	require.NotContains(t, content, "<request&42>")
}

func TestCyberPolicyInterceptionMessageStatesAccountRemainsUsable(t *testing.T) {
	message := CyberPolicyInterceptionMessage()

	require.Contains(t, message, "本次请求")
	require.Contains(t, message, "已被拦截")
	require.Contains(t, message, "账号和 API Key 均处于正常状态")
	require.Contains(t, message, "其他请求不受影响")
	require.Contains(t, message, "已发送至上游")
	require.Contains(t, message, "按正常规则扣费")
	require.NotContains(t, message, "封禁时长")
	require.NotContains(t, message, "永久封禁")
}

func TestCyberPolicyInterceptionEmailIsNonPunitiveAndPrivacySafe(t *testing.T) {
	title, content := cyberPolicyInterceptionEmail("<request&42>", "<gpt&model>", 1_700_000_000)

	require.Contains(t, title, "请求已被安全规则拦截")
	require.Contains(t, title, "账号未封禁")
	require.Contains(t, content, "仅拦截了这一次请求")
	require.Contains(t, content, "已发送至上游")
	require.Contains(t, content, "按正常规则扣费")
	require.Contains(t, content, "账号和 API Key 均处于正常状态")
	require.Contains(t, content, "&lt;gpt&amp;model&gt;")
	require.Contains(t, content, "&lt;request&amp;42&gt;")
	require.NotContains(t, content, "<gpt&model>")
	require.NotContains(t, content, "<request&42>")
	require.NotContains(t, content, "封禁时长")
	require.NotContains(t, content, "警告次数")
}
