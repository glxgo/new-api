package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRecordStreamTerminalEventIsIdempotent(t *testing.T) {
	oldDB := DB
	t.Cleanup(func() { DB = oldDB })

	var err error
	DB, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err = DB.AutoMigrate(&StreamTerminalEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	first := &StreamTerminalEvent{
		RequestId:         "req-terminal-1",
		TerminalStatus:    "failed",
		EndReason:         "eof",
		ResponseCompleted: false,
	}
	if err = RecordStreamTerminalEvent(first); err != nil {
		t.Fatalf("record first event: %v", err)
	}
	if err = RecordStreamTerminalEvent(&StreamTerminalEvent{
		RequestId:      first.RequestId,
		TerminalStatus: "completed",
		EndReason:      "done",
	}); err != nil {
		t.Fatalf("record duplicate event: %v", err)
	}

	var count int64
	if err = DB.Model(&StreamTerminalEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 1 {
		t.Fatalf("event count = %d, want 1", count)
	}
	var got StreamTerminalEvent
	if err = DB.Where("request_id = ?", first.RequestId).First(&got).Error; err != nil {
		t.Fatalf("load event: %v", err)
	}
	if got.TerminalStatus != "failed" || got.EndReason != "eof" {
		t.Fatalf("duplicate overwrote terminal event: %#v", got)
	}
}
