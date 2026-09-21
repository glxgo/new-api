package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSumUsedQuotaCachesIdenticalFiltersBriefly(t *testing.T) {
	previousLogDB := LOG_DB
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:log-stat-cache-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	LOG_DB = db
	t.Cleanup(func() {
		LOG_DB = previousLogDB
	})

	now := time.Now().Unix()
	baseLog := Log{
		CreatedAt:        now,
		Type:             LogTypeConsume,
		Username:         "log-stat-cache-user",
		TokenName:        "log-stat-cache-token",
		ModelName:        "log-stat-cache-model",
		Quota:            100,
		PromptTokens:     10,
		CompletionTokens: 20,
		ChannelId:        7,
	}
	require.NoError(t, db.Create(&baseLog).Error)

	historicLog := baseLog
	historicLog.Id = 0
	historicLog.CreatedAt = now - 300
	historicLog.Quota = 25
	historicLog.PromptTokens = 15
	historicLog.CompletionTokens = 25
	require.NoError(t, db.Create(&historicLog).Error)

	first, err := SumUsedQuota(
		LogTypeConsume,
		now-3600,
		now,
		baseLog.ModelName,
		baseLog.Username,
		baseLog.TokenName,
		0,
		"",
	)
	require.NoError(t, err)
	require.Equal(t, Stat{Quota: 125, PreDiscountQuota: 125, Rpm: 1, Tpm: 30, Tokens: 70}, first)

	secondLog := baseLog
	secondLog.Id = 0
	secondLog.Quota = 50
	require.NoError(t, db.Create(&secondLog).Error)

	cached, err := SumUsedQuota(
		LogTypeConsume,
		now-3600,
		now,
		baseLog.ModelName,
		baseLog.Username,
		baseLog.TokenName,
		0,
		"",
	)
	require.NoError(t, err)
	require.Equal(t, first, cached)

	freshFilter, err := SumUsedQuota(
		LogTypeConsume,
		now-3600,
		now,
		baseLog.ModelName,
		baseLog.Username,
		baseLog.TokenName,
		7,
		"",
	)
	require.NoError(t, err)
	require.Equal(t, Stat{Quota: 175, PreDiscountQuota: 175, Rpm: 2, Tpm: 60, Tokens: 100}, freshFilter)
}

func TestSumUsedQuotaPreservesRecentRpmForHistoricalRange(t *testing.T) {
	previousLogDB := LOG_DB
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:log-stat-history-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	LOG_DB = db
	t.Cleanup(func() {
		LOG_DB = previousLogDB
	})

	now := time.Now().Unix()
	username := fmt.Sprintf("log-stat-history-user-%d", now)
	modelName := "log-stat-history-model"
	// This row is in the selected historical window, but outside the recent
	// RPM/TPM window.
	historical := Log{
		CreatedAt:        now - 3600,
		Type:             LogTypeConsume,
		Username:         username,
		ModelName:        modelName,
		Quota:            25,
		PromptTokens:     15,
		CompletionTokens: 25,
	}
	// This row is outside the selected historical window, but must still count
	// toward the legacy “recent 60 seconds” RPM/TPM values.
	recent := historical
	recent.Id = 0
	recent.CreatedAt = now
	recent.Quota = 100
	recent.PromptTokens = 10
	recent.CompletionTokens = 20
	require.NoError(t, db.Create(&historical).Error)
	require.NoError(t, db.Create(&recent).Error)

	stat, err := SumUsedQuota(
		LogTypeConsume,
		now-3700,
		now-3500,
		modelName,
		username,
		"",
		0,
		"",
	)
	require.NoError(t, err)
	require.Equal(t, int64(25), stat.Quota)
	require.Equal(t, int64(40), stat.Tokens)
	require.Equal(t, 1, stat.Rpm)
	require.Equal(t, 30, stat.Tpm)
}
