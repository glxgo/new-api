package channel

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type headerPingFailWriter struct{ *httptest.ResponseRecorder }

func (w headerPingFailWriter) Write([]byte) (int, error) {
	return 0, errors.New("downstream disconnected")
}

func TestStreamAuditHeaderPingFailureCancelsUpstream(t *testing.T) {
	settings := operation_setting.GetGeneralSetting()
	previous := *settings
	settings.PingIntervalEnabled, settings.PingIntervalSeconds = true, 1
	t.Cleanup(func() { *settings = previous })
	upstreamCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
		close(upstreamCanceled)
	}))
	defer server.Close()
	c, _ := gin.CreateTestContext(headerPingFailWriter{httptest.NewRecorder()})
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, strings.NewReader("{}"))
	require.NoError(t, err)
	info := &relaycommon.RelayInfo{IsStream: true, ChannelMeta: &relaycommon.ChannelMeta{}}
	start := time.Now()
	_, err = doRequest(c, req, info)
	require.Error(t, err)
	require.Less(t, time.Since(start), 3*time.Second, "failed keepalive must cancel an outstanding header read")
	select {
	case <-upstreamCanceled:
	case <-time.After(time.Second):
		t.Fatal("upstream request still running")
	}
}
