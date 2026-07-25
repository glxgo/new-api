package perfmetrics

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/stretchr/testify/require"
)

func TestRecordKeepsEveryFinalRequestInChannelHealth(t *testing.T) {
	setting := perf_metrics_setting.GetSetting()
	oldEnabled := setting.Enabled
	setting.Enabled = true
	t.Cleanup(func() {
		setting.Enabled = oldEnabled
		hotBuckets = sync.Map{}
		channelHotBuckets = sync.Map{}
	})
	hotBuckets = sync.Map{}
	channelHotBuckets = sync.Map{}

	sample := Sample{
		Model:     "gpt-test",
		Group:     "default",
		ChannelId: 31,
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
	require.Equal(t, int64(3), health.requestCount)
	require.Equal(t, int64(0), health.successCount)
}

func TestRecordPreservesTheObservedSuccessRatioWithinOneConversation(t *testing.T) {
	setting := perf_metrics_setting.GetSetting()
	oldEnabled := setting.Enabled
	setting.Enabled = true
	t.Cleanup(func() {
		setting.Enabled = oldEnabled
		hotBuckets = sync.Map{}
		channelHotBuckets = sync.Map{}
	})
	hotBuckets = sync.Map{}
	channelHotBuckets = sync.Map{}

	sample := Sample{
		Model:     "gpt-test",
		Group:     "default",
		ChannelId: 31,
		Success:   true,
		LatencyMs: 10,
	}
	for range 100 {
		Record(sample)
	}
	sample.Success = false
	Record(sample)

	var health counters
	channelHotBuckets.Range(func(_, value any) bool {
		health = value.(*atomicBucket).snapshot()
		return false
	})
	require.Equal(t, int64(101), health.requestCount)
	require.Equal(t, int64(100), health.successCount)
}
