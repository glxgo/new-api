package perfmetrics

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestBuildChannelScopedGroupSummaryProjectsSharedChannelWithoutDuplicates(t *testing.T) {
	scopes := []GroupChannelScope{
		{
			Group: "gpt-pro",
			ModelChannels: map[string][]int{
				"gpt-5": {8, 8},
			},
		},
		{
			Group: "套餐专用分组",
			ModelChannels: map[string][]int{
				"gpt-5": {8, 31},
			},
		},
	}
	rows := []model.ChannelPerfMetric{
		{
			ModelName:      "gpt-5",
			ChannelId:      8,
			BucketTs:       100,
			RequestCount:   4,
			SuccessCount:   3,
			TotalLatencyMs: 1200,
			TtftSumMs:      400,
			TtftCount:      3,
		},
		{
			ModelName:      "gpt-5",
			ChannelId:      31,
			BucketTs:       100,
			RequestCount:   2,
			SuccessCount:   2,
			TotalLatencyMs: 500,
			TtftSumMs:      120,
			TtftCount:      2,
		},
	}

	result := buildChannelScopedGroupSummary(scopes, rows, nil)
	require.Equal(t, []string{"gpt-pro", "套餐专用分组"}, result.AvailableGroups)
	require.Len(t, result.Groups, 2)

	byGroup := map[string]GroupCacheSummary{}
	for _, summary := range result.Groups {
		byGroup[summary.Group] = summary
	}
	require.EqualValues(t, 4, byGroup["gpt-pro"].RequestCount)
	require.EqualValues(t, 3, byGroup["gpt-pro"].SuccessCount)
	require.EqualValues(t, 6, byGroup["套餐专用分组"].RequestCount)
	require.EqualValues(t, 5, byGroup["套餐专用分组"].SuccessCount)
}

func TestBuildChannelScopedGroupSummaryRespectsModelAbility(t *testing.T) {
	scopes := []GroupChannelScope{
		{
			Group: "gpt-pro",
			ModelChannels: map[string][]int{
				"gpt-5": {8},
			},
		},
	}
	rows := []model.ChannelPerfMetric{
		{ModelName: "gpt-5", ChannelId: 8, BucketTs: 100, RequestCount: 1, SuccessCount: 1},
		{ModelName: "gpt-4", ChannelId: 8, BucketTs: 100, RequestCount: 9, SuccessCount: 9},
	}

	result := buildChannelScopedGroupSummary(scopes, rows, nil)
	require.Len(t, result.Groups, 1)
	require.EqualValues(t, 1, result.Groups[0].RequestCount)
}
