package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestBuildDashboardTrafficUsesActiveMinutesAndSuccessfulIntervals(t *testing.T) {
	location := time.FixedZone("test", 8*60*60)
	dayStart := time.Date(2026, 7, 23, 0, 0, 0, 0, location).Unix()
	records := []model.DashboardTrafficRecord{
		{
			UserId:    1,
			ChannelId: 8,
			CreatedAt: dayStart + 10*3600 + 10,
			UseTime:   10,
			Quota:     100,
			Cost:      40,
		},
		{
			UserId:    1,
			ChannelId: 8,
			CreatedAt: dayStart + 10*3600 + 15,
			UseTime:   10,
			Quota:     200,
			Cost:      80,
		},
		{
			UserId:    1,
			ChannelId: 31,
			CreatedAt: dayStart + 10*3600 + 2*60 + 1,
			UseTime:   1,
			Quota:     300,
			Cost:      120,
		},
	}

	result := BuildDashboardTraffic(
		records,
		map[int]string{8: "kedaya", 31: "krill"},
		dayStart,
		dayStart+24*3600,
		location,
		true,
	)

	require.EqualValues(t, 3, result.Summary.RequestCount)
	require.Equal(t, 2, result.Summary.ActiveMinutes)
	require.InDelta(t, 1.5, result.Summary.AvgRPM, 0.001)
	require.Equal(t, 2, result.Summary.PeakRPM)
	require.Equal(t, 2, result.Summary.PeakConcurrency)
	require.InDelta(t, 21.0/16.0, result.Summary.AvgConcurrency, 0.001)
	require.EqualValues(t, 600, result.Summary.BilledQuota)
	require.EqualValues(t, 240, result.Summary.CostQuota)

	require.Len(t, result.Daily, 1)
	require.Len(t, result.Channels, 2)
	require.Equal(t, 8, result.Channels[0].ChannelId)
	require.Equal(t, "kedaya", result.Channels[0].ChannelName)
	require.EqualValues(t, 2, result.Channels[0].Summary.RequestCount)
	require.EqualValues(t, 300, result.Channels[0].Summary.BilledQuota)
	require.Equal(t, 2, result.Channels[0].Summary.PeakConcurrency)
}

func TestBuildDashboardTrafficOmitsChannelDetailsForUsers(t *testing.T) {
	location := time.UTC
	start := time.Date(2026, 7, 23, 0, 0, 0, 0, location).Unix()
	result := BuildDashboardTraffic(
		[]model.DashboardTrafficRecord{{
			UserId:    7,
			ChannelId: 8,
			CreatedAt: start + 60,
			UseTime:   1,
			Cost:      99,
		}},
		map[int]string{8: "private-channel"},
		start,
		start+3600,
		location,
		false,
	)

	require.Empty(t, result.Channels)
	require.EqualValues(t, 1, result.Summary.RequestCount)
	require.Zero(t, result.Summary.CostQuota)
	require.Zero(t, result.Daily[0].CostQuota)
}
