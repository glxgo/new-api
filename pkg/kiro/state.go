package kiro

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
)

// State belongs to one gateway request. Observe must only receive bytes read
// from an administrator-configured upstream response, never a client request.
type State struct {
	Snapshot  *Snapshot
	RequestID string
	mu        sync.Mutex
	result    Result
}

type Result struct {
	RequestID       string
	AttemptID       string
	ChannelID       int
	CostPrice       string
	Trusted         bool
	Complete        bool
	Error           string
	MeteringInvalid bool
	Evidence        []json.RawMessage
	Raw             json.RawMessage
	Receipt         *Receipt
}

func NewState(s *Snapshot) *State { return &State{Snapshot: s, RequestID: uuid.NewString()} }

func (s *State) Begin(channelID int) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result = Result{RequestID: s.RequestID, AttemptID: uuid.NewString(), ChannelID: channelID,
		CostPrice: s.Snapshot.CostPrice(channelID), Trusted: s.Snapshot.Trusted(channelID)}
	return s.result
}

func (s *State) Result() Result { s.mu.Lock(); defer s.mu.Unlock(); return s.result }

func (s *State) Charge() bool { return s != nil && s.Snapshot.Mode == "charge" }

func (s *State) Fail(reason string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result.Error = reason
	s.result.Complete = false
}

func (s *State) Forward() json.RawMessage {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.result.Trusted || s.result.Error != "" || s.result.Receipt == nil {
		return nil
	}
	return ForwardReceipt(s.result.Raw)
}

// Observe also strips this hop's correlation IDs before the adapter can copy
// the upstream data downstream. Raw original receipts remain in Result.
func (s *State) Observe(data []byte, streaming bool, event ...string) []byte {
	if s == nil {
		return data
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(string(data)) == "[DONE]" {
		s.result.Complete = true
		return data
	}
	var root map[string]json.RawMessage
	if common.Unmarshal(data, &root) != nil || root == nil {
		if s.Snapshot.Mode == "charge" {
			s.result.Error = "malformed_upstream_payload"
		}
		return data
	}
	var kind string
	_ = common.Unmarshal(root["type"], &kind)
	if kind == "" && len(event) > 0 {
		kind = event[0]
	}
	if kind == "message_stop" || kind == "response.completed" {
		s.result.Complete = true
	}
	if !streaming {
		s.result.Complete = true
	}
	if kind == "error" || kind == "response.failed" || kind == "response.incomplete" {
		s.result.Error = "upstream_incomplete"
	}
	checkStatus := func(object map[string]json.RawMessage) {
		if raw := object["error"]; len(raw) > 0 && string(raw) != "null" {
			s.result.Error = "upstream_error"
		}
		var status string
		_ = common.Unmarshal(object["status"], &status)
		if status != "" && status != "completed" && (!streaming || kind == "response.completed") {
			s.result.Error = "upstream_incomplete"
		}
	}
	checkStatus(root)
	changed := false
	visit := func(object map[string]json.RawMessage) bool {
		var usage map[string]json.RawMessage
		if common.Unmarshal(object["usage"], &usage) != nil || usage == nil {
			return false
		}
		raw, found := usage["kiro_metering"]
		if !found {
			return false
		}
		if len(s.result.Evidence) < 16 && len(raw) <= 8192 {
			s.result.Evidence = append(s.result.Evidence, append(json.RawMessage(nil), raw...))
		}
		if !s.result.Trusted {
			s.result.Error = "untrusted_metering_channel"
			s.result.MeteringInvalid = true
		} else {
			r, err := ParseReceipt(raw, s.result.RequestID, s.result.AttemptID)
			if err != nil {
				s.result.Error = err.Error()
				s.result.MeteringInvalid = true
			} else if s.result.Receipt != nil && s.result.Receipt.canonical() != r.canonical() {
				s.result.Error = "conflicting_metering_receipts"
				s.result.MeteringInvalid = true
			} else if s.result.Receipt == nil {
				s.result.Receipt = r
				s.result.Raw = append(json.RawMessage(nil), raw...)
			}
		}
		delete(usage, "kiro_metering")
		if s.result.Error == "" {
			usage["kiro_metering"] = ForwardReceipt(raw)
		}
		object["usage"], _ = common.Marshal(usage)
		return true
	}
	changed = visit(root)
	for _, key := range []string{"response", "message"} {
		var nested map[string]json.RawMessage
		if common.Unmarshal(root[key], &nested) == nil && nested != nil {
			if key == "response" {
				checkStatus(nested)
			}
			if visit(nested) {
				root[key], _ = common.Marshal(nested)
				changed = true
			}
		}
	}
	if changed {
		b, err := common.Marshal(root)
		if err == nil {
			return b
		}
	}
	return data
}

// ForwardPayload restores the receipt at the final wire boundary, including
// adapters that reconstruct usage from token counts or convert protocols.
// It touches only usage metadata, never assistant text or reasoning.
func (s *State) ForwardPayload(data []byte, streaming bool) []byte {
	if s == nil {
		return data
	}
	forward := s.Forward()
	if len(forward) == 0 && !bytes.Contains(data, []byte(`"kiro_metering"`)) {
		return data
	}
	var root map[string]json.RawMessage
	if common.Unmarshal(data, &root) != nil || root == nil {
		return data
	}
	var kind string
	_ = common.Unmarshal(root["type"], &kind)
	object := root
	nested := ""
	if kind == "response.completed" {
		nested = "response"
		if common.Unmarshal(root[nested], &object) != nil || object == nil {
			return data
		}
	}
	eligible := !streaming || kind == "message_delta" || kind == "response.completed"
	if streaming && kind == "" {
		var choices []json.RawMessage
		if common.Unmarshal(root["choices"], &choices) == nil && len(choices) == 0 && root["usage"] != nil {
			eligible = true
		}
	}
	if !eligible {
		return data
	}
	var usage map[string]json.RawMessage
	_ = common.Unmarshal(object["usage"], &usage)
	if usage == nil {
		if len(forward) == 0 {
			return data
		}
		usage = make(map[string]json.RawMessage)
	}
	delete(usage, "kiro_metering")
	if len(forward) > 0 {
		usage["kiro_metering"] = forward
	}
	object["usage"], _ = common.Marshal(usage)
	if nested != "" {
		root[nested], _ = common.Marshal(object)
	}
	b, err := common.Marshal(root)
	if err != nil {
		return data
	}
	return b
}
