package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestInitHttpClient_ConfiguresResponseHeaderTimeout(t *testing.T) {
	oldHeaderTimeout := common.RelayResponseHeaderTimeout
	oldRelayTimeout := common.RelayTimeout
	common.RelayResponseHeaderTimeout = 123
	common.RelayTimeout = 0
	t.Cleanup(func() {
		common.RelayResponseHeaderTimeout = oldHeaderTimeout
		common.RelayTimeout = oldRelayTimeout
		InitHttpClient()
	})

	InitHttpClient()
	client := GetHttpClient()
	require.NotNil(t, client)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.Equal(t, 123*time.Second, transport.ResponseHeaderTimeout)
	require.Zero(t, client.Timeout, "response-header timeout must not limit a long streaming body")
}

func TestInitHttpClient_ResponseHeaderTimeoutDoesNotWaitForSlowHeaders(t *testing.T) {
	oldHeaderTimeout := common.RelayResponseHeaderTimeout
	oldRelayTimeout := common.RelayTimeout
	common.RelayResponseHeaderTimeout = 1
	common.RelayTimeout = 0
	t.Cleanup(func() {
		common.RelayResponseHeaderTimeout = oldHeaderTimeout
		common.RelayTimeout = oldRelayTimeout
		InitHttpClient()
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	InitHttpClient()
	startedAt := time.Now()
	_, err := GetHttpClient().Get(server.URL)
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "timeout awaiting response headers")
	require.Less(t, time.Since(startedAt), 1500*time.Millisecond)
}

func TestNewProxyHttpClient_ConfiguresResponseHeaderTimeout(t *testing.T) {
	oldHeaderTimeout := common.RelayResponseHeaderTimeout
	common.RelayResponseHeaderTimeout = 123
	ResetProxyClientCache()
	t.Cleanup(func() {
		ResetProxyClientCache()
		common.RelayResponseHeaderTimeout = oldHeaderTimeout
	})

	client, err := NewProxyHttpClient("http://127.0.0.1:18080")
	require.NoError(t, err)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.Equal(t, 123*time.Second, transport.ResponseHeaderTimeout)
}
