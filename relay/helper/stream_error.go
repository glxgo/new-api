package helper

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const streamOutputKey = "relay_stream_output_started"
const streamWriteErrorKey = "relay_stream_write_error"

func HasStreamOutput(c *gin.Context) bool {
	return c != nil && c.GetBool(streamOutputKey)
}

func StreamWriteError(c *gin.Context) error {
	if c == nil {
		return nil
	}
	value, _ := c.Get(streamWriteErrorKey)
	err, _ := value.(error)
	return err
}

func rememberStreamWriteError(c *gin.Context, err error) error {
	if c != nil && err != nil {
		c.Set(streamWriteErrorKey, err)
	}
	return err
}

// StreamFailure turns a recorded scanner failure into a relay failure before
// adapters send a successful terminator or settle the request as successful.
// EOF is valid for protocols without a mandatory terminal event, but an empty
// response, a transport error and a fatal handler error are never successes.
func StreamFailure(c *gin.Context, info *relaycommon.RelayInfo, requireDone bool) *types.NewAPIError {
	if err := StreamWriteError(c); err != nil {
		return types.NewErrorWithStatusCode(err, "client_write_error", 499, types.ErrOptionWithSkipRetry())
	}
	if info == nil || info.StreamStatus == nil {
		return nil
	}
	status := info.StreamStatus
	if status.EndReason == relaycommon.StreamEndReasonDone && info.ReceivedResponseCount > 0 {
		return nil
	}
	if c != nil && c.Request != nil && c.Request.Context().Err() != nil {
		return types.NewErrorWithStatusCode(c.Request.Context().Err(), "client_canceled", 499, types.ErrOptionWithSkipRetry())
	}
	if status.EndReason == relaycommon.StreamEndReasonEOF && !requireDone && info.ReceivedResponseCount > 0 {
		return nil
	}
	if status.EndReason == relaycommon.StreamEndReasonHandlerStop && status.EndError == nil {
		return nil
	}
	options := []types.NewAPIErrorOptions{}
	if HasStreamOutput(c) {
		options = append(options, types.ErrOptionWithSkipRetry())
	}
	var providerErr *types.NewAPIError
	if errors.As(status.EndError, &providerErr) {
		if HasStreamOutput(c) {
			types.ErrOptionWithSkipRetry()(providerErr)
		}
		return providerErr
	}
	code := http.StatusBadGateway
	if status.EndReason == relaycommon.StreamEndReasonTimeout {
		code = http.StatusGatewayTimeout
	}
	if status.EndReason == relaycommon.StreamEndReasonClientGone || status.EndReason == relaycommon.StreamEndReasonPingFail {
		code = 499
		options = append(options, types.ErrOptionWithSkipRetry())
	}
	return types.NewErrorWithStatusCode(fmt.Errorf("upstream stream did not complete: %s", status.Summary()), types.ErrorCodeChannelIncompleteStream, code, options...)
}

// ParseStreamError catches HTTP-200 streams carrying an explicit JSON error.
// Adapters must check this before treating an empty choices/candidates array as
// successful output. Malformed JSON is left to their normal decoder.
func ParseStreamError(data string) *types.NewAPIError {
	var envelope dto.SimpleResponse
	if common.UnmarshalJsonStr(data, &envelope) != nil {
		return nil
	}
	upstream := envelope.GetOpenAIError()
	if upstream == nil || (upstream.Message == "" && upstream.Type == "" && upstream.Code == nil) {
		return nil
	}
	if upstream.Code == nil {
		upstream.Code = types.ErrorCodeUpstreamResponseFailed
	}
	status := http.StatusBadGateway
	if code, ok := upstream.Code.(float64); ok && code >= 400 && code <= 599 {
		status = int(code)
	}
	return types.WithOpenAIError(*upstream, status)
}

// WriteStreamError uses the client's wire format once HTTP headers have been
// committed (including by a heartbeat). It never appends a plain JSON document
// to an SSE response, and never emits a successful [DONE] after a failure.
func WriteStreamError(c *gin.Context, format types.RelayFormat, apiErr *types.NewAPIError) error {
	if c == nil || apiErr == nil {
		return nil
	}
	if StreamWriteError(c) != nil {
		return StreamWriteError(c)
	}
	if c.Request != nil && c.Request.Context().Err() != nil {
		return c.Request.Context().Err()
	}
	var event string
	var payload any
	switch format {
	case types.RelayFormatClaude:
		event = "error"
		payload = gin.H{"type": "error", "error": apiErr.ToClaudeError()}
	case types.RelayFormatOpenAIResponses:
		event = "response.failed"
		payload = gin.H{"type": event, "sequence_number": 0, "response": gin.H{
			"id": "resp_gateway_" + c.GetString(common.RequestIdKey), "object": "response",
			"status": "failed", "error": apiErr.ToOpenAIError(), "output": []any{},
		}}
	default:
		payload = gin.H{"error": apiErr.ToOpenAIError()}
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	err = ResponseChunkData(c, dto.ResponsesStreamResponse{Type: event}, string(data))
	common.SetContextKey(c, constant.ContextKeyRelayErrorAlreadyStreamed, true)
	return err
}

func IsSSEWritten(c *gin.Context) bool {
	return c != nil && c.Writer != nil && c.Writer.Written() && strings.HasPrefix(c.Writer.Header().Get("Content-Type"), "text/event-stream")
}
