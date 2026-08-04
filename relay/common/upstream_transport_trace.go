package common

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"
)

// UpstreamTransportSnapshot contains transport-only metadata. It deliberately
// excludes URLs, headers, IP addresses, request bodies, and credentials.
type UpstreamTransportSnapshot struct {
	Protocol              string
	ResponseHeaderLatency time.Duration
	ConnectionReused      bool
	ConnectionWasIdle     bool
	ConnectionIdleTime    time.Duration
	ConnectionFingerprint string
}

// UpstreamTransportTrace captures enough net/http evidence to distinguish
// protocol and pooled-connection failures without logging customer content or
// upstream addresses. One trace belongs to one upstream attempt.
type UpstreamTransportTrace struct {
	mu sync.Mutex

	startedAt             time.Time
	responseHeadersAt     time.Time
	protocol              string
	connectionReused      bool
	connectionWasIdle     bool
	connectionIdleTime    time.Duration
	connectionFingerprint string
}

func NewUpstreamTransportTrace(startedAt time.Time) *UpstreamTransportTrace {
	return &UpstreamTransportTrace{startedAt: startedAt}
}

func (t *UpstreamTransportTrace) Attach(req *http.Request) *http.Request {
	if t == nil || req == nil {
		return req
	}
	trace := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.connectionReused = info.Reused
			t.connectionWasIdle = info.WasIdle
			if info.IdleTime > 0 {
				t.connectionIdleTime = info.IdleTime
			}
			if info.Conn != nil && info.Conn.RemoteAddr() != nil {
				value := info.Conn.RemoteAddr().Network() + "|" + info.Conn.RemoteAddr().String()
				digest := sha256.Sum256([]byte(value))
				t.connectionFingerprint = hex.EncodeToString(digest[:6])
			}
		},
	}
	return req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
}

func (t *UpstreamTransportTrace) RecordResponse(resp *http.Response, responseHeadersAt time.Time) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.responseHeadersAt = responseHeadersAt
	if resp != nil {
		t.protocol = resp.Proto
	}
}

func (t *UpstreamTransportTrace) Snapshot() UpstreamTransportSnapshot {
	if t == nil {
		return UpstreamTransportSnapshot{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	headerLatency := time.Duration(0)
	if !t.startedAt.IsZero() && !t.responseHeadersAt.IsZero() && !t.responseHeadersAt.Before(t.startedAt) {
		headerLatency = t.responseHeadersAt.Sub(t.startedAt)
	}
	return UpstreamTransportSnapshot{
		Protocol:              t.protocol,
		ResponseHeaderLatency: headerLatency,
		ConnectionReused:      t.connectionReused,
		ConnectionWasIdle:     t.connectionWasIdle,
		ConnectionIdleTime:    t.connectionIdleTime,
		ConnectionFingerprint: t.connectionFingerprint,
	}
}
