package openai

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

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

func OpenaiTTSHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) *dto.Usage {
	// Preserve read/write failures for the adaptor. Binary output is never
	// replayed after any bytes have been exposed to the client.
	defer service.CloseResponseBodyGracefully(resp)
	defer http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})
	info.StreamStatus = relaycommon.NewStreamStatus()
	usage := &dto.Usage{}
	usage.PromptTokens = info.GetEstimatePromptTokens()
	usage.TotalTokens = info.GetEstimatePromptTokens()
	for k, v := range resp.Header {
		if !service.ShouldCopyUpstreamHeader(c, k, v) {
			continue
		}
		c.Writer.Header().Set(k, v[0])
	}
	c.Writer.WriteHeader(resp.StatusCode)

	if info.IsStream {
		helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
			if apiErr := helper.ParseStreamError(data); apiErr != nil {
				sr.Stop(apiErr)
				return
			}
			if service.SundaySearch(data, "usage") {
				var simpleResponse dto.SimpleResponse
				if err := common.Unmarshal([]byte(data), &simpleResponse); err != nil {
					logger.LogError(c, err.Error())
					sr.Stop(err)
				} else if simpleResponse.Usage.TotalTokens != 0 {
					usage.PromptTokens = simpleResponse.Usage.InputTokens
					usage.CompletionTokens = simpleResponse.OutputTokens
					usage.TotalTokens = simpleResponse.TotalTokens
				}
			}
			if err := helper.StringData(c, data); err != nil {
				sr.Stop(err)
			}
		})
	} else {
		common.SetContextKey(c, constant.ContextKeyLocalCountTokens, true)
		// 读取响应体到缓冲区
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.LogError(c, fmt.Sprintf("failed to read TTS response body: %v", err))
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonScannerErr, err)
			c.Writer.Header().Del("Content-Length")
			c.Writer.Header().Del("Content-Type")
			return usage
		}

		// Compatible providers may return a JSON/HTML gateway error with HTTP 200
		// on a binary media endpoint. Sanitize that diagnostic before it reaches
		// the client while preserving successful audio bytes byte-for-byte.
		publicBody := common.SanitizeErrorJSON(bodyBytes)
		if service.IsMediaDiagnostic(resp.Header.Get("Content-Type"), bodyBytes[:min(len(bodyBytes), 512)]) {
			publicBody = common.SanitizeHTTPErrorBody(bodyBytes)
			c.Writer.Header().Set("Content-Type", "application/json")
		}
		if !bytes.Equal(publicBody, bodyBytes) {
			c.Writer.Header().Del("Content-Encoding")
			c.Writer.Header().Set("Content-Length", strconv.Itoa(len(publicBody)))
		}
		c.Writer.WriteHeaderNow()
		_, err = c.Writer.Write(publicBody)
		if err != nil {
			logger.LogError(c, fmt.Sprintf("failed to write TTS response: %v", err))
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonWriteFail, err)
			return usage
		}
		info.ReceivedResponseCount++
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)

		// 计算音频时长并更新 usage
		audioFormat := "mp3" // 默认格式
		if audioReq, ok := info.Request.(*dto.AudioRequest); ok && audioReq.ResponseFormat != "" {
			audioFormat = audioReq.ResponseFormat
		}

		var duration float64
		var durationErr error

		if audioFormat == "pcm" {
			// PCM 格式没有文件头，根据 OpenAI TTS 的 PCM 参数计算时长
			// 采样率: 24000 Hz, 位深度: 16-bit (2 bytes), 声道数: 1
			const sampleRate = 24000
			const bytesPerSample = 2
			const channels = 1
			duration = float64(len(bodyBytes)) / float64(sampleRate*bytesPerSample*channels)
		} else {
			ext := "." + audioFormat
			reader := bytes.NewReader(bodyBytes)
			duration, durationErr = common.GetAudioDuration(c.Request.Context(), reader, ext)
		}

		usage.PromptTokensDetails.TextTokens = usage.PromptTokens

		if durationErr != nil {
			logger.LogWarn(c, fmt.Sprintf("failed to get audio duration: %v", durationErr))
			// 如果无法获取时长，则设置保底的 CompletionTokens，根据body大小计算
			sizeInKB := float64(len(bodyBytes)) / 1000.0
			estimatedTokens := int(math.Ceil(sizeInKB)) // 粗略估算每KB约等于1 token
			usage.CompletionTokens = estimatedTokens
			usage.CompletionTokenDetails.AudioTokens = estimatedTokens
		} else if duration > 0 {
			// 计算 token: ceil(duration) / 60.0 * 1000，即每分钟 1000 tokens
			completionTokens := int(math.Round(math.Ceil(duration) / 60.0 * 1000))
			usage.CompletionTokens = completionTokens
			usage.CompletionTokenDetails.AudioTokens = completionTokens
		}
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	return usage
}

func OpenaiSTTHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, responseFormat string) (*types.NewAPIError, *dto.Usage) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError), nil
	}
	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	var responseData struct {
		Usage *dto.Usage `json:"usage"`
	}
	if err := common.Unmarshal(responseBody, &responseData); err == nil && responseData.Usage != nil {
		if responseData.Usage.TotalTokens > 0 {
			usage := responseData.Usage
			if usage.PromptTokens == 0 {
				usage.PromptTokens = usage.InputTokens
			}
			if usage.CompletionTokens == 0 {
				usage.CompletionTokens = usage.OutputTokens
			}
			return nil, usage
		}
	}

	usage := &dto.Usage{}
	usage.PromptTokens = info.GetEstimatePromptTokens()
	usage.CompletionTokens = 0
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return nil, usage
}
