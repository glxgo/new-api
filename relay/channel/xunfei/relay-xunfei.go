package xunfei

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// https://console.xfyun.cn/services/cbm
// https://www.xfyun.cn/doc/spark/Web.html

func requestOpenAI2Xunfei(request dto.GeneralOpenAIRequest, xunfeiAppId string, domain string) *XunfeiChatRequest {
	messages := make([]XunfeiMessage, 0, len(request.Messages))
	shouldCovertSystemMessage := !strings.HasSuffix(request.Model, "3.5")
	for _, message := range request.Messages {
		if message.Role == "system" && shouldCovertSystemMessage {
			messages = append(messages, XunfeiMessage{
				Role:    "user",
				Content: message.StringContent(),
			})
			messages = append(messages, XunfeiMessage{
				Role:    "assistant",
				Content: "Okay",
			})
		} else {
			messages = append(messages, XunfeiMessage{
				Role:    message.Role,
				Content: message.StringContent(),
			})
		}
	}
	xunfeiRequest := XunfeiChatRequest{}
	xunfeiRequest.Header.AppId = xunfeiAppId
	xunfeiRequest.Parameter.Chat.Domain = domain
	xunfeiRequest.Parameter.Chat.Temperature = request.Temperature
	xunfeiRequest.Parameter.Chat.TopK = lo.FromPtrOr(request.N, 0)
	xunfeiRequest.Parameter.Chat.MaxTokens = request.GetMaxTokens()
	xunfeiRequest.Payload.Message.Text = messages
	return &xunfeiRequest
}

func responseXunfei2OpenAI(response *XunfeiChatResponse) *dto.OpenAITextResponse {
	if len(response.Payload.Choices.Text) == 0 {
		response.Payload.Choices.Text = []XunfeiChatResponseTextItem{
			{
				Content: "",
			},
		}
	}
	choice := dto.OpenAITextResponseChoice{
		Index: 0,
		Message: dto.Message{
			Role:    "assistant",
			Content: response.Payload.Choices.Text[0].Content,
		},
		FinishReason: constant.FinishReasonStop,
	}
	fullTextResponse := dto.OpenAITextResponse{
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Choices: []dto.OpenAITextResponseChoice{choice},
		Usage:   response.Payload.Usage.Text,
	}
	return &fullTextResponse
}

func streamResponseXunfei2OpenAI(xunfeiResponse *XunfeiChatResponse) *dto.ChatCompletionsStreamResponse {
	if len(xunfeiResponse.Payload.Choices.Text) == 0 {
		xunfeiResponse.Payload.Choices.Text = []XunfeiChatResponseTextItem{
			{
				Content: "",
			},
		}
	}
	var choice dto.ChatCompletionsStreamResponseChoice
	choice.Delta.SetContentString(xunfeiResponse.Payload.Choices.Text[0].Content)
	if xunfeiResponse.Payload.Choices.Status == 2 {
		choice.FinishReason = &constant.FinishReasonStop
	}
	response := dto.ChatCompletionsStreamResponse{
		Object:  "chat.completion.chunk",
		Created: common.GetTimestamp(),
		Model:   "SparkDesk",
		Choices: []dto.ChatCompletionsStreamResponseChoice{choice},
	}
	return &response
}

func buildXunfeiAuthUrl(hostUrl string, apiKey, apiSecret string) string {
	HmacWithShaToBase64 := func(algorithm, data, key string) string {
		mac := hmac.New(sha256.New, []byte(key))
		mac.Write([]byte(data))
		encodeData := mac.Sum(nil)
		return base64.StdEncoding.EncodeToString(encodeData)
	}
	ul, err := url.Parse(hostUrl)
	if err != nil {
		fmt.Println(err)
	}
	date := time.Now().UTC().Format(time.RFC1123)
	signString := []string{"host: " + ul.Host, "date: " + date, "GET " + ul.Path + " HTTP/1.1"}
	sign := strings.Join(signString, "\n")
	sha := HmacWithShaToBase64("hmac-sha256", sign, apiSecret)
	authUrl := fmt.Sprintf("hmac username=\"%s\", algorithm=\"%s\", headers=\"%s\", signature=\"%s\"", apiKey,
		"hmac-sha256", "host date request-line", sha)
	authorization := base64.StdEncoding.EncodeToString([]byte(authUrl))
	v := url.Values{}
	v.Add("host", ul.Host)
	v.Add("date", date)
	v.Add("authorization", authorization)
	callUrl := hostUrl + "?" + v.Encode()
	return callUrl
}

func xunfeiStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, textRequest dto.GeneralOpenAIRequest, appId string, apiSecret string, apiKey string) (*dto.Usage, *types.NewAPIError) {
	return xunfeiHandler(c, info, textRequest, appId, apiSecret, apiKey)
}

func xunfeiHandler(c *gin.Context, info *relaycommon.RelayInfo, textRequest dto.GeneralOpenAIRequest, appId string, apiSecret string, apiKey string) (*dto.Usage, *types.NewAPIError) {
	domain, authURL := getXunfeiAuthUrl(c, apiKey, apiSecret, textRequest.Model)
	conn, err := xunfeiMakeRequest(c.Request.Context(), textRequest, domain, authURL, appId)
	if err != nil {
		if c.Request.Context().Err() != nil {
			return nil, types.NewErrorWithStatusCode(c.Request.Context().Err(), "client_canceled", 499, types.ErrOptionWithSkipRetry())
		}
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeDoRequestFailed, 502)
	}
	defer conn.Close()
	return xunfeiReadResponse(c, info, conn)
}

func xunfeiReadResponse(c *gin.Context, info *relaycommon.RelayInfo, conn *websocket.Conn) (*dto.Usage, *types.NewAPIError) {
	stopWatch := helper.WatchWebSocketCancellation(c.Request.Context(), conn)
	defer stopWatch()
	defer http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})
	info.StreamStatus = relaycommon.NewStreamStatus()
	usage := &dto.Usage{}
	var content strings.Builder
	var response XunfeiChatResponse
	fail := func(err error, reason relaycommon.StreamEndReason) *types.NewAPIError {
		info.StreamStatus.SetEndReason(reason, err)
		return helper.StreamFailure(c, info, true)
	}
	for {
		_ = conn.SetReadDeadline(helper.WebSocketReadDeadline())
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return usage, fail(err, relaycommon.StreamEndReasonScannerErr)
		}
		if err = common.Unmarshal(msg, &response); err != nil {
			return usage, fail(err, relaycommon.StreamEndReasonHandlerStop)
		}
		if response.Header.Code != 0 {
			upstreamErr := types.WithOpenAIError(types.OpenAIError{Code: response.Header.Code, Message: response.Header.Message, Type: "xunfei_error"}, 502)
			return usage, fail(upstreamErr, relaycommon.StreamEndReasonHandlerStop)
		}
		info.SetFirstResponseTime()
		info.ReceivedResponseCount++
		usage.PromptTokens += response.Payload.Usage.Text.PromptTokens
		usage.CompletionTokens += response.Payload.Usage.Text.CompletionTokens
		usage.TotalTokens += response.Payload.Usage.Text.TotalTokens
		if info.IsStream {
			if err = helper.ObjectData(c, streamResponseXunfei2OpenAI(&response)); err != nil {
				return usage, fail(err, relaycommon.StreamEndReasonWriteFail)
			}
		} else {
			for _, text := range response.Payload.Choices.Text {
				content.WriteString(text.Content)
			}
		}
		if response.Payload.Choices.Status == 2 {
			break
		}
	}
	if info.IsStream {
		if err := helper.StringData(c, "[DONE]"); err != nil {
			return usage, fail(err, relaycommon.StreamEndReasonWriteFail)
		}
	} else {
		response.Payload.Choices.Text = []XunfeiChatResponseTextItem{{Content: content.String()}}
		result := responseXunfei2OpenAI(&response)
		result.Usage = *usage
		data, err := common.Marshal(result)
		if err != nil {
			return usage, fail(err, relaycommon.StreamEndReasonHandlerStop)
		}
		c.Header("Content-Type", "application/json")
		helper.ExtendWriteDeadline(c)
		n, err := c.Writer.Write(data)
		if err == nil && n != len(data) {
			err = io.ErrShortWrite
		}
		if err != nil {
			return usage, types.NewErrorWithStatusCode(err, "client_write_error", 499, types.ErrOptionWithSkipRetry())
		}
	}
	info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
	return usage, nil
}

func xunfeiMakeRequest(ctx context.Context, textRequest dto.GeneralOpenAIRequest, domain, authURL, appID string) (*websocket.Conn, error) {
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, resp, err := dialer.DialContext(ctx, authURL, nil)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil, err
	}
	payload, err := common.Marshal(requestOpenAI2Xunfei(textRequest, appID, domain))
	if err == nil {
		stopWatch := helper.WatchWebSocketCancellation(ctx, conn)
		_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		err = conn.WriteMessage(websocket.TextMessage, payload)
		stopWatch()
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func apiVersion2domain(apiVersion string) string {
	switch apiVersion {
	case "v1.1":
		return "lite"
	case "v2.1":
		return "generalv2"
	case "v3.1":
		return "generalv3"
	case "v3.5":
		return "generalv3.5"
	case "v4.0":
		return "4.0Ultra"
	}
	return "general" + apiVersion
}

func getXunfeiAuthUrl(c *gin.Context, apiKey string, apiSecret string, modelName string) (string, string) {
	apiVersion := getAPIVersion(c, modelName)
	domain := apiVersion2domain(apiVersion)
	authUrl := buildXunfeiAuthUrl(fmt.Sprintf("wss://spark-api.xf-yun.com/%s/chat", apiVersion), apiKey, apiSecret)
	return domain, authUrl
}

func getAPIVersion(c *gin.Context, modelName string) string {
	query := c.Request.URL.Query()
	apiVersion := query.Get("api-version")
	if apiVersion != "" {
		return apiVersion
	}
	parts := strings.Split(modelName, "-")
	if len(parts) == 2 {
		apiVersion = parts[1]
		return apiVersion

	}
	apiVersion = c.GetString("api_version")
	if apiVersion != "" {
		return apiVersion
	}
	apiVersion = "v1.1"
	common.SysLog("api_version not found, using default: " + apiVersion)
	return apiVersion
}
