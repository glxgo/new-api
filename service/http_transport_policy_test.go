package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestNormalizeHTTPTransportPolicyClampsAndDisablesShardsForHTTP1(t *testing.T) {
	policy := NormalizeHTTPTransportPolicy(dto.ChannelSettings{HTTPProtocol: "http1", HTTP2ConnectionShards: 8})
	require.Equal(t, dto.HTTPProtocolHTTP1, policy.Protocol)
	require.Equal(t, 1, policy.Shards)

	policy = NormalizeHTTPTransportPolicy(dto.ChannelSettings{HTTPProtocol: "unexpected", HTTP2ConnectionShards: 99})
	require.Equal(t, dto.HTTPProtocolAuto, policy.Protocol)
	require.Equal(t, dto.MaxHTTP2ConnectionShards, policy.Shards)
}
