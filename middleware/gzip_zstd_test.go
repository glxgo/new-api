package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestDecompressRequestMiddlewareZstd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var compressed bytes.Buffer
	writer, err := zstd.NewWriter(&compressed)
	require.NoError(t, err)
	_, err = writer.Write([]byte(`{"ok":true}`))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	router := gin.New()
	router.Use(DecompressRequestMiddleware())
	router.POST("/", func(c *gin.Context) {
		body, readErr := io.ReadAll(c.Request.Body)
		require.NoError(t, readErr)
		c.String(http.StatusOK, string(body))
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(compressed.Bytes()))
	req.Header.Set("Content-Encoding", "zstd")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, `{"ok":true}`, resp.Body.String())
}
