package perfmetrics

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestScopesForModelKeepsOnlyServingGroups(t *testing.T) {
	scopes := []GroupChannelScope{
		{
			Group: "gpt-pro",
			ModelChannels: map[string][]int{
				"gpt-5": {8},
			},
		},
		{
			Group: "套餐专用分组",
			ModelChannels: map[string][]int{
				"gpt-4": {31},
			},
		},
		{
			Group: "",
			ModelChannels: map[string][]int{
				"gpt-5": {9},
			},
		},
	}

	modelScopes := scopesForModel(scopes, "gpt-5", "")
	require.Len(t, modelScopes, 1)
	require.Equal(t, "gpt-pro", modelScopes[0].Group)
	require.Equal(t, []int{8}, modelScopes[0].ModelChannels["gpt-5"])

	groupScoped := scopesForModel(scopes, "gpt-5", "套餐专用分组")
	require.Empty(t, groupScoped)

	groupScoped = scopesForModel(scopes, "gpt-5", "gpt-pro")
	require.Len(t, groupScoped, 1)
	require.Equal(t, "gpt-pro", groupScoped[0].Group)

	require.Empty(t, scopesForModel(scopes, "unknown-model", ""))
}

func TestBuildScopedModelTotalsExcludesOutOfScopeChannels(t *testing.T) {
	modelChannels := map[string]map[int]struct{}{
		"gpt-5": {8: {}},
	}
	rows := []model.ChannelPerfMetric{
		{ModelName: "gpt-5", ChannelId: 8, BucketTs: 200, RequestCount: 10, SuccessCount: 10},
		{ModelName: "gpt-5", ChannelId: 13, BucketTs: 200, RequestCount: 90, SuccessCount: 0},
		{ModelName: "gpt-4", ChannelId: 8, BucketTs: 200, RequestCount: 50, SuccessCount: 50},
	}

	totals := buildScopedModelTotals(modelChannels, rows, nil, nil)
	require.EqualValues(t, 10, totals["gpt-5"].requestCount)
	require.EqualValues(t, 10, totals["gpt-5"].successCount)
	_, hasGpt4 := totals["gpt-4"]
	require.False(t, hasGpt4)
}

func TestBuildScopedModelTotalsMergesHotAndLegacyBeforeCutover(t *testing.T) {
	modelChannels := map[string]map[int]struct{}{
		"gpt-5": {8: {}},
	}
	rows := []model.ChannelPerfMetric{
		{ModelName: "gpt-5", ChannelId: 8, BucketTs: 200, RequestCount: 2, SuccessCount: 2},
	}
	hot := map[channelBucketKey]counters{
		{model: "gpt-5", channelId: 8, bucketTs: 300}:  {requestCount: 3, successCount: 3},
		{model: "gpt-5", channelId: 13, bucketTs: 300}: {requestCount: 40, successCount: 0},
	}
	legacyRows := []model.PerfMetricModelBucket{
		{ModelName: "gpt-5", BucketTs: 100, RequestCount: 7, SuccessCount: 6},
		{ModelName: "gpt-5", BucketTs: 250, RequestCount: 99, SuccessCount: 99},
		{ModelName: "gpt-4", BucketTs: 100, RequestCount: 50, SuccessCount: 50},
	}

	totals := buildScopedModelTotals(modelChannels, rows, hot, legacyRows)
	require.EqualValues(t, 12, totals["gpt-5"].requestCount)
	require.EqualValues(t, 11, totals["gpt-5"].successCount)
	_, hasGpt4 := totals["gpt-4"]
	require.False(t, hasGpt4)
}

func TestBuildScopedModelTotalsUsesLegacyWhenNoChannelRows(t *testing.T) {
	modelChannels := map[string]map[int]struct{}{
		"gpt-5": {8: {}},
	}
	legacyRows := []model.PerfMetricModelBucket{
		{ModelName: "gpt-5", BucketTs: 100, RequestCount: 4, SuccessCount: 3},
	}

	totals := buildScopedModelTotals(modelChannels, nil, nil, legacyRows)
	require.EqualValues(t, 4, totals["gpt-5"].requestCount)
	require.EqualValues(t, 3, totals["gpt-5"].successCount)
}
