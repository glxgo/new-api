package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func OaiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	// read response body
	var responsesResponse dto.OpenAIResponsesResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	err = common.Unmarshal(responseBody, &responsesResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := responsesResponse.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	if responsesResponse.HasImageGenerationCall() {
		c.Set("image_generation_call", true)
		c.Set("image_generation_call_quality", responsesResponse.GetQuality())
		c.Set("image_generation_call_size", responsesResponse.GetSize())
	}

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	// compute usage
	usage := dto.Usage{}
	if responsesResponse.Usage != nil {
		usage.PromptTokens = responsesResponse.Usage.InputTokens
		usage.CompletionTokens = responsesResponse.Usage.OutputTokens
		usage.TotalTokens = responsesResponse.Usage.TotalTokens
		if responsesResponse.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = responsesResponse.Usage.InputTokensDetails.CachedTokens
		}
	}
	if info == nil || info.ResponsesUsageInfo == nil || info.ResponsesUsageInfo.BuiltInTools == nil {
		return &usage, nil
	}
	// 解析 Tools 用量
	for _, tool := range responsesResponse.Tools {
		buildToolinfo, ok := info.ResponsesUsageInfo.BuiltInTools[common.Interface2String(tool["type"])]
		if !ok || buildToolinfo == nil {
			logger.LogError(c, fmt.Sprintf("BuiltInTools not found for tool type: %v", tool["type"]))
			continue
		}
		buildToolinfo.CallCount++
	}
	return &usage, nil
}

func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid response or response body")
		return nil, types.NewError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse)
	}

	defer service.CloseResponseBodyGracefully(resp)

	var usage = &dto.Usage{}
	var responseTextBuilder strings.Builder
	var failureUsageTextBuilder strings.Builder
	var streamErr *types.NewAPIError
	var latestResponseSnapshot *responsesFailureSnapshot
	lastSequenceNumber := 0
	forwardedEventCount := 0
	pendingPrelude := make([]responsesBufferedEvent, 0, 2)
	failureItemText := make(map[string]string)
	info.ForwardedResponsesEventCount = 0
	info.ResponsesFailureUsageEligible = false
	info.ResponsesFailureUsageEstimated = false

	markFailureUsageProgress := func(eventType string) {
		if isResponsesBillingProgressEvent(eventType) {
			info.ResponsesFailureUsageEligible = true
		}
	}
	setUsageFromResponse := func(response *dto.OpenAIResponsesResponse) {
		if response == nil || response.Usage == nil {
			return
		}
		if response.Usage.InputTokens != 0 {
			usage.PromptTokens = response.Usage.InputTokens
			usage.InputTokens = response.Usage.InputTokens
		}
		if response.Usage.OutputTokens != 0 {
			usage.CompletionTokens = response.Usage.OutputTokens
			usage.OutputTokens = response.Usage.OutputTokens
		}
		if response.Usage.TotalTokens != 0 {
			usage.TotalTokens = response.Usage.TotalTokens
		}
		if response.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = response.Usage.InputTokensDetails.CachedTokens
			usage.PromptTokensDetails.ImageTokens = response.Usage.InputTokensDetails.ImageTokens
			usage.PromptTokensDetails.AudioTokens = response.Usage.InputTokensDetails.AudioTokens
		}
		usage.CompletionTokenDetails = response.Usage.CompletionTokenDetails
	}
	buildFailureUsage := func() *dto.Usage {
		if !info.ResponsesFailureUsageEligible {
			return nil
		}
		if usage.PromptTokens == 0 {
			usage.PromptTokens = info.GetEstimatePromptTokens()
			info.ResponsesFailureUsageEstimated = true
		}
		if usage.CompletionTokens == 0 && failureUsageTextBuilder.Len() > 0 {
			usage.CompletionTokens = service.CountTextToken(failureUsageTextBuilder.String(), info.UpstreamModelName)
			info.ResponsesFailureUsageEstimated = true
		}
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		if usage.TotalTokens == 0 {
			return nil
		}
		if info.ResponsesFailureUsageEstimated {
			usage.UsageSource = "estimated_stream_failure"
		}
		return usage
	}
	failureItemKey := func(itemID string) string {
		key := strings.TrimSpace(itemID)
		if key == "" {
			return "unknown"
		}
		return key
	}
	appendFailureItemPayload := func(itemID, payload string) {
		if payload == "" {
			return
		}
		key := failureItemKey(itemID)
		previous := failureItemText[key]
		if previous != "" && strings.HasPrefix(payload, previous) {
			failureUsageTextBuilder.WriteString(payload[len(previous):])
		} else if previous == "" || payload != previous {
			failureUsageTextBuilder.WriteString(payload)
		}
		failureItemText[key] = payload
	}

	forwardEvent := func(streamResponse dto.ResponsesStreamResponse, data string, sr *helper.StreamResult) bool {
		if err := sendResponsesStreamData(c, streamResponse, data); err != nil {
			streamErr = types.NewErrorWithStatusCode(
				err,
				types.ErrorCodeChannelIncompleteStream,
				http.StatusBadGateway,
				types.ErrOptionWithSkipRetry(),
			)
			sr.Error(err)
			return false
		}
		forwardedEventCount++
		info.ForwardedResponsesEventCount = forwardedEventCount
		return true
	}
	flushPrelude := func(sr *helper.StreamResult) bool {
		for _, event := range pendingPrelude {
			if !forwardEvent(event.Response, event.Data, sr) {
				return false
			}
		}
		pendingPrelude = pendingPrelude[:0]
		return true
	}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {

		// 检查当前数据是否包含 completed 状态和 usage 信息
		var streamResponse dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
			logger.LogError(c, "failed to unmarshal stream response: "+err.Error())
			sr.Error(err)
			return
		}
		info.UpstreamEventBytes += int64(len(data))
		info.UpstreamLastEventType = streamResponse.Type
		if streamResponse.SequenceNumber != nil {
			info.UpstreamLastSequence = *streamResponse.SequenceNumber
			if *streamResponse.SequenceNumber > lastSequenceNumber {
				lastSequenceNumber = *streamResponse.SequenceNumber
			}
		}
		if streamResponse.Response != nil && info.StreamStatus != nil {
			info.StreamStatus.SetUpstreamResponseID(streamResponse.Response.ID)
			latestResponseSnapshot = newResponsesFailureSnapshot(streamResponse.Response)
			setUsageFromResponse(streamResponse.Response)
			if streamResponse.Response.Usage != nil &&
				(streamResponse.Response.Usage.InputTokens > 0 || streamResponse.Response.Usage.OutputTokens > 0 || streamResponse.Response.Usage.TotalTokens > 0) {
				info.ResponsesFailureUsageEligible = true
			}
		}
		if streamResponse.Type == "error" || streamResponse.Type == "response.failed" || streamResponse.Type == "response.error" {
			recordResponsesUpstreamTerminal(info, resp.StatusCode, streamResponse)
			streamErr = responsesTerminalError(streamResponse, types.ErrorCodeUpstreamResponseFailed, false)
			nonRetryableRequestError := isNonRetryableResponsesRequestError(streamErr)

			// Prelude-only failures have not exposed a response ID or output to the
			// downstream yet. Keep those control events buffered and let the outer
			// channel loop retry transparently for transient provider/transport
			// failures. Billing is outside that loop, so this does not pre-consume
			// twice. Deterministic request errors must be surfaced instead.
			if forwardedEventCount == 0 && !nonRetryableRequestError {
				pendingPrelude = pendingPrelude[:0]
				sr.Stop(streamErr)
				return
			}

			types.ErrOptionWithSkipRetry()(streamErr)
			if common.CyberPolicyInterceptionEnabled && service.IsCyberPolicyError(streamErr) {
				if interceptedData, rewriteErr := cyberPolicyInterceptionStreamData(c, streamResponse); rewriteErr != nil {
					logger.LogError(c, "failed to rewrite cyber policy stream response: "+rewriteErr.Error())
				} else {
					data = interceptedData
				}
			}

			// Official Codex clients do not treat event:error as a Responses
			// terminal event. Convert only deterministic request faults; transient
			// provider failures remain reconnectable, matching CPA's compatibility
			// policy and avoiding a hard failure where recovery is possible.
			if (streamResponse.Type == "error" || streamResponse.Type == "response.error") &&
				nonRetryableRequestError && isCodexResponsesClient(c) {
				clientError := streamResponse.GetOpenAIError()
				if clientError == nil || (common.CyberPolicyInterceptionEnabled && service.IsCyberPolicyError(streamErr)) {
					safeError := streamErr.ToOpenAIError()
					clientError = &safeError
				}
				sequenceNumber := streamResponse.SequenceNumber
				if sequenceNumber == nil {
					sequenceNumber = common.GetPointer(lastSequenceNumber + 1)
				}
				converted, convertedData, convertErr := buildResponsesFailedEvent(
					c,
					info,
					latestResponseSnapshot,
					sequenceNumber,
					*clientError,
				)
				if convertErr != nil {
					logger.LogError(c, "failed to convert Responses error event: "+convertErr.Error())
				} else {
					streamResponse = converted
					data = convertedData
				}
			}

			// Once semantic output was forwarded, never replay another channel.
			// For a transient Codex error, close the typed stream cleanly and let
			// the client perform its own recovery; forwarding event:error would be
			// misread as a non-terminal event by Codex.
			if !(forwardedEventCount > 0 && !nonRetryableRequestError && isCodexResponsesClient(c)) {
				if !forwardEvent(streamResponse, data, sr) {
					return
				}
			}
			common.SetContextKey(c, constant.ContextKeyRelayErrorAlreadyStreamed, true)
			sr.Stop(streamErr)
			return
		}

		if isResponsesPreludeEvent(streamResponse.Type) {
			pendingPrelude = append(pendingPrelude, responsesBufferedEvent{Response: streamResponse, Data: data})
			return
		}
		markFailureUsageProgress(streamResponse.Type)
		if !flushPrelude(sr) || !forwardEvent(streamResponse, data, sr) {
			return
		}
		switch streamResponse.Type {
		case "response.completed", "response.done":
			if streamResponse.Response != nil {
				recordResponsesUpstreamTerminal(info, resp.StatusCode, streamResponse)
				setUsageFromResponse(streamResponse.Response)
				if streamResponse.Response.HasImageGenerationCall() {
					c.Set("image_generation_call", true)
					c.Set("image_generation_call_quality", streamResponse.Response.GetQuality())
					c.Set("image_generation_call_size", streamResponse.Response.GetSize())
				}
			}
			sr.Done()
		case "response.incomplete":
			markFailureUsageProgress(streamResponse.Type)
			recordResponsesUpstreamTerminal(info, resp.StatusCode, streamResponse)
			streamErr = responsesTerminalError(streamResponse, types.ErrorCodeUpstreamResponseIncomplete, true)
			common.SetContextKey(c, constant.ContextKeyRelayErrorAlreadyStreamed, true)
			sr.Stop(streamErr)
		case "response.output_text.delta":
			// 处理输出文本
			responseTextBuilder.WriteString(streamResponse.Delta)
			failureUsageTextBuilder.WriteString(streamResponse.Delta)
		case "response.reasoning_text.delta", "response.reasoning_summary_text.delta":
			failureUsageTextBuilder.WriteString(streamResponse.Delta)
		case "response.function_call_arguments.delta", "response.custom_tool_call_input.delta":
			key := failureItemKey(streamResponse.ItemID)
			appendFailureItemPayload(streamResponse.ItemID, failureItemText[key]+streamResponse.Delta)
		case dto.ResponsesOutputTypeItemAdded, dto.ResponsesOutputTypeItemDone:
			// 函数调用处理
			if streamResponse.Item != nil {
				if args := streamResponse.Item.ArgumentsString(); args != "" {
					appendFailureItemPayload(streamResponse.Item.ID, args)
				} else if input := common.JsonRawMessageToString(streamResponse.Item.Input); input != "" {
					appendFailureItemPayload(streamResponse.Item.ID, input)
				}
				if streamResponse.Type == dto.ResponsesOutputTypeItemDone {
					switch streamResponse.Item.Type {
					case dto.BuildInCallWebSearchCall:
						if info != nil && info.ResponsesUsageInfo != nil && info.ResponsesUsageInfo.BuiltInTools != nil {
							if webSearchTool, exists := info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; exists && webSearchTool != nil {
								webSearchTool.CallCount++
							}
						}
					}
				}
			}
		}
	})

	if streamErr != nil {
		service.PreventChannelAffinityRecord(c)
		service.ClearCurrentChannelAffinityCache(c)
		return buildFailureUsage(), streamErr
	}

	if info.StreamStatus == nil || info.StreamStatus.EndReason != relaycommon.StreamEndReasonDone {
		streamSummary := "status unavailable"
		if info.StreamStatus != nil {
			streamSummary = info.StreamStatus.Summary()
		}
		streamErr := fmt.Errorf("responses stream ended before response.completed: %s", streamSummary)
		logResponsesEOFTransportTrace(c, info)
		service.PreventChannelAffinityRecord(c)
		service.ClearCurrentChannelAffinityCache(c)

		errorOptions := []types.NewAPIErrorOptions{}
		if forwardedEventCount > 0 || c.Request.Context().Err() != nil {
			// Retrying after forwarding upstream events can duplicate output. Let
			// the client reconnect; the cleared affinity will route that retry away
			// from the failed channel.
			errorOptions = append(errorOptions, types.ErrOptionWithSkipRetry())
		}
		if forwardedEventCount > 0 {
			// The downstream already received SSE data. Prevent the controller from
			// corrupting that stream by appending a non-SSE JSON error document.
			common.SetContextKey(c, constant.ContextKeyRelayErrorAlreadyStreamed, true)
		}
		incompleteErr := types.NewErrorWithStatusCode(
			streamErr,
			types.ErrorCodeChannelIncompleteStream,
			http.StatusBadGateway,
			errorOptions...,
		)
		return buildFailureUsage(), incompleteErr
	}

	if usage.CompletionTokens == 0 {
		// 计算输出文本的 token 数量
		tempStr := responseTextBuilder.String()
		if len(tempStr) > 0 {
			// 非正常结束，使用输出文本的 token 数量
			completionTokens := service.CountTextToken(tempStr, info.UpstreamModelName)
			usage.CompletionTokens = completionTokens
		}
	}

	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens

	return usage, nil
}

func isResponsesBillingProgressEvent(eventType string) bool {
	eventType = strings.TrimSpace(eventType)
	if eventType == "response.incomplete" {
		return true
	}
	if strings.HasSuffix(eventType, ".delta") {
		return strings.HasPrefix(eventType, "response.output_") ||
			strings.HasPrefix(eventType, "response.reasoning_") ||
			strings.HasPrefix(eventType, "response.function_call_") ||
			strings.HasPrefix(eventType, "response.custom_tool_call_")
	}
	return eventType == "response.output_item.added" || eventType == "response.output_item.done" ||
		eventType == "response.content_part.added" || eventType == "response.content_part.done"
}

type responsesBufferedEvent struct {
	Response dto.ResponsesStreamResponse
	Data     string
}

func isResponsesPreludeEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "response.created", "response.in_progress", "response.queued":
		return true
	default:
		return false
	}
}

func isNonRetryableResponsesRequestError(apiErr *types.NewAPIError) bool {
	if apiErr == nil {
		return false
	}
	if service.IsCyberPolicyError(apiErr) {
		return true
	}
	openAIError := apiErr.ToOpenAIError()
	errorType := strings.ToLower(strings.TrimSpace(openAIError.Type))
	code := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", openAIError.Code)))
	if errorType == "invalid_request_error" {
		return true
	}
	if strings.HasPrefix(code, "invalid_") || strings.HasPrefix(code, "unsupported_") {
		return true
	}
	switch code {
	case "context_length_exceeded",
		"context_window_exceeded",
		"previous_response_not_found",
		"item_not_found",
		"invalid_encrypted_content",
		"cyber_policy":
		return true
	default:
		return false
	}
}

func isCodexResponsesClient(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	userAgent := strings.ToLower(strings.TrimSpace(c.Request.UserAgent()))
	originator := strings.ToLower(strings.TrimSpace(c.GetHeader("Originator")))
	return hasAnyResponsesClientPrefix(userAgent, []string{
		"codex desktop/",
		"codex-tui/",
		"codex_cli_rs/",
		"cc-switch/",
		"ccswitch/",
	}) || hasAnyResponsesClientPrefix(originator, []string{
		"codex desktop",
		"codex-tui",
		"codex_cli_rs",
		"codex cli",
		"cc-switch",
		"ccswitch",
	})
}

func hasAnyResponsesClientPrefix(value string, prefixes []string) bool {
	if value == "" {
		return false
	}
	for _, prefix := range prefixes {
		if value == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

type responsesFailureSnapshot struct {
	ID        string
	Object    string
	CreatedAt int
	Model     string
	Store     bool
}

func newResponsesFailureSnapshot(response *dto.OpenAIResponsesResponse) *responsesFailureSnapshot {
	if response == nil {
		return nil
	}
	return &responsesFailureSnapshot{
		ID:        response.ID,
		Object:    response.Object,
		CreatedAt: response.CreatedAt,
		Model:     response.Model,
		Store:     response.Store,
	}
}

func buildResponsesFailedEvent(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	snapshot *responsesFailureSnapshot,
	sequenceNumber *int,
	openAIError types.OpenAIError,
) (dto.ResponsesStreamResponse, string, error) {
	responseID := "resp_gateway_failed"
	if c != nil {
		if requestID := c.GetString(common.RequestIdKey); requestID != "" {
			responseID = "resp_gateway_" + requestID
		}
	}
	object := "response"
	createdAt := time.Now().Unix()
	modelName := ""
	store := false
	if info != nil {
		modelName = info.UpstreamModelName
	}
	if snapshot != nil {
		if snapshot.ID != "" {
			responseID = snapshot.ID
		}
		if snapshot.Object != "" {
			object = snapshot.Object
		}
		if snapshot.CreatedAt > 0 {
			createdAt = int64(snapshot.CreatedAt)
		}
		if snapshot.Model != "" {
			modelName = snapshot.Model
		}
		store = snapshot.Store
	}
	if strings.TrimSpace(fmt.Sprintf("%v", openAIError.Code)) == "" {
		openAIError.Code = types.ErrorCodeUpstreamResponseFailed
	}
	if openAIError.Message == "" {
		openAIError.Message = "The upstream rejected the Responses request."
	}

	sequence := 0
	if sequenceNumber != nil {
		sequence = *sequenceNumber
	}
	event := map[string]any{
		"type":            "response.failed",
		"sequence_number": sequence,
		"response": map[string]any{
			"id":                 responseID,
			"object":             object,
			"created_at":         createdAt,
			"status":             "failed",
			"error":              openAIError,
			"incomplete_details": nil,
			"model":              modelName,
			"output":             []any{},
			"store":              store,
			"usage":              nil,
		},
	}
	data, err := common.Marshal(event)
	if err != nil {
		return dto.ResponsesStreamResponse{}, "", err
	}
	return dto.ResponsesStreamResponse{
		Type:           "response.failed",
		SequenceNumber: common.GetPointer(sequence),
	}, string(data), nil
}

func logResponsesEOFTransportTrace(c *gin.Context, info *relaycommon.RelayInfo) {
	if info == nil {
		return
	}
	trace := relaycommon.UpstreamTransportSnapshot{}
	if info.UpstreamTransportTrace != nil {
		trace = info.UpstreamTransportTrace.Snapshot()
	}
	firstEventMs := int64(0)
	if !info.UpstreamStartTime.IsZero() && !info.FirstResponseTime.IsZero() && !info.FirstResponseTime.Before(info.UpstreamStartTime) {
		firstEventMs = info.FirstResponseTime.Sub(info.UpstreamStartTime).Milliseconds()
	}
	durationMs := int64(0)
	if !info.UpstreamStartTime.IsZero() {
		durationMs = time.Since(info.UpstreamStartTime).Milliseconds()
	}
	logger.LogInfo(c, fmt.Sprintf(
		"responses EOF transport trace: channel=%d protocol=%s proxy=%t header_ms=%d first_event_ms=%d duration_ms=%d conn_reused=%t conn_was_idle=%t conn_idle_ms=%d conn_fp=%s request_body_bytes=%d estimated_prompt_tokens=%d received_events=%d forwarded_events=%d upstream_event_bytes=%d response_bytes=%d last_event=%s last_sequence=%d",
		info.ChannelId,
		trace.Protocol,
		info.UpstreamProxyUsed,
		trace.ResponseHeaderLatency.Milliseconds(),
		firstEventMs,
		durationMs,
		trace.ConnectionReused,
		trace.ConnectionWasIdle,
		trace.ConnectionIdleTime.Milliseconds(),
		trace.ConnectionFingerprint,
		info.UpstreamRequestBodySize,
		info.GetEstimatePromptTokens(),
		info.ReceivedResponseCount,
		info.ForwardedResponsesEventCount,
		info.UpstreamEventBytes,
		c.Writer.Size(),
		info.UpstreamLastEventType,
		info.UpstreamLastSequence,
	))
}

func cyberPolicyInterceptionStreamData(c *gin.Context, streamResponse dto.ResponsesStreamResponse) (string, error) {
	requestId := ""
	if c != nil {
		requestId = c.GetString(common.RequestIdKey)
	}
	message := common.MessageWithRequestId(service.CyberPolicyInterceptionMessage(), requestId)
	safeError := types.OpenAIError{
		Message: message,
		Type:    "invalid_request_error",
		Code:    model.UserSecurityErrorCodeContentPolicyBlocked,
	}
	if streamResponse.Type == "error" {
		streamResponse.Code = safeError.Code
		streamResponse.Message = safeError.Message
		streamResponse.Param = ""
		streamResponse.Error = nil
	} else if streamResponse.Response != nil {
		streamResponse.Response.Error = safeError
	}
	if streamResponse.Type != "error" && (streamResponse.Response == nil || streamResponse.Error != nil) {
		streamResponse.Error = safeError
	}
	bytes, err := common.Marshal(streamResponse)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func recordResponsesUpstreamTerminal(info *relaycommon.RelayInfo, upstreamHTTPStatus int, streamResponse dto.ResponsesStreamResponse) {
	if info == nil || info.StreamStatus == nil {
		return
	}
	terminal := relaycommon.UpstreamTerminal{
		EventType:  streamResponse.Type,
		HTTPStatus: upstreamHTTPStatus,
	}
	terminal.ResponseID = info.StreamStatus.UpstreamTerminalSnapshot().ResponseID
	if streamResponse.Response != nil {
		terminal.ResponseID = streamResponse.Response.ID
		terminal.ResponseStatus = jsonRawString(streamResponse.Response.Status)
		if streamResponse.Response.IncompleteDetails != nil {
			terminal.IncompleteReason = strings.TrimSpace(streamResponse.Response.IncompleteDetails.Reason)
			if terminal.IncompleteReason == "" {
				terminal.IncompleteReason = strings.TrimSpace(streamResponse.Response.IncompleteDetails.Reasoning)
			}
		}
		if oaiErr := streamResponse.Response.GetOpenAIError(); oaiErr != nil {
			terminal.ErrorCode = fmt.Sprintf("%v", oaiErr.Code)
			terminal.ErrorMessage = oaiErr.Message
		}
	}
	if terminal.ErrorCode == "" || terminal.ErrorMessage == "" {
		if oaiErr := streamResponse.GetOpenAIError(); oaiErr != nil {
			if terminal.ErrorCode == "" {
				terminal.ErrorCode = fmt.Sprintf("%v", oaiErr.Code)
			}
			if terminal.ErrorMessage == "" {
				terminal.ErrorMessage = oaiErr.Message
			}
		}
	}
	info.StreamStatus.SetUpstreamTerminal(terminal)
}

func responsesTerminalError(streamResponse dto.ResponsesStreamResponse, fallbackCode types.ErrorCode, skipRetry bool) *types.NewAPIError {
	oaiErr := streamResponse.GetOpenAIError()
	errorOptions := make([]types.NewAPIErrorOptions, 0, 1)
	if skipRetry {
		errorOptions = append(errorOptions, types.ErrOptionWithSkipRetry())
	}
	if oaiErr != nil {
		if strings.TrimSpace(fmt.Sprintf("%v", oaiErr.Code)) == "" {
			oaiErr.Code = fallbackCode
		}
		if oaiErr.Message == "" {
			oaiErr.Message = fmt.Sprintf("upstream Responses terminal event: %s", streamResponse.Type)
		}
		return types.WithOpenAIError(*oaiErr, http.StatusBadGateway, errorOptions...)
	}

	code := fallbackCode
	message := fmt.Sprintf("upstream Responses terminal event: %s", streamResponse.Type)
	if streamResponse.Response != nil && streamResponse.Response.IncompleteDetails != nil {
		reason := strings.TrimSpace(streamResponse.Response.IncompleteDetails.Reason)
		if reason == "" {
			reason = strings.TrimSpace(streamResponse.Response.IncompleteDetails.Reasoning)
		}
		if reason != "" {
			if fallbackCode == types.ErrorCodeUpstreamResponseIncomplete {
				code = types.ErrorCode(string(fallbackCode) + ":" + reason)
			}
			message += ", reason=" + reason
		}
	}
	return types.NewOpenAIError(errors.New(message), code, http.StatusBadGateway, errorOptions...)
}

func jsonRawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	return strings.TrimSpace(string(raw))
}
