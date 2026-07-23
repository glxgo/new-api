package perfmetrics

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/stretchr/testify/require"
)

func TestRecordKeepsRawSamplesButDeduplicatesChannelHealthByConversation(t *testing.T) {
	setting := perf_metrics_setting.GetSetting()
	oldEnabled := setting.Enabled
	setting.Enabled = true
	t.Cleanup(func() {
		setting.Enabled = oldEnabled
		hotBuckets = sync.Map{}
		channelHotBuckets = sync.Map{}
		channelHealthDedup = sync.Map{}
	})
	hotBuckets = sync.Map{}
	channelHotBuckets = sync.Map{}
	channelHealthDedup = sync.Map{}

	sample := Sample{
		Model:     "gpt-test",
		Group:     "default",
		ChannelId: 31,
		HealthKey: "affinity-fingerprint",
		Success:   false,
		LatencyMs: 10,
	}
	Record(sample)
	Record(sample)
	Record(sample)

	var raw counters
	hotBuckets.Range(func(_, value any) bool {
		raw = value.(*atomicBucket).snapshot()
		return false
	})
	require.Equal(t, int64(3), raw.requestCount)

	var health counters
	channelHotBuckets.Range(func(_, value any) bool {
		health = value.(*atomicBucket).snapshot()
		return false
	})
	require.Equal(t, int64(1), health.requestCount)
	require.Equal(t, int64(0), health.successCount)
}

func TestChannelHealthDedupKeepsOneSuccessAndOneFailure(t *testing.T) {
	channelHealthDedup = sync.Map{}
	t.Cleanup(func() { channelHealthDedup = sync.Map{} })

	sample := Sample{Model: "gpt-test", HealthKey: "same-conversation"}
	require.True(t, shouldRecordChannelHealthSample(sample, 100))
	require.False(t, shouldRecordChannelHealthSample(sample, 100))
	sample.Success = true
	require.True(t, shouldRecordChannelHealthSample(sample, 100))
	require.False(t, shouldRecordChannelHealthSample(sample, 100))
	require.True(t, shouldRecordChannelHealthSample(sample, 200))
}
