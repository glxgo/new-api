package perfmetrics

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestFixedStatusBucketWindowUsesRangeCadence(t *testing.T) {
	start := int64(1_700_000_400)
	for _, test := range []struct {
		name  string
		hours int64
		count int
	}{
		{name: "1 hour", hours: 1, count: 12},
		{name: "24 hours", hours: 24, count: 48},
		{name: "7 days", hours: 24 * 7, count: 48},
		{name: "30 days", hours: 24 * 30, count: 48},
	} {
		t.Run(test.name, func(t *testing.T) {
			first, last, bucketSeconds, count := fixedStatusBucketWindow(
				start,
				start+test.hours*3600,
			)
			require.Equal(t, test.count, count)
			require.EqualValues(t, int64(count-1)*bucketSeconds, last-first)
			require.Zero(t, last%bucketSeconds)
		})
	}
}

func TestHourlyGroupSummaryUsesFiveMinuteSamples(t *testing.T) {
	db := setupChannelMetricsDB(t)
	last := time.Now().Unix() / 300 * 300
	first := last - 11*300
	rows := []model.ChannelPerfMetric{
		{ModelName: "a", ChannelId: 1, BucketTs: first, BucketSeconds: 60, RequestCount: 9, SuccessCount: 9},
		{ModelName: "a", ChannelId: 1, BucketTs: first + 60, BucketSeconds: 60, RequestCount: 1},
		{ModelName: "a", ChannelId: 1, BucketTs: first + 300, BucketSeconds: 60, RequestCount: 2, SuccessCount: 1},
		{ModelName: "a", ChannelId: 1, BucketTs: first + 600, BucketSeconds: 1800, RequestCount: 1000, SuccessCount: 1000},
	}
	require.NoError(t, db.Create(&rows).Error)
	require.NoError(t, db.Create(&model.PerfMetric{ModelName: "a", Group: "g", BucketTs: first - 300, RequestCount: 1000, SuccessCount: 1000}).Error)
	b := &atomicBucket{}
	b.add(Sample{Success: true})
	channelHotBuckets.Store(channelBucketKey{model: "a", channelId: 1, bucketTs: last}, b)
	result, err := QueryGroupSummaryByChannels(1, []GroupChannelScope{{Group: "g", ModelChannels: map[string][]int{"a": {1}}}})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	group := result.Groups[0]
	require.EqualValues(t, 13, group.RequestCount)
	require.Len(t, group.Series, 12)
	require.Equal(t, first, group.Series[0].Ts)
	require.Equal(t, 90.0, group.Series[0].SuccessRate)
	require.Equal(t, 50.0, group.Series[1].SuccessRate)
	require.Zero(t, group.Series[2].RequestCount)
	require.EqualValues(t, 1, group.Series[11].RequestCount)
	for i := 1; i < len(group.Series); i++ {
		require.EqualValues(t, 300, group.Series[i].Ts-group.Series[i-1].Ts)
	}
}

func TestFillQueryResultSeriesMarksEmptySlots(t *testing.T) {
	start := int64(1_700_000_400)
	result := QueryResult{
		Groups: []GroupResult{{
			Group:  "gpt-pro",
			Series: []BucketPoint{{Ts: start + 24*3600, RequestCount: 1, SuccessCount: 1, SuccessRate: 100}},
		}},
	}

	fillQueryResultSeries(&result, start, start+24*3600)
	require.Len(t, result.Groups[0].Series, 48)
	require.Zero(t, result.Groups[0].Series[0].RequestCount)
	require.EqualValues(t, 1, result.Groups[0].Series[len(result.Groups[0].Series)-1].RequestCount)
}

func TestFillGroupSummarySeriesMarksEmptySlots(t *testing.T) {
	start := int64(1_700_000_400)
	result := GroupSummaryAllResult{
		Groups: []GroupCacheSummary{{
			Group:  "gpt-pro",
			Series: []BucketPoint{{Ts: start + 7*24*3600, RequestCount: 1, SuccessCount: 1, SuccessRate: 100}},
		}},
	}

	fillGroupSummarySeries(&result, start, start+7*24*3600)
	require.Len(t, result.Groups[0].Series, 48)
	require.Zero(t, result.Groups[0].Series[0].RequestCount)
	require.EqualValues(t, 1, result.Groups[0].Series[len(result.Groups[0].Series)-1].RequestCount)
}
