package model

import (
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
)

type PlatformUsageToday struct {
	RequestCount           int64 `json:"request_count"`
	PromptTokens           int64 `json:"prompt_tokens"`
	CompletionTokens       int64 `json:"completion_tokens"`
	TotalTokens            int64 `json:"total_tokens"`
	PreDiscountQuota       int64 `json:"pre_discount_quota"`
	ExactQuotaLogCount     int64 `json:"exact_quota_log_count"`
	EstimatedQuotaLogCount int64 `json:"estimated_quota_log_count"`
}

type platformUsageLogRow struct {
	Quota            int64
	PromptTokens     int64
	CompletionTokens int64
	Other            string
}

func numberFromOther(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	}
	return 0, false
}

func platformQuotaFromLog(row platformUsageLogRow) (int64, bool) {
	var other map[string]interface{}
	if row.Other != "" {
		_ = common.UnmarshalJsonStr(row.Other, &other)
	}
	if value, ok := numberFromOther(other[common.PlatformBaseQuotaKey()]); ok && value >= 0 {
		return int64(math.Round(value)), other["platform_base_quota_estimated"] != true
	}

	ingressOriginal := row.Quota
	if value, ok := numberFromOther(other["ingress_original_quota"]); ok && value > 0 {
		ingressOriginal = int64(math.Round(value))
	}
	groupRatio, ok := numberFromOther(other["group_ratio"])
	if !ok {
		return ingressOriginal, false
	}
	return common.PlatformBaseQuota(row.Quota, groupRatio, ingressOriginal)
}

func resolvePreDiscountQuota(quota int, other map[string]interface{}) int {
	resolved, _ := platformQuotaFromLog(platformUsageLogRow{
		Quota: int64(quota),
		Other: common.MapToJsonStr(other),
	})
	if resolved <= 0 && quota > 0 {
		return quota
	}
	return int(resolved)
}

// GetPlatformUsageToday aggregates successful relay logs for the whole site.
// It scans only the bounded current-day window so the calculation remains
// portable across SQLite, MySQL and PostgreSQL JSON implementations.
func GetPlatformUsageToday(startTime, endTime int64) (PlatformUsageToday, error) {
	var result PlatformUsageToday
	if startTime <= 0 || endTime <= startTime {
		return result, fmt.Errorf("invalid platform usage range")
	}

	rows, err := LOG_DB.Table("logs").
		Select("quota", "prompt_tokens", "completion_tokens", "other").
		Where("created_at >= ? AND created_at < ?", startTime, endTime).
		Where("type = ?", LogTypeConsume).
		Rows()
	if err != nil {
		return result, fmt.Errorf("query platform usage today: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var row platformUsageLogRow
		if err = LOG_DB.ScanRows(rows, &row); err != nil {
			return result, fmt.Errorf("scan platform usage today: %w", err)
		}
		result.RequestCount++
		result.PromptTokens += row.PromptTokens
		result.CompletionTokens += row.CompletionTokens
		quota, exact := platformQuotaFromLog(row)
		result.PreDiscountQuota += quota
		if exact {
			result.ExactQuotaLogCount++
		} else {
			result.EstimatedQuotaLogCount++
		}
	}
	if err = rows.Err(); err != nil {
		return result, fmt.Errorf("iterate platform usage today: %w", err)
	}
	result.TotalTokens = result.PromptTokens + result.CompletionTokens
	return result, nil
}
