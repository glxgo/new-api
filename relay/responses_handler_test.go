package relay

import (
	"io"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestResponsesPassThroughBodyRecordsUnmodifiedSize(t *testing.T) {
	payload := []byte(`{"model":"gpt-test","stream":true,"input":"hello"}`)
	storage, err := common.CreateBodyStorage(payload)
	if err != nil {
		t.Fatalf("CreateBodyStorage() error = %v", err)
	}
	defer storage.Close()
	info := &relaycommon.RelayInfo{}
	body := responsesPassThroughBody(info, storage)
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("body changed: got %q, want %q", got, payload)
	}
	if info.UpstreamRequestBodySize != int64(len(payload)) {
		t.Fatalf("UpstreamRequestBodySize = %d, want %d", info.UpstreamRequestBodySize, len(payload))
	}
}
