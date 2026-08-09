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
		RequestId:             "req-terminal-1",
		IngressRequestId:      "edge-terminal-1",
		AffinityKeyFp:         "89abcdef",
		TerminalStatus:        "failed",
		EndReason:             "eof",
		FailureSource:         "upstream",
		UpstreamTerminalEvent: "response.failed",
		UpstreamResponseId:    "resp_failed_1",
		UpstreamErrorCode:     "server_error",
		UpstreamHost:          "api.example.com",
		UpstreamProtocol:      "HTTP/2.0",
		UpstreamConnReused:    true,
		UpstreamConnFp:        "0123456789ab",
		EstimatedPromptTokens: 12345,
		UpstreamLastEventType: "response.output_text.delta",
		UpstreamLastSequence:  42,
		UpstreamEventBytes:    8192,
		ReceivedEvents:        5,
		ForwardedEvents:       3,
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
	if got.IngressRequestId != first.IngressRequestId || got.AffinityKeyFp != first.AffinityKeyFp {
		t.Fatalf("correlation fields not persisted: %#v", got)
	}
	if got.FailureSource != first.FailureSource ||
		got.UpstreamTerminalEvent != first.UpstreamTerminalEvent ||
		got.UpstreamResponseId != first.UpstreamResponseId ||
		got.UpstreamErrorCode != first.UpstreamErrorCode {
		t.Fatalf("upstream terminal fields not persisted: %#v", got)
	}
	if got.UpstreamHost != first.UpstreamHost ||
		got.UpstreamProtocol != first.UpstreamProtocol ||
		got.UpstreamConnReused != first.UpstreamConnReused ||
		got.UpstreamConnFp != first.UpstreamConnFp ||
		got.EstimatedPromptTokens != first.EstimatedPromptTokens ||
		got.UpstreamLastEventType != first.UpstreamLastEventType ||
		got.UpstreamLastSequence != first.UpstreamLastSequence ||
		got.UpstreamEventBytes != first.UpstreamEventBytes ||
		got.ReceivedEvents != first.ReceivedEvents ||
		got.ForwardedEvents != first.ForwardedEvents {
		t.Fatalf("transport diagnostics not persisted: %#v", got)
	}
}
