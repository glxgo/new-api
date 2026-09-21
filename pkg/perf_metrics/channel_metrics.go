package perfmetrics

import (
	"context"

	"github.com/QuantumNous/new-api/model"
)

type ChannelSummary struct {
	RequestCount     int64    `json:"request_count"`
	SuccessCount     int64    `json:"success_count"`
	SuccessRate      *float64 `json:"success_rate"`
	CacheRate        *float64 `json:"cache_rate"`
	AvgTtftMs        *float64 `json:"avg_ttft_ms"`
	TtftCount        int64    `json:"ttft_count"`
	CacheTokens      int64    `json:"cache_tokens"`
	PromptTokens     int64    `json:"prompt_tokens"`
	LegacyResolution bool     `json:"legacy_resolution"`
}

// QueryChannelSummaries uses an exclusive end aligned to a complete minute.
// Persisted data from every instance plus this instance's unflushed buckets.
func QueryChannelSummaries(ctx context.Context, start, end int64, ids []int) (map[int]ChannelSummary, error) {
	channelMetricsMu.RLock()
	defer channelMetricsMu.RUnlock()
	rows, err := model.GetChannelMetricTotals(ctx, start, end, ids)
	if err != nil {
		return nil, err
	}
	merged := make(map[int]counters, len(ids))
	legacy := make(map[int]bool)
	for _, id := range ids {
		merged[id] = counters{}
	}
	for _, row := range rows {
		merged[row.ChannelId] = counters{requestCount: row.RequestCount, successCount: row.SuccessCount, ttftSumMs: row.TtftSumMs, ttftCount: row.TtftCount, cacheTokens: row.CacheTokens, promptTokens: row.PromptTokens}
		legacy[row.ChannelId] = row.LegacyBuckets > 0
	}
	channelHotBuckets.Range(func(key, value any) bool {
		k := key.(channelBucketKey)
		total, allowed := merged[k.channelId]
		if !allowed || k.bucketTs < start || k.bucketTs >= end {
			return true
		}
		s := value.(*atomicBucket).snapshot()
		total.requestCount += s.requestCount
		total.successCount += s.successCount
		total.ttftSumMs += s.ttftSumMs
		total.ttftCount += s.ttftCount
		total.cacheTokens += s.cacheTokens
		total.promptTokens += s.promptTokens
		merged[k.channelId] = total
		return true
	})
	result := make(map[int]ChannelSummary, len(ids))
	for id, v := range merged {
		s := ChannelSummary{RequestCount: v.requestCount, SuccessCount: v.successCount, TtftCount: v.ttftCount, CacheTokens: v.cacheTokens, PromptTokens: v.promptTokens, LegacyResolution: legacy[id]}
		if v.requestCount > 0 {
			rate := successRate(v)
			s.SuccessRate = &rate
		}
		if v.promptTokens > 0 {
			rate := cacheRate(v)
			s.CacheRate = &rate
		}
		if v.ttftCount > 0 {
			ttft := float64(v.ttftSumMs) / float64(v.ttftCount)
			s.AvgTtftMs = &ttft
		}
		result[id] = s
	}
	return result, ctx.Err()
}
