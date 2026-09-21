package common

import (
	"context"
	"testing"
	"time"
)

func TestMaintenancePoolCanRunAllLongLivedLoops(t *testing.T) {
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	finished := make(chan struct{}, 3)
	for i := 0; i < 3; i++ {
		if !BackgroundCtxGo("maintenance", context.Background(), func() {
			started <- struct{}{}
			<-release
			finished <- struct{}{}
		}) {
			t.Fatalf("maintenance task %d was rejected", i)
		}
	}

	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for i := 0; i < 3; i++ {
		select {
		case <-started:
		case <-deadline.C:
			t.Fatalf("maintenance task %d did not start; a long-lived loop may be starved", i)
		}
	}
	close(release)
	for i := 0; i < 3; i++ {
		select {
		case <-finished:
		case <-deadline.C:
			t.Fatalf("maintenance task %d did not finish", i)
		}
	}
}
