package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelSettingsValidateHTTPTransport(t *testing.T) {
	require.NoError(t, (&ChannelSettings{HTTPProtocol: "auto", HTTP2ConnectionShards: 8}).ValidateHTTPTransport())
	require.NoError(t, (&ChannelSettings{HTTPProtocol: "http1", HTTP2ConnectionShards: 1}).ValidateHTTPTransport())
	require.Error(t, (&ChannelSettings{HTTPProtocol: "http3"}).ValidateHTTPTransport())
	require.Error(t, (&ChannelSettings{HTTPProtocol: "http1", HTTP2ConnectionShards: 2}).ValidateHTTPTransport())
	require.Error(t, (&ChannelSettings{HTTP2ConnectionShards: 9}).ValidateHTTPTransport())
}
