package middleware

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicErrorPanicRecovery(t *testing.T) {
	for _, recovery := range []gin.HandlerFunc{gin.CustomRecoveryWithWriter(io.Discard, RecoverResponse), RelayPanicRecover()} {
		for _, streaming := range []bool{false, true} {
			router := gin.New()
			router.Use(recovery)
			router.GET("/", func(c *gin.Context) {
				if streaming {
					c.Header("Content-Type", "text/event-stream")
					_, _ = c.Writer.WriteString("data: {\"delta\":\"hello\"}\n\n")
					c.Writer.Flush()
				}
				panic("Post https://private.example/v1: dial tcp 10.0.0.1:443 failed")
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest("GET", "/", nil))
			require.NotContains(t, recorder.Body.String(), "private.example")
			require.NotContains(t, recorder.Body.String(), "10.0.0.1")
			if streaming {
				require.Equal(t, "data: {\"delta\":\"hello\"}\n\n", recorder.Body.String())
			} else {
				require.Equal(t, 500, recorder.Code)
				require.Contains(t, recorder.Body.String(), "new_api_panic")
			}
		}
	}
}

func TestPublicErrorMiddlewareAbort(t *testing.T) {
	for _, mj := range []bool{false, true} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest("POST", "/", nil)
		if mj {
			abortWithMidjourneyMessage(c, 502, 4, "502, url: https://private.example/mj")
		} else {
			abortWithOpenAiMessage(c, 502, "502, url: https://private.example/chat")
		}
		require.Equal(t, 502, recorder.Code)
		require.NotContains(t, recorder.Body.String(), "private.example")
		require.NotContains(t, recorder.Body.String(), "url:")
		require.True(t, c.IsAborted())
	}
}
