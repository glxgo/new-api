package common

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"testing"
	"time"
)

func TestUpstreamTransportTraceCapturesSanitizedConnectionMetadata(t *testing.T) {
	startedAt := time.Unix(100, 0)
	trace := NewUpstreamTransportTrace(startedAt)
	req := httptest.NewRequest(http.MethodPost, "https://api.example.com/v1/responses", nil)
	req = trace.Attach(req)
	clientTrace := httptrace.ContextClientTrace(req.Context())
	if clientTrace == nil || clientTrace.GotConn == nil {
		t.Fatal("GotConn trace is not attached")
	}
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	clientTrace.GotConn(httptrace.GotConnInfo{
		Conn:     clientConn,
		Reused:   true,
		WasIdle:  true,
		IdleTime: 2 * time.Second,
	})
	trace.RecordResponse(&http.Response{Proto: "HTTP/2.0"}, startedAt.Add(350*time.Millisecond))

	snapshot := trace.Snapshot()
	if snapshot.Protocol != "HTTP/2.0" || snapshot.ResponseHeaderLatency != 350*time.Millisecond {
		t.Fatalf("unexpected protocol/header trace: %#v", snapshot)
	}
	if !snapshot.ConnectionReused || !snapshot.ConnectionWasIdle || snapshot.ConnectionIdleTime != 2*time.Second {
		t.Fatalf("unexpected connection reuse trace: %#v", snapshot)
	}
	if len(snapshot.ConnectionFingerprint) != 12 {
		t.Fatalf("connection fingerprint length = %d, want 12", len(snapshot.ConnectionFingerprint))
	}
	if snapshot.ConnectionFingerprint == clientConn.RemoteAddr().String() {
		t.Fatal("connection fingerprint exposed the raw remote address")
	}
}
