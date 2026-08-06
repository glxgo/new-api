package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newResponsesStreamTest(t *testing.T, body string) (*gin.Context, *relaycommon.RelayInfo, *http.Response, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	setting := operation_setting.GetGeneralSetting()
	oldPingEnabled := setting.PingIntervalEnabled
	oldStreamingTimeout := appconstant.StreamingTimeout
	setting.PingIntervalEnabled = false
	appconstant.StreamingTimeout = 30
	t.Cleanup(func() {
		setting.PingIntervalEnabled = oldPingEnabled
		appconstant.StreamingTimeout = oldStreamingTimeout
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	return c, info, resp, recorder
}

func TestOaiResponsesStreamHandler_CompletedEventMarksNormalEnd(t *testing.T) {
	c, info, resp, _ := newResponsesStreamTest(t,
		"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":3,\"output_tokens\":2,\"total_tokens\":5}}}\n\n")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 2, usage.CompletionTokens)
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
}

func TestOaiResponsesStreamHandler_PrematureEOFIsIncomplete(t *testing.T) {
	c, info, resp, _ := newResponsesStreamTest(t,
		"data: {\"type\":\"response.created\",\"sequence_number\":1,\"response\":{\"id\":\"resp_partial_1\"}}\n\n"+
			"data: {\"type\":\"response.output_text.delta\",\"sequence_number\":2,\"delta\":\"partial\"}\n\n")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode("channel:incomplete_stream"), apiErr.GetErrorCode())
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamEndReasonEOF, info.StreamStatus.EndReason)
	require.Equal(t, "response.output_text.delta", info.UpstreamLastEventType)
	require.Equal(t, 2, info.UpstreamLastSequence)
	require.Greater(t, info.UpstreamEventBytes, int64(0))
	require.Equal(t, "resp_partial_1", info.StreamStatus.UpstreamTerminalSnapshot().ResponseID)
}

func TestOaiResponsesStreamHandler_ResponseFailedPreservesUpstreamCause(t *testing.T) {
	c, info, resp, _ := newResponsesStreamTest(t,
		"data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_failed_1\",\"status\":\"failed\",\"error\":{\"type\":\"server_error\",\"code\":\"server_error\",\"message\":\"upstream exploded\"}}}\n\n")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode("server_error"), apiErr.GetErrorCode())
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.True(t, types.IsSkipRetryError(apiErr))
	require.True(t, common.GetContextKeyBool(c, appconstant.ContextKeyRelayErrorAlreadyStreamed))
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamEndReasonHandlerStop, info.StreamStatus.EndReason)
	terminal := info.StreamStatus.UpstreamTerminalSnapshot()
	require.Equal(t, "response.failed", terminal.EventType)
	require.Equal(t, "resp_failed_1", terminal.ResponseID)
	require.Equal(t, "failed", terminal.ResponseStatus)
	require.Equal(t, "server_error", terminal.ErrorCode)
	require.Equal(t, "upstream exploded", terminal.ErrorMessage)
}

func TestOaiResponsesStreamHandler_ResponseIncompleteCapturesReason(t *testing.T) {
	c, info, resp, _ := newResponsesStreamTest(t,
		"data: {\"type\":\"response.incomplete\",\"response\":{\"id\":\"resp_incomplete_1\",\"status\":\"incomplete\",\"incomplete_details\":{\"reason\":\"max_output_tokens\"}}}\n\n")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode("upstream:response_incomplete:max_output_tokens"), apiErr.GetErrorCode())
	require.True(t, types.IsSkipRetryError(apiErr))
	terminal := info.StreamStatus.UpstreamTerminalSnapshot()
	require.Equal(t, "response.incomplete", terminal.EventType)
	require.Equal(t, "incomplete", terminal.ResponseStatus)
	require.Equal(t, "max_output_tokens", terminal.IncompleteReason)
}

func TestOaiResponsesStreamHandler_TopLevelResponseError(t *testing.T) {
	c, info, resp, _ := newResponsesStreamTest(t,
		"data: {\"type\":\"response.error\",\"error\":{\"type\":\"invalid_request_error\",\"code\":\"invalid_prompt\",\"message\":\"bad prompt\"}}\n\n")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode("invalid_prompt"), apiErr.GetErrorCode())
	terminal := info.StreamStatus.UpstreamTerminalSnapshot()
	require.Equal(t, "response.error", terminal.EventType)
	require.Equal(t, "invalid_prompt", terminal.ErrorCode)
	require.Equal(t, "bad prompt", terminal.ErrorMessage)
}

func TestOaiResponsesStreamHandler_TopLevelErrorEventIsTerminal(t *testing.T) {
	c, info, resp, _ := newResponsesStreamTest(t,
		"data: {\"type\":\"error\",\"error\":{\"type\":\"server_error\",\"code\":\"upstream_overloaded\",\"message\":\"upstream overloaded\"}}\n\n")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode("upstream_overloaded"), apiErr.GetErrorCode())
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.True(t, types.IsSkipRetryError(apiErr))
	require.True(t, common.GetContextKeyBool(c, appconstant.ContextKeyRelayErrorAlreadyStreamed))
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamEndReasonHandlerStop, info.StreamStatus.EndReason)
	terminal := info.StreamStatus.UpstreamTerminalSnapshot()
	require.Equal(t, "error", terminal.EventType)
	require.Equal(t, "upstream_overloaded", terminal.ErrorCode)
	require.Equal(t, "upstream overloaded", terminal.ErrorMessage)
}

func TestOaiResponsesStreamHandler_CyberPolicyRewritesClientEvent(t *testing.T) {
	oldInterception := common.CyberPolicyInterceptionEnabled
	common.CyberPolicyInterceptionEnabled = true
	t.Cleanup(func() { common.CyberPolicyInterceptionEnabled = oldInterception })

	c, info, resp, recorder := newResponsesStreamTest(t,
		"data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_cyber_1\",\"status\":\"failed\",\"error\":{\"type\":\"invalid_request_error\",\"code\":\"cyber_policy\",\"message\":\"This content was flagged for possible cybersecurity risk.\"}}}\n\n")
	c.Set(common.RequestIdKey, "request-cyber-1")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode(model.UserSecurityRuleCyberPolicy), apiErr.GetErrorCode())
	require.True(t, types.IsSkipRetryError(apiErr))
	require.True(t, common.GetContextKeyBool(c, appconstant.ContextKeyRelayErrorAlreadyStreamed))

	body := recorder.Body.String()
	require.Contains(t, body, model.UserSecurityErrorCodeContentPolicyBlocked)
	require.Contains(t, body, "账号和 API Key 均处于正常状态")
	require.Contains(t, body, "request-cyber-1")
	require.NotContains(t, body, "flagged for possible cybersecurity risk")

	terminal := info.StreamStatus.UpstreamTerminalSnapshot()
	require.Equal(t, model.UserSecurityRuleCyberPolicy, terminal.ErrorCode)
	require.Contains(t, terminal.ErrorMessage, "flagged for possible cybersecurity risk")
}
