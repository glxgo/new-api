package common

import (
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
