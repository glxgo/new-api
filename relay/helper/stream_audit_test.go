package helper

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type auditFailWriter struct {
	*httptest.ResponseRecorder
	writeErr error
	flushErr error
}

func (w *auditFailWriter) Write(p []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.ResponseRecorder.Write(p)
}

func (w *auditFailWriter) FlushError() error { return w.flushErr }

func TestStreamAuditWriteFailuresPropagate(t *testing.T) {
	for _, flush := range []bool{false, true} {
		for _, format := range []string{"chat", "responses", "claude", "ping"} {
			t.Run(format+map[bool]string{true: "/flush", false: "/write"}[flush], func(t *testing.T) {
				want := errors.New("downstream unavailable")
				w := &auditFailWriter{ResponseRecorder: httptest.NewRecorder()}
				if flush {
					w.flushErr = want
				} else {
					w.writeErr = want
				}
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
				var err error
				switch format {
				case "chat":
					err = StringData(c, `{"choices":[]}`)
				case "responses":
					err = ResponseChunkData(c, dto.ResponsesStreamResponse{Type: "response.created"}, `{"type":"response.created"}`)
				case "claude":
					err = ClaudeData(c, dto.ClaudeResponse{Type: "message_stop"})
				case "ping":
					err = PingData(c)
				}
				require.ErrorIs(t, err, want)
			})
		}
	}
}

func TestStreamAuditDoneDoesNotHideEarlierHandlerFailure(t *testing.T) {
	c, resp, info := setupStreamTest(t, strings.NewReader("data: failure\n\ndata: [DONE]\n\n"))
	want := errors.New("upstream stream_read_error")
	StreamScannerHandler(c, resp, info, func(_ string, sr *StreamResult) {
		// Ensure the scanner can consume the terminator before handling this event.
		time.Sleep(10 * time.Millisecond)
		sr.Stop(want)
	})
	require.Equal(t, relaycommon.StreamEndReasonHandlerStop, info.StreamStatus.EndReason)
	require.ErrorIs(t, info.StreamStatus.EndError, want)
}

func TestStreamAuditPingFailureStopsBlockedRead(t *testing.T) {
	settings := operation_setting.GetGeneralSetting()
	old := *settings
	oldTimeout := constant.StreamingTimeout
	settings.PingIntervalEnabled, settings.PingIntervalSeconds = true, 1
	constant.StreamingTimeout = 30
	t.Cleanup(func() { *settings = old; constant.StreamingTimeout = oldTimeout })
	r, w := io.Pipe()
	defer w.Close()
	writer := &auditFailWriter{ResponseRecorder: httptest.NewRecorder(), writeErr: errors.New("broken pipe")}
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	done := make(chan struct{})
	go func() {
		StreamScannerHandler(c, &http.Response{Body: r}, info, func(string, *StreamResult) {})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		w.Close()
		<-done
		t.Fatal("failed ping left upstream read alive until the idle timeout")
	}
	require.Equal(t, relaycommon.StreamEndReasonPingFail, info.StreamStatus.EndReason)
}

func TestStreamAuditErrorAfterHeartbeatUsesSSE(t *testing.T) {
	for _, format := range []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, types.RelayFormatClaude, types.RelayFormatGemini} {
		t.Run(string(format), func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			SetEventStreamHeaders(c)
			require.NoError(t, PingData(c))
			err := types.NewErrorWithStatusCode(errors.New("upstream timeout"), "stream_timeout", http.StatusGatewayTimeout)
			require.NoError(t, WriteStreamError(c, format, err))
			require.True(t, IsSSEWritten(c))
			require.Contains(t, w.Body.String(), "\ndata: ")
			for _, line := range strings.Split(w.Body.String(), "\n") {
				require.True(t, line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "data:") || strings.HasPrefix(line, "event:"), "raw non-SSE line: %s", line)
			}
			require.NotContains(t, w.Body.String(), "[DONE]")
		})
	}
}

func TestStreamAuditOversizedEventFails(t *testing.T) {
	old := constant.StreamScannerMaxBufferMB
	constant.StreamScannerMaxBufferMB = 1
	t.Cleanup(func() { constant.StreamScannerMaxBufferMB = old })
	for _, data := range []string{"data: " + strings.Repeat("x", 2<<20) + "\n", "data: {\n" + strings.Repeat("data: "+strings.Repeat(" ", 128)+strings.Repeat("x", 128)+"\n", 8192)} {
		c, resp, info := setupStreamTest(t, strings.NewReader(data))
		StreamScannerHandler(c, resp, info, func(string, *StreamResult) {})
		require.Equal(t, relaycommon.StreamEndReasonScannerErr, info.StreamStatus.EndReason)
		require.NotNil(t, StreamFailure(c, info, true))
	}
}

func TestStreamAuditCompletedWriteFailureIsNotSuccess(t *testing.T) {
	w := &auditFailWriter{ResponseRecorder: httptest.NewRecorder(), flushErr: errors.New("flush failed")}
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	StreamScannerHandler(c, &http.Response{Body: io.NopCloser(strings.NewReader("data: complete\n\n"))}, info, func(_ string, sr *StreamResult) {
		_ = StringData(c, "complete")
		sr.Done()
	})
	require.Equal(t, relaycommon.StreamEndReasonWriteFail, info.StreamStatus.EndReason)
	require.Equal(t, 499, StreamFailure(c, info, false).StatusCode)
}

func TestStreamAuditWriteFailureWithoutScanner(t *testing.T) {
	w := &auditFailWriter{ResponseRecorder: httptest.NewRecorder(), writeErr: io.ErrClosedPipe}
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)
	require.Error(t, StringData(c, "hello"))
	err := StreamFailure(c, &relaycommon.RelayInfo{}, false)
	require.NotNil(t, err)
	require.Equal(t, 499, err.StatusCode)
}
