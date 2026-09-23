package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestPublicErrorSSEProtocols(t *testing.T) {
	for _, protocol := range []string{"chat", "claude", "responses"} {
		t.Run(protocol, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest("POST", "/", nil)
			data := `{"type":"error","error":{"message":"https://private.example/v1","metadata":{"url":"https://private.example/debug"}}}`
			switch protocol {
			case "chat":
				require.NoError(t, StringData(c, data))
			case "claude":
				ClaudeChunkData(c, dto.ClaudeResponse{Type: "error"}, data)
			case "responses":
				require.NoError(t, ResponseChunkData(c, dto.ResponsesStreamResponse{Type: "error"}, data))
			}
			require.NotContains(t, recorder.Body.String(), "private.example")
			require.Contains(t, recorder.Body.String(), "data: ")
			require.True(t, strings.HasSuffix(recorder.Body.String(), "\n\n"))
		})
	}
}

func TestPublicErrorWebSocket(t *testing.T) {
	upgrader := websocket.Upgrader{}
	errCh := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		errCh <- WssObject(nil, conn, map[string]any{"type": "error", "error": map[string]any{"message": "https://private.example/ws"}})
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer conn.Close()
	_, payload, err := conn.ReadMessage()
	require.NoError(t, err)
	require.NoError(t, <-errCh)
	require.NotContains(t, string(payload), "private.example")
	var data map[string]any
	require.NoError(t, common.Unmarshal(payload, &data))
	require.Equal(t, "error", data["type"])
}
