package volcengine

import (
	"context"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type auditAudioWriter struct{ *httptest.ResponseRecorder }

func (w *auditAudioWriter) FlushError() error { return io.ErrClosedPipe }

func TestStreamAuditVolcengineWebSocket(t *testing.T) {
	for _, scenario := range []string{"complete", "premature_close", "cancel", "flush_failure"} {
		t.Run(scenario, func(t *testing.T) {
			ready := make(chan struct{})
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				if _, _, err = conn.ReadMessage(); err != nil {
					return
				}
				close(ready)
				if scenario == "cancel" {
					<-release
					return
				}
				if scenario == "premature_close" {
					conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(1000, ""), time.Now().Add(time.Second))
					return
				}
				msg, _ := NewMessage(MsgTypeAudioOnlyServer, MsgTypeFlagNegativeSeq)
				msg.Sequence = -1
				msg.Payload = []byte("audio")
				frame, _ := msg.Marshal()
				conn.WriteMessage(websocket.BinaryMessage, frame)
			}))
			defer server.Close()
			defer close(release)
			recorder := httptest.NewRecorder()
			var writer http.ResponseWriter = recorder
			if scenario == "flush_failure" {
				writer = &auditAudioWriter{recorder}
			}
			c, _ := gin.CreateTestContext(writer)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c.Request = httptest.NewRequest("POST", "/v1/audio/speech", nil).WithContext(ctx)
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "app|test"}}
			result := make(chan *types.NewAPIError, 1)
			go func() {
				_, err := handleTTSWebSocketResponse(c, "ws"+strings.TrimPrefix(server.URL, "http"), VolcengineTTSRequest{}, info, "mp3")
				result <- err
			}()
			<-ready
			if scenario == "cancel" {
				cancel()
			}
			select {
			case err := <-result:
				if scenario == "complete" {
					require.Nil(t, err)
					require.Equal(t, "audio", recorder.Body.String())
				} else {
					require.NotNil(t, err)
					if scenario != "premature_close" {
						require.Equal(t, 499, err.StatusCode)
					}
				}
			case <-time.After(time.Second):
				// Release the fixture without hiding the cancellation failure.
				go func() { release <- struct{}{} }()
				<-result
				t.Fatal("cancellation left audio read blocked")
			}
		})
	}
}

func TestStreamAuditVolcengineRejectsTruncatedTerminalPayload(t *testing.T) {
	msg, _ := NewMessage(MsgTypeAudioOnlyServer, MsgTypeFlagNegativeSeq)
	msg.Sequence = -1
	msg.Payload = []byte("audio")
	frame, err := msg.Marshal()
	require.NoError(t, err)
	_, err = NewMessageFromBytes(frame[:len(frame)-2])
	require.Error(t, err, "a terminal frame with missing audio bytes must not complete successfully")
}
