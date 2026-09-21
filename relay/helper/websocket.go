package helper

import (
	"context"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gorilla/websocket"
	"time"
)

// WatchWebSocketCancellation interrupts blocked reads and writes on cancellation.
// Cleanup joins the watcher before the request's resources may be reused.
func WatchWebSocketCancellation(ctx context.Context, conn *websocket.Conn) func() {
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stop:
		}
	}()
	return func() { close(stop); <-done }
}

func WebSocketReadDeadline() time.Time {
	timeout := time.Duration(constant.StreamingTimeout) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	return time.Now().Add(timeout)
}
