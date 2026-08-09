package common

import (
	"io"
	"testing"

	"github.com/tidwall/gjson"
)

func TestApplyMappedModelToOutboundJSON(t *testing.T) {
	original := []byte(`{"model":"gpt-5.6","input":"keep-me","custom":{"enabled":true}}`)
	info := &RelayInfo{
		ChannelMeta: &ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "gpt-5.6-sol",
		},
	}

	result, changed, err := ApplyMappedModelToOutboundJSON(original, info)
	if err != nil {
		t.Fatalf("ApplyMappedModelToOutboundJSON() error = %v", err)
	}
	if !changed {
		t.Fatal("ApplyMappedModelToOutboundJSON() changed = false, want true")
	}
	if got := gjson.GetBytes(result, "model").String(); got != "gpt-5.6-sol" {
		t.Fatalf("model = %q, want %q", got, "gpt-5.6-sol")
	}
	if got := gjson.GetBytes(result, "input").String(); got != "keep-me" {
		t.Fatalf("input = %q, want %q", got, "keep-me")
	}
	if !gjson.GetBytes(result, "custom.enabled").Bool() {
		t.Fatal("custom.enabled was not preserved")
	}
	if got := gjson.GetBytes(original, "model").String(); got != "gpt-5.6" {
		t.Fatalf("original model mutated to %q", got)
	}
}

func TestNewOutboundJSONBodyGetBodyReplaysFullBody(t *testing.T) {
	payload := []byte(`{"model":"gpt-test","input":"replay-me"}`)
	body, size, getBody, closer, err := NewOutboundJSONBody(payload)
	if err != nil {
		t.Fatalf("NewOutboundJSONBody() error = %v", err)
	}
	defer closer.Close()
	if size != int64(len(payload)) {
		t.Fatalf("size = %d, want %d", size, len(payload))
	}

	prefix := make([]byte, 7)
	if _, err := io.ReadFull(body, prefix); err != nil {
		t.Fatalf("partial primary read failed: %v", err)
	}
	for i := 0; i < 2; i++ {
		replay, err := getBody()
		if err != nil {
			t.Fatalf("getBody[%d]() error = %v", i, err)
		}
		got, err := io.ReadAll(replay)
		_ = replay.Close()
		if err != nil {
			t.Fatalf("replay[%d] read failed: %v", i, err)
		}
		if string(got) != string(payload) {
			t.Fatalf("replay[%d] = %q, want %q", i, got, payload)
		}
	}
}

func TestApplyMappedModelToOutboundJSONNoMapping(t *testing.T) {
	original := []byte(`{"model":"gpt-5.6","input":"keep-me"}`)
	result, changed, err := ApplyMappedModelToOutboundJSON(original, &RelayInfo{})
	if err != nil {
		t.Fatalf("ApplyMappedModelToOutboundJSON() error = %v", err)
	}
	if changed {
		t.Fatal("ApplyMappedModelToOutboundJSON() changed = true, want false")
	}
	if string(result) != string(original) {
		t.Fatalf("result = %s, want original %s", result, original)
	}
}
