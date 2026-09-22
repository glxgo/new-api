package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCapabilityPublicMetricsOnlyObservedGeneration(t *testing.T) {
	a := model.CapabilityAttempt{Kind: "logic", Status: "complete", StartedAt: 100, CompletedAt: 142, UsageSource: "provider_reported", Usage: `{"prompt_tokens":1455,"completion_tokens":0,"secret":"must-not-leak"}`, ChannelID: 42, Endpoint: "https://private.example", KeySlot: 7}
	m := capabilityPublicCallMetrics([]model.CapabilityAttempt{a, {Kind: "judge", StartedAt: 142, CompletedAt: 190}})
	require.Len(t, m, 1)
	require.EqualValues(t, 42, *m["logic"].DurationSeconds)
	require.EqualValues(t, 0, *m["logic"].OutputTokens)
	raw, err := common.Marshal(m)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "secret")
	require.NotContains(t, string(raw), "private.example")
	require.NotContains(t, string(raw), "channel_id")
	require.NotContains(t, string(raw), "key_slot")
	m = capabilityPublicCallMetrics([]model.CapabilityAttempt{a, a})
	require.Equal(t, 2, m["logic"].Attempts)
	require.Nil(t, m["logic"].InputTokens)
	require.Nil(t, m["logic"].DurationSeconds)
}

func TestCapabilityPublicMetricsUsageDoesNotInventValues(t *testing.T) {
	for _, test := range []struct {
		usage         string
		input, output *int64
	}{
		{`{"input_tokens":11,"output_tokens":12}`, ptrCapabilityMetric(11), ptrCapabilityMetric(12)},
		{`{"promptTokenCount":13,"candidatesTokenCount":14}`, ptrCapabilityMetric(13), ptrCapabilityMetric(14)},
		{`[{"input_tokens":11},{"input_tokens":20,"output_tokens":12}]`, nil, nil},
		{`{"input_tokens":-1,"output_tokens":1.5}`, nil, nil},
		{`{"input_tokens":"15","output_tokens":9007199254740992}`, nil, nil},
		{`{"input_tokens":5,"prompt_tokens":6}`, nil, nil},
		{`{broken`, nil, nil},
	} {
		t.Run(test.usage, func(t *testing.T) {
			m := capabilityPublicCallMetrics([]model.CapabilityAttempt{{Kind: "scene", Status: "complete", UsageSource: "provider_reported", Usage: test.usage}})["scene"]
			require.Equal(t, test.input, m.InputTokens)
			require.Equal(t, test.output, m.OutputTokens)
			require.Nil(t, m.DurationSeconds)
		})
	}
	for _, source := range []string{"", "estimated"} {
		m := capabilityPublicCallMetrics([]model.CapabilityAttempt{{Kind: "geometry", Status: "complete", UsageSource: source, Usage: `{"input_tokens":15}`}})["geometry"]
		require.Nil(t, m.InputTokens)
	}
	unknown := capabilityPublicCallMetrics([]model.CapabilityAttempt{{Kind: "logic", Status: "outcome_unknown", StartedAt: 100, CompletedAt: 200}})["logic"]
	require.Nil(t, unknown.DurationSeconds, "worker recovery time is not the upstream completion time")
}
func ptrCapabilityMetric(n int64) *int64 { return &n }
