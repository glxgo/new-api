package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
)

// HTTPTransportPolicy is the runtime-normalized channel transport policy.
type HTTPTransportPolicy struct {
	Protocol string
	Shards   int
}

var httpTransportPolicyWarnings sync.Map

func defaultHTTPTransportPolicy() HTTPTransportPolicy {
	return HTTPTransportPolicy{Protocol: dto.HTTPProtocolAuto, Shards: 1}
}

// NormalizeHTTPTransportPolicy clamps old or malformed stored settings instead
// of allowing an invalid channel record to break all outbound requests.
func NormalizeHTTPTransportPolicy(settings dto.ChannelSettings) HTTPTransportPolicy {
	policy := defaultHTTPTransportPolicy()
	switch protocol := strings.ToLower(strings.TrimSpace(settings.HTTPProtocol)); protocol {
	case "", dto.HTTPProtocolAuto:
	case dto.HTTPProtocolHTTP1:
		policy.Protocol = dto.HTTPProtocolHTTP1
	default:
		warnHTTPTransportPolicyOnce("http_protocol", settings.HTTPProtocol)
	}
	switch shards := settings.HTTP2ConnectionShards; {
	case shards == 0:
	case shards < 1:
		warnHTTPTransportPolicyOnce("http2_connection_shards", fmt.Sprintf("%d", shards))
	case shards > dto.MaxHTTP2ConnectionShards:
		warnHTTPTransportPolicyOnce("http2_connection_shards", fmt.Sprintf("%d", shards))
		policy.Shards = dto.MaxHTTP2ConnectionShards
	default:
		policy.Shards = shards
	}
	if policy.Protocol == dto.HTTPProtocolHTTP1 {
		if settings.HTTP2ConnectionShards > 1 {
			warnHTTPTransportPolicyOnce("http_protocol+http2_connection_shards", fmt.Sprintf("http1+%d", settings.HTTP2ConnectionShards))
		}
		policy.Shards = 1
	}
	return policy
}

func warnHTTPTransportPolicyOnce(field, value string) {
	key := field + "=" + value
	if _, loaded := httpTransportPolicyWarnings.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	logger.LogWarn(context.Background(), fmt.Sprintf("invalid channel http transport setting clamped: %s=%q", field, value))
}

func (p HTTPTransportPolicy) cacheKeyPart() string {
	return fmt.Sprintf("%s|%d", p.Protocol, p.Shards)
}
