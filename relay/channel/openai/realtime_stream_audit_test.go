package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func auditWSPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err == nil {
			accepted <- conn
		}
	}))
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	peer := <-accepted
	t.Cleanup(func() { client.Close(); peer.Close(); server.Close() })
	return client, peer
}

func TestStreamAuditRealtimeCancellationJoinsReaders(t *testing.T) {
	client, downstream := auditWSPair(t)
	upstream, provider := auditWSPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/realtime", nil).WithContext(ctx)
	info := &relaycommon.RelayInfo{ClientWs: downstream, TargetWs: upstream, ChannelMeta: &relaycommon.ChannelMeta{}}
	done := make(chan *types.NewAPIError, 1)
	go func() { err, _ := OpenaiRealtimeHandler(c, info); done <- err }()
	cancel()
	select {
	case err := <-done:
		require.NotNil(t, err)
		require.Equal(t, 499, err.StatusCode)
	case <-time.After(time.Second):
		client.Close()
		provider.Close()
		<-done
		t.Fatal("request cancellation did not interrupt WebSocket readers")
	}
	// The client connection must still accept an error frame from the controller.
	require.NoError(t, downstream.WriteMessage(websocket.TextMessage, []byte(`{"type":"error"}`)))
	_, msg, err := client.ReadMessage()
	require.NoError(t, err)
	require.Contains(t, string(msg), "error")
}

func TestStreamAuditRealtimeBidirectionalAndTruncation(t *testing.T) {
	client, downstream := auditWSPair(t)
	upstream, provider := auditWSPair(t)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/realtime", nil)
	info := &relaycommon.RelayInfo{ClientWs: downstream, TargetWs: upstream, ChannelMeta: &relaycommon.ChannelMeta{}, IsFirstRequest: true}
	info.UsePrice = true
	done := make(chan *types.NewAPIError, 1)
	go func() { err, _ := OpenaiRealtimeHandler(c, info); done <- err }()
	upstreamDone := make(chan error, 1)
	go func() {
		for i := 0; i < 25; i++ {
			if err := provider.WriteMessage(websocket.TextMessage, []byte(`{"type":"session.updated","session":{"input_audio_format":"pcm16"}}`)); err != nil {
				upstreamDone <- err
				return
			}
			if _, _, err := provider.ReadMessage(); err != nil {
				upstreamDone <- err
				return
			}
		}
		upstreamDone <- nil
	}()
	for i := 0; i < 25; i++ {
		require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(`{"type":"session.update","session":{"tools":[]}}`)))
		_, _, err := client.ReadMessage()
		require.NoError(t, err)
	}
	require.NoError(t, <-upstreamDone)
	provider.Close()
	select {
	case err := <-done:
		require.NotNil(t, err)
		require.Equal(t, 502, err.StatusCode)
		require.True(t, types.IsSkipRetryError(err))
	case <-time.After(time.Second):
		client.Close()
		<-done
		t.Fatal("upstream close left relay running")
	}
}

type auditImageFailWriter struct{ *httptest.ResponseRecorder }

func (w *auditImageFailWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestStreamAuditImageJSONWriteFailure(t *testing.T) {
	_, _, resp, info := newImageTestContext(t, `{"data":[{"b64_json":"abc"}]}`, "application/json", true)
	c, _ := gin.CreateTestContext(&auditImageFailWriter{httptest.NewRecorder()})
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	_, err := OpenaiImageJSONAsStreamHandler(c, info, resp)
	require.NotNil(t, err)
	require.Equal(t, 499, err.StatusCode)
	require.True(t, types.IsSkipRetryError(err))
}

func TestStreamAuditRealtimeNormalCompletionUsage(t *testing.T) {
	client, downstream := auditWSPair(t)
	upstream, provider := auditWSPair(t)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/realtime", nil)
	info := &relaycommon.RelayInfo{ClientWs: downstream, TargetWs: upstream, ChannelMeta: &relaycommon.ChannelMeta{}}
	info.UsePrice = true
	type result struct {
		err   *types.NewAPIError
		total int
	}
	done := make(chan result, 1)
	go func() { err, usage := OpenaiRealtimeHandler(c, info); done <- result{err, usage.TotalTokens} }()
	require.NoError(t, provider.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.done","response":{"usage":{"total_tokens":5,"input_tokens":2,"output_tokens":3}}}`)))
	_, msg, err := client.ReadMessage()
	require.NoError(t, err)
	require.Contains(t, string(msg), "response.done")
	require.NoError(t, provider.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(1000, ""), time.Now().Add(time.Second)))
	r := <-done
	require.Nil(t, r.err)
	require.Equal(t, 5, r.total)
}

func TestStreamAuditTTSBinaryReadFailure(t *testing.T) {
	c, info, resp, w := newResponsesStreamTest(t, "")
	info.IsStream = false
	resp.Header = http.Header{"Content-Type": []string{"audio/mpeg"}}
	resp.Body = io.NopCloser(io.MultiReader(strings.NewReader("partial"), auditReadError{io.ErrUnexpectedEOF}))
	_ = OpenaiTTSHandler(c, resp, info)
	require.NotNil(t, helper.StreamFailure(c, info, false))
	require.Empty(t, w.Body.String())
}
