package model

import (
	"context"
	"strconv"
	"strings"
)

// Deliberately select only public-to-admin metadata, never channel credentials.
type ChannelMetricIdentity struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Type   int    `json:"type"`
	Status int    `json:"status"`
}

func GetChannelMetricPage(ctx context.Context, keyword string, offset, limit int) ([]ChannelMetricIdentity, int64, error) {
	rows := make([]ChannelMetricIdentity, 0)
	q := DB.WithContext(ctx).Model(&Channel{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		// Treat LIKE metacharacters literally, consistently on all supported DBs.
		pattern := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(keyword) + "%"
		if id, err := strconv.Atoi(keyword); err == nil && id > 0 {
			q = q.Where("id = ? OR name LIKE ? ESCAPE '!'", id, pattern)
		} else {
			q = q.Where("name LIKE ? ESCAPE '!'", pattern)
		}
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return rows, 0, err
	}
	err := q.Select("id, name, type, status").Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}

type ChannelMetricTotals struct {
	ChannelId     int
	RequestCount  int64
	SuccessCount  int64
	TtftSumMs     int64
	TtftCount     int64
	CacheTokens   int64
	PromptTokens  int64
	LegacyBuckets int64
}

// Aggregate in the database for only the requested page, never load raw logs.
// Legacy buckets overlapping the lower boundary are flagged but not guessed or
// prorated. Their original timestamp determines whether their counts belong here.
func GetChannelMetricTotals(ctx context.Context, start, end int64, ids []int) ([]ChannelMetricTotals, error) {
	rows := make([]ChannelMetricTotals, 0)
	if len(ids) == 0 {
		return rows, nil
	}
	columns := []string{"channel_id"}
	args := make([]interface{}, 0, 8)
	for _, column := range []string{"request_count", "success_count", "ttft_sum_ms", "ttft_count", "cache_tokens", "prompt_tokens"} {
		columns = append(columns, "SUM(CASE WHEN bucket_ts >= ? THEN "+column+" ELSE 0 END) AS "+column)
		args = append(args, start)
	}
	columns = append(columns, "SUM(CASE WHEN bucket_seconds > 60 AND bucket_ts + bucket_seconds > ? THEN 1 ELSE 0 END) AS legacy_buckets")
	args = append(args, start)
	err := DB.WithContext(ctx).Model(&ChannelPerfMetric{}).
		Select(strings.Join(columns, ", "), args...).
		Where("channel_id IN ? AND bucket_ts >= ? AND bucket_ts < ?", ids, start-3600, end).
		Group("channel_id").Scan(&rows).Error
	return rows, err
}
