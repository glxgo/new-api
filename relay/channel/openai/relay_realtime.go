package openai

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func OpenaiRealtimeHandler(c *gin.Context, info *relaycommon.RelayInfo) (apiErr *types.NewAPIError, sumUsage *dto.RealtimeUsage) {
	if info == nil || info.ClientWs == nil || info.TargetWs == nil {
		return types.NewError(fmt.Errorf("invalid websocket connection"), types.ErrorCodeBadResponse), nil
	}
	info.IsStream = true
	clientConn, targetConn := info.ClientWs, info.TargetWs
	ctx := c.Request.Context()
	type frame struct {
		fromClient bool
		kind       int
		data       []byte
		err        error
	}
	frames := make(chan frame, 16)
	stop := make(chan struct{})
	var readers sync.WaitGroup
	read := func(conn *websocket.Conn, fromClient bool) {
		defer readers.Done()
		for {
			kind, data, err := conn.ReadMessage()
			select {
			case frames <- frame{fromClient, kind, data, err}:
			case <-stop:
				return
			}
			if err != nil {
				return
			}
		}
	}
	readers.Add(2)
	go read(clientConn, true)
	go read(targetConn, false)
	defer func() {
		close(stop)
		_ = targetConn.Close()
		// Interrupt only the read side: the controller still owns the client
		// socket and may need to write the final error after this handler returns.
		_ = clientConn.UnderlyingConn().SetReadDeadline(time.Now())
		readers.Wait()
	}()
	sumUsage = &dto.RealtimeUsage{}
	localUsage := &dto.RealtimeUsage{}
	// All usage, session metadata and writes belong to this event loop. Reader
	// goroutines never access Gin or mutable relay state.
	defer func() {
		if localUsage.TotalTokens != 0 {
			if err := preConsumeUsage(c, info, localUsage, sumUsage); err != nil && apiErr == nil {
				apiErr = types.NewErrorWithStatusCode(err, "realtime_usage_error", 500, types.ErrOptionWithSkipRetry())
			}
		}
	}()
	failure := func(err error, code types.ErrorCode, status int) *types.NewAPIError {
		return types.NewErrorWithStatusCode(err, code, status, types.ErrOptionWithSkipRetry())
	}
	for {
		select {
		case <-ctx.Done():
			return failure(ctx.Err(), "client_canceled", 499), sumUsage
		case f := <-frames:
			if f.err != nil {
				if ctx.Err() != nil {
					return failure(ctx.Err(), "client_canceled", 499), sumUsage
				}
				if websocket.IsCloseError(f.err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					return nil, sumUsage
				}
				if f.fromClient {
					return failure(f.err, "client_canceled", 499), sumUsage
				}
				return failure(f.err, "realtime_stream_error", 502), sumUsage
			}
			event := &dto.RealtimeEvent{}
			if err := common.Unmarshal(f.data, event); err != nil || event.Type == "" {
				if err == nil {
					err = fmt.Errorf("realtime event has no type")
				}
				status := 502
				if f.fromClient {
					status = 400
				}
				return failure(err, types.ErrorCodeBadResponseBody, status), sumUsage
			}
			if f.fromClient && event.Type == dto.RealtimeEventTypeSessionUpdate && event.Session != nil && event.Session.Tools != nil {
				info.RealtimeTools = event.Session.Tools
			}
			if !f.fromClient {
				info.SetFirstResponseTime()
				if (event.Type == dto.RealtimeEventTypeSessionUpdated || event.Type == dto.RealtimeEventTypeSessionCreated) && event.Session != nil {
					info.InputAudioFormat = common.GetStringIfEmpty(event.Session.InputAudioFormat, info.InputAudioFormat)
					info.OutputAudioFormat = common.GetStringIfEmpty(event.Session.OutputAudioFormat, info.OutputAudioFormat)
				}
			}
			if !f.fromClient && event.Type == dto.RealtimeEventTypeResponseDone {
				if event.Response == nil {
					return failure(fmt.Errorf("response.done has no response"), types.ErrorCodeBadResponseBody, 502), sumUsage
				}
				turnUsage := event.Response.Usage
				if turnUsage == nil {
					text, audio, err := service.CountTokenRealtime(info, *event, info.UpstreamModelName)
					if err != nil {
						return failure(err, "realtime_usage_error", 502), sumUsage
					}
					localUsage.TotalTokens += text + audio
					localUsage.InputTokens += text + audio
					localUsage.InputTokenDetails.TextTokens += text
					localUsage.InputTokenDetails.AudioTokens += audio
					turnUsage = localUsage
				}
				info.IsFirstRequest = false
				// Transfer ownership before settlement so a failed settlement is never
				// attempted a second time during deferred cleanup.
				localUsage = &dto.RealtimeUsage{}
				if err := preConsumeUsage(c, info, turnUsage, sumUsage); err != nil {
					return failure(err, "realtime_usage_error", 500), sumUsage
				}
			} else {
				text, audio, err := service.CountTokenRealtime(info, *event, info.UpstreamModelName)
				if err != nil {
					return failure(err, "realtime_usage_error", 502), sumUsage
				}
				localUsage.TotalTokens += text + audio
				if f.fromClient {
					localUsage.InputTokens += text + audio
					localUsage.InputTokenDetails.TextTokens += text
					localUsage.InputTokenDetails.AudioTokens += audio
				} else {
					localUsage.OutputTokens += text + audio
					localUsage.OutputTokenDetails.TextTokens += text
					localUsage.OutputTokenDetails.AudioTokens += audio
				}
			}
			destination := clientConn
			if f.fromClient {
				destination = targetConn
			}
			_ = destination.SetWriteDeadline(time.Now().Add(30 * time.Second))
			data := f.data
			if !f.fromClient {
				// Realtime model events are forwarded byte-for-byte on the internal
				// leg, but error events must not disclose the upstream endpoint to
				// the public websocket client.
				data = common.SanitizeErrorJSON(data)
			}
			if err := destination.WriteMessage(f.kind, data); err != nil {
				if !f.fromClient {
					return failure(err, "client_write_error", 499), sumUsage
				}
				return failure(err, "realtime_stream_error", 502), sumUsage
			}
		}
	}
}

func preConsumeUsage(ctx *gin.Context, info *relaycommon.RelayInfo, usage *dto.RealtimeUsage, totalUsage *dto.RealtimeUsage) error {
	if usage == nil || totalUsage == nil {
		return fmt.Errorf("invalid usage pointer")
	}

	totalUsage.TotalTokens += usage.TotalTokens
	totalUsage.InputTokens += usage.InputTokens
	totalUsage.OutputTokens += usage.OutputTokens
	totalUsage.InputTokenDetails.CachedTokens += usage.InputTokenDetails.CachedTokens
	totalUsage.InputTokenDetails.TextTokens += usage.InputTokenDetails.TextTokens
	totalUsage.InputTokenDetails.AudioTokens += usage.InputTokenDetails.AudioTokens
	totalUsage.OutputTokenDetails.TextTokens += usage.OutputTokenDetails.TextTokens
	totalUsage.OutputTokenDetails.AudioTokens += usage.OutputTokenDetails.AudioTokens
	// clear usage
	err := service.PreWssConsumeQuota(ctx, info, usage)
	return err
}
