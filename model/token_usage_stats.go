package model

import (
	"fmt"
	"time"
)

// TokenUsageStats is the settled billing usage visible on the API-key page.
// The lifetime value is intentionally reconstructed from both live detail
// rows and archived daily aggregates so it remains stable after retention.
type TokenUsageStats struct {
	TodayUsedQuota    int64
	LifetimeUsedQuota int64
}

// chinaDayRangeForTokenUsage returns the current Asia/Shanghai calendar day.
// This is deliberately independent from time.Local so the displayed "today"
// boundary is stable across hosts and containers.
func chinaDayRangeForTokenUsage(now time.Time) (int64, int64) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	localNow := now.In(location)
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	return start.Unix(), now.Unix()
}

type tokenUsageQuotaRow struct {
	TokenId int   `gorm:"column:token_id"`
	Quota   int64 `gorm:"column:quota"`
}

// GetTokenUsageStats returns today's and lifetime settled consumption for a
// bounded set of token IDs. It performs two grouped queries for live detail
// rows and one grouped query for archived daily aggregates, avoiding an N+1
// query when the API-key list is rendered.
func GetTokenUsageStats(tokenIDs []int, now time.Time) (map[int]TokenUsageStats, error) {
	result := make(map[int]TokenUsageStats, len(tokenIDs))
	if len(tokenIDs) == 0 {
		return result, nil
	}
	if LOG_DB == nil {
		return nil, fmt.Errorf("log database is unavailable")
	}
	for _, tokenID := range tokenIDs {
		if tokenID > 0 {
			result[tokenID] = TokenUsageStats{}
		}
	}
	if len(result) == 0 {
		return result, nil
	}
	ids := make([]int, 0, len(result))
	for tokenID := range result {
		ids = append(ids, tokenID)
	}
	todayStart, todayEnd := chinaDayRangeForTokenUsage(now)

	var todayRows []tokenUsageQuotaRow
	if err := LOG_DB.Model(&Log{}).
		Select("token_id, COALESCE(SUM(quota), 0) AS quota").
		Where("token_id IN ? AND type = ? AND settled = ? AND created_at >= ? AND created_at < ?", ids, LogTypeConsume, true, todayStart, todayEnd).
		Group("token_id").Find(&todayRows).Error; err != nil {
		return nil, fmt.Errorf("query token daily usage: %w", err)
	}
	for _, row := range todayRows {
		stats := result[row.TokenId]
		stats.TodayUsedQuota = row.Quota
		result[row.TokenId] = stats
	}

	var liveRows []tokenUsageQuotaRow
	if err := LOG_DB.Model(&Log{}).
		Select("token_id, COALESCE(SUM(quota), 0) AS quota").
		Where("token_id IN ? AND type = ? AND settled = ?", ids, LogTypeConsume, true).
		Group("token_id").Find(&liveRows).Error; err != nil {
		return nil, fmt.Errorf("query token lifetime detail usage: %w", err)
	}
	for _, row := range liveRows {
		stats := result[row.TokenId]
		stats.LifetimeUsedQuota += row.Quota
		result[row.TokenId] = stats
	}

	var archivedRows []tokenUsageQuotaRow
	if err := LOG_DB.Model(&UsageLogDailyAggregate{}).
		Select("token_id, COALESCE(SUM(quota), 0) AS quota").
		Where("token_id IN ? AND type = ?", ids, LogTypeConsume).
		Group("token_id").Find(&archivedRows).Error; err != nil {
		return nil, fmt.Errorf("query token archived usage: %w", err)
	}
	for _, row := range archivedRows {
		stats := result[row.TokenId]
		stats.LifetimeUsedQuota += row.Quota
		result[row.TokenId] = stats
	}
	return result, nil
}

// AttachTokenUsageStats enriches API-key response objects in one grouped read.
func AttachTokenUsageStats(tokens []*Token, now time.Time) error {
	if len(tokens) == 0 {
		return nil
	}
	ids := make([]int, 0, len(tokens))
	for _, token := range tokens {
		if token != nil {
			ids = append(ids, token.Id)
		}
	}
	stats, err := GetTokenUsageStats(ids, now)
	if err != nil {
		return err
	}
	for _, token := range tokens {
		if token == nil {
			continue
		}
		usage := stats[token.Id]
		token.TodayUsedQuota = usage.TodayUsedQuota
		token.LifetimeUsedQuota = usage.LifetimeUsedQuota
	}
	return nil
}
