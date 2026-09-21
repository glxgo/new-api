package xunfei

import (
	"context"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamAuditXunfeiBufferedMessagesAreNotDropped(t *testing.T) {
	sent := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.ReadMessage()
		for i := 0; i < 32; i++ {
			status := 0
			if i == 31 {
				status = 2
			}
			conn.WriteJSON(map[string]any{"payload": map[string]any{"choices": map[string]any{"status": status, "text": []any{map[string]any{"content": "a"}}}}})
		}
		close(sent)
	}))
	defer server.Close()
	conn, err := xunfeiMakeRequest(context.Background(), dto.GeneralOpenAIRequest{}, "test", "ws"+strings.TrimPrefix(server.URL, "http"), "app")
	require.NoError(t, err)
	<-sent
	time.Sleep(20 * time.Millisecond)
	defer conn.Close()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/", nil)
	info := &relaycommon.RelayInfo{IsStream: true, ChannelMeta: &relaycommon.ChannelMeta{}}
	_, apiErr := xunfeiReadResponse(c, info, conn)
	require.Nil(t, apiErr)
	require.Equal(t, 32, info.ReceivedResponseCount)
	require.Equal(t, 32, strings.Count(recorder.Body.String(), `"content":"a"`))
	require.Equal(t, 1, strings.Count(recorder.Body.String(), "[DONE]"))
}

func TestStreamAuditXunfeiFailuresAndNonStream(t *testing.T) {
	for _, scenario := range []string{"nonstream", "error", "malformed", "premature_close", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			ready := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				conn.ReadMessage()
				close(ready)
				switch scenario {
				case "nonstream":
					conn.WriteMessage(websocket.TextMessage, []byte(`{"payload":{"choices":{"status":0,"text":[{"content":"hello "}]}}}`))
					conn.WriteMessage(websocket.TextMessage, []byte(`{"payload":{"choices":{"status":2,"text":[{"content":"world"}]},"usage":{"text":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}}}`))
				case "error":
					conn.WriteMessage(websocket.TextMessage, []byte(`{"header":{"code":10013,"message":"upstream failure"}}`))
				case "malformed":
					conn.WriteMessage(websocket.TextMessage, []byte(`{broken`))
				case "cancel":
					conn.ReadMessage()
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			conn, err := xunfeiMakeRequest(ctx, dto.GeneralOpenAIRequest{}, "test", "ws"+strings.TrimPrefix(server.URL, "http"), "app")
			require.NoError(t, err)
			defer conn.Close()
			<-ready
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest("POST", "/", nil).WithContext(ctx)
			info := &relaycommon.RelayInfo{IsStream: scenario != "nonstream", ChannelMeta: &relaycommon.ChannelMeta{}}
			if scenario == "cancel" {
				cancel()
			}
			usage, apiErr := xunfeiReadResponse(c, info, conn)
			if scenario == "nonstream" {
				require.Nil(t, apiErr)
				require.Contains(t, recorder.Body.String(), "hello world")
				require.Equal(t, 5, usage.TotalTokens)
			} else {
				require.NotNil(t, apiErr)
				require.NotContains(t, recorder.Body.String(), "[DONE]")
			}
		})
	}
}
