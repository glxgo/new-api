package helper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/gin-gonic/gin"
)

const (
	InitialScannerBufferSize    = 64 << 10  // 64KB (64*1024)
	DefaultMaxScannerBufferSize = 128 << 20 // 64MB (64*1024*1024) default SSE buffer size
	DefaultPingInterval         = 10 * time.Second
	MaxResponsesPingInterval    = 15 * time.Second
	streamWriteTimeout          = 30 * time.Second
)

// ResolveStreamPing keeps Responses streams alive even when the global ping
// switch is disabled. Responses requests can legitimately spend longer than a
// reverse proxy timeout waiting for the first upstream event, so relying on an
// optional administrator setting leaves the default deployment vulnerable to
// truncated streams.
func ResolveStreamPing(info *relaycommon.RelayInfo, generalSettings *operation_setting.GeneralSetting) (bool, time.Duration) {
	if info == nil || info.DisablePing {
		return false, 0
	}

	isResponses := info.RelayMode == relayconstant.RelayModeResponses ||
		info.RelayMode == relayconstant.RelayModeResponsesCompact
	globalEnabled := generalSettings != nil && generalSettings.PingIntervalEnabled
	if !isResponses && !globalEnabled {
		return false, 0
	}

	pingInterval := DefaultPingInterval
	if generalSettings != nil && generalSettings.PingIntervalSeconds > 0 {
		pingInterval = time.Duration(generalSettings.PingIntervalSeconds) * time.Second
	}
	if isResponses && (!globalEnabled || pingInterval > MaxResponsesPingInterval) {
		pingInterval = DefaultPingInterval
	}
	return true, pingInterval
}

func getScannerBufferSize() int {
	if constant.StreamScannerMaxBufferMB > 0 {
		return constant.StreamScannerMaxBufferMB << 20
	}
	return DefaultMaxScannerBufferSize
}

func NewStreamScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, InitialScannerBufferSize), getScannerBufferSize())
	return scanner
}

// ExtendWriteDeadline prevents a slow downstream write from keeping a stream
// goroutine alive forever while cleanup waits for it. Unsupported test writers
// simply ignore the best-effort deadline.
func ExtendWriteDeadline(c *gin.Context) {
	if c == nil || c.Writer == nil {
		return
	}
	_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Now().Add(streamWriteTimeout))
}

func StreamScannerHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, dataHandler func(data string, sr *StreamResult)) {
	StreamScannerHandlerWithOptions(c, resp, info, StreamScannerOptions{}, dataHandler)
}

type StreamScannerOptions struct {
	RawLines    bool // NDJSON transports such as Ollama and Cohere.
	IncludeMeta bool // Legacy Zhipu sends usage in a meta: field.
}

type streamChunk struct{ data, event string }

func StreamScannerHandlerWithOptions(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, options StreamScannerOptions, dataHandler func(data string, sr *StreamResult)) {

	if resp == nil || resp.Body == nil || info == nil || dataHandler == nil {
		return
	}

	// 无条件新建 StreamStatus
	info.StreamStatus = relaycommon.NewStreamStatus()

	ctx, cancel := context.WithCancel(context.Background())
	streamingTimeout := time.Duration(constant.StreamingTimeout) * time.Second
	if streamingTimeout <= 0 {
		streamingTimeout = 300 * time.Second
	}

	var (
		stopChan    = make(chan bool)
		scanner     = NewStreamScanner(resp.Body)
		ticker      = time.NewTicker(streamingTimeout)
		pingTicker  *time.Ticker
		writeMutex  sync.Mutex
		wg          sync.WaitGroup
		cleanupOnce sync.Once
		stopOnce    sync.Once
	)
	stop := func() {
		stopOnce.Do(func() { close(stopChan) })
	}

	generalSettings := operation_setting.GetGeneralSetting()
	pingEnabled, pingInterval := ResolveStreamPing(info, generalSettings)

	if pingEnabled {
		pingTicker = time.NewTicker(pingInterval)
	}

	logger.LogDebug(c, "relay timeout seconds: %d", common.RelayTimeout)
	logger.LogDebug(c, "relay max idle conns: %d", common.RelayMaxIdleConns)
	logger.LogDebug(c, "relay max idle conns per host: %d", common.RelayMaxIdleConnsPerHost)
	logger.LogDebug(c, "streaming timeout seconds: %d", int64(streamingTimeout.Seconds()))
	logger.LogDebug(c, "ping interval seconds: %d", int64(pingInterval.Seconds()))

	cleanup := func() {
		cleanupOnce.Do(func() {
			cancel()
			stop()
			if resp.Body != nil {
				_ = resp.Body.Close()
			}
			ticker.Stop()
			if pingTicker != nil {
				pingTicker.Stop()
			}
			// Gin may recycle c immediately after this function returns. Every
			// goroutine that can access c must therefore be joined first.
			wg.Wait()
			_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})
		})
	}
	defer cleanup()

	scanner.Split(bufio.ScanLines)
	SetEventStreamHeaders(c)

	ctx = context.WithValue(ctx, "stop_chan", stopChan)

	// Handle ping data sending with improved error handling
	if pingEnabled && pingTicker != nil {
		wg.Add(1)
		gopool.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					logger.LogError(c, fmt.Sprintf("ping goroutine panic: %v", r))
					info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("ping panic: %v", r))
					stop()
				}
				logger.LogDebug(c, "ping goroutine exited")
				wg.Done()
			}()

			// 添加超时保护，防止 goroutine 无限运行
			maxPingDuration := 30 * time.Minute // 最大 ping 持续时间
			pingTimeout := time.NewTimer(maxPingDuration)
			defer pingTimeout.Stop()

			for {
				select {
				case <-pingTicker.C:
					var err error
					func() {
						writeMutex.Lock()
						defer writeMutex.Unlock()
						ExtendWriteDeadline(c)
						err = PingData(c)
					}()
					if err != nil {
						logger.LogError(c, "ping data error: "+err.Error())
						info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPingFail, err)
						stop()
						return
					}
					logger.LogDebug(c, "ping data sent")
				case <-ctx.Done():
					return
				case <-stopChan:
					return
				case <-c.Request.Context().Done():
					// 监听客户端断开连接
					return
				case <-pingTimeout.C:
					logger.LogError(c, "ping goroutine max duration reached")
					return
				}
			}
		})
	}

	dataChan := make(chan streamChunk, 10)

	wg.Add(1)
	gopool.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				logger.LogError(c, fmt.Sprintf("data handler goroutine panic: %v", r))
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("handler panic: %v", r))
			}
			// The scanner may reach EOF before buffered events are handled. Only
			// classify EOF after the handler has drained dataChan, so handler-level
			// terminators such as response.completed can win deterministically.
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonEOF, nil)
			stop()
			wg.Done()
		}()
		sr := newStreamResult(info.StreamStatus)
		for chunk := range dataChan {
			data := chunk.data
			if data == "[DONE]" {
				isResponses := info.RelayMode == relayconstant.RelayModeResponses ||
					info.RelayMode == relayconstant.RelayModeResponsesCompact
				if !isResponses {
					sr.Done()
				}
				return
			}
			info.SetFirstResponseTime()
			info.ReceivedResponseCount++
			sr.reset()
			sr.EventType = chunk.event
			func() {
				writeMutex.Lock()
				defer writeMutex.Unlock()
				ExtendWriteDeadline(c)
				dataHandler(data, sr)
			}()
			if err := StreamWriteError(c); err != nil {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonWriteFail, err)
				return
			}
			if sr.IsStopped() {
				return
			}
		}
	})

	// Scanner goroutine with improved error handling
	wg.Add(1)
	gopool.Go(func() {
		defer func() {
			close(dataChan)
			if r := recover(); r != nil {
				logger.LogError(c, fmt.Sprintf("scanner goroutine panic: %v", r))
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("scanner panic: %v", r))
			}
			logger.LogDebug(c, "scanner goroutine exited")
			wg.Done()
		}()

		var pending strings.Builder
		var currentEvent string
		dispatch := func(data string) bool {
			if data != "[DONE]" {
				ticker.Reset(streamingTimeout)
			}
			select {
			case dataChan <- streamChunk{data: data, event: currentEvent}:
				return data != "[DONE]"
			case <-ctx.Done():
				return false
			case <-stopChan:
				return false
			}
		}
		flushPending := func() bool {
			if pending.Len() == 0 {
				return true
			}
			data := pending.String()
			pending.Reset()
			return dispatch(data)
		}
		firstLine := true
		for scanner.Scan() {
			// 检查是否需要停止
			select {
			case <-stopChan:
				return
			case <-ctx.Done():
				return
			default:
			}

			data := scanner.Text()
			if firstLine {
				data = strings.TrimPrefix(data, "\ufeff")
				firstLine = false
			}
			logger.LogDebug(c, "stream scanner data: %s", data)
			if data == "" {
				if !flushPending() {
					return
				}
				currentEvent = ""
				continue
			}
			if !options.RawLines && strings.HasPrefix(data, "event:") {
				if !flushPending() {
					return
				}
				currentEvent = strings.TrimSpace(strings.TrimPrefix(data, "event:"))
				continue
			}

			trimmedLine := strings.TrimSpace(data)
			bareDone := trimmedLine == "[DONE]"
			isMeta := options.IncludeMeta && strings.HasPrefix(data, "meta:")
			if !options.RawLines && !isMeta && !bareDone && (len(data) < 5 || data[:5] != "data:") {
				continue
			}
			if bareDone {
				data = "[DONE]"
			} else if !options.RawLines {
				data = data[5:]
			}
			if isMeta {
				currentEvent = "meta"
			}
			data = strings.TrimSpace(data)
			if data == "" {
				continue
			}
			if data == "[DONE]" {
				if !flushPending() {
					return
				}
				_ = dispatch(data)
				return
			}
			// Providers also send one complete JSON value per line without blank
			// separators. Keep that compatibility while assembling pretty-printed
			// JSON split across several SSE data fields.
			if options.RawLines || (pending.Len() == 0 && ((!strings.HasPrefix(data, "{") && !strings.HasPrefix(data, "[")) || common.IsValidJSON(common.StringToByteSlice(data)))) {
				if !dispatch(data) {
					return
				}
				continue
			}
			if pending.Len() > 0 {
				pending.WriteByte('\n')
			}
			pending.WriteString(data)
			if pending.Len() > getScannerBufferSize() {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonScannerErr, fmt.Errorf("SSE event exceeds configured buffer limit"))
				return
			}
		}
		if !flushPending() {
			return
		}

		if err := scanner.Err(); err != nil {
			requestCanceled := c != nil && c.Request != nil && c.Request.Context().Err() != nil
			if err != io.EOF && !requestCanceled && ctx.Err() == nil {
				logger.LogError(c, "scanner error: "+err.Error())
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonScannerErr, err)
			}
		}
	})

	// 主循环等待完成或超时
	select {
	case <-ticker.C:
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonTimeout, nil)
	case <-stopChan:
		// EndReason already set by the goroutine that triggered stopChan
	case <-c.Request.Context().Done():
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
	}

	cleanup()
	if info.StreamStatus.IsNormalEnd() && !info.StreamStatus.HasErrors() {
		logger.LogInfo(c, fmt.Sprintf("stream ended: %s", info.StreamStatus.Summary()))
	} else {
		logger.LogError(c, fmt.Sprintf("stream ended: %s, received=%d", info.StreamStatus.Summary(), info.ReceivedResponseCount))
	}
}
