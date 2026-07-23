package model

// DashboardTrafficRecord contains only the successful consume-log fields needed
// to build dashboard traffic statistics. Error logs are deliberately excluded
// by GetDashboardTrafficRecords, so they never inflate dashboard RPM.
type DashboardTrafficRecord struct {
	UserId    int
	ChannelId int
	CreatedAt int64
	UseTime   int
	Quota     int
	Cost      int
}

func GetDashboardTrafficRecords(userId int, startTime, endTime int64) ([]DashboardTrafficRecord, error) {
	records := make([]DashboardTrafficRecord, 0)
	tx := LOG_DB.Table("logs").
		Select("user_id, channel_id, created_at, use_time, quota, cost").
		Where("type = ?", LogTypeConsume).
		Where("created_at >= ? AND created_at < ?", startTime, endTime)
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
