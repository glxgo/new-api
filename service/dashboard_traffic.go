package service

import (
	"fmt"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/model"
)

type DashboardTrafficSummary struct {
	RequestCount    int64   `json:"request_count"`
	ActiveMinutes   int     `json:"active_minutes"`
	AvgRPM          float64 `json:"avg_rpm"`
	PeakRPM         int     `json:"peak_rpm"`
	AvgConcurrency  float64 `json:"avg_concurrency"`
	PeakConcurrency int     `json:"peak_concurrency"`
	BilledQuota     int64   `json:"billed_quota"`
	CostQuota       int64   `json:"cost_quota,omitempty"`
}

type DashboardTrafficDaily struct {
	DayStart int64 `json:"day_start"`
	DashboardTrafficSummary
}

type DashboardChannelTraffic struct {
	ChannelId   int                     `json:"channel_id"`
	ChannelName string                  `json:"channel_name"`
	Summary     DashboardTrafficSummary `json:"summary"`
	Daily       []DashboardTrafficDaily `json:"daily"`
}

type DashboardTrafficResult struct {
	Summary  DashboardTrafficSummary   `json:"summary"`
	Daily    []DashboardTrafficDaily   `json:"daily"`
	Channels []DashboardChannelTraffic `json:"channels,omitempty"`
}

type trafficInterval struct {
	start int64
	end   int64
}

type dashboardTrafficAccumulator struct {
	requestCount      int64
	timedRequestCount int64
	billedQuota       int64
	costQuota         int64
	minuteCounts      map[int64]int
	intervals         []trafficInterval
}

func newDashboardTrafficAccumulator() *dashboardTrafficAccumulator {
	return &dashboardTrafficAccumulator{
		minuteCounts: make(map[int64]int),
		intervals:    make([]trafficInterval, 0),
	}
}

func (a *dashboardTrafficAccumulator) add(record model.DashboardTrafficRecord, rangeStart, rangeEnd int64) {
	if record.CreatedAt < rangeStart || record.CreatedAt >= rangeEnd {
		return
	}
	duration := int64(record.UseTime)
	if duration <= 0 {
		duration = 1
	}
	start := record.CreatedAt - duration
	if start < rangeStart {
		start = rangeStart
	}
	end := record.CreatedAt
	if end <= start {
		end = start + 1
	}
	if end > rangeEnd {
		end = rangeEnd
	}

	requestCount := record.RequestCount
	if requestCount <= 0 {
		requestCount = 1
	}
	a.requestCount += requestCount
	a.billedQuota += int64(record.Quota)
	a.costQuota += int64(record.Cost)
	if record.Aggregated {
		// Compacted rows intentionally retain financial/request totals but no
		// request-level timestamps, so exact RPM/concurrency remains a 7-day
		// detailed metric instead of being reconstructed from invented data.
		return
	}
	a.timedRequestCount += requestCount
	a.minuteCounts[start/60]++
	if end > start {
		a.intervals = append(a.intervals, trafficInterval{start: start, end: end})
	}
}

func (a *dashboardTrafficAccumulator) summary() DashboardTrafficSummary {
	result := DashboardTrafficSummary{
		RequestCount:  a.requestCount,
		ActiveMinutes: len(a.minuteCounts),
		BilledQuota:   a.billedQuota,
		CostQuota:     a.costQuota,
	}
	if result.ActiveMinutes > 0 {
		result.AvgRPM = float64(a.timedRequestCount) / float64(result.ActiveMinutes)
	}
	for _, count := range a.minuteCounts {
		if count > result.PeakRPM {
			result.PeakRPM = count
		}
	}

	events := make(map[int64]int, len(a.intervals)*2)
	for _, interval := range a.intervals {
		events[interval.start]++
		events[interval.end]--
	}
	timestamps := make([]int64, 0, len(events))
	for timestamp := range events {
		timestamps = append(timestamps, timestamp)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })

	current := 0
	var previous int64
	var activeSeconds int64
	var requestSeconds int64
	for i, timestamp := range timestamps {
		if i > 0 && current > 0 && timestamp > previous {
			seconds := timestamp - previous
			activeSeconds += seconds
			requestSeconds += int64(current) * seconds
		}
		current += events[timestamp]
		if current > result.PeakConcurrency {
			result.PeakConcurrency = current
		}
		previous = timestamp
	}
	if activeSeconds > 0 {
		result.AvgConcurrency = float64(requestSeconds) / float64(activeSeconds)
	}
	return result
}

func dayStartFor(timestamp int64, location *time.Location) int64 {
	current := time.Unix(timestamp, 0).In(location)
	return time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, location).Unix()
}

func dashboardDayStarts(startTime, endTime int64, location *time.Location) []int64 {
	start := dayStartFor(startTime, location)
	days := make([]int64, 0)
	for current := start; current < endTime; {
		days = append(days, current)
		next := time.Unix(current, 0).In(location).AddDate(0, 0, 1)
		current = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, location).Unix()
	}
	return days
}

func addRecordToDaily(
	daily map[int64]*dashboardTrafficAccumulator,
	record model.DashboardTrafficRecord,
	rangeStart, rangeEnd int64,
	location *time.Location,
) {
	duration := int64(record.UseTime)
	if duration <= 0 {
		duration = 1
	}
	requestStart := record.CreatedAt - duration
	if requestStart < rangeStart {
		requestStart = rangeStart
	}
	dayStart := dayStartFor(requestStart, location)
	accumulator := daily[dayStart]
	if accumulator == nil {
		return
	}

	// Keep amount attribution on the completion day, while RPM follows request
	// start. Requests crossing midnight are split into each day's concurrency
	// interval below.
	requestCount := record.RequestCount
	if requestCount <= 0 {
		requestCount = 1
	}
	accumulator.requestCount += requestCount
	if !record.Aggregated {
		accumulator.timedRequestCount += requestCount
		accumulator.minuteCounts[requestStart/60]++
	}
	completionDay := dayStartFor(record.CreatedAt, location)
	if costAccumulator := daily[completionDay]; costAccumulator != nil {
		costAccumulator.billedQuota += int64(record.Quota)
		costAccumulator.costQuota += int64(record.Cost)
	}

	if record.Aggregated {
		return
	}
	intervalEnd := record.CreatedAt
	if intervalEnd <= requestStart {
		intervalEnd = requestStart + 1
	}
	for current := requestStart; current < intervalEnd; {
		currentDay := dayStartFor(current, location)
		nextDay := time.Unix(currentDay, 0).In(location).AddDate(0, 0, 1)
		boundary := time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 0, 0, 0, 0, location).Unix()
		partEnd := intervalEnd
		if partEnd > boundary {
			partEnd = boundary
		}
		if dayAccumulator := daily[currentDay]; dayAccumulator != nil && partEnd > current {
			dayAccumulator.intervals = append(dayAccumulator.intervals, trafficInterval{start: current, end: partEnd})
		}
		current = partEnd
	}
}

func buildDailyTraffic(
	records []model.DashboardTrafficRecord,
	startTime, endTime int64,
	location *time.Location,
) []DashboardTrafficDaily {
	dayStarts := dashboardDayStarts(startTime, endTime, location)
	accumulators := make(map[int64]*dashboardTrafficAccumulator, len(dayStarts))
	for _, start := range dayStarts {
		accumulators[start] = newDashboardTrafficAccumulator()
	}
	for _, record := range records {
		addRecordToDaily(accumulators, record, startTime, endTime, location)
	}
	result := make([]DashboardTrafficDaily, 0, len(dayStarts))
	for _, start := range dayStarts {
		result = append(result, DashboardTrafficDaily{
			DayStart:                start,
			DashboardTrafficSummary: accumulators[start].summary(),
		})
	}
	return result
}

func BuildDashboardTraffic(
	records []model.DashboardTrafficRecord,
	channelNames map[int]string,
	startTime, endTime int64,
	location *time.Location,
	includeChannels bool,
) DashboardTrafficResult {
	if location == nil {
		location = time.UTC
	}
	total := newDashboardTrafficAccumulator()
	for _, record := range records {
		total.add(record, startTime, endTime)
	}
	result := DashboardTrafficResult{
		Summary: total.summary(),
		Daily:   buildDailyTraffic(records, startTime, endTime, location),
	}
	if !includeChannels {
		// Upstream cost is an operator-only business metric.
		result.Summary.CostQuota = 0
		for index := range result.Daily {
			result.Daily[index].CostQuota = 0
		}
		return result
	}

	byChannel := make(map[int][]model.DashboardTrafficRecord)
	for _, record := range records {
		if record.ChannelId > 0 {
			byChannel[record.ChannelId] = append(byChannel[record.ChannelId], record)
		}
	}
	channelIds := make([]int, 0, len(byChannel))
	for channelId := range byChannel {
		channelIds = append(channelIds, channelId)
	}
	sort.Ints(channelIds)
	result.Channels = make([]DashboardChannelTraffic, 0, len(channelIds))
	for _, channelId := range channelIds {
		channelRecords := byChannel[channelId]
		channelAccumulator := newDashboardTrafficAccumulator()
		for _, record := range channelRecords {
			channelAccumulator.add(record, startTime, endTime)
		}
		name := channelNames[channelId]
		if name == "" {
			name = fmt.Sprintf("#%d", channelId)
		}
		result.Channels = append(result.Channels, DashboardChannelTraffic{
			ChannelId:   channelId,
			ChannelName: name,
			Summary:     channelAccumulator.summary(),
			Daily:       buildDailyTraffic(channelRecords, startTime, endTime, location),
		})
	}
	return result
}
