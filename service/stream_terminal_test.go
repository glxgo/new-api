package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func TestBuildResponsesStreamTerminalEventCompleted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "https://token.stellaisle.com/v1/responses", nil)
	c.Request.Header.Set(ingressRequestIDHeader, "edge-abc_123")
	completedContext, cancelCompleted := context.WithCancel(c.Request.Context())
	cancelCompleted()
	c.Request = c.Request.WithContext(completedContext)
	c.Set(common.RequestIdKey, "req-completed")
	c.Set(common.UpstreamRequestIdKey, "upstream-1")
	c.Set("use_channel", []string{"8", "31"})
	setChannelAffinityContext(c, channelAffinityMeta{
		CacheKey:       "new-api:channel_affinity:v1:test",
		TTLSeconds:     3600,
		RuleName:       "codex cli trace",
		KeyFingerprint: "89abcdef",
	})
	started := time.Unix(100, 0)
	common.SetContextKey(c, constant.ContextKeyRequestStartTime, started)
	c.Status(200)
	_, _ = c.Writer.WriteString("data: response.completed\n\n")
	status := relaycommon.NewStreamStatus()
	status.SetEndReason(relaycommon.StreamEndReasonDone, nil)
	upstreamStarted := started.Add(250 * time.Millisecond)
	transportTrace := relaycommon.NewUpstreamTransportTrace(upstreamStarted)
	transportTrace.RecordResponse(&http.Response{Proto: "HTTP/2.0"}, upstreamStarted.Add(350*time.Millisecond))
	info := &relaycommon.RelayInfo{
		RequestId:                    "req-completed",
		UserId:                       264,
		TokenId:                      9,
		UsingGroup:                   "default",
		OriginModelName:              "gpt-5.6-sol",
		IsStream:                     true,
		StreamStatus:                 status,
		BillingSource:                BillingSourceWallet,
		ChannelMeta:                  &relaycommon.ChannelMeta{ChannelId: 31},
		FinalPreConsumedQuota:        123,
		UpstreamStartTime:            upstreamStarted,
		FirstResponseTime:            upstreamStarted.Add(900 * time.Millisecond),
		UpstreamTransportTrace:       transportTrace,
		UpstreamHost:                 "api.example.com",
		UpstreamProxyUsed:            true,
		UpstreamLastEventType:        "response.completed",
		UpstreamLastSequence:         42,
		UpstreamEventBytes:           8192,
		ReceivedResponseCount:        5,
		ForwardedResponsesEventCount: 3,
	}
	info.SetEstimatePromptTokens(12345)

	event := buildResponsesStreamTerminalEvent(c, info, nil, started.Add(3*time.Second))
	if event == nil {
		t.Fatal("event is nil")
	}
	if event.TerminalStatus != "completed" || !event.ResponseCompleted || event.ClientGone || event.EndReason != "done" {
		t.Fatalf("unexpected terminal event: %#v", event)
	}
	if event.HttpStatus != 200 || event.IntendedStatus != 200 || event.ChannelId != 31 {
		t.Fatalf("unexpected status/channel: %#v", event)
	}
	if event.UsedChannels != "8,31" || event.DurationMs != 3000 || event.ResponseBytes == 0 {
		t.Fatalf("unexpected diagnostics: %#v", event)
	}
	if event.IngressRequestId != "edge-abc_123" || event.AffinityRuleName != "codex cli trace" || event.AffinityKeyFp != "89abcdef" {
		t.Fatalf("unexpected correlation diagnostics: %#v", event)
	}
	if event.UpstreamProtocol != "HTTP/2.0" || event.UpstreamResponseHeaderMs != 350 || event.UpstreamFirstEventMs != 900 {
		t.Fatalf("unexpected upstream timing diagnostics: %#v", event)
	}
	if event.UpstreamHost != "api.example.com" || !event.UpstreamProxyUsed || event.EstimatedPromptTokens != 12345 {
		t.Fatalf("unexpected upstream request diagnostics: %#v", event)
	}
	if event.UpstreamLastEventType != "response.completed" || event.UpstreamLastSequence != 42 || event.UpstreamEventBytes != 8192 {
		t.Fatalf("unexpected upstream event diagnostics: %#v", event)
	}
	if event.ReceivedEvents != 5 || event.ForwardedEvents != 3 {
		t.Fatalf("unexpected received/forwarded event diagnostics: %#v", event)
	}
}

func TestNormalizeIngressRequestID(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "nginx id", value: "0123456789abcdef0123456789abcdef", want: "0123456789abcdef0123456789abcdef"},
		{name: "trim safe id", value: "  edge.req_1:retry-2  ", want: "edge.req_1:retry-2"},
		{name: "reject whitespace", value: "edge request", want: ""},
		{name: "reject control", value: "edge\nrequest", want: ""},
		{name: "reject oversized", value: string(make([]byte, 65)), want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeIngressRequestID(tt.value); got != tt.want {
				t.Fatalf("normalizeIngressRequestID(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestBuildResponsesStreamTerminalEventClientGone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	request := httptest.NewRequest("POST", "https://direct-token.stellaisle.com/v1/responses", nil)
	requestContext, cancel := context.WithCancel(request.Context())
	cancel()
	c.Request = request.WithContext(requestContext)
	c.Set(common.RequestIdKey, "req-client-gone")
	status := relaycommon.NewStreamStatus()
	status.SetEndReason(relaycommon.StreamEndReasonClientGone, context.Canceled)
	info := &relaycommon.RelayInfo{RequestId: "req-client-gone", IsStream: true, StreamStatus: status}
	apiErr := types.NewErrorWithStatusCode(errors.New("context canceled"), types.ErrorCodeChannelIncompleteStream, 502)

	event := buildResponsesStreamTerminalEvent(c, info, apiErr, time.Now())
	if event == nil {
		t.Fatal("event is nil")
	}
	if event.TerminalStatus != "client_gone" || !event.ClientGone || event.ResponseCompleted {
		t.Fatalf("unexpected client-gone event: %#v", event)
	}
	if event.HttpStatus != 502 || event.IntendedStatus != 502 || event.ErrorCode != string(types.ErrorCodeChannelIncompleteStream) {
		t.Fatalf("unexpected error diagnostics: %#v", event)
	}
}

func TestBuildResponsesStreamTerminalEventUpstreamFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "https://token.stellaisle.com/v1/responses", nil)
	c.Set(common.RequestIdKey, "req-upstream-failed")
	status := relaycommon.NewStreamStatus()
	status.SetUpstreamTerminal(relaycommon.UpstreamTerminal{
		EventType:      "error",
		HTTPStatus:     200,
		ResponseID:     "resp_failed_1",
		ResponseStatus: "failed",
		ErrorCode:      "server_error",
		ErrorMessage:   "upstream failed at https://api.example.com/v1/responses",
	})
	status.SetEndReason(relaycommon.StreamEndReasonHandlerStop, errors.New("upstream failed"))
	info := &relaycommon.RelayInfo{
		RequestId:               "req-upstream-failed",
		IsStream:                true,
		StreamStatus:            status,
		UpstreamRequestBodySize: 826000,
	}
	apiErr := types.WithOpenAIError(types.OpenAIError{
		Type: "server_error", Code: "server_error", Message: "upstream failed",
	}, 502)

	event := buildResponsesStreamTerminalEvent(c, info, apiErr, time.Now())
	if event == nil {
		t.Fatal("event is nil")
	}
	if event.TerminalStatus != "upstream_failed" || event.UpstreamTerminalEvent != "error" {
		t.Fatalf("unexpected terminal classification: %#v", event)
	}
	if event.UpstreamHttpStatus != 200 || event.UpstreamResponseId != "resp_failed_1" || event.UpstreamResponseStatus != "failed" {
		t.Fatalf("unexpected upstream terminal metadata: %#v", event)
	}
	if event.UpstreamErrorCode != "server_error" || event.UpstreamRequestBodyBytes != 826000 {
		t.Fatalf("unexpected upstream diagnostics: %#v", event)
	}
	if event.FailureSource != "upstream" {
		t.Fatalf("unexpected failure source: %#v", event)
	}
	if event.UpstreamErrorMessage == "upstream failed at https://api.example.com/v1/responses" {
		t.Fatalf("upstream error message was not masked: %#v", event)
	}
}

func TestBillingSessionLifecycleState(t *testing.T) {
	session := &BillingSession{}
	if got := session.LifecycleState(); got != "initialized" {
		t.Fatalf("initial lifecycle = %q", got)
	}
	session.preConsumedQuota = 1
	if got := session.LifecycleState(); got != "preconsumed" {
		t.Fatalf("preconsumed lifecycle = %q", got)
	}
	session.refunded = true
	if got := session.LifecycleState(); got != "refund_requested" {
		t.Fatalf("refund lifecycle = %q", got)
	}
}
