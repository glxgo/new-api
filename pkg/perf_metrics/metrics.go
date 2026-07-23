package perfmetrics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

var hotBuckets sync.Map
var channelHotBuckets sync.Map
var channelHealthDedup sync.Map

// seriesSchema is a stable client cache/schema marker. Do not change it when
// hiding fields or making response-only privacy hardening changes.
const seriesSchema = "dbcd0a3c01b55203"

func Init() {
	go flushLoop()
}

func RecordRelaySample(info *relaycommon.RelayInfo, success bool, outputTokens, cacheTokens, promptTokens int64) {
	RecordRelaySampleWithHealthKey(info, success, outputTokens, cacheTokens, promptTokens, "")
}

func RecordRelaySampleWithHealthKey(info *relaycommon.RelayInfo, success bool, outputTokens, cacheTokens, promptTokens int64, healthKey string) {
	if info == nil {
		return
	}
	now := time.Now()
	hasTtft := info.IsStream && info.HasSendResponse()
	// 起点用"发给上游的时刻"；未到上游(本地鉴权失败等)时回退到请求进入站点的时间
	startTime := info.UpstreamStartTime
	if startTime.IsZero() {
		startTime = info.StartTime
	}
	ttftMs := int64(0)
	if hasTtft {
		ttftMs = info.FirstResponseTime.Sub(startTime).Milliseconds()
	}
	latencyMs := now.Sub(startTime).Milliseconds()
	generationMs := latencyMs
	if hasTtft {
		generationMs = now.Sub(info.FirstResponseTime).Milliseconds()
	}
	if generationMs <= 0 {
		generationMs = latencyMs
	}
	channelId := 0
	if info.ChannelMeta != nil {
		channelId = info.ChannelMeta.ChannelId
	}
	Record(Sample{
		Model:        info.OriginModelName,
		Group:        info.UsingGroup,
		ChannelId:    channelId,
		HealthKey:    healthKey,
		LatencyMs:    latencyMs,
		TtftMs:       ttftMs,
		HasTtft:      hasTtft,
		Success:      success,
		OutputTokens: outputTokens,
		GenerationMs: generationMs,
		CacheTokens:  cacheTokens,
		PromptTokens: promptTokens,
	})
}

func Record(sample Sample) {
	setting := perf_metrics_setting.GetSetting()
	if !setting.Enabled || sample.Model == "" {
		return
	}
	if sample.Group == "" {
		sample.Group = "default"
	}
	if sample.LatencyMs < 0 {
		sample.LatencyMs = 0
	}

	key := bucketKey{
		model:    sample.Model,
		group:    sample.Group,
		bucketTs: bucketStart(time.Now().Unix()),
	}
	actual, _ := hotBuckets.LoadOrStore(key, &atomicBucket{})
	actual.(*atomicBucket).add(sample)
	recordRedis(key, sample)
	if sample.ChannelId > 0 && shouldRecordChannelHealthSample(sample, key.bucketTs) {
		channelKey := channelBucketKey{
			model:     sample.Model,
			channelId: sample.ChannelId,
			bucketTs:  key.bucketTs,
		}
		channelActual, _ := channelHotBuckets.LoadOrStore(channelKey, &atomicBucket{})
		channelActual.(*atomicBucket).add(sample)
	}
}

func shouldRecordChannelHealthSample(sample Sample, bucketTs int64) bool {
	healthKey := strings.TrimSpace(sample.HealthKey)
	if healthKey == "" {
		return true
	}
	key := channelHealthDedupKey{
		model:     sample.Model,
		bucketTs:  bucketTs,
		healthKey: healthKey,
		success:   sample.Success,
	}
	_, loaded := channelHealthDedup.LoadOrStore(key, struct{}{})
	return !loaded
}

func cleanupChannelHealthDedupBefore(bucketTs int64) {
	channelHealthDedup.Range(func(rawKey, _ any) bool {
		key := rawKey.(channelHealthDedupKey)
		if key.bucketTs < bucketTs {
			channelHealthDedup.Delete(rawKey)
		}
		return true
	})
}

func Query(params QueryParams) (QueryResult, error) {
	if params.Hours <= 0 {
		params.Hours = 24
	}
	// 桶数 = 时间窗口 / 桶大小(原来误用 hours 当 limit, 导致 5min 桶也只能显示 24 根)
	bucketSeconds := perf_metrics_setting.GetBucketSeconds()
	if bucketSeconds <= 0 {
		bucketSeconds = 3600
	}
	limit := int(int64(params.Hours) * 3600 / bucketSeconds)
	if limit > 1000 {
		limit = 1000
	}

	// B 方案：取该 model 最近 limit 个有数据的 bucket_ts（跨多天凑满，不固定 24h 时间窗口）。
	// 这样即使流量稀疏（冷门模型），也能展示满 limit 根连续绿柱，不会因某些小时无请求而出现空柱。
	bucketTsList, err := model.GetRecentPerfBucketTs(params.Model, limit)
	if err != nil {
		return QueryResult{}, err
	}
	if len(bucketTsList) == 0 {
		return QueryResult{ModelName: params.Model, SeriesSchema: seriesSchema}, nil
	}

	rows, err := model.GetPerfMetricsByBucketTs(params.Model, params.Group, bucketTsList)
	if err != nil {
		return QueryResult{}, err
	}

	merged := map[bucketKey]counters{}
	for _, row := range rows {
		mergeCounters(merged, bucketKey{
			model:    row.ModelName,
			group:    row.Group,
			bucketTs: row.BucketTs,
		}, counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			ttftSumMs:      row.TtftSumMs,
			ttftCount:      row.TtftCount,
			outputTokens:   row.OutputTokens,
			generationMs:   row.GenerationMs,
			cacheTokens:    row.CacheTokens,
			promptTokens:   row.PromptTokens,
		})
	}

	// 热桶（当前小时实时数据）：仅合并 bucketTs 在已选 bucket 列表内的热桶
	tsSet := make(map[int64]bool, len(bucketTsList))
	for _, ts := range bucketTsList {
		tsSet[ts] = true
	}
	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.model != params.Model || !tsSet[k.bucketTs] {
			return true
		}
		if params.Group != "" && k.group != params.Group {
			return true
		}
		mergeCounters(merged, k, value.(*atomicBucket).snapshot())
		return true
	})

	return buildQueryResult(params.Model, merged), nil
}

func QuerySummaryAll(hours int, groups []string) (SummaryAllResult, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours > 24*30 {
		hours = 24 * 30
	}
	endTs := time.Now().Unix()
	startTs := endTs - int64(hours)*3600
	allowedGroups := allowedGroupSet(groups)

	rows, err := model.GetPerfMetricsSummaryAll(startTs, endTs, groups)
	if err != nil {
		return SummaryAllResult{}, err
	}

	totals := map[string]counters{}
	for _, row := range rows {
		totals[row.ModelName] = counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			outputTokens:   row.OutputTokens,
			generationMs:   row.GenerationMs,
			cacheTokens:    row.CacheTokens,
			promptTokens:   row.PromptTokens,
		}
	}

	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs < startTs || k.bucketTs > endTs {
			return true
		}
		if allowedGroups != nil {
			if _, ok := allowedGroups[k.group]; !ok {
				return true
			}
		}
		snap := value.(*atomicBucket).snapshot()
		if snap.requestCount == 0 {
			return true
		}
		cur := totals[k.model]
		cur.requestCount += snap.requestCount
		cur.successCount += snap.successCount
		cur.totalLatencyMs += snap.totalLatencyMs
		cur.outputTokens += snap.outputTokens
		cur.generationMs += snap.generationMs
		cur.cacheTokens += snap.cacheTokens
		cur.promptTokens += snap.promptTokens
		totals[k.model] = cur
		return true
	})

	models := make([]ModelSummary, 0, len(totals))
	for name, total := range totals {
		if total.requestCount == 0 {
			continue
		}
		avgLatency := avg(total.totalLatencyMs, total.successCount)
		successRate := float64(total.successCount) / float64(total.requestCount) * 100
		avgTps := 0.0
		if total.generationMs > 0 {
			avgTps = float64(total.outputTokens) / (float64(total.generationMs) / 1000.0)
		}
		models = append(models, ModelSummary{
			ModelName:    name,
			AvgLatencyMs: avgLatency,
			SuccessRate:  math.Round(successRate*100) / 100,
			AvgTps:       math.Round(avgTps*100) / 100,
			CacheRate:    math.Round(cacheRate(total)*100) / 100,
			RequestCount: total.requestCount,
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].RequestCount > models[j].RequestCount
	})

	return SummaryAllResult{Models: models}, nil
}

// QueryGroupSummaryAll aggregates every model into one health timeline per
// user-facing group. The response remains backward compatible with the cache
// summary consumer while exposing latency, TTFT, TPS and availability.
func QueryGroupSummaryAll(hours int, groups []string) (GroupSummaryAllResult, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours > 24*30 {
		hours = 24 * 30
	}
	endTs := time.Now().Unix()
	startTs := endTs - int64(hours)*3600
	allowedGroups := allowedGroupSet(groups)

	rows, err := model.GetPerfMetricsGroupBuckets(startTs, endTs, groups)
	if err != nil {
		return GroupSummaryAllResult{}, err
	}

	merged := map[bucketKey]counters{}
	for _, row := range rows {
		mergeCounters(merged, bucketKey{group: row.Group, bucketTs: row.BucketTs}, counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			ttftSumMs:      row.TtftSumMs,
			ttftCount:      row.TtftCount,
			outputTokens:   row.OutputTokens,
			generationMs:   row.GenerationMs,
			cacheTokens:    row.CacheTokens,
			promptTokens:   row.PromptTokens,
		})
	}

	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs < startTs || k.bucketTs > endTs {
			return true
		}
		if allowedGroups != nil {
			if _, ok := allowedGroups[k.group]; !ok {
				return true
			}
		}
		snap := value.(*atomicBucket).snapshot()
		if snap.requestCount == 0 {
			return true
		}
		mergeCounters(merged, bucketKey{group: k.group, bucketTs: k.bucketTs}, snap)
		return true
	})

	result := buildGroupSummaryResult(merged)
	result.AvailableGroups = append([]string(nil), groups...)
	sort.Strings(result.AvailableGroups)
	return result, nil
}

// QueryGroupSummaryByChannels projects real relay measurements from final
// upstream channels onto every group/model ability that can use those
// channels. This means traffic submitted through one group can keep another
// group's status current when both groups share the same channel.
func QueryGroupSummaryByChannels(hours int, scopes []GroupChannelScope) (GroupSummaryAllResult, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours > 24*30 {
		hours = 24 * 30
	}
	endTs := time.Now().Unix()
	startTs := endTs - int64(hours)*3600
	channelIds := channelIdsFromScopes(scopes)

	rows, err := model.GetChannelPerfMetrics(startTs, endTs, channelIds)
	if err != nil {
		return GroupSummaryAllResult{}, err
	}
	groups := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if scope.Group != "" {
			groups = append(groups, scope.Group)
		}
	}
	legacyRows, err := model.GetPerfMetricsGroupBuckets(startTs, endTs, groups)
	if err != nil {
		return GroupSummaryAllResult{}, err
	}

	hot := make(map[channelBucketKey]counters)
	allowedChannels := make(map[int]struct{}, len(channelIds))
	for _, channelId := range channelIds {
		allowedChannels[channelId] = struct{}{}
	}
	channelHotBuckets.Range(func(key, value any) bool {
		k := key.(channelBucketKey)
		if k.bucketTs < startTs || k.bucketTs > endTs {
			return true
		}
		if _, ok := allowedChannels[k.channelId]; !ok {
			return true
		}
		snap := value.(*atomicBucket).snapshot()
		if snap.requestCount > 0 {
			hot[k] = snap
		}
		return true
	})
	return buildChannelScopedGroupSummary(scopes, rows, hot, legacyRows), nil
}

func channelIdsFromScopes(scopes []GroupChannelScope) []int {
	channelSet := make(map[int]struct{})
	for _, scope := range scopes {
		for _, channelIds := range scope.ModelChannels {
			for _, channelId := range channelIds {
				if channelId > 0 {
					channelSet[channelId] = struct{}{}
				}
			}
		}
	}
	channelIds := make([]int, 0, len(channelSet))
	for channelId := range channelSet {
		channelIds = append(channelIds, channelId)
	}
	sort.Ints(channelIds)
	return channelIds
}

func buildChannelScopedGroupSummary(
	scopes []GroupChannelScope,
	rows []model.ChannelPerfMetric,
	hot map[channelBucketKey]counters,
	legacyRows []model.PerfMetricGroupBucket,
) GroupSummaryAllResult {
	type modelChannelKey struct {
		model     string
		channelId int
	}
	groupsByModelChannel := make(map[modelChannelKey][]string)
	availableSet := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if scope.Group == "" {
			continue
		}
		availableSet[scope.Group] = struct{}{}
		for modelName, channelIds := range scope.ModelChannels {
			seenChannels := make(map[int]struct{}, len(channelIds))
			for _, channelId := range channelIds {
				if channelId <= 0 {
					continue
				}
				if _, duplicate := seenChannels[channelId]; duplicate {
					continue
				}
				seenChannels[channelId] = struct{}{}
				key := modelChannelKey{model: modelName, channelId: channelId}
				if !containsString(groupsByModelChannel[key], scope.Group) {
					groupsByModelChannel[key] = append(groupsByModelChannel[key], scope.Group)
				}
			}
		}
	}

	merged := make(map[bucketKey]counters)
	channelCutoverTs := int64(0)
	mergeIntoGroups := func(modelName string, channelId int, bucketTs int64, value counters) {
		groups := groupsByModelChannel[modelChannelKey{model: modelName, channelId: channelId}]
		if len(groups) == 0 {
			return
		}
		if channelCutoverTs == 0 || bucketTs < channelCutoverTs {
			channelCutoverTs = bucketTs
		}
		for _, group := range groups {
			mergeCounters(merged, bucketKey{group: group, bucketTs: bucketTs}, value)
		}
	}
	for _, row := range rows {
		mergeIntoGroups(row.ModelName, row.ChannelId, row.BucketTs, counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			ttftSumMs:      row.TtftSumMs,
			ttftCount:      row.TtftCount,
			outputTokens:   row.OutputTokens,
			generationMs:   row.GenerationMs,
			cacheTokens:    row.CacheTokens,
			promptTokens:   row.PromptTokens,
		})
	}
	for key, value := range hot {
		mergeIntoGroups(key.model, key.channelId, key.bucketTs, value)
	}
	// Channel metrics only exist after the channel-scoped rollout. Preserve the
	// legacy group history before that first channel bucket, but never project
	// it to other groups because the original channel identity was not stored.
	// Buckets at/after the cutover use channel data exclusively to avoid double
	// counting and to keep disabled/unmonitored channels out of public health.
	for _, row := range legacyRows {
		if _, visible := availableSet[row.Group]; !visible {
			continue
		}
		if channelCutoverTs > 0 && row.BucketTs >= channelCutoverTs {
			continue
		}
		mergeCounters(merged, bucketKey{group: row.Group, bucketTs: row.BucketTs}, counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			ttftSumMs:      row.TtftSumMs,
			ttftCount:      row.TtftCount,
			outputTokens:   row.OutputTokens,
			generationMs:   row.GenerationMs,
			cacheTokens:    row.CacheTokens,
			promptTokens:   row.PromptTokens,
		})
	}

	result := buildGroupSummaryResult(merged)
	result.AvailableGroups = make([]string, 0, len(availableSet))
	for group := range availableSet {
		result.AvailableGroups = append(result.AvailableGroups, group)
	}
	sort.Strings(result.AvailableGroups)
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func buildGroupSummaryResult(merged map[bucketKey]counters) GroupSummaryAllResult {
	groupBuckets := map[string]map[int64]counters{}
	for key, value := range merged {
		if value.requestCount == 0 {
			continue
		}
		if groupBuckets[key.group] == nil {
			groupBuckets[key.group] = map[int64]counters{}
		}
		groupBuckets[key.group][key.bucketTs] = value
	}

	groupList := make([]GroupCacheSummary, 0, len(groupBuckets))
	for name, buckets := range groupBuckets {
		timestamps := make([]int64, 0, len(buckets))
		for ts := range buckets {
			timestamps = append(timestamps, ts)
		}
		sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
		total := counters{}
		series := make([]BucketPoint, 0, len(timestamps))
		for _, ts := range timestamps {
			value := buckets[ts]
			total.requestCount += value.requestCount
			total.successCount += value.successCount
			total.totalLatencyMs += value.totalLatencyMs
			total.ttftSumMs += value.ttftSumMs
			total.ttftCount += value.ttftCount
			total.outputTokens += value.outputTokens
			total.generationMs += value.generationMs
			total.cacheTokens += value.cacheTokens
			total.promptTokens += value.promptTokens
			series = append(series, bucketPoint(ts, value))
		}
		groupList = append(groupList, GroupCacheSummary{
			Group:        name,
			AvgTtftMs:    avg(total.ttftSumMs, total.ttftCount),
			AvgLatencyMs: avg(total.totalLatencyMs, total.successCount),
			SuccessRate:  successRate(total),
			AvgTps:       avgTps(total),
			CacheRate:    cacheRate(total),
			RequestCount: total.requestCount,
			SuccessCount: total.successCount,
			CacheTokens:  total.cacheTokens,
			PromptTokens: total.promptTokens,
			Series:       series,
		})
	}
	sort.Slice(groupList, func(i, j int) bool {
		return groupList[i].RequestCount > groupList[j].RequestCount
	})

	return GroupSummaryAllResult{Groups: groupList}
}

func allowedGroupSet(groups []string) map[string]struct{} {
	if groups == nil {
		return nil
	}
	allowed := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		allowed[group] = struct{}{}
	}
	return allowed
}

func bucketStart(ts int64) int64 {
	bucketSeconds := perf_metrics_setting.GetBucketSeconds()
	if bucketSeconds <= 0 {
		bucketSeconds = 3600
	}
	return ts - (ts % bucketSeconds)
}

func mergeCounters(merged map[bucketKey]counters, key bucketKey, value counters) {
	if value.requestCount == 0 {
		return
	}
	current := merged[key]
	current.requestCount += value.requestCount
	current.successCount += value.successCount
	current.totalLatencyMs += value.totalLatencyMs
	current.ttftSumMs += value.ttftSumMs
	current.ttftCount += value.ttftCount
	current.outputTokens += value.outputTokens
	current.generationMs += value.generationMs
	current.cacheTokens += value.cacheTokens
	current.promptTokens += value.promptTokens
	merged[key] = current
}

func buildQueryResult(modelName string, merged map[bucketKey]counters) QueryResult {
	groupBuckets := map[string]map[int64]counters{}
	for key, value := range merged {
		if value.requestCount == 0 {
			continue
		}
		if _, ok := groupBuckets[key.group]; !ok {
			groupBuckets[key.group] = map[int64]counters{}
		}
		groupBuckets[key.group][key.bucketTs] = value
	}

	groups := make([]string, 0, len(groupBuckets))
	for group := range groupBuckets {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	results := make([]GroupResult, 0, len(groups))
	for _, group := range groups {
		buckets := groupBuckets[group]
		timestamps := make([]int64, 0, len(buckets))
		for ts := range buckets {
			timestamps = append(timestamps, ts)
		}
		sort.Slice(timestamps, func(i, j int) bool {
			return timestamps[i] < timestamps[j]
		})

		total := counters{}
		series := make([]BucketPoint, 0, len(timestamps))
		for _, ts := range timestamps {
			value := buckets[ts]
			total.requestCount += value.requestCount
			total.successCount += value.successCount
			total.totalLatencyMs += value.totalLatencyMs
			total.ttftSumMs += value.ttftSumMs
			total.ttftCount += value.ttftCount
			total.outputTokens += value.outputTokens
			total.generationMs += value.generationMs
			total.cacheTokens += value.cacheTokens
			total.promptTokens += value.promptTokens
			series = append(series, bucketPoint(ts, value))
		}

		results = append(results, GroupResult{
			Group:        group,
			AvgTtftMs:    avg(total.ttftSumMs, total.ttftCount),
			AvgLatencyMs: avg(total.totalLatencyMs, total.successCount),
			SuccessRate:  successRate(total),
			AvgTps:       avgTps(total),
			CacheRate:    cacheRate(total),
			Series:       series,
		})
	}

	return QueryResult{
		ModelName:    modelName,
		SeriesSchema: seriesSchema,
		Groups:       results,
	}
}

func bucketPoint(ts int64, value counters) BucketPoint {
	return BucketPoint{
		Ts:           ts,
		RequestCount: value.requestCount,
		SuccessCount: value.successCount,
		AvgTtftMs:    avg(value.ttftSumMs, value.ttftCount),
		AvgLatencyMs: avg(value.totalLatencyMs, value.successCount),
		SuccessRate:  successRate(value),
		AvgTps:       avgTps(value),
		CacheRate:    cacheRate(value),
	}
}

// cacheRate = 命中缓存 token / 总输入 token × 100
// 大多数上游 promptTokens 是总输入(已含缓存命中), 直接作分母。
// 少数渠道会把命中缓存单独给, promptTokens 只含非缓存输入；当 cache > prompt 时，
// 用 prompt+cache 重建总输入，避免 100% 或偏差。
func cacheRate(value counters) float64 {
	if value.promptTokens <= 0 {
		return 0
	}
	denom := value.promptTokens
	if value.cacheTokens > value.promptTokens {
		denom = value.promptTokens + value.cacheTokens
	}
	if denom <= 0 {
		return 0
	}
	return float64(value.cacheTokens) / float64(denom) * 100
}

func avg(sum int64, count int64) int64 {
	if count <= 0 {
		return 0
	}
	return sum / count
}

func successRate(value counters) float64 {
	if value.requestCount <= 0 {
		return 0
	}
	return float64(value.successCount) / float64(value.requestCount) * 100
}

func avgTps(value counters) float64 {
	if value.outputTokens <= 0 || value.generationMs <= 0 {
		return 0
	}
	return float64(value.outputTokens) / (float64(value.generationMs) / 1000)
}

func recordRedis(key bucketKey, sample Sample) {
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	redisKey := redisBucketKey(key)
	pipe := common.RDB.TxPipeline()
	pipe.HIncrBy(ctx, redisKey, "req", 1)
	if sample.Success {
		pipe.HIncrBy(ctx, redisKey, "ok", 1)
	}
	if sample.Success && sample.LatencyMs > 0 {
		pipe.HIncrBy(ctx, redisKey, "lat", sample.LatencyMs)
	}
	if sample.HasTtft && sample.TtftMs >= 0 {
		pipe.HIncrBy(ctx, redisKey, "ttft", sample.TtftMs)
		pipe.HIncrBy(ctx, redisKey, "ttft_n", 1)
	}
	if sample.OutputTokens > 0 && sample.GenerationMs > 0 {
		pipe.HIncrBy(ctx, redisKey, "out", sample.OutputTokens)
		pipe.HIncrBy(ctx, redisKey, "gen_ms", sample.GenerationMs)
	}
	if sample.CacheTokens > 0 {
		pipe.HIncrBy(ctx, redisKey, "cache", sample.CacheTokens)
	}
	if sample.PromptTokens > 0 {
		pipe.HIncrBy(ctx, redisKey, "prompt", sample.PromptTokens)
	}
	pipe.Expire(ctx, redisKey, time.Hour)
	_, _ = pipe.Exec(ctx)
}

func mergeRedisActiveBuckets(merged map[bucketKey]counters, params QueryParams, startTs int64, endTs int64) {
	if !common.RedisEnabled || common.RDB == nil || params.Model == "" || params.Group == "" {
		return
	}
	active := bucketStart(time.Now().Unix())
	if active < startTs || active > endTs {
		return
	}
	key := bucketKey{model: params.Model, group: params.Group, bucketTs: active}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	values, err := common.RDB.HGetAll(ctx, redisBucketKey(key)).Result()
	if err != nil || len(values) == 0 {
		return
	}
	mergeCounters(merged, key, redisCounters(values))
}

func redisBucketKey(key bucketKey) string {
	return fmt.Sprintf("perf:%s:%s:%d", key.model, key.group, key.bucketTs)
}
