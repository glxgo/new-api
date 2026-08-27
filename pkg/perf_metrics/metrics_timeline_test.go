package perfmetrics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFixedStatusBucketWindowUsesThirtyMinuteCadence(t *testing.T) {
	start := int64(1_700_000_400)
	for _, test := range []struct {
		name  string
		hours int64
		count int
	}{
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
