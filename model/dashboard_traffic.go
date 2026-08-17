package model

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
	records := make([]DashboardTrafficRecord, 0)
	startBucket := usageLogAggregateBucketStart(startTime)
	raw := LOG_DB.Raw(`
		SELECT user_id, channel_id, created_at, use_time, quota, cost, 1 AS request_count, ? AS aggregated
		FROM logs WHERE type = ? AND created_at >= ? AND created_at < ?
		UNION ALL
		SELECT user_id, channel_id, last_log_at AS created_at, use_time, quota, cost, request_count, ? AS aggregated
		FROM usage_log_daily_aggregates
		WHERE type = ? AND bucket_start >= ? AND bucket_start < ?`,
		false, LogTypeConsume, startTime, endTime,
		true, LogTypeConsume, startBucket, endTime,
	)
	tx := LOG_DB.Table("(?) AS dashboard_usage_rows", raw)
	if userId > 0 {
		tx = tx.Where("user_id = ?", userId)
	}
	// Aggregation sorts interval boundaries itself and does not depend on row
	// order. Avoiding ORDER BY prevents a large filesort for admin-wide ranges.
	err := tx.Scan(&records).Error
	return records, err
}

func GetDashboardChannelNames(channelIds []int) (map[int]string, error) {
	names := make(map[int]string, len(channelIds))
	if len(channelIds) == 0 {
		return names, nil
	}
	var channels []struct {
		Id   int
		Name string
	}
	if err := DB.Model(&Channel{}).
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
