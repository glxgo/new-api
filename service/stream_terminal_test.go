package service

import (
	"context"
	"errors"
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
	completedContext, cancelCompleted := context.WithCancel(c.Request.Context())
	cancelCompleted()
	c.Request = c.Request.WithContext(completedContext)
	c.Set(common.RequestIdKey, "req-completed")
	c.Set(common.UpstreamRequestIdKey, "upstream-1")
	c.Set("use_channel", []string{"8", "31"})
	started := time.Unix(100, 0)
	common.SetContextKey(c, constant.ContextKeyRequestStartTime, started)
	c.Status(200)
	_, _ = c.Writer.WriteString("data: response.completed\n\n")
	status := relaycommon.NewStreamStatus()
	status.SetEndReason(relaycommon.StreamEndReasonDone, nil)
	info := &relaycommon.RelayInfo{
		RequestId:             "req-completed",
		UserId:                264,
		TokenId:               9,
		UsingGroup:            "default",
		OriginModelName:       "gpt-5.6-sol",
		IsStream:              true,
		StreamStatus:          status,
		BillingSource:         BillingSourceWallet,
		ChannelMeta:           &relaycommon.ChannelMeta{ChannelId: 31},
		FinalPreConsumedQuota: 123,
	}

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
