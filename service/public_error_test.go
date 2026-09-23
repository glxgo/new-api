package service

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicErrorHTTPBodyAndHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(common.RequestIdKey, "local-request")
	c.Header(common.RequestIdKey, "local-request")
	resp := &http.Response{StatusCode: 502, Header: http.Header{
		"Content-Type": {"application/json"}, "Location": {"https://private.example/fail"},
		"X-Upstream-Addr": {"private.example:443"}, "Server": {"private.example"},
		"Set-Cookie": {"route=private.example"}, "Retry-After": {"3"},
		"X-Oneapi-Request-Id": {"upstream-request"}, "X-Ratelimit-Remaining-Tokens": {"42"},
	}}
	IOCopyBytesGracefully(c, resp, []byte(`{"error":{"message":"https://private.example/v1/responses","metadata":{"url":"https://private.example/debug"}}}`))
	require.Equal(t, 502, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "private.example")
	for _, header := range []string{"Location", "X-Upstream-Addr", "Server", "Set-Cookie"} {
		require.Empty(t, recorder.Header().Get(header))
	}
	require.Equal(t, "3", recorder.Header().Get("Retry-After"))
	require.Equal(t, "42", recorder.Header().Get("X-Ratelimit-Remaining-Tokens"))
	require.Equal(t, "local-request", recorder.Header().Get(common.RequestIdKey))
	require.Equal(t, "upstream-request", c.GetString(common.UpstreamRequestIdKey))
	require.Equal(t, strconv.Itoa(recorder.Body.Len()), recorder.Header().Get("Content-Length"))
}

func TestPublicErrorHTTPSuccessUnchanged(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"data":[{"url":"https://images.example/success.png"}],"error":null}`),
		{0x1f, 0x8b, 0x08, 0x00, 0xff}, // opaque compressed media is not JSON
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Encoding": {"gzip"}}}
		IOCopyBytesGracefully(c, resp, body)
		require.Equal(t, body, recorder.Body.Bytes())
		require.Equal(t, "gzip", recorder.Header().Get("Content-Encoding"))
	}
}

func TestPublicErrorHTTPGatewayHTML(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	resp := &http.Response{StatusCode: 502, Header: http.Header{"Content-Type": {"text/html"}, "Content-Encoding": {"gzip"}}}
	IOCopyBytesGracefully(c, resp, []byte(`<html>Bad gateway: https://private.example/fail</html>`))
	require.Equal(t, 502, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "private.example")
	require.Empty(t, recorder.Header().Get("Content-Encoding"))
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
}

func TestPublicErrorMediaGateway(t *testing.T) {
	for _, body := range []string{`{"error":{"message":"https://private.example"}}`, `<html>https://private.example</html>`, `502 https://private.example`} {
		resp := &http.Response{Header: http.Header{"Content-Type": {"video/mp4"}}, Body: io.NopCloser(strings.NewReader(body))}
		require.True(t, HasMediaDiagnostic(resp))
		require.NoError(t, resp.Body.Close())
	}
	// A peek must not consume or rewrite successful media bytes.
	body := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0xff, 0x00}
	resp := &http.Response{Header: http.Header{"Content-Type": {"video/mp4"}}, Body: io.NopCloser(bytes.NewReader(body))}
	require.False(t, HasMediaDiagnostic(resp))
	actual, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, body, actual)
	require.NoError(t, resp.Body.Close())
}
