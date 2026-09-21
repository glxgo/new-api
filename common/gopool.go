package common

import (
	"context"
	"fmt"

	"github.com/bytedance/gopkg/util/gopool"
)

var relayGoPool gopool.Pool
var relayAdmission = make(chan struct{}, 512)

type boundedBackgroundPool struct {
	pool      gopool.Pool
	admission chan struct{}
}

var backgroundPools = map[string]*boundedBackgroundPool{
	"archive": {pool: gopool.NewPool("gopool.ArchivePool", 2, gopool.NewConfig()), admission: make(chan struct{}, 4)},
	// Long-lived maintenance loops (retention and optional projections) each
	// occupy one worker for their lifetime. Keep them in a separate bounded
	// pool so enabling several loops cannot starve one another or the short
	// archive jobs.
	"maintenance": {pool: gopool.NewPool("gopool.MaintenancePool", 4, gopool.NewConfig()), admission: make(chan struct{}, 4)},
	"report":      {pool: gopool.NewPool("gopool.ReportPool", 4, gopool.NewConfig()), admission: make(chan struct{}, 8)},
	"probe":       {pool: gopool.NewPool("gopool.ProbePool", 8, gopool.NewConfig()), admission: make(chan struct{}, 16)},
	"general":     {pool: gopool.NewPool("gopool.BackgroundPool", 16, gopool.NewConfig()), admission: make(chan struct{}, 32)},
}

func init() {
	// Keep relay workers bounded. The old math.MaxInt32 pool allowed a burst
	// of long streams/retries to turn into an unbounded runnable queue.
	relayGoPool = gopool.NewPool("gopool.RelayPool", 256, gopool.NewConfig())
	relayGoPool.SetPanicHandler(func(ctx context.Context, i interface{}) {
		if stopChan, ok := ctx.Value("stop_chan").(chan bool); ok {
			SafeSendBool(stopChan, true)
		}
		SysError(fmt.Sprintf("panic in gopool.RelayPool: %v", i))
	})
}

func RelayCtxGo(ctx context.Context, f func()) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case relayAdmission <- struct{}{}:
		relayGoPool.CtxGo(ctx, func() {
			defer func() { <-relayAdmission }()
			f()
		})
	case <-ctx.Done():
		return
	}
}

// BackgroundCtxGo submits optional work to a class-specific bounded pool.
// Returning false means the queue is full or the context was cancelled; the
// caller should leave the work for its next scheduled tick rather than block
// a relay/HTTP goroutine.
func BackgroundCtxGo(class string, ctx context.Context, f func()) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	background, ok := backgroundPools[class]
	if !ok {
		background = backgroundPools["general"]
	}
	select {
	case background.admission <- struct{}{}:
		background.pool.CtxGo(ctx, func() {
			defer func() { <-background.admission }()
			f()
		})
		return true
	case <-ctx.Done():
		return false
	default:
		return false
	}
}
