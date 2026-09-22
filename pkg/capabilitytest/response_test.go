package capabilitytest

import "testing"

func TestCapabilityMultilineSSEAndTrailingData(t *testing.T) {
	stream := "data: {\"choices\":[{\"index\":0,\n" + "data: \"delta\":{\"content\":\"29\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
	answer, _, err := ParseResponse([]byte(stream), false)
	if err != nil || answer != "29" {
		t.Fatal(answer, err)
	}
	if _, _, err = ParseResponse([]byte(stream+"data: {\"choices\":[]}\n\n"), false); err == nil {
		t.Fatal("accepted post-terminal data")
	}
	if _, err := ParseLogic(`{"answers":[{"id":"anchor","value":1,"value":29},{"id":"random","value":30}]}`, 30); err == nil {
		t.Fatal("accepted duplicate key")
	}
}

func TestCapabilityResponseRequiresTerminalEvidence(t *testing.T) {
	for _, body := range []string{`{"choices":[{"message":{"content":"29"},"finish_reason":"length"}]}`, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"29\"}}]}\n\n", "data: [DONE]\n\n", `{"status":"incomplete","output":[]}`} {
		if _, _, err := ParseResponse([]byte(body), false); err == nil {
			t.Errorf("accepted incomplete: %s", body)
		}
	}
	complete := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"答案\"},\"finish_reason\":null}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"29\"},\"finish_reason\":\"stop\"}],\"usage\":{\"total_tokens\":4}}\n\ndata: [DONE]\n\n"
	answer, usage, err := ParseResponse([]byte(complete), false)
	if err != nil || answer != "答案29" || usage == "" {
		t.Fatal(answer, usage, err)
	}
}
