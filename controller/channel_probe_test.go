package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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

func TestClassifyChannelProbeError(t *testing.T) {
	require.Equal(t, "timeout", classifyChannelProbeError(testResult{localErr: context.DeadlineExceeded}))
	require.Equal(t, "rate_limit", classifyChannelProbeError(testResult{httpStatus: 429}))
	require.Equal(t, "upstream", classifyChannelProbeError(testResult{httpStatus: 502}))
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
	require.Equal(t, "degraded", summaries["gpt-plus"].Status)
	require.Equal(t, 2, summaries["gpt-plus"].TotalChannels)
	require.Equal(t, 2, summaries["gpt-plus"].CheckedChannels)
	require.Equal(t, 1, summaries["gpt-plus"].HealthyChannels)
	require.Equal(t, 1, summaries["gpt-plus"].UnhealthyChannels)
	require.Equal(t, 50.0, summaries["gpt-plus"].SuccessRate)
	require.EqualValues(t, 800, summaries["gpt-plus"].AvgLatencyMs)
	require.Equal(t, "upstream", summaries["gpt-plus"].LastErrorCategory)

	require.Equal(t, "degraded", summaries["shared"].Status)
	require.Equal(t, 2, summaries["shared"].TotalChannels)
	require.Equal(t, 1, summaries["shared"].CheckedChannels)
}
