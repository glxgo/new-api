package helper

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func FlushWriter(c *gin.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("flush panic recovered: %v", r)
		}
	}()

	if c == nil || c.Writer == nil {
		return nil
	}

	if c.Request != nil && c.Request.Context().Err() != nil {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	// Gin's Flush() discards the underlying FlushError. Commit its header
	// bookkeeping, then reach the transport's error-aware flush operation.
	c.Writer.WriteHeaderNow()
	var writer http.ResponseWriter = c.Writer
	for {
		unwrapper, ok := writer.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			break
		}
		writer = unwrapper.Unwrap()
	}
	return http.NewResponseController(writer).Flush()
}

func SetEventStreamHeaders(c *gin.Context) {
	// 检查是否已经设置过头部
	if _, exists := c.Get("event_stream_headers_set"); exists {
		return
	}

	// 设置标志，表示头部已经设置过
	c.Set("event_stream_headers_set", true)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
}

func ClaudeData(c *gin.Context, resp dto.ClaudeResponse) error {
	jsonData, err := common.Marshal(resp)
	if err != nil {
		return err
	}
	return writeSSE(c, resp.Type, string(jsonData))
}

func ClaudeChunkData(c *gin.Context, resp dto.ClaudeResponse, data string) error {
	return writeSSE(c, resp.Type, data)
}

func ResponseChunkData(c *gin.Context, resp dto.ResponsesStreamResponse, data string) error {
	if c == nil || c.Writer == nil {
		return errors.New("context or writer is nil")
	}
	if c.Request != nil && c.Request.Context().Err() != nil {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}
	return writeSSE(c, resp.Type, data)
}

func StringData(c *gin.Context, str string) error {
	if c == nil || c.Writer == nil {
		return errors.New("context or writer is nil")
	}

	if c.Request != nil && c.Request.Context().Err() != nil {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	return writeSSE(c, "", str)
}

// writeSSE returns both write and flush failures. Gin Render and http.Flusher
// hide these errors, which otherwise lets the relay keep consuming upstream.
func writeSSE(c *gin.Context, event, data string) error {
	if c == nil || c.Writer == nil {
		return errors.New("context or writer is nil")
	}
	if strings.ContainsAny(event, "\r\n") {
		return errors.New("invalid SSE event name")
	}
	if c.Request != nil && c.Request.Context().Err() != nil {
		return rememberStreamWriteError(c, c.Request.Context().Err())
	}
	if err := StreamWriteError(c); err != nil {
		return err
	}
	// Sanitize only error envelopes. Successful model output, including image
	// and media URLs, is preserved by SanitizeErrorJSON.
	data = string(common.SanitizeErrorJSON([]byte(data)))
	data = strings.ReplaceAll(strings.ReplaceAll(data, "\r\n", "\n"), "\r", "\n")
	frame := "data: " + strings.ReplaceAll(data, "\n", "\ndata: ") + "\n\n"
	if event != "" {
		frame = "event: " + event + "\n" + frame
	}
	SetEventStreamHeaders(c)
	ExtendWriteDeadline(c)
	c.Set(streamOutputKey, true)
	if n, err := c.Writer.Write([]byte(frame)); err != nil {
		return rememberStreamWriteError(c, err)
	} else if n != len(frame) {
		return rememberStreamWriteError(c, io.ErrShortWrite)
	}
	return rememberStreamWriteError(c, FlushWriter(c))
}

func PingData(c *gin.Context) error {
	if c == nil || c.Writer == nil {
		return errors.New("context or writer is nil")
	}

	if c.Request != nil && c.Request.Context().Err() != nil {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	if _, err := c.Writer.Write([]byte(": PING\n\n")); err != nil {
		return rememberStreamWriteError(c, fmt.Errorf("write ping data failed: %w", err))
	}
	return rememberStreamWriteError(c, FlushWriter(c))
}

func ObjectData(c *gin.Context, object interface{}) error {
	if object == nil {
		return errors.New("object is nil")
	}
	jsonData, err := common.Marshal(object)
	if err != nil {
		return fmt.Errorf("error marshalling object: %w", err)
	}
	return StringData(c, string(jsonData))
}

func Done(c *gin.Context) {
	_ = StringData(c, "[DONE]")
}

func WssString(c *gin.Context, ws *websocket.Conn, str string) error {
	if ws == nil {
		logger.LogError(c, "websocket connection is nil")
		return errors.New("websocket connection is nil")
	}
	//common.LogInfo(c, fmt.Sprintf("sending message: %s", str))
	return ws.WriteMessage(1, []byte(str))
}

func WssObject(c *gin.Context, ws *websocket.Conn, object interface{}) error {
	jsonData, err := common.Marshal(object)
	if err != nil {
		return fmt.Errorf("error marshalling object: %w", err)
	}
	if ws == nil {
		logger.LogError(c, "websocket connection is nil")
		return errors.New("websocket connection is nil")
	}
	//common.LogInfo(c, fmt.Sprintf("sending message: %s", jsonData))
	return ws.WriteMessage(1, common.SanitizeErrorJSON(jsonData))
}

func WssError(c *gin.Context, ws *websocket.Conn, openaiError types.OpenAIError) {
	if ws == nil {
		return
	}
	errorObj := &dto.RealtimeEvent{
		Type:    "error",
		EventId: GetLocalRealtimeID(c),
		Error:   &openaiError,
	}
	_ = WssObject(c, ws, errorObj)
}

func GetResponseID(c *gin.Context) string {
	logID := c.GetString(common.RequestIdKey)
	return fmt.Sprintf("chatcmpl-%s", logID)
}

func GetLocalRealtimeID(c *gin.Context) string {
	logID := c.GetString(common.RequestIdKey)
	return fmt.Sprintf("evt_%s", logID)
}

func GenerateStartEmptyResponse(id string, createAt int64, model string, systemFingerprint *string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:                id,
		Object:            "chat.completion.chunk",
		Created:           createAt,
		Model:             model,
		SystemFingerprint: systemFingerprint,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role:    "assistant",
					Content: common.GetPointer(""),
				},
			},
		},
	}
}

func GenerateStopResponse(id string, createAt int64, model string, finishReason string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:                id,
		Object:            "chat.completion.chunk",
		Created:           createAt,
		Model:             model,
		SystemFingerprint: nil,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				FinishReason: &finishReason,
			},
		},
	}
}

func GenerateFinalUsageResponse(id string, createAt int64, model string, usage dto.Usage) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:                id,
		Object:            "chat.completion.chunk",
		Created:           createAt,
		Model:             model,
		SystemFingerprint: nil,
		Choices:           make([]dto.ChatCompletionsStreamResponseChoice, 0),
		Usage:             &usage,
	}
}

// WriteStreamBytes checks downstream delivery for binary streaming protocols.
func WriteStreamBytes(c *gin.Context, data []byte) error {
	if err := StreamWriteError(c); err != nil {
		return err
	}
	if c.Request.Context().Err() != nil {
		return rememberStreamWriteError(c, c.Request.Context().Err())
	}
	ExtendWriteDeadline(c)
	c.Set(streamOutputKey, true)
	n, err := c.Writer.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = FlushWriter(c)
	}
	return rememberStreamWriteError(c, err)
}
