package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// UsageStatisticsSummary contains user-scoped request and billing totals for a
// bounded period. RequestCount includes both successful relay calls and relay
// errors, while token/quota fields include successful consume logs only.
type UsageStatisticsSummary struct {
	RequestCount      int64   `json:"request_count"`
	SuccessCount      int64   `json:"success_count"`
	ErrorCount        int64   `json:"error_count"`
	SuccessRate       float64 `json:"success_rate"`
	Quota             int64   `json:"quota"`
	WalletQuota       int64   `json:"wallet_quota"`
	SubscriptionQuota int64   `json:"subscription_quota"`
	PromptTokens      int64   `json:"prompt_tokens"`
	CacheTokens       int64   `json:"cache_tokens"`
	EffectivePrompt   int64   `json:"effective_prompt_tokens"`
	CompletionTokens  int64   `json:"completion_tokens"`
	TotalTokens       int64   `json:"total_tokens"`
	CacheHitRate      float64 `json:"cache_hit_rate"`
}

type UsageStatisticsPoint struct {
	Timestamp       int64 `json:"timestamp" gorm:"column:bucket"`
	RequestCount    int64 `json:"request_count"`
	SuccessCount    int64 `json:"success_count"`
	ErrorCount      int64 `json:"error_count"`
	Quota           int64 `json:"quota"`
	TotalTokens     int64 `json:"total_tokens"`
	CacheTokens     int64 `json:"cache_tokens"`
	EffectivePrompt int64 `json:"effective_prompt_tokens"`
}

type UsageStatisticsModel struct {
	ModelName        string `json:"model_name"`
	RequestCount     int64  `json:"request_count"`
	Quota            int64  `json:"quota"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CacheTokens      int64  `json:"cache_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
}

// UsageStatisticsSubscription contains only subscription quota that was
// actually consumed in the selected period. It is intentionally log-backed,
// so unused and merely active subscription instances never appear.
type UsageStatisticsSubscription struct {
	SubscriptionId int    `json:"subscription_id"`
	Title          string `json:"title"`
	RequestCount   int64  `json:"request_count"`
	Quota          int64  `json:"quota"`
}

type UsageStatistics struct {
	Summary       UsageStatisticsSummary        `json:"summary"`
	Series        []UsageStatisticsPoint        `json:"series"`
	Models        []UsageStatisticsModel        `json:"models"`
	Subscriptions []UsageStatisticsSubscription `json:"subscriptions"`
}

func usageStatisticsAggregateSelect() string {
	return fmt.Sprintf(`
		COALESCE(SUM(CASE WHEN type = %d THEN 1 ELSE 0 END), 0) AS success_count,
		COALESCE(SUM(CASE WHEN type = %d THEN 1 ELSE 0 END), 0) AS error_count,
		COALESCE(SUM(CASE WHEN type = %d THEN quota ELSE 0 END), 0) AS quota,
		COALESCE(SUM(CASE WHEN type = %d AND COALESCE(billing_source, '') = 'subscription' THEN quota ELSE 0 END), 0) AS subscription_quota,
		COALESCE(SUM(CASE WHEN type = %d AND COALESCE(billing_source, '') <> 'subscription' THEN quota ELSE 0 END), 0) AS wallet_quota,
		COALESCE(SUM(CASE WHEN type = %d THEN prompt_tokens ELSE 0 END), 0) AS prompt_tokens,
		COALESCE(SUM(CASE WHEN type = %d AND prompt_tokens > 0 THEN cache_tokens ELSE 0 END), 0) AS cache_tokens,
		COALESCE(SUM(CASE WHEN type = %d AND prompt_tokens > 0 THEN CASE WHEN cache_tokens > prompt_tokens THEN prompt_tokens + cache_tokens ELSE prompt_tokens END ELSE 0 END), 0) AS effective_prompt,
		COALESCE(SUM(CASE WHEN type = %d THEN completion_tokens ELSE 0 END), 0) AS completion_tokens,
		COALESCE(SUM(CASE WHEN type = %d THEN prompt_tokens + completion_tokens ELSE 0 END), 0) AS total_tokens`,
		LogTypeConsume,
		LogTypeError,
		LogTypeConsume,
		LogTypeConsume,
		LogTypeConsume,
		LogTypeConsume,
		LogTypeConsume,
		LogTypeConsume,
		LogTypeConsume,
		LogTypeConsume,
	)
}

func usageStatisticsBucketExpression(bucketSeconds int64) string {
	if common.UsingPostgreSQL {
		return fmt.Sprintf("FLOOR(created_at::numeric / %d) * %d", bucketSeconds, bucketSeconds)
	}
	if common.UsingMySQL {
		return fmt.Sprintf("FLOOR(created_at / %d) * %d", bucketSeconds, bucketSeconds)
	}
	return fmt.Sprintf("(created_at / %d) * %d", bucketSeconds, bucketSeconds)
}

func usageStatisticsBaseQuery(userID int, startTime, endTime int64) *gorm.DB {
	return LOG_DB.Table("logs").
		Where("user_id = ?", userID).
		Where("created_at >= ? AND created_at < ?", startTime, endTime).
		Where("type IN ?", []int{LogTypeConsume, LogTypeError})
}

func fillUsageStatisticsSeries(
	points []UsageStatisticsPoint,
	startTime, endTime, bucketSeconds int64,
) []UsageStatisticsPoint {
	firstBucket := startTime / bucketSeconds * bucketSeconds
	byBucket := make(map[int64]UsageStatisticsPoint, len(points))
	for _, point := range points {
		byBucket[point.Timestamp] = point
	}
	result := make([]UsageStatisticsPoint, 0, (endTime-firstBucket)/bucketSeconds+1)
	for timestamp := firstBucket; timestamp < endTime; timestamp += bucketSeconds {
		point, ok := byBucket[timestamp]
		if !ok {
			point = UsageStatisticsPoint{Timestamp: timestamp}
		}
		point.RequestCount = point.SuccessCount + point.ErrorCount
		result = append(result, point)
	}
	return result
}

func hydrateUsageStatisticsSubscriptionTitles(
	userID int,
	items []UsageStatisticsSubscription,
) error {
	ids := make([]int, 0, len(items))
	for _, item := range items {
		if item.SubscriptionId > 0 {
			ids = append(ids, item.SubscriptionId)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	var subscriptions []UserSubscription
	if err := DB.
		Select("id", "plan_title", "remark").
		Where("user_id = ? AND id IN ?", userID, ids).
		Find(&subscriptions).Error; err != nil {
		return fmt.Errorf("query usage statistics subscription titles: %w", err)
	}
	titles := make(map[int]string, len(subscriptions))
	for _, subscription := range subscriptions {
		title := subscription.Remark
		if title == "" {
			title = subscription.PlanTitle
		}
		titles[subscription.Id] = title
	}
	for index := range items {
		items[index].Title = titles[items[index].SubscriptionId]
	}
	return nil
}

// GetUserUsageStatistics returns bounded, user-only usage analytics. It uses
// bounded aggregate queries instead of loading individual logs into memory.
func GetUserUsageStatistics(userID int, startTime, endTime, bucketSeconds int64) (UsageStatistics, error) {
	result := UsageStatistics{
		Series:        make([]UsageStatisticsPoint, 0),
		Models:        make([]UsageStatisticsModel, 0),
		Subscriptions: make([]UsageStatisticsSubscription, 0),
	}
	if userID <= 0 {
		return result, errors.New("invalid user id")
	}
	if startTime <= 0 || endTime <= startTime || bucketSeconds <= 0 {
		return result, errors.New("invalid usage statistics range")
	}

	if err := usageStatisticsBaseQuery(userID, startTime, endTime).
		Select(usageStatisticsAggregateSelect()).
		Scan(&result.Summary).Error; err != nil {
		return result, fmt.Errorf("query usage statistics summary: %w", err)
	}
	result.Summary.RequestCount = result.Summary.SuccessCount + result.Summary.ErrorCount
	if result.Summary.RequestCount > 0 {
		result.Summary.SuccessRate = float64(result.Summary.SuccessCount) * 100 / float64(result.Summary.RequestCount)
	}
	if result.Summary.EffectivePrompt > 0 {
		result.Summary.CacheHitRate = float64(result.Summary.CacheTokens) * 100 / float64(result.Summary.EffectivePrompt)
	}

	bucketExpr := usageStatisticsBucketExpression(bucketSeconds)
	seriesSelect := fmt.Sprintf(`
		%s AS bucket,
		COALESCE(SUM(CASE WHEN type = %d THEN 1 ELSE 0 END), 0) AS success_count,
		COALESCE(SUM(CASE WHEN type = %d THEN 1 ELSE 0 END), 0) AS error_count,
		COALESCE(SUM(CASE WHEN type = %d THEN quota ELSE 0 END), 0) AS quota,
		COALESCE(SUM(CASE WHEN type = %d THEN prompt_tokens + completion_tokens ELSE 0 END), 0) AS total_tokens,
		COALESCE(SUM(CASE WHEN type = %d AND prompt_tokens > 0 THEN cache_tokens ELSE 0 END), 0) AS cache_tokens,
		COALESCE(SUM(CASE WHEN type = %d AND prompt_tokens > 0 THEN CASE WHEN cache_tokens > prompt_tokens THEN prompt_tokens + cache_tokens ELSE prompt_tokens END ELSE 0 END), 0) AS effective_prompt_tokens`,
		bucketExpr,
		LogTypeConsume,
		LogTypeError,
		LogTypeConsume,
		LogTypeConsume,
		LogTypeConsume,
		LogTypeConsume,
	)
	var points []UsageStatisticsPoint
	if err := usageStatisticsBaseQuery(userID, startTime, endTime).
		Select(seriesSelect).
		Group("bucket").
		Order("bucket").
		Scan(&points).Error; err != nil {
		return result, fmt.Errorf("query usage statistics series: %w", err)
	}
	result.Series = fillUsageStatisticsSeries(points, startTime, endTime, bucketSeconds)

	modelSelect := `
		CASE WHEN model_name = '' THEN 'unknown' ELSE model_name END AS model_name,
		COUNT(*) AS request_count,
		COALESCE(SUM(quota), 0) AS quota,
		COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
		COALESCE(SUM(CASE WHEN prompt_tokens > 0 THEN cache_tokens ELSE 0 END), 0) AS cache_tokens,
		COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
		COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS total_tokens`
	if err := usageStatisticsBaseQuery(userID, startTime, endTime).
		Where("type = ?", LogTypeConsume).
		Select(modelSelect).
		Group("model_name").
		Order("request_count DESC").
		Limit(10).
		Scan(&result.Models).Error; err != nil {
		return result, fmt.Errorf("query usage statistics models: %w", err)
	}

	subscriptionSelect := `
		subscription_id,
		COUNT(*) AS request_count,
		COALESCE(SUM(quota), 0) AS quota`
	if err := usageStatisticsBaseQuery(userID, startTime, endTime).
		Where("type = ? AND billing_source = ?", LogTypeConsume, "subscription").
		Select(subscriptionSelect).
		Group("subscription_id").
		Having("SUM(quota) > 0").
		Order("SUM(quota) DESC, subscription_id ASC").
		Scan(&result.Subscriptions).Error; err != nil {
		return result, fmt.Errorf("query usage statistics subscriptions: %w", err)
	}
	if err := hydrateUsageStatisticsSubscriptionTitles(userID, result.Subscriptions); err != nil {
		return result, err
	}

	return result, nil
}
