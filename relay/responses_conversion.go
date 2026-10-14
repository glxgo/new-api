package relay

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	kittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func responsesConversionTarget(info *relaycommon.RelayInfo) kittypes.RelayFormat {
	switch info.ApiType {
	case constant.APITypeAnthropic:
		return kittypes.RelayFormatClaude
	case constant.APITypeGemini:
		return kittypes.RelayFormatGemini
	case constant.APITypeOpenAI:
		if info.ChannelSetting.ResponsesToChat {
			return kittypes.RelayFormatOpenAI
		}
	}
	return ""
}

func responsesViaConversion(c *gin.Context, info *relaycommon.RelayInfo, request *dto.OpenAIResponsesRequest, target kittypes.RelayFormat) (apiError *types.NewAPIError) {
	defer func() {
		if apiError != nil {
			service.ResetStatusCode(apiError, c.GetString("status_code_mapping"))
		}
	}()
	converted, meta, err := service.ConvertResponsesWithRelayKit(c, info, request, target)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeConvertRequestFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	data, err := common.Marshal(converted.Value)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed)
	}
	data, err = relaycommon.RemoveDisabledFields(data, info.ChannelOtherSettings, false)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed)
	}
	if len(info.ParamOverride) > 0 {
		data, err = relaycommon.ApplyParamOverrideWithRelayInfo(data, info)
		if err != nil {
			return newAPIErrorFromParamOverride(err)
		}
	}
	if apiErr := service.PreparePriorityBillingForOutbound(info, data); apiErr != nil {
		return apiErr
	}
	info.FinalRequestRelayFormat = types.RelayFormat(target)
	relaycommon.AppendRequestConversionFromRequest(info, converted.Value)
	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(errors.New("unsupported conversion adapter"), types.ErrorCodeInvalidApiType)
	}
	adaptor.Init(info)
	oldMode, oldPath := info.RelayMode, info.RequestURLPath
	if target == kittypes.RelayFormatOpenAI {
		info.RelayMode, info.RequestURLPath = relayconstant.RelayModeChatCompletions, "/v1/chat/completions"
	}
	info.UpstreamRequestBodySize = int64(len(data))
	info.UpstreamRequestGetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
	raw, err := adaptor.DoRequest(c, info, bytes.NewReader(data))
	info.RelayMode, info.RequestURLPath = oldMode, oldPath
	if err != nil {
		return types.NewError(err, types.ErrorCodeDoRequestFailed)
	}
	response, ok := raw.(*http.Response)
	if !ok || response == nil || response.Body == nil {
		return types.NewError(errors.New("empty upstream response"), types.ErrorCodeBadResponse)
	}
	if response.StatusCode != http.StatusOK {
		return service.RelayErrorHandler(c.Request.Context(), response, false)
	}
	usage, apiErr := convertedResponsesResult(c, info, meta, target, response)
	if apiErr != nil {
		service.SettleResponsesStreamFailureUsage(c, info, usage)
		return apiErr
	}
	service.PostTextConsumeQuota(c, info, usage, nil)
	return nil
}

func decodeConversionResponse(data []byte, from kittypes.RelayFormat, stream bool) (any, error) {
	var value any
	switch from {
	case kittypes.RelayFormatClaude:
		value = &kitdto.ClaudeResponse{}
	case kittypes.RelayFormatGemini:
		value = &kitdto.GeminiChatResponse{}
	default:
		if stream {
			value = &kitdto.ChatCompletionsStreamResponse{}
		} else {
			value = &kitdto.OpenAITextResponse{}
		}
	}
	return value, common.Unmarshal(data, value)
}

func convertedResponsesResult(c *gin.Context, info *relaycommon.RelayInfo, meta *convmeta.Values, from kittypes.RelayFormat, response *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(response)
	fail := func(err error) *types.NewAPIError {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway, types.ErrOptionWithSkipRetry())
	}
	if !info.IsStream {
		data, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, fail(err)
		}
		data = info.ObserveKiroPayload(data, false)
		if apiErr := info.KiroCompletionError(); apiErr != nil {
			return nil, apiErr
		}
		if gjson.GetBytes(data, "error").Exists() {
			return nil, fail(errors.New("upstream returned an error"))
		}
		native, err := decodeConversionResponse(data, from, false)
		if err != nil {
			return nil, fail(err)
		}
		result, err := relayconvert.ConvertResponse(service.RelayKitContext(c), meta, kittypes.RelayFormatOpenAIResponses, native)
		if err != nil {
			return nil, fail(err)
		}
		usage, err := service.RelayKitUsage(result.Usage)
		if err != nil {
			return nil, fail(err)
		}
		output, err := common.Marshal(result.Value)
		if err != nil {
			return usage, fail(err)
		}
		service.IOCopyBytesGracefully(c, response, common.SanitizeErrorJSON(output))
		return usage, nil
	}
	state, err := relayconvert.NewResponseStreamState(from, kittypes.RelayFormatOpenAIResponses, relayconvert.ResponseStreamOptions{
		ID: "resp_" + common.GetUUID(), Model: info.OriginModelName, Created: time.Now().Unix(), IncludeUsage: true, EmitSequenceNumber: true,
	})
	if err != nil {
		return nil, fail(err)
	}
	var streamErr error
	terminal := false
	var completed []relayconvert.ResponseResult
	flushCompleted := false
	emit := func(results []relayconvert.ResponseResult) error {
		for _, result := range results {
			value := result.Value
			switch event := value.(type) {
			case relayconvert.ChatToResponsesStreamEvent:
				value = event.Payload
			case *relayconvert.ChatToResponsesStreamEvent:
				value = event.Payload
			}
			encoded, err := common.Marshal(value)
			if err != nil {
				return err
			}
			event := gjson.GetBytes(encoded, "type").String()
			if event == "response.completed" || event == "response.incomplete" {
				if !flushCompleted {
					completed = append(completed, result)
					continue
				}
				if err := info.KiroCompletionError(); err != nil {
					return err
				}
			}
			if err := helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: event}, string(common.SanitizeErrorJSON(encoded))); err != nil {
				return err
			}
		}
		return nil
	}
	helper.StreamScannerHandler(c, response, info, func(raw string, sr *helper.StreamResult) {
		data := info.ObserveKiroPayload([]byte(raw), true)
		if gjson.GetBytes(data, "error").Exists() || gjson.GetBytes(data, "type").String() == "error" {
			streamErr = errors.New("upstream stream failed")
			sr.Stop(streamErr)
			return
		}
		native, err := decodeConversionResponse(data, from, true)
		if err != nil {
			streamErr = err
			sr.Stop(err)
			return
		}
		switch from {
		case kittypes.RelayFormatClaude:
			terminal = terminal || gjson.GetBytes(data, "type").String() == "message_stop"
		case kittypes.RelayFormatGemini:
			terminal = terminal || gjson.GetBytes(data, "candidates.0.finishReason").String() != ""
		default:
			terminal = terminal || (gjson.GetBytes(data, "choices.0.finish_reason").Exists() && gjson.GetBytes(data, "choices.0.finish_reason").String() != "")
		}
		results, err := relayconvert.ConvertStreamResponseChunk(service.RelayKitContext(c), meta, state, native)
		if err == nil {
			err = emit(results)
		}
		if err != nil {
			streamErr = err
			sr.Stop(err)
		}
	})
	usage, err := service.RelayKitUsage(state.Usage())
	if err != nil && streamErr == nil {
		streamErr = err
	}
	if streamErr == nil && info.StreamStatus != nil && info.StreamStatus.EndError != nil {
		streamErr = info.StreamStatus.EndError
	}
	if streamErr == nil && !terminal {
		streamErr = errors.New("upstream stream ended before a terminal event")
	}
	if streamErr == nil {
		if apiErr := info.KiroCompletionError(); apiErr != nil {
			streamErr = apiErr
		}
	}
	if streamErr == nil {
		results, err := relayconvert.FinalizeStreamResponse(service.RelayKitContext(c), meta, state)
		if err == nil {
			err = emit(results)
		}
		streamErr = err
	}
	if streamErr == nil {
		flushCompleted = true
		streamErr = emit(completed)
	}
	if streamErr != nil {
		// A failure after partial output must remain a failure on the wire and in billing.
		event := map[string]any{"type": "response.failed", "response": map[string]any{"status": "failed", "error": map[string]any{"code": "upstream_stream_error", "message": "Upstream stream interrupted"}}}
		encoded, _ := common.Marshal(event)
		_ = helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: "response.failed"}, string(encoded))
		common.SetContextKey(c, constant.ContextKeyRelayErrorAlreadyStreamed, true)
		return usage, fail(fmt.Errorf("response conversion stream: %w", streamErr))
	}
	return usage, nil
}
