package minimax

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicErrorChatFallback(t *testing.T) {
	for _, body := range []string{
		`{"base_resp":{"status_code":1000,"status_msg":"502, url: https://private.example/chat","request_url":"https://private.example/chat"}}`,
		`{"success":false,"data":{"error":{"message":"https://private.example/chat"}}}`,
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		_, apiErr := handleChatCompletionResponse(c, &http.Response{StatusCode: 200, Header: http.Header{"X-Upstream-Url": {"https://private.example/chat"}}, Body: io.NopCloser(strings.NewReader(body))}, nil)
		require.Nil(t, apiErr)
		require.NotContains(t, recorder.Body.String(), "private.example")
		require.NotContains(t, recorder.Body.String(), "request_url")
		require.Empty(t, recorder.Header().Get("X-Upstream-Url"))
	}
}
