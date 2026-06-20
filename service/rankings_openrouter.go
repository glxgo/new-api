package service

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	openRouterRankingsBaseURL          = "https://openrouter.ai/api/frontend/v1/rankings"
	openRouterRankingsRequestTimeout   = 20 * time.Second
	openRouterRankingsPerformanceLimit = 8
	openRouterRankingsBenchmarkLimit   = 5
	openRouterRankingsUserAgent        = "NewAPI-Rankings/1.0"
)

type openRouterModelsResponse struct {
	Data []openRouterModelRow `json:"data"`
}

type openRouterModelRow struct {
	Date                       string   `json:"date"`
	ModelPermaslug             string   `json:"model_permaslug"`
	Variant                    string   `json:"variant"`
	VariantPermaslug           string   `json:"variant_permaslug"`
	TotalCompletionTokens      int64    `json:"total_completion_tokens"`
	TotalPromptTokens          int64    `json:"total_prompt_tokens"`
	TotalNativeTokensReasoning int64    `json:"total_native_tokens_reasoning"`
	TotalNativeTokensCached    int64    `json:"total_native_tokens_cached"`
	Count                      int64    `json:"count"`
	Change                     *float64 `json:"change"`
}

type openRouterTimeseriesResponse struct {
	Data openRouterTimeseriesData `json:"data"`
}

type openRouterTimeseriesData struct {
	Data []openRouterTimeseriesPoint `json:"data"`
}

type openRouterMarketShareResponse struct {
	Data []openRouterTimeseriesPoint `json:"data"`
}

type openRouterTimeseriesPoint struct {
	X  string           `json:"x"`
	Ys map[string]int64 `json:"ys"`
}

type openRouterPerformanceResponse struct {
	Data []openRouterPerformanceRow `json:"data"`
}

type openRouterPerformanceRow struct {
	ID            string  `json:"id"`
	Slug          string  `json:"slug"`
	Name          string  `json:"name"`
	Author        string  `json:"author"`
	RequestCount  int64   `json:"request_count"`
	P50Latency    float64 `json:"p50_latency"`
	P50Throughput float64 `json:"p50_throughput"`
	ProviderCount int     `json:"provider_count"`
}

type openRouterBenchmarksResponse struct {
	Data openRouterBenchmarksData `json:"data"`
}

type openRouterBenchmarksData struct {
	AAData map[string][]openRouterBenchmarkRow `json:"aaData"`
}

type openRouterBenchmarkRow struct {
	UID                     string  `json:"uid"`
	Permaslug               string  `json:"permaslug"`
	OpenRouterSlug          string  `json:"openrouter_slug"`
	HeuristicOpenRouterSlug string  `json:"heuristic_openrouter_slug"`
	AAName                  string  `json:"aa_name"`
	Score                   float64 `json:"score"`
}

type openRouterModelAggregate struct {
	slug           string
	displayName    string
	vendor         string
	vendorIcon     string
	totalTokens    int64
	previousTokens int64
	count          int64
	change         *float64
}

type openRouterFetchResult struct {
	models      openRouterModelsResponse
	previous    openRouterModelsResponse
	chart       openRouterTimeseriesResponse
	marketShare openRouterMarketShareResponse
	performance openRouterPerformanceResponse
	benchmarks  openRouterBenchmarksResponse
}

type openRouterModelDetails struct {
	displayName string
	vendor      string
	vendorIcon  string
	requests    int64
}

func buildOpenRouterRankingsSnapshot(config rankingPeriodConfig, now time.Time) (*RankingsResponse, error) {
	view := openRouterModelsView(config)
	ctx, cancel := context.WithTimeout(context.Background(), openRouterRankingsRequestTimeout)
	defer cancel()

	payload, err := fetchOpenRouterRankings(ctx, view)
	if err != nil {
		return nil, err
	}

	return buildOpenRouterRankingsSnapshotFromPayload(config, now, payload), nil
}

func buildOpenRouterRankingsSnapshotFromPayload(config rankingPeriodConfig, now time.Time, payload openRouterFetchResult) *RankingsResponse {
	details := buildOpenRouterModelDetails(payload.performance.Data)

	var rankedModels []RankedModel
	var vendors []RankedVendor
	var modelHistory ModelHistorySeries
	var vendorHistory VendorShareSeries

	currentModelPoints, previousModelPoints := selectOpenRouterPeriodPoints(payload.chart.Data.Data, config, now)
	currentVendorPoints, previousVendorPoints := selectOpenRouterPeriodPoints(payload.marketShare.Data, config, now)

	modelHistoryConfig, err := rankingConfig("year")
	if err != nil {
		modelHistoryConfig = config
	}
	modelHistoryPoints, _ := selectOpenRouterPeriodPoints(payload.chart.Data.Data, modelHistoryConfig, now)
	if len(modelHistoryPoints) == 0 {
		modelHistoryPoints = sortedOpenRouterPoints(payload.chart.Data.Data)
	}
	var periodTotals []model.RankingQuotaTotal
	if openRouterUseModelRowsForPeriod(config) {
		periodTotals, rankedModels = buildOpenRouterRankedModels(payload.models.Data, payload.previous.Data, details)
	}
	if len(rankedModels) == 0 {
		periodTotals, rankedModels = buildOpenRouterRankedModelsFromTimeseries(currentModelPoints, previousModelPoints, details)
	}
	if len(rankedModels) == 0 {
		periodTotals, rankedModels = buildOpenRouterRankedModels(payload.models.Data, payload.previous.Data, details)
	}

	vendors = buildOpenRouterRankedVendorsFromSharePoints(currentVendorPoints, previousVendorPoints)
	if len(vendors) == 0 {
		vendors = buildOpenRouterRankedVendors(rankedModels, periodTotals)
	}
	modelHistory = buildOpenRouterModelHistory(modelHistoryPoints, modelHistoryConfig, details)
	vendorHistory = buildOpenRouterVendorShareHistory(currentVendorPoints, config)
	if len(modelHistory.Points) == 0 {
		modelHistory = buildOpenRouterModelHistory(openRouterModelRowsToTimeseries(payload.models.Data), openRouterRowHistoryConfig(modelHistoryConfig), details)
	}
	if len(vendorHistory.Points) == 0 {
		vendorHistory = buildOpenRouterVendorShareHistory(openRouterModelRowsToVendorTimeseries(payload.models.Data), config)
	}
	movers, droppers := buildRankingMovers(rankedModels)

	return &RankingsResponse{
		Source:             RankingDataSourceOpenRouter,
		Models:             limitRankedModels(rankedModels, rankingLeaderboardLimit),
		Vendors:            vendors,
		TopMovers:          movers,
		TopDroppers:        droppers,
		ModelsHistory:      modelHistory,
		VendorShareHistory: vendorHistory,
		Benchmarks:         buildOpenRouterBenchmarks(payload.benchmarks.Data.AAData),
		Performance:        buildOpenRouterPerformance(payload.performance.Data, details),
	}
}

func fetchOpenRouterRankings(ctx context.Context, view string) (openRouterFetchResult, error) {
	var result openRouterFetchResult
	modelErr := fetchOpenRouterJSON(ctx, fmt.Sprintf("%s/models?view=%s", openRouterRankingsBaseURL, view), &result.models)
	if previousView := openRouterPreviousModelsView(view); previousView != "" {
		_ = fetchOpenRouterJSON(ctx, fmt.Sprintf("%s/models?view=%s", openRouterRankingsBaseURL, previousView), &result.previous)
	}
	if err := fetchOpenRouterJSON(ctx, openRouterRankingsBaseURL+"/model-rankings-chart", &result.chart); err != nil {
		return result, err
	}
	if err := fetchOpenRouterJSON(ctx, openRouterRankingsBaseURL+"/market-share", &result.marketShare); err != nil {
		return result, err
	}
	_ = fetchOpenRouterJSON(ctx, openRouterRankingsBaseURL+"/performance", &result.performance)
	_ = fetchOpenRouterJSON(ctx, openRouterRankingsBaseURL+"/benchmarks", &result.benchmarks)
	if modelErr != nil && len(result.chart.Data.Data) == 0 {
		return result, modelErr
	}
	return result, nil
}

func fetchOpenRouterJSON(ctx context.Context, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", openRouterRankingsUserAgent)

	client := GetHttpClient()
	if client == nil {
		client = &http.Client{Timeout: openRouterRankingsRequestTimeout}
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("fetch OpenRouter rankings failed: %w", ctx.Err())
			case <-time.After(time.Duration(attempt) * 250 * time.Millisecond):
			}
		}

		resp, err := client.Do(req.Clone(ctx))
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			lastErr = fmt.Errorf("unexpected status: %s", resp.Status)
			resp.Body.Close()
			if resp.StatusCode < http.StatusInternalServerError {
				break
			}
			continue
		}
		if err := common.DecodeJson(resp.Body, target); err != nil {
			lastErr = err
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		return nil
	}
	return fmt.Errorf("fetch OpenRouter rankings failed: %w", lastErr)
}

func openRouterModelsView(config rankingPeriodConfig) string {
	switch config.id {
	case "today":
		return "day"
	case "month":
		return "month"
	case "year", "all":
		return "week"
	default:
		return "week"
	}
}

func openRouterPreviousModelsView(view string) string {
	switch view {
	case "day":
		return "week"
	case "week":
		return "month"
	default:
		return ""
	}
}

func openRouterUseModelRowsForPeriod(config rankingPeriodConfig) bool {
	return config.id == "today" || config.id == "week" || config.id == "month"
}

func openRouterRowHistoryConfig(config rankingPeriodConfig) rankingPeriodConfig {
	rowConfig := config
	if openRouterUseModelRowsForPeriod(config) {
		rowConfig.id = config.id + "_rows"
		rowConfig.labelLayout = "Jan 2"
	}
	return rowConfig
}

func openRouterUseTimeseriesForLeaderboard(config rankingPeriodConfig) bool {
	return config.id == "year" || config.id == "all"
}

func selectOpenRouterPeriodPoints(points []openRouterTimeseriesPoint, config rankingPeriodConfig, now time.Time) ([]openRouterTimeseriesPoint, []openRouterTimeseriesPoint) {
	sorted := sortedOpenRouterPoints(points)
	if len(sorted) == 0 {
		return nil, nil
	}
	if config.id == "all" {
		return sorted, nil
	}

	start, end := openRouterPeriodRange(config, now)
	previousStart, previousEnd := openRouterPreviousPeriodRange(config, start)
	current := make([]openRouterTimeseriesPoint, 0)
	previous := make([]openRouterTimeseriesPoint, 0)
	for _, point := range sorted {
		pointTime, ok := openRouterPointTime(point.X)
		if !ok {
			continue
		}
		if !pointTime.Before(start) && pointTime.Before(end) {
			current = append(current, point)
		}
		if !previousStart.IsZero() && !pointTime.Before(previousStart) && pointTime.Before(previousEnd) {
			previous = append(previous, point)
		}
	}
	return current, previous
}

func sortedOpenRouterPoints(points []openRouterTimeseriesPoint) []openRouterTimeseriesPoint {
	sorted := append([]openRouterTimeseriesPoint(nil), points...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, leftOk := openRouterPointTime(sorted[i].X)
		right, rightOk := openRouterPointTime(sorted[j].X)
		if leftOk && rightOk {
			return left.Before(right)
		}
		return sorted[i].X < sorted[j].X
	})
	return sorted
}

func openRouterPeriodRange(config rankingPeriodConfig, now time.Time) (time.Time, time.Time) {
	current := now
	if current.IsZero() {
		current = time.Now()
	}
	current = current.UTC()
	end := current.Add(24 * time.Hour)
	switch config.id {
	case "today":
		return beginningOfDay(current), end
	case "week":
		return beginningOfWeek(current), end
	case "month":
		return time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, time.UTC), end
	case "year":
		return time.Date(current.Year(), time.January, 1, 0, 0, 0, 0, time.UTC), end
	default:
		start, _ := rankingTimeRange(config, current)
		return time.Unix(start, 0).UTC(), end
	}
}

func openRouterPreviousPeriodRange(config rankingPeriodConfig, start time.Time) (time.Time, time.Time) {
	switch config.id {
	case "today":
		return start.AddDate(0, 0, -1), start
	case "week":
		return start.AddDate(0, 0, -7), start
	case "month":
		return start.AddDate(0, -1, 0), start
	case "year":
		return start.AddDate(-1, 0, 0), start
	default:
		if config.duration <= 0 {
			return time.Time{}, time.Time{}
		}
		return start.Add(-config.duration), start
	}
	return time.Time{}, time.Time{}
}

func openRouterPointTime(value string) (time.Time, bool) {
	if ts, err := time.ParseInLocation("2006-01-02", value, time.UTC); err == nil {
		return ts, true
	}
	if ts, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC); err == nil {
		return ts, true
	}
	return time.Time{}, false
}

func buildOpenRouterModelDetails(rows []openRouterPerformanceRow) map[string]openRouterModelDetails {
	details := make(map[string]openRouterModelDetails, len(rows))
	for _, row := range rows {
		slug := firstNonEmpty(row.Slug, row.ID)
		if slug == "" {
			continue
		}
		vendorSlug := firstNonEmpty(row.Author, openRouterVendorSlug(slug))
		details[slug] = openRouterModelDetails{
			displayName: strings.TrimSpace(row.Name),
			vendor:      openRouterVendorDisplayName(vendorSlug),
			vendorIcon:  openRouterVendorIcon(vendorSlug),
			requests:    row.RequestCount,
		}
	}
	return details
}

func buildOpenRouterRankedModels(currentRows []openRouterModelRow, previousRows []openRouterModelRow, details map[string]openRouterModelDetails) ([]model.RankingQuotaTotal, []RankedModel) {
	aggregates := aggregateOpenRouterModelRows(currentRows, details)
	previousAggregates := aggregateOpenRouterModelRows(previousRows, details)
	return buildOpenRouterRankedModelsFromAggregates(aggregates, previousAggregates)
}

func buildOpenRouterRankedModelsFromTimeseries(currentPoints []openRouterTimeseriesPoint, previousPoints []openRouterTimeseriesPoint, details map[string]openRouterModelDetails) ([]model.RankingQuotaTotal, []RankedModel) {
	aggregates := aggregateOpenRouterModelPoints(currentPoints, details)
	previousAggregates := aggregateOpenRouterModelPoints(previousPoints, details)
	return buildOpenRouterRankedModelsFromAggregates(aggregates, previousAggregates)
}

func buildOpenRouterRankedModelsFromAggregates(aggregates map[string]*openRouterModelAggregate, previousAggregates map[string]*openRouterModelAggregate) ([]model.RankingQuotaTotal, []RankedModel) {
	rows := make([]*openRouterModelAggregate, 0, len(aggregates))
	for _, agg := range aggregates {
		normalizeOpenRouterAggregateTokens(agg)
		if prev, ok := previousAggregates[agg.slug]; ok {
			normalizeOpenRouterAggregateTokens(prev)
			agg.previousTokens = prev.totalTokens
		}
		if agg.totalTokens > 0 {
			rows = append(rows, agg)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].totalTokens == rows[j].totalTokens {
			return rows[i].slug < rows[j].slug
		}
		return rows[i].totalTokens > rows[j].totalTokens
	})

	previousRanks := make(map[string]int, len(previousAggregates))
	previousSorted := make([]*openRouterModelAggregate, 0, len(previousAggregates))
	for _, agg := range previousAggregates {
		normalizeOpenRouterAggregateTokens(agg)
		if agg.totalTokens > 0 {
			previousSorted = append(previousSorted, agg)
		}
	}
	sort.Slice(previousSorted, func(i, j int) bool {
		if previousSorted[i].totalTokens == previousSorted[j].totalTokens {
			return previousSorted[i].slug < previousSorted[j].slug
		}
		return previousSorted[i].totalTokens > previousSorted[j].totalTokens
	})
	for idx, agg := range previousSorted {
		previousRanks[agg.slug] = idx + 1
	}

	totalTokens := int64(0)
	for _, agg := range rows {
		totalTokens += agg.totalTokens
	}

	totals := make([]model.RankingQuotaTotal, 0, len(rows))
	ranked := make([]RankedModel, 0, len(rows))
	for idx, agg := range rows {
		var previousRank *int
		if rank, ok := previousRanks[agg.slug]; ok {
			rankCopy := rank
			previousRank = &rankCopy
		}
		growth := 0.0
		if len(previousAggregates) > 0 {
			growth = rankingGrowthPct(agg.totalTokens, agg.previousTokens)
		}
		if agg.change != nil {
			growth = roundRankingFloat(*agg.change * 100)
		}
		totals = append(totals, model.RankingQuotaTotal{
			ModelName:   agg.slug,
			TotalTokens: agg.totalTokens,
		})
		ranked = append(ranked, RankedModel{
			Rank:         idx + 1,
			PreviousRank: previousRank,
			ModelName:    agg.slug,
			DisplayName:  agg.displayName,
			Vendor:       agg.vendor,
			VendorIcon:   agg.vendorIcon,
			Category:     "all",
			TotalTokens:  agg.totalTokens,
			Share:        rankingShare(agg.totalTokens, totalTokens),
			GrowthPct:    growth,
		})
	}
	return totals, ranked
}

func applyOpenRouterTimeseriesGrowth(models []RankedModel, currentPoints []openRouterTimeseriesPoint, previousPoints []openRouterTimeseriesPoint) {
	if len(models) == 0 || len(currentPoints) == 0 || len(previousPoints) == 0 {
		return
	}
	currentTotals := aggregateOpenRouterPointTokens(currentPoints)
	previousTotals := aggregateOpenRouterPointTokens(previousPoints)
	for idx := range models {
		current := currentTotals[models[idx].ModelName]
		if current <= 0 {
			current = models[idx].TotalTokens
		}
		previous := previousTotals[models[idx].ModelName]
		if current > 0 || previous > 0 {
			models[idx].GrowthPct = rankingGrowthPct(current, previous)
		}
	}
}

func aggregateOpenRouterPointTokens(points []openRouterTimeseriesPoint) map[string]int64 {
	totals := make(map[string]int64)
	for _, point := range points {
		for slug, tokens := range point.Ys {
			if !openRouterIsOthersSlug(slug) && tokens > 0 {
				totals[slug] += tokens
			}
		}
	}
	return totals
}

func normalizeOpenRouterAggregateTokens(agg *openRouterModelAggregate) {
	if agg != nil && agg.totalTokens <= 0 && agg.count > 0 {
		agg.totalTokens = agg.count
	}
}

func aggregateOpenRouterModelRows(rows []openRouterModelRow, details map[string]openRouterModelDetails) map[string]*openRouterModelAggregate {
	aggregates := make(map[string]*openRouterModelAggregate)
	for _, row := range rows {
		slug := firstNonEmpty(row.VariantPermaslug, row.ModelPermaslug)
		if slug == "" {
			continue
		}
		tokens := openRouterRowTokens(row)
		if tokens <= 0 && row.Count <= 0 {
			continue
		}

		detail := details[openRouterBaseModelSlug(slug)]
		if detail.displayName == "" {
			detail = details[slug]
		}
		vendorSlug := openRouterVendorSlug(slug)
		vendor := firstNonEmpty(detail.vendor, openRouterVendorDisplayName(vendorSlug))
		vendorIcon := firstNonEmpty(detail.vendorIcon, openRouterVendorIcon(vendorSlug))
		displayName := firstNonEmpty(detail.displayName, openRouterModelDisplayName(slug))

		agg, ok := aggregates[slug]
		if !ok {
			agg = &openRouterModelAggregate{
				slug:        slug,
				displayName: displayName,
				vendor:      vendor,
				vendorIcon:  vendorIcon,
			}
			aggregates[slug] = agg
		}
		agg.totalTokens += tokens
		agg.count += row.Count
		if row.Change != nil {
			change := *row.Change
			agg.change = &change
		}
		if agg.displayName == "" {
			agg.displayName = displayName
		}
		if agg.vendor == "" {
			agg.vendor = vendor
		}
		if agg.vendorIcon == "" {
			agg.vendorIcon = vendorIcon
		}
	}
	return aggregates
}

func openRouterModelRowsToTimeseries(rows []openRouterModelRow) []openRouterTimeseriesPoint {
	byDate := make(map[string]map[string]int64)
	for _, row := range rows {
		slug := firstNonEmpty(row.VariantPermaslug, row.ModelPermaslug)
		if slug == "" {
			continue
		}
		tokens := openRouterRowTokens(row)
		if tokens <= 0 && row.Count > 0 {
			tokens = row.Count
		}
		if tokens <= 0 {
			continue
		}
		date := openRouterRowDate(row.Date)
		if date == "" {
			date = row.Date
		}
		if _, ok := byDate[date]; !ok {
			byDate[date] = make(map[string]int64)
		}
		byDate[date][slug] += tokens
	}
	return openRouterTimeseriesFromDateMap(byDate)
}

func openRouterModelRowsToVendorTimeseries(rows []openRouterModelRow) []openRouterTimeseriesPoint {
	byDate := make(map[string]map[string]int64)
	for _, row := range rows {
		slug := firstNonEmpty(row.VariantPermaslug, row.ModelPermaslug)
		if slug == "" {
			continue
		}
		tokens := openRouterRowTokens(row)
		if tokens <= 0 && row.Count > 0 {
			tokens = row.Count
		}
		if tokens <= 0 {
			continue
		}
		date := openRouterRowDate(row.Date)
		if date == "" {
			date = row.Date
		}
		vendor := openRouterVendorSlug(slug)
		if _, ok := byDate[date]; !ok {
			byDate[date] = make(map[string]int64)
		}
		byDate[date][vendor] += tokens
	}
	return openRouterTimeseriesFromDateMap(byDate)
}

func openRouterTimeseriesFromDateMap(byDate map[string]map[string]int64) []openRouterTimeseriesPoint {
	points := make([]openRouterTimeseriesPoint, 0, len(byDate))
	for date, ys := range byDate {
		points = append(points, openRouterTimeseriesPoint{X: date, Ys: ys})
	}
	return sortedOpenRouterPoints(points)
}

func openRouterRowTokens(row openRouterModelRow) int64 {
	return row.TotalPromptTokens + row.TotalCompletionTokens + row.TotalNativeTokensReasoning + row.TotalNativeTokensCached
}

func openRouterRowDate(value string) string {
	if ts, err := time.Parse("2006-01-02 15:04:05", value); err == nil {
		return ts.Format("2006-01-02")
	}
	if ts, err := time.Parse("2006-01-02", value); err == nil {
		return ts.Format("2006-01-02")
	}
	return value
}

func aggregateOpenRouterModelPoints(points []openRouterTimeseriesPoint, details map[string]openRouterModelDetails) map[string]*openRouterModelAggregate {
	aggregates := make(map[string]*openRouterModelAggregate)
	for _, point := range points {
		for slug, tokens := range point.Ys {
			if openRouterIsOthersSlug(slug) || tokens <= 0 {
				continue
			}
			detail := details[openRouterBaseModelSlug(slug)]
			if detail.displayName == "" {
				detail = details[slug]
			}
			vendorSlug := openRouterVendorSlug(slug)
			vendor := firstNonEmpty(detail.vendor, openRouterVendorDisplayName(vendorSlug))
			vendorIcon := firstNonEmpty(detail.vendorIcon, openRouterVendorIcon(vendorSlug))
			displayName := firstNonEmpty(detail.displayName, openRouterModelDisplayName(slug))

			agg, ok := aggregates[slug]
			if !ok {
				agg = &openRouterModelAggregate{
					slug:        slug,
					displayName: displayName,
					vendor:      vendor,
					vendorIcon:  vendorIcon,
				}
				aggregates[slug] = agg
			}
			agg.totalTokens += tokens
			if agg.displayName == "" {
				agg.displayName = displayName
			}
			if agg.vendor == "" {
				agg.vendor = vendor
			}
			if agg.vendorIcon == "" {
				agg.vendorIcon = vendorIcon
			}
		}
	}
	return aggregates
}

func buildOpenRouterRankedVendors(models []RankedModel, totalRows []model.RankingQuotaTotal) []RankedVendor {
	totalTokens := sumRankingTokens(totalRows)
	aggregates := make(map[string]*vendorAggregate)
	for _, item := range models {
		agg := ensureVendorAggregate(aggregates, rankingModelMeta{vendor: item.Vendor, vendorIcon: item.VendorIcon})
		agg.totalTokens += item.TotalTokens
		agg.models[item.ModelName] = struct{}{}
		if item.TotalTokens > agg.topModelTokens {
			agg.topModel = item.ModelName
			agg.topModelTokens = item.TotalTokens
		}
	}

	rows := make([]RankedVendor, 0, len(aggregates))
	for _, agg := range aggregates {
		rows = append(rows, RankedVendor{
			Vendor:      agg.name,
			DisplayName: agg.name,
			VendorIcon:  agg.icon,
			TotalTokens: agg.totalTokens,
			Share:       rankingShare(agg.totalTokens, totalTokens),
			GrowthPct:   0,
			ModelsCount: len(agg.models),
			TopModel:    agg.topModel,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalTokens == rows[j].TotalTokens {
			return rows[i].Vendor < rows[j].Vendor
		}
		return rows[i].TotalTokens > rows[j].TotalTokens
	})
	for idx := range rows {
		rows[idx].Rank = idx + 1
	}
	return rows
}

func buildOpenRouterRankedVendorsFromSharePoints(currentPoints []openRouterTimeseriesPoint, previousPoints []openRouterTimeseriesPoint) []RankedVendor {
	currentTotals := aggregateOpenRouterVendorPoints(currentPoints)
	previousTotals := aggregateOpenRouterVendorPoints(previousPoints)
	totalTokens := int64(0)
	for _, total := range currentTotals {
		totalTokens += total
	}

	type vendorRow struct {
		slug  string
		total int64
	}
	sorted := make([]vendorRow, 0, len(currentTotals))
	for slug, total := range currentTotals {
		if total > 0 {
			sorted = append(sorted, vendorRow{slug: slug, total: total})
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].total == sorted[j].total {
			return sorted[i].slug < sorted[j].slug
		}
		return sorted[i].total > sorted[j].total
	})

	rows := make([]RankedVendor, 0, len(sorted))
	for _, item := range sorted {
		if openRouterIsOthersSlug(item.slug) {
			continue
		}
		displayName := openRouterVendorDisplayName(item.slug)
		rows = append(rows, RankedVendor{
			Rank:        len(rows) + 1,
			Vendor:      displayName,
			DisplayName: displayName,
			VendorIcon:  openRouterVendorIcon(item.slug),
			TotalTokens: item.total,
			Share:       rankingShare(item.total, totalTokens),
			GrowthPct:   rankingGrowthPct(item.total, previousTotals[item.slug]),
			ModelsCount: 0,
			TopModel:    "",
		})
	}
	return rows
}

func aggregateOpenRouterVendorPoints(points []openRouterTimeseriesPoint) map[string]int64 {
	totals := make(map[string]int64)
	for _, point := range points {
		for slug, tokens := range point.Ys {
			if tokens > 0 {
				totals[strings.ToLower(slug)] += tokens
			}
		}
	}
	return totals
}

func buildOpenRouterModelHistory(points []openRouterTimeseriesPoint, config rankingPeriodConfig, details map[string]openRouterModelDetails) ModelHistorySeries {
	modelTotals := make(map[string]int64)
	for _, point := range points {
		for slug, tokens := range point.Ys {
			if !openRouterIsOthersSlug(slug) && tokens > 0 {
				modelTotals[slug] += tokens
			}
		}
	}

	models := topOpenRouterHistoryModels(modelTotals, rankingHistoryLimit, details)
	selected := make(map[string]ModelHistoryModel, len(models))
	for _, item := range models {
		selected[item.Name] = item
	}

	historyPoints := make([]ModelHistoryPoint, 0, len(points)*len(models))
	for _, point := range points {
		ts, label := openRouterHistoryTime(point.X, config)
		otherTokens := int64(0)
		for slug, tokens := range point.Ys {
			if tokens <= 0 {
				continue
			}
			if item, ok := selected[slug]; ok {
				historyPoints = append(historyPoints, ModelHistoryPoint{
					Ts:          ts,
					Label:       label,
					Model:       item.Name,
					DisplayName: item.DisplayName,
					Vendor:      item.Vendor,
					Tokens:      tokens,
				})
			} else {
				otherTokens += tokens
			}
		}
		if otherTokens > 0 {
			historyPoints = append(historyPoints, ModelHistoryPoint{
				Ts:          ts,
				Label:       label,
				Model:       rankingOthersLabel,
				DisplayName: rankingOthersLabel,
				Vendor:      "Various",
				Tokens:      otherTokens,
			})
		}
	}

	return ModelHistorySeries{Points: historyPoints, Models: models, Buckets: len(points)}
}

func topOpenRouterHistoryModels(totals map[string]int64, limit int, details map[string]openRouterModelDetails) []ModelHistoryModel {
	type row struct {
		slug  string
		total int64
	}
	rows := make([]row, 0, len(totals))
	for slug, total := range totals {
		if !openRouterIsOthersSlug(slug) && total > 0 {
			rows = append(rows, row{slug: slug, total: total})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].total == rows[j].total {
			return rows[i].slug < rows[j].slug
		}
		return rows[i].total > rows[j].total
	})

	models := make([]ModelHistoryModel, 0, minInt(len(rows), limit)+1)
	otherTotal := int64(0)
	for idx, item := range rows {
		if idx < limit {
			detail := details[openRouterBaseModelSlug(item.slug)]
			models = append(models, ModelHistoryModel{
				Name:        item.slug,
				DisplayName: firstNonEmpty(detail.displayName, openRouterModelDisplayName(item.slug)),
				Vendor:      firstNonEmpty(detail.vendor, openRouterVendorDisplayName(openRouterVendorSlug(item.slug))),
				Total:       item.total,
			})
			continue
		}
		otherTotal += item.total
	}
	if otherTotal > 0 {
		models = append(models, ModelHistoryModel{Name: rankingOthersLabel, DisplayName: rankingOthersLabel, Vendor: "Various", Total: otherTotal})
	}
	return models
}

func buildOpenRouterVendorShareHistory(points []openRouterTimeseriesPoint, config rankingPeriodConfig) VendorShareSeries {
	vendorTotals := make(map[string]int64)
	for _, point := range points {
		for slug, tokens := range point.Ys {
			if tokens > 0 {
				vendorTotals[slug] += tokens
			}
		}
	}

	vendors := topOpenRouterShareVendors(vendorTotals, rankingVendorLimit)
	selected := make(map[string]VendorShareVendor, len(vendors))
	for _, item := range vendors {
		selected[item.Name] = item
	}

	historyPoints := make([]VendorSharePoint, 0, len(points)*len(vendors))
	for _, point := range points {
		ts, label := openRouterHistoryTime(point.X, config)
		bucketTotal := int64(0)
		otherTokens := int64(0)
		for _, tokens := range point.Ys {
			if tokens > 0 {
				bucketTotal += tokens
			}
		}
		for slug, tokens := range point.Ys {
			if tokens <= 0 {
				continue
			}
			if item, ok := selected[slug]; ok {
				historyPoints = append(historyPoints, VendorSharePoint{
					Ts:          ts,
					Label:       label,
					Vendor:      item.Name,
					DisplayName: item.DisplayName,
					Share:       rankingShare(tokens, bucketTotal),
					Tokens:      tokens,
				})
			} else {
				otherTokens += tokens
			}
		}
		if otherTokens > 0 {
			historyPoints = append(historyPoints, VendorSharePoint{
				Ts:          ts,
				Label:       label,
				Vendor:      rankingOthersLabel,
				DisplayName: rankingOthersLabel,
				Share:       rankingShare(otherTokens, bucketTotal),
				Tokens:      otherTokens,
			})
		}
	}

	return VendorShareSeries{Points: historyPoints, Vendors: vendors, Buckets: len(points)}
}

func topOpenRouterShareVendors(totals map[string]int64, limit int) []VendorShareVendor {
	type row struct {
		slug  string
		total int64
	}
	rows := make([]row, 0, len(totals))
	totalTokens := int64(0)
	for slug, total := range totals {
		if total > 0 {
			rows = append(rows, row{slug: slug, total: total})
			totalTokens += total
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].total == rows[j].total {
			return rows[i].slug < rows[j].slug
		}
		return rows[i].total > rows[j].total
	})

	vendors := make([]VendorShareVendor, 0, minInt(len(rows), limit)+1)
	otherTotal := int64(0)
	for idx, item := range rows {
		if idx < limit {
			vendors = append(vendors, VendorShareVendor{
				Name:        item.slug,
				DisplayName: openRouterVendorDisplayName(item.slug),
				Total:       item.total,
				Share:       rankingShare(item.total, totalTokens),
			})
			continue
		}
		otherTotal += item.total
	}
	if otherTotal > 0 {
		vendors = append(vendors, VendorShareVendor{Name: rankingOthersLabel, DisplayName: rankingOthersLabel, Total: otherTotal, Share: rankingShare(otherTotal, totalTokens)})
	}
	return vendors
}

func buildOpenRouterPerformance(rows []openRouterPerformanceRow, details map[string]openRouterModelDetails) []RankingPerformance {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].RequestCount == rows[j].RequestCount {
			return rows[i].Slug < rows[j].Slug
		}
		return rows[i].RequestCount > rows[j].RequestCount
	})

	limit := minInt(len(rows), openRouterRankingsPerformanceLimit)
	result := make([]RankingPerformance, 0, limit)
	for idx := 0; idx < limit; idx++ {
		row := rows[idx]
		slug := firstNonEmpty(row.Slug, row.ID)
		detail := details[slug]
		vendorSlug := firstNonEmpty(row.Author, openRouterVendorSlug(slug))
		result = append(result, RankingPerformance{
			Rank:          idx + 1,
			ModelName:     slug,
			DisplayName:   firstNonEmpty(row.Name, detail.displayName, openRouterModelDisplayName(slug)),
			Vendor:        firstNonEmpty(detail.vendor, openRouterVendorDisplayName(vendorSlug)),
			RequestCount:  row.RequestCount,
			P50Latency:    row.P50Latency,
			P50Throughput: row.P50Throughput,
			ProviderCount: row.ProviderCount,
		})
	}
	return result
}

func buildOpenRouterBenchmarks(groups map[string][]openRouterBenchmarkRow) []RankingBenchmark {
	preferred := []string{"intelligence", "coding", "math", "vision"}
	result := make([]RankingBenchmark, 0, len(preferred)*openRouterRankingsBenchmarkLimit)
	seenCategories := make(map[string]struct{}, len(groups))

	for _, category := range preferred {
		if rows, ok := groups[category]; ok {
			result = append(result, topOpenRouterBenchmarks(category, rows)...)
			seenCategories[category] = struct{}{}
		}
	}

	categories := make([]string, 0, len(groups))
	for category := range groups {
		if _, ok := seenCategories[category]; !ok {
			categories = append(categories, category)
		}
	}
	sort.Strings(categories)
	for _, category := range categories {
		result = append(result, topOpenRouterBenchmarks(category, groups[category])...)
		if len(result) >= openRouterRankingsBenchmarkLimit*len(preferred) {
			break
		}
	}
	return result
}

func topOpenRouterBenchmarks(category string, rows []openRouterBenchmarkRow) []RankingBenchmark {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score == rows[j].Score {
			return rows[i].Permaslug < rows[j].Permaslug
		}
		return rows[i].Score > rows[j].Score
	})

	limit := minInt(len(rows), openRouterRankingsBenchmarkLimit)
	result := make([]RankingBenchmark, 0, limit)
	for idx := 0; idx < limit; idx++ {
		row := rows[idx]
		slug := firstNonEmpty(row.OpenRouterSlug, row.HeuristicOpenRouterSlug, row.Permaslug, row.UID)
		result = append(result, RankingBenchmark{
			Category:    category,
			Rank:        idx + 1,
			ModelName:   slug,
			DisplayName: firstNonEmpty(row.AAName, openRouterModelDisplayName(slug)),
			Vendor:      openRouterVendorDisplayName(openRouterVendorSlug(slug)),
			Score:       row.Score,
		})
	}
	return result
}

func openRouterHistoryTime(value string, config rankingPeriodConfig) (string, string) {
	if ts, err := time.ParseInLocation("2006-01-02", value, time.UTC); err == nil {
		return ts.UTC().Format(time.RFC3339), openRouterHistoryLabel(ts, config)
	}
	if ts, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC); err == nil {
		return ts.UTC().Format(time.RFC3339), openRouterHistoryLabel(ts, config)
	}
	return value, value
}

func openRouterHistoryLabel(ts time.Time, config rankingPeriodConfig) string {
	ts = ts.UTC()
	if config.id == "week" {
		return fmt.Sprintf("%s-%s", ts.Format("Jan 2"), ts.AddDate(0, 0, 6).Format("Jan 2"))
	}
	return ts.Format(config.labelLayout)
}

func openRouterBaseModelSlug(slug string) string {
	return strings.TrimSuffix(slug, ":free")
}

func openRouterVendorSlug(slug string) string {
	if idx := strings.Index(slug, "/"); idx > 0 {
		return strings.ToLower(slug[:idx])
	}
	return strings.ToLower(slug)
}

func openRouterModelDisplayName(slug string) string {
	clean := openRouterBaseModelSlug(slug)
	if idx := strings.Index(clean, "/"); idx >= 0 && idx+1 < len(clean) {
		clean = clean[idx+1:]
	}
	clean = strings.ReplaceAll(clean, "-", " ")
	clean = strings.ReplaceAll(clean, "_", " ")
	return titleSlug(clean)
}

func openRouterVendorDisplayName(slug string) string {
	normalized := strings.ToLower(strings.TrimSpace(slug))
	switch normalized {
	case "":
		return rankingUnknownVendor
	case "anthropic":
		return "Anthropic"
	case "openai":
		return "OpenAI"
	case "google":
		return "Google"
	case "deepseek":
		return "DeepSeek"
	case "x-ai":
		return "xAI"
	case "cohere":
		return "Cohere"
	case "meta-llama", "meta":
		return "Meta"
	case "mistralai":
		return "Mistral"
	case "qwen", "alibaba":
		return "Qwen"
	case "moonshotai":
		return "Moonshot"
	case "minimax":
		return "MiniMax"
	case "z-ai":
		return "Z.ai"
	case "tencent":
		return "Tencent"
	case "xiaomi":
		return "Xiaomi"
	case "stepfun":
		return "StepFun"
	case "poolside":
		return "Poolside"
	case "nex-agi":
		return "Nex AGI"
	case "inclusionai":
		return "inclusionAI"
	case "ibm-granite":
		return "IBM"
	case "openrouter":
		return "OpenRouter"
	case "nvidia":
		return "NVIDIA"
	case "microsoft":
		return "Microsoft"
	case "nousresearch":
		return "Nous Research"
	case "others":
		return rankingOthersLabel
	default:
		return titleSlug(normalized)
	}
}

func openRouterVendorIcon(slug string) string {
	normalized := strings.ToLower(strings.TrimSpace(slug))
	switch normalized {
	case "anthropic":
		return "Claude.Color"
	case "openai":
		return "OpenAI.Color"
	case "google":
		return "Gemini.Color"
	case "deepseek":
		return "DeepSeek.Color"
	case "x-ai":
		return "Grok.Color"
	case "cohere":
		return "Cohere.Color"
	case "meta-llama", "meta":
		return "Meta.Color"
	case "mistralai":
		return "Mistral.Color"
	case "moonshotai":
		return "Moonshot.Color"
	case "minimax":
		return "Minimax.Color"
	case "z-ai":
		return "ZAI"
	case "qwen", "alibaba":
		return "Qwen.Color"
	case "tencent":
		return "Tencent.Color"
	case "xiaomi":
		return "XiaomiMiMo"
	case "stepfun":
		return "Stepfun.Color"
	case "nvidia":
		return "Nvidia.Color"
	case "openrouter":
		return "OpenRouter.Color"
	default:
		if normalized == "" {
			return ""
		}
		return titleSlug(normalized)
	}
}

func openRouterIsOthersSlug(slug string) bool {
	normalized := strings.ToLower(strings.TrimSpace(slug))
	return normalized == "" || normalized == rankingOthersLabelLower()
}

func rankingOthersLabelLower() string {
	return strings.ToLower(rankingOthersLabel)
}

func titleSlug(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for idx, part := range parts {
		if part == "" {
			continue
		}
		parts[idx] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
