package controller

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

const channelProbeRetention = 30 * 24 * time.Hour

var probeSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(bearer\s+)[^\s,;]+`),
	regexp.MustCompile(`(?i)sk-[a-z0-9_-]{8,}`),
	regexp.MustCompile(`(?i)(api[_-]?key=)[^&\s]+`),
}

type adminChannelProbeState struct {
	ChannelId            int      `json:"channel_id"`
	ChannelName          string   `json:"channel_name"`
	ChannelStatus        int      `json:"channel_status"`
	Groups               []string `json:"groups"`
	Status               string   `json:"status"`
	ModelName            string   `json:"model_name"`
	LastProbeTs          int64    `json:"last_probe_ts"`
	LastSuccessTs        int64    `json:"last_success_ts"`
	LastFailureTs        int64    `json:"last_failure_ts"`
	LastLatencyMs        int64    `json:"last_latency_ms"`
	LastTtftMs           int64    `json:"last_ttft_ms"`
	HasTtft              bool     `json:"has_ttft"`
	LastHttpStatus       int      `json:"last_http_status"`
	LastErrorCode        string   `json:"last_error_code"`
	LastErrorCategory    string   `json:"last_error_category"`
	LastErrorMessage     string   `json:"last_error_message"`
	ConsecutiveFailures  int      `json:"consecutive_failures"`
	ConsecutiveSuccesses int      `json:"consecutive_successes"`
}

func classifyChannelProbeError(result testResult) string {
	message := strings.ToLower(probeErrorMessage(result))
	code := ""
	if result.newAPIError != nil {
		code = strings.ToLower(string(result.newAPIError.GetErrorCode()))
	}
	if errors.Is(result.localErr, context.DeadlineExceeded) || strings.Contains(message, "deadline exceeded") || strings.Contains(message, "timeout") {
		return "timeout"
	}
	if result.httpStatus == http.StatusUnauthorized || result.httpStatus == http.StatusForbidden || strings.Contains(code, "invalid_key") || strings.Contains(message, "invalid api key") {
		return "authentication"
	}
	if result.httpStatus == http.StatusTooManyRequests {
		return "rate_limit"
	}
	if result.httpStatus >= 500 {
		return "upstream"
	}
	if strings.Contains(code, "response") || strings.Contains(code, "read_response") {
		return "response"
	}
	if strings.Contains(code, "channel:") || strings.Contains(code, "convert_request") || strings.Contains(code, "gen_relay") {
		return "configuration"
	}
	return "unknown"
}

func probeErrorMessage(result testResult) string {
	if result.newAPIError != nil {
		return result.newAPIError.Error()
	}
	if result.localErr != nil {
		return result.localErr.Error()
	}
	return ""
}

func sanitizeProbeError(message string) string {
	message = strings.Join(strings.Fields(message), " ")
	for _, pattern := range probeSecretPatterns {
		message = pattern.ReplaceAllString(message, "[REDACTED]")
	}
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

func executeChannelProbe(channel *model.Channel, testUserID int) (*model.ChannelProbeState, error) {
	settings := operation_setting.GetMonitorSetting()
	result := testChannelWithOptions(
		channel,
		testUserID,
		"",
		"",
		shouldUseStreamForAutomaticChannelTest(channel),
		channelTestOptions{
			recordConsumeLog: false,
			timeout:          time.Duration(settings.ChannelCanaryTimeoutSeconds) * time.Second,
		},
	)

	record := &model.ChannelProbeRecord{
		ChannelId:  channel.Id,
		ModelName:  result.model,
		Groups:     strings.Join(channel.GetGroups(), ","),
		ProbeTs:    time.Now().Unix(),
		Success:    result.localErr == nil && result.newAPIError == nil,
		LatencyMs:  result.latencyMs,
		TtftMs:     result.ttftMs,
		HasTtft:    result.hasTtft,
		HttpStatus: result.httpStatus,
	}
	if !record.Success {
		record.ErrorCategory = classifyChannelProbeError(result)
		record.ErrorMessage = sanitizeProbeError(probeErrorMessage(result))
		if result.newAPIError != nil {
			record.ErrorCode = string(result.newAPIError.GetErrorCode())
		}
	}

	state, err := model.RecordChannelProbe(
		record,
		settings.ChannelCanaryFailureThreshold,
		settings.ChannelCanaryRecoveryThreshold,
	)
	if err != nil {
		return nil, err
	}
	return state, nil
}

var (
	channelProbeBatchMu      sync.Mutex
	channelProbeBatchRunning bool
)

func runChannelProbeBatch() error {
	channelProbeBatchMu.Lock()
	if channelProbeBatchRunning {
		channelProbeBatchMu.Unlock()
		return errors.New("渠道金丝雀探测已在运行")
	}
	channelProbeBatchRunning = true
	channelProbeBatchMu.Unlock()
	defer func() {
		channelProbeBatchMu.Lock()
		channelProbeBatchRunning = false
		channelProbeBatchMu.Unlock()
	}()

	testUserID, err := resolveChannelTestUserID(nil)
	if err != nil {
		return err
	}
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return err
	}
	selectedIds := operation_setting.GetMonitorSetting().ChannelCanaryChannelIds
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		if len(selectedIds) > 0 && !lo.Contains(selectedIds, channel.Id) {
			continue
		}
		if _, probeErr := executeChannelProbe(channel, testUserID); probeErr != nil {
			common.SysError(fmt.Sprintf("channel canary probe failed to persist: channel_id=%d err=%v", channel.Id, probeErr))
		}
		time.Sleep(common.RequestInterval)
	}
	return model.DeleteChannelProbeRecordsBefore(time.Now().Add(-channelProbeRetention).Unix())
}

var automaticChannelProbeOnce sync.Once

func AutomaticallyProbeChannels() {
	if !common.IsMasterNode {
		return
	}
	automaticChannelProbeOnce.Do(func() {
		for {
			settings := operation_setting.GetMonitorSetting()
			if !settings.ChannelCanaryEnabled {
				time.Sleep(time.Minute)
				continue
			}
			common.SysLog("running channel canary probes (observation only; no channel status changes)")
			if err := runChannelProbeBatch(); err != nil {
				common.SysError("channel canary probe batch failed: " + err.Error())
			}
			time.Sleep(time.Duration(settings.ChannelCanaryMinutes) * time.Minute)
		}
	})
}

func GetChannelProbeStatus(c *gin.Context) {
	channels, err := model.GetAllChannelsWithoutKey()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	states, err := model.GetChannelProbeStates()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	stateByChannel := lo.SliceToMap(states, func(state model.ChannelProbeState) (int, model.ChannelProbeState) {
		return state.ChannelId, state
	})
	items := make([]adminChannelProbeState, 0, len(channels))
	for _, channel := range channels {
		state, ok := stateByChannel[channel.Id]
		item := adminChannelProbeState{
			ChannelId:     channel.Id,
			ChannelName:   channel.Name,
			ChannelStatus: channel.Status,
			Groups:        channel.GetGroups(),
			Status:        model.ChannelProbeStatusChecking,
		}
		if ok {
			item.Status = state.Status
			item.ModelName = state.ModelName
			item.LastProbeTs = state.LastProbeTs
			item.LastSuccessTs = state.LastSuccessTs
			item.LastFailureTs = state.LastFailureTs
			item.LastLatencyMs = state.LastLatencyMs
			item.LastTtftMs = state.LastTtftMs
			item.HasTtft = state.HasTtft
			item.LastHttpStatus = state.LastHttpStatus
			item.LastErrorCode = state.LastErrorCode
			item.LastErrorCategory = state.LastErrorCategory
			item.LastErrorMessage = state.LastErrorMessage
			item.ConsecutiveFailures = state.ConsecutiveFailures
			item.ConsecutiveSuccesses = state.ConsecutiveSuccesses
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ChannelId < items[j].ChannelId })
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func ProbeChannelNow(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil || channelId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的渠道 ID"})
		return
	}
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if channel.Status != common.ChannelStatusEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "只能探测已启用渠道"})
		return
	}
	testUserID, err := resolveChannelTestUserID(nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	state, err := executeChannelProbe(channel, testUserID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": state})
}

type groupProbeAccumulator struct {
	summary       *perfmetrics.GroupProbeSummary
	latencySum    int64
	latencyCount  int64
	ttftSum       int64
	ttftCount     int64
	probeCount    int64
	successCount  int64
	series        map[int64]*probeSeriesAccumulator
	latestErrorTs int64
}

type probeSeriesAccumulator struct {
	probeCount   int64
	successCount int64
	latencySum   int64
	latencyCount int64
	ttftSum      int64
	ttftCount    int64
}

func buildGroupProbeSummaries(hours int, visibleGroups []string) (map[string]*perfmetrics.GroupProbeSummary, error) {
	result := make(map[string]*perfmetrics.GroupProbeSummary)
	if model.DB == nil || !model.DB.Migrator().HasTable(&model.ChannelProbeState{}) || !model.DB.Migrator().HasTable(&model.ChannelProbeRecord{}) {
		return result, nil
	}
	if hours < 1 {
		hours = 24
	}
	visible := make(map[string]struct{}, len(visibleGroups))
	for _, group := range visibleGroups {
		visible[group] = struct{}{}
	}

	channels, err := model.GetAllChannelsWithoutKey()
	if err != nil {
		return nil, err
	}
	states, err := model.GetChannelProbeStates()
	if err != nil {
		return nil, err
	}
	records, err := model.GetChannelProbeRecordsSince(time.Now().Add(-time.Duration(hours) * time.Hour).Unix())
	if err != nil {
		return nil, err
	}
	stateByChannel := lo.SliceToMap(states, func(state model.ChannelProbeState) (int, model.ChannelProbeState) {
		return state.ChannelId, state
	})

	groupsByChannel := make(map[int][]string)
	accumulators := make(map[string]*groupProbeAccumulator)
	ensureGroup := func(group string) *groupProbeAccumulator {
		if existing, ok := accumulators[group]; ok {
			return existing
		}
		created := &groupProbeAccumulator{
			summary: &perfmetrics.GroupProbeSummary{Status: "unknown"},
			series:  make(map[int64]*probeSeriesAccumulator),
		}
		accumulators[group] = created
		return created
	}

	settings := operation_setting.GetMonitorSetting()
	freshFor := time.Duration(settings.ChannelCanaryMinutes*3) * time.Minute
	if freshFor < 15*time.Minute {
		freshFor = 15 * time.Minute
	}
	freshAfter := time.Now().Add(-freshFor).Unix()
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		channelGroups := make([]string, 0, len(channel.GetGroups())+1)
		for _, group := range channel.GetGroups() {
			if _, ok := visible[group]; ok {
				channelGroups = append(channelGroups, group)
			}
		}
		if _, ok := visible["auto"]; ok && len(channelGroups) > 0 {
			channelGroups = append(channelGroups, "auto")
		}
		channelGroups = lo.Uniq(channelGroups)
		groupsByChannel[channel.Id] = channelGroups
		state, hasState := stateByChannel[channel.Id]
		for _, group := range channelGroups {
			acc := ensureGroup(group)
			acc.summary.TotalChannels++
			if !hasState || state.LastProbeTs < freshAfter {
				continue
			}
			acc.summary.CheckedChannels++
			if state.LastProbeTs > acc.summary.LastProbeTs {
				acc.summary.LastProbeTs = state.LastProbeTs
			}
			if state.LastSuccessTs > acc.summary.LastSuccessTs {
				acc.summary.LastSuccessTs = state.LastSuccessTs
			}
			if state.LastFailureTs == state.LastProbeTs && state.LastFailureTs > acc.latestErrorTs {
				acc.latestErrorTs = state.LastFailureTs
				acc.summary.LastErrorCategory = state.LastErrorCategory
				acc.summary.LastErrorCode = state.LastErrorCode
			}
			switch state.Status {
			case model.ChannelProbeStatusHealthy:
				acc.summary.HealthyChannels++
			case model.ChannelProbeStatusUnhealthy:
				acc.summary.UnhealthyChannels++
			default:
				acc.summary.DegradedChannels++
			}
		}
	}

	bucketSeconds := int64(math.Ceil(float64(hours)/24.0) * 3600)
	if bucketSeconds < 3600 {
		bucketSeconds = 3600
	}
	for _, record := range records {
		for _, group := range groupsByChannel[record.ChannelId] {
			acc := ensureGroup(group)
			acc.probeCount++
			bucketTs := record.ProbeTs - record.ProbeTs%bucketSeconds
			bucket := acc.series[bucketTs]
			if bucket == nil {
				bucket = &probeSeriesAccumulator{}
				acc.series[bucketTs] = bucket
			}
			bucket.probeCount++
			if record.Success {
				acc.successCount++
				bucket.successCount++
				if record.LatencyMs > 0 {
					acc.latencySum += record.LatencyMs
					acc.latencyCount++
					bucket.latencySum += record.LatencyMs
					bucket.latencyCount++
				}
				if record.HasTtft && record.TtftMs >= 0 {
					acc.ttftSum += record.TtftMs
					acc.ttftCount++
					bucket.ttftSum += record.TtftMs
					bucket.ttftCount++
				}
			}
		}
	}

	for group, acc := range accumulators {
		summary := acc.summary
		switch {
		case summary.CheckedChannels == 0:
			summary.Status = "unknown"
		case summary.CheckedChannels == summary.TotalChannels && summary.HealthyChannels == summary.TotalChannels:
			summary.Status = model.ChannelProbeStatusHealthy
		case summary.HealthyChannels == 0 && summary.UnhealthyChannels > 0 && summary.DegradedChannels == 0:
			summary.Status = model.ChannelProbeStatusUnhealthy
		default:
			summary.Status = model.ChannelProbeStatusDegraded
		}
		if acc.probeCount > 0 {
			summary.SuccessRate = float64(acc.successCount) * 100 / float64(acc.probeCount)
		}
		if acc.latencyCount > 0 {
			summary.AvgLatencyMs = acc.latencySum / acc.latencyCount
		}
		if acc.ttftCount > 0 {
			summary.AvgTtftMs = acc.ttftSum / acc.ttftCount
		}
		bucketTimestamps := make([]int64, 0, len(acc.series))
		for ts := range acc.series {
			bucketTimestamps = append(bucketTimestamps, ts)
		}
		sort.Slice(bucketTimestamps, func(i, j int) bool { return bucketTimestamps[i] < bucketTimestamps[j] })
		for _, ts := range bucketTimestamps {
			bucket := acc.series[ts]
			point := perfmetrics.ProbeSeriesPoint{
				Ts:           ts,
				ProbeCount:   bucket.probeCount,
				SuccessCount: bucket.successCount,
			}
			if bucket.probeCount > 0 {
				point.SuccessRate = float64(bucket.successCount) * 100 / float64(bucket.probeCount)
			}
			if bucket.latencyCount > 0 {
				point.AvgLatencyMs = bucket.latencySum / bucket.latencyCount
			}
			if bucket.ttftCount > 0 {
				point.AvgTtftMs = bucket.ttftSum / bucket.ttftCount
			}
			summary.Series = append(summary.Series, point)
		}
		result[group] = summary
	}
	return result, nil
}
