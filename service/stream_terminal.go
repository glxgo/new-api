package service

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const maxStreamTerminalErrorRunes = 2048

const ingressRequestIDHeader = "X-Request-ID"

type billingLifecycleReporter interface {
	LifecycleState() string
}

// RecordResponsesStreamTerminal persists the transport outcome independently
// from the consumption log. In particular, refunded and client-cancelled
// streams remain queryable even when no consumption row is written.
func RecordResponsesStreamTerminal(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) {
	event := buildResponsesStreamTerminalEvent(c, info, apiErr, time.Now())
	if event == nil {
		return
	}
	if err := model.RecordStreamTerminalEvent(event); err != nil {
		logger.LogError(c, "failed to persist Responses stream terminal event: "+err.Error())
	}
}

func buildResponsesStreamTerminalEvent(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError, now time.Time) *model.StreamTerminalEvent {
	if c == nil || info == nil || !info.IsStream || c.Request == nil {
		return nil
	}
	path := c.Request.URL.Path
	if path != "/v1/responses" && path != "/v1/responses/compact" {
		return nil
	}

	startedAt := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if startedAt.IsZero() {
		startedAt = info.StartTime
	}
	if startedAt.IsZero() {
		startedAt = now
	}
	duration := now.Sub(startedAt)
	if duration < 0 {
		duration = 0
	}

	requestID := c.GetString(common.RequestIdKey)
	if requestID == "" {
		requestID = info.RequestId
	}

	actualStatus := 0
	responseBytes := int64(0)
	if c.Writer != nil {
		if c.Writer.Written() {
			actualStatus = c.Writer.Status()
		}
		if size := c.Writer.Size(); size > 0 {
			responseBytes = int64(size)
		}
	}
	intendedStatus := actualStatus
	if apiErr != nil {
		intendedStatus = apiErr.StatusCode
		if intendedStatus == 0 {
			intendedStatus = 500
		}
	}
	if actualStatus == 0 {
		actualStatus = intendedStatus
	}
	if actualStatus == 0 {
		actualStatus = 200
	}
	if intendedStatus == 0 {
		intendedStatus = actualStatus
	}

	endReason := "unknown"
	endError := ""
	softErrorCount := 0
	responseCompleted := false
	clientGone := false
	terminalStatus := "failed"
	if info.StreamStatus != nil {
		endReason = string(info.StreamStatus.EndReason)
		if endReason == "" {
			endReason = "unknown"
		}
		if info.StreamStatus.EndError != nil {
			endError = info.StreamStatus.EndError.Error()
		}
		softErrorCount = info.StreamStatus.TotalErrorCount()
		responseCompleted = info.StreamStatus.EndReason == relaycommon.StreamEndReasonDone
		clientGone = info.StreamStatus.EndReason == relaycommon.StreamEndReasonClientGone
	}
	// net/http cancels the request context when a successfully completed
	// handler returns as well. Treat it as client_gone only while the stream is
	// still incomplete; response.completed is authoritative for Responses.
	if !responseCompleted && c.Request.Context().Err() != nil {
		clientGone = true
	}
	if apiErr != nil {
		if endReason == "unknown" {
			endReason = "request_error"
		}
		if endError == "" {
			endError = apiErr.MaskSensitiveErrorWithStatusCode()
		}
	}
	switch {
	case apiErr == nil && responseCompleted:
		terminalStatus = "completed"
		clientGone = false
	case clientGone:
		terminalStatus = "client_gone"
	}

	billingState := "none"
	if info.Billing != nil {
		billingState = "unknown"
		if reporter, ok := info.Billing.(billingLifecycleReporter); ok {
			billingState = reporter.LifecycleState()
		}
	}

	channelID := 0
	if info.ChannelMeta != nil {
		channelID = info.ChannelId
	}
	affinityRuleName := ""
	affinityKeyFp := ""
	if affinity, ok := GetChannelAffinityStatsContext(c); ok {
		affinityRuleName = affinity.RuleName
		affinityKeyFp = affinity.KeyFingerprint
	}
	event := &model.StreamTerminalEvent{
		RequestId:         requestID,
		IngressRequestId:  normalizeIngressRequestID(c.Request.Header.Get(ingressRequestIDHeader)),
		UpstreamRequestId: c.GetString(common.UpstreamRequestIdKey),
		CreatedAt:         now.Unix(),
		StartedAt:         startedAt.Unix(),
		DurationMs:        duration.Milliseconds(),
		UserId:            info.UserId,
		TokenId:           info.TokenId,
		ChannelId:         channelID,
		ModelName:         info.OriginModelName,
		Group:             info.UsingGroup,
		AffinityRuleName:  affinityRuleName,
		AffinityKeyFp:     affinityKeyFp,
		RequestHost:       c.Request.Host,
		RequestPath:       path,
		TerminalStatus:    terminalStatus,
		EndReason:         endReason,
		EndError:          truncateRunes(common.MaskSensitiveInfo(endError), maxStreamTerminalErrorRunes),
		HttpStatus:        actualStatus,
		IntendedStatus:    intendedStatus,
		ResponseBytes:     responseBytes,
		ReceivedEvents:    info.ReceivedResponseCount,
		SoftErrorCount:    softErrorCount,
		ResponseCompleted: responseCompleted,
		ClientGone:        clientGone,
		BillingSource:     info.BillingSource,
		BillingState:      billingState,
		PreConsumedQuota:  info.FinalPreConsumedQuota,
		UsedChannels:      strings.Join(c.GetStringSlice("use_channel"), ","),
	}
	if apiErr != nil {
		event.ErrorType = string(apiErr.GetErrorType())
		event.ErrorCode = string(apiErr.GetErrorCode())
	}
	return event
}

func normalizeIngressRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 {
		return ""
	}
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		switch ch {
		case '-', '_', '.', ':':
			continue
		default:
			return ""
		}
	}
	return value
}

func truncateRunes(value string, max int) string {
	if max <= 0 || utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}
