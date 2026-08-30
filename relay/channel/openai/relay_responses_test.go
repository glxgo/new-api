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
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
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
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	return c, info, resp, recorder
}

func TestOaiResponsesStreamHandler_DONEWithoutCompletedIsRetryableIncomplete(t *testing.T) {
	c, info, resp, recorder := newResponsesStreamTest(t,
		"data: {\"type\":\"response.created\",\"sequence_number\":1,\"response\":{\"id\":\"resp_done_only\"}}\n\n"+
			"data: [DONE]\n\n")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeChannelIncompleteStream, apiErr.GetErrorCode())
	require.False(t, types.IsSkipRetryError(apiErr))
	require.Empty(t, recorder.Body.String())
	require.Equal(t, relaycommon.StreamEndReasonEOF, info.StreamStatus.EndReason)
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

func TestOaiResponsesStreamHandler_DoneEventMarksNormalEnd(t *testing.T) {
	c, info, resp, _ := newResponsesStreamTest(t,
		"data: {\"type\":\"response.done\",\"response\":{\"usage\":{\"input_tokens\":4,\"output_tokens\":1,\"total_tokens\":5}}}\n\n")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 4, usage.PromptTokens)
	require.Equal(t, 1, usage.CompletionTokens)
	require.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
}

func TestOaiResponsesStreamHandler_PrematureEOFIsIncomplete(t *testing.T) {
	c, info, resp, recorder := newResponsesStreamTest(t,
		"data: {\"type\":\"response.created\",\"sequence_number\":1,\"response\":{\"id\":\"resp_partial_1\"}}\n\n"+
			"data: {\"type\":\"response.output_text.delta\",\"sequence_number\":2,\"delta\":\"partial\"}\n\n")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.NotNil(t, usage)
	require.Equal(t, 2, usage.CompletionTokens)
	require.Equal(t, "estimated_stream_failure", usage.UsageSource)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode("channel:incomplete_stream"), apiErr.GetErrorCode())
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamEndReasonEOF, info.StreamStatus.EndReason)
	require.Equal(t, "response.output_text.delta", info.UpstreamLastEventType)
	require.Equal(t, 2, info.UpstreamLastSequence)
	require.Greater(t, info.UpstreamEventBytes, int64(0))
	require.Equal(t, "resp_partial_1", info.StreamStatus.UpstreamTerminalSnapshot().ResponseID)
	require.True(t, common.GetContextKeyBool(c, appconstant.ContextKeyRelayErrorAlreadyStreamed))
	require.NotContains(t, recorder.Body.String(), "event: response.failed")
}

func TestOaiResponsesStreamHandler_EmptyEOFRemainsRetryableBeforeOutput(t *testing.T) {
	c, info, resp, recorder := newResponsesStreamTest(t, "")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeChannelIncompleteStream, apiErr.GetErrorCode())
	require.False(t, types.IsSkipRetryError(apiErr))
	require.False(t, common.GetContextKeyBool(c, appconstant.ContextKeyRelayErrorAlreadyStreamed))
	require.Empty(t, recorder.Body.String())
}

func TestOaiResponsesStreamHandler_ResponseFailedPreservesUpstreamCause(t *testing.T) {
	c, info, resp, _ := newResponsesStreamTest(t,
		"data: {\"type\":\"response.output_text.delta\",\"sequence_number\":1,\"delta\":\"partial\"}\n\n"+
			"data: {\"type\":\"response.failed\",\"sequence_number\":2,\"response\":{\"id\":\"resp_failed_1\",\"status\":\"failed\",\"error\":{\"type\":\"server_error\",\"code\":\"server_error\",\"message\":\"upstream exploded\"}}}\n\n")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.NotNil(t, usage)
	require.Equal(t, 2, usage.CompletionTokens)
	require.Equal(t, "estimated_stream_failure", usage.UsageSource)
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
	info.SetEstimatePromptTokens(120)

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.NotNil(t, usage)
	require.Equal(t, 120, usage.PromptTokens)
	require.Equal(t, "estimated_stream_failure", usage.UsageSource)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode("upstream:response_incomplete:max_output_tokens"), apiErr.GetErrorCode())
	require.True(t, types.IsSkipRetryError(apiErr))
	terminal := info.StreamStatus.UpstreamTerminalSnapshot()
	require.Equal(t, "response.incomplete", terminal.EventType)
	require.Equal(t, "incomplete", terminal.ResponseStatus)
	require.Equal(t, "max_output_tokens", terminal.IncompleteReason)
}

func TestOaiResponsesStreamHandler_CustomToolInputFailureEstimatesUsage(t *testing.T) {
	c, info, resp, _ := newResponsesStreamTest(t,
		"data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"custom_tool_call\",\"id\":\"ctc_1\",\"input\":\"{\\\"query\\\":\\\"status\\\"}\"}}\n\n")
	info.SetEstimatePromptTokens(80)

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.NotNil(t, usage)
	require.Equal(t, 80, usage.PromptTokens)
	require.Greater(t, usage.CompletionTokens, 0)
	require.Equal(t, "estimated_stream_failure", usage.UsageSource)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeChannelIncompleteStream, apiErr.GetErrorCode())
}

func TestOaiResponsesToChatStreamHandler_PrematureEOFFailsAndEstimatesUsage(t *testing.T) {
	c, info, resp, _ := newResponsesStreamTest(t,
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n")
	info.RelayFormat = types.RelayFormatOpenAI
	info.SetEstimatePromptTokens(90)

	usage, apiErr := OaiResponsesToChatStreamHandler(c, info, resp)

	require.NotNil(t, usage)
	require.Equal(t, 90, usage.PromptTokens)
	require.Greater(t, usage.CompletionTokens, 0)
	require.Equal(t, "estimated_stream_failure", usage.UsageSource)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeChannelIncompleteStream, apiErr.GetErrorCode())
	require.True(t, common.GetContextKeyBool(c, appconstant.ContextKeyRelayErrorAlreadyStreamed))
}

func TestOaiResponsesToChatStreamHandler_ReasoningSummaryFailureCountsDeltaOnce(t *testing.T) {
	const delta = "reasoning summary"
	c, info, resp, _ := newResponsesStreamTest(t,
		"data: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\""+delta+"\"}\n\n")
	info.RelayFormat = types.RelayFormatOpenAI
	info.SetEstimatePromptTokens(90)

	usage, apiErr := OaiResponsesToChatStreamHandler(c, info, resp)

	require.NotNil(t, usage)
	require.Equal(t, service.CountTextToken(delta, info.UpstreamModelName), usage.CompletionTokens)
	require.Equal(t, "estimated_stream_failure", usage.UsageSource)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeChannelIncompleteStream, apiErr.GetErrorCode())
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
	c, info, resp, recorder := newResponsesStreamTest(t,
		"data: {\"type\":\"error\",\"sequence_number\":9,\"code\":\"invalid_value\",\"message\":\"invalid input item\",\"param\":\"input[5].id\"}\n\n")
	c.Request.Header.Set("Originator", "codex_cli_rs")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode("invalid_value"), apiErr.GetErrorCode())
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.True(t, types.IsSkipRetryError(apiErr))
	require.True(t, common.GetContextKeyBool(c, appconstant.ContextKeyRelayErrorAlreadyStreamed))
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamEndReasonHandlerStop, info.StreamStatus.EndReason)
	terminal := info.StreamStatus.UpstreamTerminalSnapshot()
	require.Equal(t, "error", terminal.EventType)
	require.Equal(t, "invalid_value", terminal.ErrorCode)
	require.Equal(t, "invalid input item", terminal.ErrorMessage)
	body := recorder.Body.String()
	require.Contains(t, body, "event: response.failed")
	require.Contains(t, body, "\"type\":\"response.failed\"")
	require.Contains(t, body, "\"code\":\"invalid_value\"")
	require.Contains(t, body, "\"message\":\"invalid input item\"")
	require.Contains(t, body, "\"status\":\"failed\"")
	require.NotContains(t, body, "event: error")
}

func TestOaiResponsesStreamHandler_CodexTransientErrorBeforeOutputRemainsRetryable(t *testing.T) {
	c, info, resp, recorder := newResponsesStreamTest(t,
		"data: {\"type\":\"error\",\"sequence_number\":3,\"code\":\"server_is_overloaded\",\"message\":\"Our servers are currently overloaded. Please try again later.\",\"param\":null}\n\n")
	c.Request.Header.Set("User-Agent", "Codex Desktop/1.2.3")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode("server_is_overloaded"), apiErr.GetErrorCode())
	require.False(t, types.IsSkipRetryError(apiErr))
	require.False(t, common.GetContextKeyBool(c, appconstant.ContextKeyRelayErrorAlreadyStreamed))
	require.Empty(t, recorder.Body.String())
}

func TestOaiResponsesStreamHandler_CodexTransientErrorAfterPreludeRemainsRetryable(t *testing.T) {
	c, info, resp, recorder := newResponsesStreamTest(t,
		"data: {\"type\":\"response.created\",\"sequence_number\":1,\"response\":{\"id\":\"resp_retryable_1\",\"status\":\"in_progress\"}}\n\n"+
			"data: {\"type\":\"response.in_progress\",\"sequence_number\":2,\"response\":{\"id\":\"resp_retryable_1\",\"status\":\"in_progress\"}}\n\n"+
			"data: {\"type\":\"error\",\"sequence_number\":3,\"code\":\"server_is_overloaded\",\"message\":\"Our servers are currently overloaded. Please try again later.\",\"param\":null}\n\n")
	c.Request.Header.Set("Originator", "codex-tui")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode("server_is_overloaded"), apiErr.GetErrorCode())
	require.False(t, types.IsSkipRetryError(apiErr))
	require.False(t, common.GetContextKeyBool(c, appconstant.ContextKeyRelayErrorAlreadyStreamed))
	require.Empty(t, recorder.Body.String(), "control-only prelude must stay buffered so another channel can retry invisibly")
}

func TestOaiResponsesStreamHandler_NonCodexPreservesOfficialErrorEvent(t *testing.T) {
	c, info, resp, recorder := newResponsesStreamTest(t,
		"data: {\"type\":\"error\",\"sequence_number\":4,\"code\":\"invalid_request_error\",\"message\":\"invalid input\",\"param\":\"input\"}\n\n")
	c.Request.Header.Set("User-Agent", "generic-openai-sdk/1.0")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode("invalid_request_error"), apiErr.GetErrorCode())
	body := recorder.Body.String()
	require.Contains(t, body, "event: error")
	require.Contains(t, body, "\"type\":\"error\"")
	require.Contains(t, body, "\"message\":\"invalid input\"")
	require.NotContains(t, body, "event: response.failed")
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

func TestIsCodexResponsesClient(t *testing.T) {
	tests := []struct {
		name       string
		userAgent  string
		originator string
		want       bool
	}{
		{name: "desktop", userAgent: "Codex Desktop/1.0", want: true},
		{name: "tui", userAgent: "codex-tui/0.5", want: true},
		{name: "cli", originator: "codex_cli_rs", want: true},
		{name: "legacy cli header", originator: "Codex CLI", want: true},
		{name: "cc switch", userAgent: "cc-switch/3.2", want: true},
		{name: "generic sdk", userAgent: "openai-go/3.0", want: false},
		{name: "lookalike", userAgent: "my-codex-client/1.0", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, _, _, _ := newResponsesStreamTest(t, "")
			c.Request.Header.Set("User-Agent", test.userAgent)
			c.Request.Header.Set("Originator", test.originator)
			require.Equal(t, test.want, isCodexResponsesClient(c))
		})
	}
}

func TestIsNonRetryableResponsesRequestError(t *testing.T) {
	tests := []struct {
		code      string
		errorType string
		want      bool
	}{
		{code: "invalid_value", errorType: "invalid_request_error", want: true},
		{code: "context_length_exceeded", errorType: "upstream_error", want: true},
		{code: "previous_response_not_found", errorType: "upstream_error", want: true},
		{code: "server_is_overloaded", errorType: "server_error", want: false},
		{code: "rate_limit_exceeded", errorType: "rate_limit_error", want: false},
		{code: "server_error", errorType: "upstream_error", want: false},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			apiErr := types.WithOpenAIError(types.OpenAIError{
				Message: "test",
				Type:    test.errorType,
				Code:    test.code,
			}, http.StatusBadGateway)
			require.Equal(t, test.want, isNonRetryableResponsesRequestError(apiErr))
		})
	}
}

func TestIsResponsesBillingProgressEvent(t *testing.T) {
	tests := []struct {
		event string
		want  bool
	}{
		{event: "response.output_text.delta", want: true},
		{event: "response.function_call_arguments.delta", want: true},
		{event: "response.custom_tool_call_input.delta", want: true},
		{event: "response.output_item.added", want: true},
		{event: "response.in_progress", want: false},
		{event: "response.created", want: false},
		{event: "error", want: false},
	}
	for _, test := range tests {
		t.Run(test.event, func(t *testing.T) {
			require.Equal(t, test.want, isResponsesBillingProgressEvent(test.event))
		})
	}
}
