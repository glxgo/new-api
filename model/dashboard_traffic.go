package model

import (
	"context"

	"gorm.io/gorm"
)

// DashboardTrafficRecord contains only the successful consume-log fields needed
// to build dashboard traffic statistics. Error logs are deliberately excluded
// by GetDashboardTrafficRecords, so they never inflate dashboard RPM.
type DashboardTrafficRecord struct {
	UserId       int
	ChannelId    int
	CreatedAt    int64
	UseTime      int
	Quota        int
	Cost         int
	RequestCount int64
	Aggregated   bool
}

func GetDashboardTrafficRecords(userId int, startTime, endTime int64) ([]DashboardTrafficRecord, error) {
	return GetDashboardTrafficRecordsWithContext(context.Background(), userId, startTime, endTime)
}

func GetDashboardTrafficRecordsWithContext(ctx context.Context, userId int, startTime, endTime int64) ([]DashboardTrafficRecord, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if metricRecords, ready, metricErr := readDashboardTrafficFromMetricBuckets(ctx, userId, startTime, endTime); ready && metricErr == nil {
		return metricRecords, nil
	}
	archiveStart, archiveEnd := usageStatisticsArchiveWindow(startTime, endTime)
	records, err := queryDashboardTrafficRecords(ctx, userId, startTime, endTime, archiveStart, archiveEnd, archiveStart < archiveEnd)
	if err == nil || archiveStart >= archiveEnd {
		return records, err
	}
	// Older installations may not have the archive table yet. The dashboard
	// must remain usable from raw logs while the optional retention migration is
	// being staged.
	return queryDashboardTrafficRecords(ctx, userId, startTime, endTime, 0, 0, false)
}

func queryDashboardTrafficRecords(ctx context.Context, userId int, startTime, endTime, archiveStart, archiveEnd int64, includeArchive bool) ([]DashboardTrafficRecord, error) {
	records := make([]DashboardTrafficRecord, 0)
	// Filter at each source, before UNION materialization (especially on
	// MySQL 5.7). An outer-only predicate may read every user's detailed logs.
	userFilter := ""
	liveArgs := []any{false, LogTypeConsume, startTime, endTime}
	if userId > 0 {
		userFilter = " AND user_id = ?"
		liveArgs = append(liveArgs, userId)
	}
	var raw *gorm.DB
	if includeArchive && archiveStart < archiveEnd {
		args := append(liveArgs, true, LogTypeConsume, archiveStart, archiveEnd)
		if userId > 0 {
			args = append(args, userId)
		}
		raw = LOG_DB.WithContext(ctx).Raw(`
			SELECT user_id, channel_id, created_at, use_time, quota, cost, 1 AS request_count, ? AS aggregated
			FROM logs WHERE type = ? AND created_at >= ? AND created_at < ?`+userFilter+`
			UNION ALL
			SELECT user_id, channel_id, last_log_at AS created_at, use_time, quota, cost, request_count, ? AS aggregated
			FROM usage_log_daily_aggregates
			WHERE type = ? AND bucket_start >= ? AND bucket_start < ?`+userFilter,
			args...,
		)
	} else {
		raw = LOG_DB.WithContext(ctx).Raw(`
			SELECT user_id, channel_id, created_at, use_time, quota, cost, 1 AS request_count, ? AS aggregated
			FROM logs WHERE type = ? AND created_at >= ? AND created_at < ?`+userFilter,
			liveArgs...,
		)
	}
	tx := LOG_DB.WithContext(ctx).Table("(?) AS dashboard_usage_rows", raw)
	// Aggregation sorts interval boundaries itself and does not depend on row
	// order. Avoiding ORDER BY prevents a large filesort for admin-wide ranges.
	if err := tx.Scan(&records).Error; err != nil {
		return records, err
	}
	return records, nil
}

func GetDashboardChannelNames(channelIds []int) (map[int]string, error) {
	return GetDashboardChannelNamesWithContext(context.Background(), channelIds)
}

func GetDashboardChannelNamesWithContext(ctx context.Context, channelIds []int) (map[int]string, error) {
	names := make(map[int]string, len(channelIds))
	if len(channelIds) == 0 {
		return names, nil
	}
	var channels []struct {
		Id   int
		Name string
	}
	if err := DB.WithContext(ctx).Model(&Channel{}).
		Select("id, name").
		Where("id IN ?", channelIds).
		Scan(&channels).Error; err != nil {
		return nil, err
	}
	for _, channel := range channels {
		names[channel.Id] = channel.Name
	}
	return names, nil
}
