package perfmetrics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildGroupSummaryResultAggregatesHealthAndSeries(t *testing.T) {
	merged := map[bucketKey]counters{
		{group: "gpt-plus", bucketTs: 100}: {
			requestCount:   2,
			successCount:   2,
			totalLatencyMs: 1000,
			ttftSumMs:      300,
			ttftCount:      2,
			outputTokens:   30,
			generationMs:   3000,
			cacheTokens:    10,
			promptTokens:   100,
		},
		{group: "gpt-plus", bucketTs: 200}: {
			requestCount:   2,
			successCount:   1,
			totalLatencyMs: 900,
			ttftSumMs:      600,
			ttftCount:      1,
			outputTokens:   10,
			generationMs:   1000,
			cacheTokens:    20,
			promptTokens:   100,
		},
	}

	result := buildGroupSummaryResult(merged)
	require.Len(t, result.Groups, 1)
	group := result.Groups[0]
	require.Equal(t, "gpt-plus", group.Group)
	require.EqualValues(t, 4, group.RequestCount)
	require.EqualValues(t, 3, group.SuccessCount)
	require.Equal(t, 75.0, group.SuccessRate)
	require.EqualValues(t, 633, group.AvgLatencyMs)
	require.EqualValues(t, 300, group.AvgTtftMs)
	require.Equal(t, 10.0, group.AvgTps)
	require.Equal(t, 15.0, group.CacheRate)
	require.Len(t, group.Series, 2)
	require.EqualValues(t, 100, group.Series[0].Ts)
	require.EqualValues(t, 2, group.Series[0].RequestCount)
	require.EqualValues(t, 2, group.Series[0].SuccessCount)
	require.Equal(t, 100.0, group.Series[0].SuccessRate)
	require.EqualValues(t, 200, group.Series[1].Ts)
	require.Equal(t, 50.0, group.Series[1].SuccessRate)
}
