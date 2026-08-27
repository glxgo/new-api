package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSanitizeProbeErrorRedactsCredentials(t *testing.T) {
	message := sanitizeProbeError("Bearer secret-token sk-abcdefghijk api_key=visible-secret upstream failed")
	require.NotContains(t, message, "secret-token")
	require.NotContains(t, message, "sk-abcdefghijk")
	require.NotContains(t, message, "visible-secret")
	require.True(t, strings.Contains(message, "upstream failed"))
}

func TestVisibleStatusGroupsFollowSelectedCanaryChannels(t *testing.T) {
	channels := []*model.Channel{
		{Id: 8, Status: common.ChannelStatusEnabled, Group: "套餐专用分组,shared"},
		{Id: 20, Status: common.ChannelStatusEnabled, Group: "gpt-image-2(0.15/张),shared"},
		{Id: 30, Status: common.ChannelStatusManuallyDisabled, Group: "disabled-only"},
	}
	settings := &operation_setting.MonitorSetting{ChannelCanaryChannelIds: []int{8}}

	groups := filterStatusGroupsByCanarySelection(
		[]string{"auto", "套餐专用分组", "gpt-image-2(0.15/张)", "shared", "disabled-only"},
		channels,
		settings,
	)

	require.Equal(t, []string{"auto", "shared", "套餐专用分组"}, groups)
}

func TestVisibleStatusGroupRatiosOnlyExposeStatusGroups(t *testing.T) {
	original := ratio_setting.GetGroupRatioCopy()
	originalJSON, err := common.Marshal(original)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"status-only":1.75,"internal":9}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(string(originalJSON)))
	})

	require.Equal(t, map[string]float64{
		"default":     1,
		"status-only": 1.75,
		"missing":     1,
	}, visibleStatusGroupRatios([]string{"default", "status-only", "missing"}))
}

func TestClassifyChannelProbeError(t *testing.T) {
	require.Equal(t, "timeout", classifyChannelProbeError(testResult{localErr: context.DeadlineExceeded}))
	require.Equal(t, "rate_limit", classifyChannelProbeError(testResult{httpStatus: 429}))
	require.Equal(t, "upstream", classifyChannelProbeError(testResult{httpStatus: 502}))
}

func TestModelStatusBucketWindowUsesThirtyMinuteCadence(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	for _, test := range []struct {
		hours int
		count int
	}{
		{hours: 24, count: 48},
		{hours: 24 * 7, count: 48},
		{hours: 24 * 30, count: 48},
	} {
		first, last, bucketSeconds, count := modelStatusBucketWindow(test.hours, now)
		require.Equal(t, test.count, count)
		require.EqualValues(t, int64(count-1)*bucketSeconds, last-first)
		require.Zero(t, last%bucketSeconds)
		switch test.hours {
		case 24:
			require.EqualValues(t, 30*60, bucketSeconds)
		case 24 * 7:
			require.EqualValues(t, 210*60, bucketSeconds)
		case 24 * 30:
			require.EqualValues(t, 15*60*60, bucketSeconds)
		}
	}
}

func TestClassifyGroupProbeStatusByRemainingHealthyChannels(t *testing.T) {
	tests := []struct {
		name     string
		summary  *perfmetrics.GroupProbeSummary
		expected string
	}{
		{
			name:     "single channel group remains healthy",
			summary:  &perfmetrics.GroupProbeSummary{TotalChannels: 1, CheckedChannels: 1, HealthyChannels: 1},
			expected: model.ChannelProbeStatusHealthy,
		},
		{
			name:     "all channels healthy",
			summary:  &perfmetrics.GroupProbeSummary{TotalChannels: 3, CheckedChannels: 3, HealthyChannels: 3},
			expected: model.ChannelProbeStatusHealthy,
		},
		{
			name:     "two healthy channels keep a multi-channel group green",
			summary:  &perfmetrics.GroupProbeSummary{TotalChannels: 3, CheckedChannels: 3, HealthyChannels: 2, UnhealthyChannels: 1},
			expected: model.ChannelProbeStatusHealthy,
		},
		{
			name:     "less than half healthy marks the group degraded",
			summary:  &perfmetrics.GroupProbeSummary{TotalChannels: 3, CheckedChannels: 3, HealthyChannels: 1, UnhealthyChannels: 2},
			expected: model.ChannelProbeStatusDegraded,
		},
		{
			name:     "all confirmed failures are unhealthy",
			summary:  &perfmetrics.GroupProbeSummary{TotalChannels: 2, CheckedChannels: 2, UnhealthyChannels: 2},
			expected: model.ChannelProbeStatusUnhealthy,
		},
		{
			name:     "all unavailable channels are unhealthy",
			summary:  &perfmetrics.GroupProbeSummary{TotalChannels: 2, CheckedChannels: 2, DegradedChannels: 2},
			expected: model.ChannelProbeStatusUnhealthy,
		},
		{
			name:     "unchecked groups stay unknown",
			summary:  &perfmetrics.GroupProbeSummary{TotalChannels: 2},
			expected: "unknown",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, classifyGroupProbeStatus(test.summary))
		})
	}
}

func TestBuildGroupProbeSummariesKeepsSyntheticHealthSeparate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:channel_probe_summary?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.ChannelProbeRecord{}, &model.ChannelProbeState{}))
	oldDB := model.DB
	oldSQLite, oldMySQL, oldPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	model.DB = db
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	t.Cleanup(func() {
		model.DB = oldDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPostgres
	})

	now := time.Now().Unix()
	channels := []model.Channel{
		{Id: 1, Name: "healthy", Key: "x", Status: common.ChannelStatusEnabled, Group: "gpt-plus,shared"},
		{Id: 2, Name: "failing", Key: "x", Status: common.ChannelStatusEnabled, Group: "gpt-plus"},
		{Id: 3, Name: "unchecked", Key: "x", Status: common.ChannelStatusEnabled, Group: "shared"},
	}
	require.NoError(t, db.Create(&channels).Error)
	require.NoError(t, db.Create(&[]model.ChannelProbeState{
		{ChannelId: 1, Status: model.ChannelProbeStatusHealthy, LastProbeTs: now, LastSuccessTs: now},
		{ChannelId: 2, Status: model.ChannelProbeStatusUnhealthy, LastProbeTs: now, LastFailureTs: now, LastErrorCategory: "upstream", LastErrorCode: "bad_response"},
	}).Error)
	require.NoError(t, db.Create(&[]model.ChannelProbeRecord{
		{ChannelId: 1, ProbeTs: now, Success: true, LatencyMs: 800},
		{ChannelId: 2, ProbeTs: now, Success: false, ErrorCategory: "upstream", ErrorCode: "bad_response"},
	}).Error)

	summaries, err := buildGroupProbeSummaries(24, []string{"gpt-plus", "shared"})
	require.NoError(t, err)
	require.Equal(t, "healthy", summaries["gpt-plus"].Status)
	require.Equal(t, 2, summaries["gpt-plus"].TotalChannels)
	require.Equal(t, 2, summaries["gpt-plus"].CheckedChannels)
	require.Equal(t, 1, summaries["gpt-plus"].HealthyChannels)
	require.Equal(t, 1, summaries["gpt-plus"].UnhealthyChannels)
	require.Equal(t, 50.0, summaries["gpt-plus"].SuccessRate)
	require.EqualValues(t, 800, summaries["gpt-plus"].AvgLatencyMs)
	require.Equal(t, "upstream", summaries["gpt-plus"].LastErrorCategory)
	require.Len(t, summaries["gpt-plus"].Series, 48)
	var observed *perfmetrics.ProbeSeriesPoint
	for index := range summaries["gpt-plus"].Series {
		point := &summaries["gpt-plus"].Series[index]
		if point.ProbeCount > 0 {
			observed = point
			break
		}
	}
	require.NotNil(t, observed)
	require.Equal(t, 2, observed.TotalChannels)
	require.Equal(t, 2, observed.CheckedChannels)
	require.Equal(t, 1, observed.HealthyChannels)
	// Intervals with no probe samples are retained and remain unknown.
	require.Equal(t, 0, summaries["gpt-plus"].Series[0].CheckedChannels)
	require.Equal(t, 0, summaries["gpt-plus"].Series[0].HealthyChannels)

	require.Equal(t, "healthy", summaries["shared"].Status)
	require.Equal(t, 2, summaries["shared"].TotalChannels)
	require.Equal(t, 1, summaries["shared"].CheckedChannels)
}
