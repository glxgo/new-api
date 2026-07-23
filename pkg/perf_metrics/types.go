package perfmetrics

import "sync/atomic"

type Store interface {
	Record(sample Sample)
	Query(params QueryParams) (QueryResult, error)
}

type Sample struct {
	Model        string
	Group        string
	ChannelId    int
	HealthKey    string
	LatencyMs    int64
	TtftMs       int64
	HasTtft      bool
	Success      bool
	OutputTokens int64
	GenerationMs int64
	CacheTokens  int64 // prompt cache 命中 token
	PromptTokens int64 // 输入 token（未命中缓存部分）
}

type QueryParams struct {
	Model string
	Group string
	Hours int
}

type BucketPoint struct {
	Ts           int64   `json:"ts"`
	RequestCount int64   `json:"request_count"`
	SuccessCount int64   `json:"success_count"`
	AvgTtftMs    int64   `json:"avg_ttft_ms"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
	CacheRate    float64 `json:"cache_rate"`
}

type GroupResult struct {
	Group        string        `json:"group"`
	AvgTtftMs    int64         `json:"avg_ttft_ms"`
	AvgLatencyMs int64         `json:"avg_latency_ms"`
	SuccessRate  float64       `json:"success_rate"`
	AvgTps       float64       `json:"avg_tps"`
	CacheRate    float64       `json:"cache_rate"`
	Series       []BucketPoint `json:"series"`
}

type QueryResult struct {
	ModelName    string        `json:"model_name"`
	SeriesSchema string        `json:"series_schema"`
	Groups       []GroupResult `json:"groups"`
}

type ModelSummary struct {
	ModelName    string  `json:"model_name"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
	CacheRate    float64 `json:"cache_rate"`
	RequestCount int64   `json:"-"`
}

type SummaryAllResult struct {
	Models []ModelSummary `json:"models"`
}

// GroupCacheSummary keeps its historical name for API compatibility, but now
// carries the complete group-level health summary used by both dashboard and
// model-status views.
type GroupCacheSummary struct {
	Group        string             `json:"group"`
	AvgTtftMs    int64              `json:"avg_ttft_ms"`
	AvgLatencyMs int64              `json:"avg_latency_ms"`
	SuccessRate  float64            `json:"success_rate"`
	AvgTps       float64            `json:"avg_tps"`
	CacheRate    float64            `json:"cache_rate"`
	RequestCount int64              `json:"request_count"`
	SuccessCount int64              `json:"success_count"`
	CacheTokens  int64              `json:"cache_tokens"`
	PromptTokens int64              `json:"prompt_tokens"`
	Series       []BucketPoint      `json:"series"`
	Probe        *GroupProbeSummary `json:"probe,omitempty"`
}

type ProbeSeriesPoint struct {
	Ts           int64   `json:"ts"`
	ProbeCount   int64   `json:"probe_count"`
	SuccessCount int64   `json:"success_count"`
	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	AvgTtftMs    int64   `json:"avg_ttft_ms"`
}

type GroupProbeSummary struct {
	Status            string             `json:"status"`
	TotalChannels     int                `json:"total_channels"`
	CheckedChannels   int                `json:"checked_channels"`
	HealthyChannels   int                `json:"healthy_channels"`
	DegradedChannels  int                `json:"degraded_channels"`
	UnhealthyChannels int                `json:"unhealthy_channels"`
	SuccessRate       float64            `json:"success_rate"`
	AvgLatencyMs      int64              `json:"avg_latency_ms"`
	AvgTtftMs         int64              `json:"avg_ttft_ms"`
	LastProbeTs       int64              `json:"last_probe_ts"`
	LastSuccessTs     int64              `json:"last_success_ts"`
	LastErrorCategory string             `json:"last_error_category,omitempty"`
	LastErrorCode     string             `json:"last_error_code,omitempty"`
	Series            []ProbeSeriesPoint `json:"series"`
}

type GroupSummaryAllResult struct {
	Groups          []GroupCacheSummary `json:"groups"`
	AvailableGroups []string            `json:"available_groups,omitempty"`
}

// GroupChannelScope describes which final channels may serve each public model
// in a user-facing group. A channel may intentionally appear in several scopes
// when groups share the same upstream capacity.
type GroupChannelScope struct {
	Group         string
	ModelChannels map[string][]int
}

type bucketKey struct {
	model    string
	group    string
	bucketTs int64
}

type channelBucketKey struct {
	model     string
	channelId int
	bucketTs  int64
}

type channelHealthDedupKey struct {
	model     string
	bucketTs  int64
	healthKey string
	success   bool
}

type counters struct {
	requestCount   int64
	successCount   int64
	totalLatencyMs int64
	ttftSumMs      int64
	ttftCount      int64
	outputTokens   int64
	generationMs   int64
	cacheTokens    int64
	promptTokens   int64
}

type atomicBucket struct {
	requestCount   atomic.Int64
	successCount   atomic.Int64
	totalLatencyMs atomic.Int64
	ttftSumMs      atomic.Int64
	ttftCount      atomic.Int64
	outputTokens   atomic.Int64
	generationMs   atomic.Int64
	cacheTokens    atomic.Int64
	promptTokens   atomic.Int64
}

func (b *atomicBucket) add(sample Sample) {
	b.requestCount.Add(1)
	if sample.Success {
		b.successCount.Add(1)
	}
	if sample.Success && sample.LatencyMs > 0 {
		b.totalLatencyMs.Add(sample.LatencyMs)
	}
	if sample.HasTtft && sample.TtftMs >= 0 {
		b.ttftSumMs.Add(sample.TtftMs)
		b.ttftCount.Add(1)
	}
	if sample.OutputTokens > 0 && sample.GenerationMs > 0 {
		b.outputTokens.Add(sample.OutputTokens)
		b.generationMs.Add(sample.GenerationMs)
	}
	if sample.CacheTokens > 0 {
		b.cacheTokens.Add(sample.CacheTokens)
	}
	if sample.PromptTokens > 0 {
		b.promptTokens.Add(sample.PromptTokens)
	}
}

func (b *atomicBucket) snapshot() counters {
	return counters{
		requestCount:   b.requestCount.Load(),
		successCount:   b.successCount.Load(),
		totalLatencyMs: b.totalLatencyMs.Load(),
		ttftSumMs:      b.ttftSumMs.Load(),
		ttftCount:      b.ttftCount.Load(),
		outputTokens:   b.outputTokens.Load(),
		generationMs:   b.generationMs.Load(),
		cacheTokens:    b.cacheTokens.Load(),
		promptTokens:   b.promptTokens.Load(),
	}
}

func (b *atomicBucket) drain() counters {
	return counters{
		requestCount:   b.requestCount.Swap(0),
		successCount:   b.successCount.Swap(0),
		totalLatencyMs: b.totalLatencyMs.Swap(0),
		ttftSumMs:      b.ttftSumMs.Swap(0),
		ttftCount:      b.ttftCount.Swap(0),
		outputTokens:   b.outputTokens.Swap(0),
		generationMs:   b.generationMs.Swap(0),
		cacheTokens:    b.cacheTokens.Swap(0),
		promptTokens:   b.promptTokens.Swap(0),
	}
}

func (b *atomicBucket) addCounters(c counters) {
	if c.requestCount != 0 {
		b.requestCount.Add(c.requestCount)
	}
	if c.successCount != 0 {
		b.successCount.Add(c.successCount)
	}
	if c.totalLatencyMs != 0 {
		b.totalLatencyMs.Add(c.totalLatencyMs)
	}
	if c.ttftSumMs != 0 {
		b.ttftSumMs.Add(c.ttftSumMs)
	}
	if c.ttftCount != 0 {
		b.ttftCount.Add(c.ttftCount)
	}
	if c.outputTokens != 0 {
		b.outputTokens.Add(c.outputTokens)
	}
	if c.generationMs != 0 {
		b.generationMs.Add(c.generationMs)
	}
	if c.cacheTokens != 0 {
		b.cacheTokens.Add(c.cacheTokens)
	}
	if c.promptTokens != 0 {
		b.promptTokens.Add(c.promptTokens)
	}
}
