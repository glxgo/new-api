package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
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
	var streamErr *types.NewAPIError

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {

		// 检查当前数据是否包含 completed 状态和 usage 信息
		var streamResponse dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
			logger.LogError(c, "failed to unmarshal stream response: "+err.Error())
			sr.Error(err)
			return
		}
		sendResponsesStreamData(c, streamResponse, data)
		switch streamResponse.Type {
		case "response.completed":
			if streamResponse.Response != nil {
				recordResponsesUpstreamTerminal(info, resp.StatusCode, streamResponse)
				if streamResponse.Response.Usage != nil {
					if streamResponse.Response.Usage.InputTokens != 0 {
						usage.PromptTokens = streamResponse.Response.Usage.InputTokens
					}
					if streamResponse.Response.Usage.OutputTokens != 0 {
						usage.CompletionTokens = streamResponse.Response.Usage.OutputTokens
					}
					if streamResponse.Response.Usage.TotalTokens != 0 {
						usage.TotalTokens = streamResponse.Response.Usage.TotalTokens
					}
					if streamResponse.Response.Usage.InputTokensDetails != nil {
						usage.PromptTokensDetails.CachedTokens = streamResponse.Response.Usage.InputTokensDetails.CachedTokens
					}
				}
				if streamResponse.Response.HasImageGenerationCall() {
					c.Set("image_generation_call", true)
					c.Set("image_generation_call_quality", streamResponse.Response.GetQuality())
					c.Set("image_generation_call_size", streamResponse.Response.GetSize())
				}
			}
			sr.Done()
		case "response.failed", "response.error":
			recordResponsesUpstreamTerminal(info, resp.StatusCode, streamResponse)
			streamErr = responsesTerminalError(streamResponse, types.ErrorCodeUpstreamResponseFailed)
			common.SetContextKey(c, constant.ContextKeyRelayErrorAlreadyStreamed, true)
			sr.Stop(streamErr)
		case "response.incomplete":
			recordResponsesUpstreamTerminal(info, resp.StatusCode, streamResponse)
			streamErr = responsesTerminalError(streamResponse, types.ErrorCodeUpstreamResponseIncomplete)
			common.SetContextKey(c, constant.ContextKeyRelayErrorAlreadyStreamed, true)
			sr.Stop(streamErr)
		case "response.output_text.delta":
			// 处理输出文本
			responseTextBuilder.WriteString(streamResponse.Delta)
		case dto.ResponsesOutputTypeItemDone:
			// 函数调用处理
			if streamResponse.Item != nil {
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
	})

	if streamErr != nil {
		service.PreventChannelAffinityRecord(c)
		service.ClearCurrentChannelAffinityCache(c)
		return nil, streamErr
	}

	if info.StreamStatus == nil || info.StreamStatus.EndReason != relaycommon.StreamEndReasonDone {
		streamSummary := "status unavailable"
		if info.StreamStatus != nil {
			streamSummary = info.StreamStatus.Summary()
		}
		streamErr := fmt.Errorf("responses stream ended before response.completed: %s", streamSummary)
		service.PreventChannelAffinityRecord(c)
		service.ClearCurrentChannelAffinityCache(c)

		errorOptions := []types.NewAPIErrorOptions{}
		if info.ReceivedResponseCount > 0 || c.Request.Context().Err() != nil {
			// Retrying after forwarding upstream events can duplicate output. Let
			// the client reconnect; the cleared affinity will route that retry away
			// from the failed channel.
			errorOptions = append(errorOptions, types.ErrOptionWithSkipRetry())
		}
		return nil, types.NewErrorWithStatusCode(
			streamErr,
			types.ErrorCodeChannelIncompleteStream,
			http.StatusBadGateway,
			errorOptions...,
		)
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

func recordResponsesUpstreamTerminal(info *relaycommon.RelayInfo, upstreamHTTPStatus int, streamResponse dto.ResponsesStreamResponse) {
	if info == nil || info.StreamStatus == nil {
		return
	}
	terminal := relaycommon.UpstreamTerminal{
		EventType:  streamResponse.Type,
		HTTPStatus: upstreamHTTPStatus,
	}
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
		if oaiErr := dto.GetOpenAIError(streamResponse.Error); oaiErr != nil {
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

func responsesTerminalError(streamResponse dto.ResponsesStreamResponse, fallbackCode types.ErrorCode) *types.NewAPIError {
	var oaiErr *types.OpenAIError
	if streamResponse.Response != nil {
		oaiErr = streamResponse.Response.GetOpenAIError()
	}
	if oaiErr == nil {
		oaiErr = dto.GetOpenAIError(streamResponse.Error)
	}
	if oaiErr != nil {
		if strings.TrimSpace(fmt.Sprintf("%v", oaiErr.Code)) == "" {
			oaiErr.Code = fallbackCode
		}
		if oaiErr.Message == "" {
			oaiErr.Message = fmt.Sprintf("upstream Responses terminal event: %s", streamResponse.Type)
		}
		return types.WithOpenAIError(*oaiErr, http.StatusBadGateway, types.ErrOptionWithSkipRetry())
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
	return types.NewOpenAIError(errors.New(message), code, http.StatusBadGateway, types.ErrOptionWithSkipRetry())
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
