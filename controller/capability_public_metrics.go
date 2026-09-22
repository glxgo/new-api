package controller

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/tidwall/gjson"
)

// All timing is the recorded generation call (seconds), excluding judging and
// rendering. Non-streaming requests have no measured first-token latency.
type capabilityPublicMetrics struct {
	Attempts        int    `json:"attempts"`
	StartedAt       *int64 `json:"started_at"`
	DurationSeconds *int64 `json:"duration_seconds"`
	InputTokens     *int64 `json:"input_tokens"`
	OutputTokens    *int64 `json:"output_tokens"`
}

func capabilityPublicCallMetrics(attempts []model.CapabilityAttempt) map[string]capabilityPublicMetrics {
	result := map[string]capabilityPublicMetrics{}
	for _, a := range attempts {
		if a.Kind != "logic" && a.Kind != "geometry" && a.Kind != "scene" {
			continue
		}
		m := result[a.Kind]
		m.Attempts++
		// Multiple calls cannot safely be attributed to the displayed answer.
		// Preserve their count, but do not invent per-answer timing or usage.
		if m.Attempts > 1 {
			result[a.Kind] = capabilityPublicMetrics{Attempts: m.Attempts}
			continue
		}
		if a.StartedAt > 0 {
			start := a.StartedAt
			m.StartedAt = &start
			if a.Status == "complete" && a.CompletedAt >= start {
				duration := a.CompletedAt - start
				m.DurationSeconds = &duration
			}
		}
		if a.Status == "complete" && a.UsageSource == "provider_reported" && gjson.Valid(a.Usage) {
			u := gjson.Parse(a.Usage)
			// Multi-event usage reports are intentionally not summed or guessed.
			if u.IsObject() {
				m.InputTokens = capabilityReportedToken(u, "prompt_tokens", "input_tokens", "promptTokenCount")
				m.OutputTokens = capabilityReportedToken(u, "completion_tokens", "output_tokens", "candidatesTokenCount")
			}
		}
		result[a.Kind] = m
	}
	return result
}

func capabilityReportedToken(usage gjson.Result, names ...string) *int64 {
	var known *int64
	for _, name := range names {
		v := usage.Get(name)
		if !v.Exists() {
			continue
		}
		if v.Type != gjson.Number || v.Float() < 0 || v.Float() > 9007199254740991 || v.Float() != float64(v.Int()) {
			return nil
		}
		n := v.Int()
		if known != nil && *known != n {
			return nil
		}
		known = &n
	}
	return known
}
