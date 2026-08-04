package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
)

// shardedRoundTripper spreads requests for each origin across independent
// transports. This gives HTTP/2 a bounded number of reusable connections.
type shardedRoundTripper struct {
	shards   []http.RoundTripper
	n        uint32
	policy   HTTPTransportPolicy
	counters sync.Map
}

func newShardedRoundTripper(policy HTTPTransportPolicy, factory func() *http.Transport) *shardedRoundTripper {
	n := policy.Shards
	if n < 1 {
		n = 1
	}
	shards := make([]http.RoundTripper, n)
	for i := range shards {
		transport := factory()
		transport.MaxIdleConns = maxInt(1, transport.MaxIdleConns/n)
		transport.MaxIdleConnsPerHost = maxInt(1, transport.MaxIdleConnsPerHost/n)
		shards[i] = transport
	}
	return &shardedRoundTripper{shards: shards, n: uint32(n), policy: policy}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func originKey(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	return strings.ToLower(req.URL.Scheme) + "://" + req.URL.Host
}

func (s *shardedRoundTripper) pickShard(origin string) uint32 {
	if s.n <= 1 {
		return 0
	}
	counterAny, _ := s.counters.LoadOrStore(origin, &atomic.Uint32{})
	return (counterAny.(*atomic.Uint32).Add(1) - 1) % s.n
}

func (s *shardedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	idx := s.pickShard(originKey(req))
	resp, err := s.shards[idx].RoundTrip(req)
	if common.DebugEnabled {
		protocol := ""
		if resp != nil {
			protocol = resp.Proto
		}
		host := ""
		if req != nil && req.URL != nil {
			host = req.URL.Host
		}
		ctx := contextOrBackground(req)
		logger.LogDebug(ctx, fmt.Sprintf("http transport: host=%s protocol=%s shard=%d/%d policy=%s negotiated=%s", host, s.policy.Protocol, idx, s.n, s.policy.cacheKeyPart(), protocol))
	}
	return resp, err
}

func contextOrBackground(req *http.Request) context.Context {
	if req == nil {
		return context.Background()
	}
	return req.Context()
}

func (s *shardedRoundTripper) CloseIdleConnections() {
	for _, shard := range s.shards {
		if closer, ok := shard.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
	}
}

func applyHTTP1Force(transport *http.Transport) {
	if transport == nil {
		return
	}
	transport.ForceAttemptHTTP2 = false
	transport.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
	if transport.TLSClientConfig != nil {
		cfg := transport.TLSClientConfig.Clone()
		cfg.NextProtos = nil
		transport.TLSClientConfig = cfg
	}
}

func applyHTTPTransportPolicy(transport *http.Transport, policy HTTPTransportPolicy) {
	if transport == nil {
		return
	}
	if policy.Protocol == dto.HTTPProtocolHTTP1 {
		applyHTTP1Force(transport)
		return
	}
	transport.ForceAttemptHTTP2 = true
}
