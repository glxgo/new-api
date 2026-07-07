package model

import (
	"database/sql"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PerfMetric stores aggregated relay performance metrics for the model square.
type PerfMetric struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	ModelName      string `json:"model_name" gorm:"size:128;uniqueIndex:idx_perf_model_group_bucket,priority:1"`
	Group          string `json:"group" gorm:"column:group;size:64;uniqueIndex:idx_perf_model_group_bucket,priority:2"`
	BucketTs       int64  `json:"bucket_ts" gorm:"uniqueIndex:idx_perf_model_group_bucket,priority:3;index:idx_perf_bucket_ts"`
	RequestCount   int64  `json:"-" gorm:"default:0"`
	SuccessCount   int64  `json:"-" gorm:"default:0"`
	TotalLatencyMs int64  `json:"-" gorm:"default:0"`
	TtftSumMs      int64  `json:"-" gorm:"default:0"`
	TtftCount      int64  `json:"-" gorm:"default:0"`
	OutputTokens   int64  `json:"-" gorm:"default:0"`
	GenerationMs   int64  `json:"-" gorm:"default:0"`
	CacheTokens    int64  `json:"-" gorm:"default:0"` // prompt cache 命中 token 累计
	PromptTokens   int64  `json:"-" gorm:"default:0"` // 输入 token（未命中缓存）累计
}

func (PerfMetric) TableName() string {
	return "perf_metrics"
}

func UpsertPerfMetric(metric *PerfMetric) error {
	if metric == nil || metric.RequestCount == 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "model_name"},
			{Name: "group"},
			{Name: "bucket_ts"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"request_count":   gorm.Expr("perf_metrics.request_count + ?", metric.RequestCount),
			"success_count":   gorm.Expr("perf_metrics.success_count + ?", metric.SuccessCount),
			"total_latency_ms": gorm.Expr("perf_metrics.total_latency_ms + ?", metric.TotalLatencyMs),
			"ttft_sum_ms":     gorm.Expr("perf_metrics.ttft_sum_ms + ?", metric.TtftSumMs),
			"ttft_count":      gorm.Expr("perf_metrics.ttft_count + ?", metric.TtftCount),
			"output_tokens":   gorm.Expr("perf_metrics.output_tokens + ?", metric.OutputTokens),
			"generation_ms":   gorm.Expr("perf_metrics.generation_ms + ?", metric.GenerationMs),
			"cache_tokens":    gorm.Expr("perf_metrics.cache_tokens + ?", metric.CacheTokens),
			"prompt_tokens":   gorm.Expr("perf_metrics.prompt_tokens + ?", metric.PromptTokens),
		}),
	}).Create(metric).Error
}

func GetPerfMetrics(modelName string, group string, startTs int64, endTs int64) ([]PerfMetric, error) {
	var metrics []PerfMetric
	query := DB.Model(&PerfMetric{}).
		Where("model_name = ? AND bucket_ts >= ? AND bucket_ts <= ?", modelName, startTs, endTs)
	if group != "" {
		query = query.Where(commonGroupCol+" = ?", group)
	}
	err := query.Order("bucket_ts ASC").Find(&metrics).Error
	return metrics, err
}

// GetLastPerfBucketTs 返回某模型在 beforeTs 之前最近一个有数据的 bucket_ts（无数据返回 0）。
func GetLastPerfBucketTs(modelName string, beforeTs int64) (int64, error) {
	var ts sql.NullInt64
	err := DB.Model(&PerfMetric{}).
		Where("model_name = ? AND bucket_ts <= ?", modelName, beforeTs).
		Select("MAX(bucket_ts)").
		Scan(&ts).Error
	if err != nil {
		return 0, err
	}
	if !ts.Valid {
		return 0, nil
	}
	return ts.Int64, nil
}

// GetRecentPerfBucketTs 取该 model 最近 limit 个有数据的 bucket_ts（去重、跨所有 group、按时间倒序）。
func GetRecentPerfBucketTs(modelName string, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 24
	}
	var tsList []int64
	err := DB.Model(&PerfMetric{}).
		Where("model_name = ?", modelName).
		Distinct("bucket_ts").
		Order("bucket_ts DESC").
		Limit(limit).
		Pluck("bucket_ts", &tsList).Error
	return tsList, err
}

// GetPerfMetricsByBucketTs 取该 model 在指定 bucket_ts 列表内的数据（可按 group 过滤），按 bucket_ts 升序。
func GetPerfMetricsByBucketTs(modelName string, group string, bucketTsList []int64) ([]PerfMetric, error) {
	var metrics []PerfMetric
	if len(bucketTsList) == 0 {
		return metrics, nil
	}
	query := DB.Model(&PerfMetric{}).
		Where("model_name = ? AND bucket_ts IN ?", modelName, bucketTsList)
	if group != "" {
		query = query.Where(commonGroupCol + " = ?", group)
	}
	err := query.Order("bucket_ts ASC").Find(&metrics).Error
	return metrics, err
}

type PerfMetricSummary struct {
	ModelName      string `json:"model_name"`
	RequestCount   int64  `json:"request_count"`
	SuccessCount   int64  `json:"success_count"`
	TotalLatencyMs int64  `json:"total_latency_ms"`
	OutputTokens   int64  `json:"output_tokens"`
	GenerationMs   int64  `json:"generation_ms"`
	CacheTokens    int64  `json:"cache_tokens"`
	PromptTokens   int64  `json:"prompt_tokens"`
}

func GetPerfMetricsSummaryAll(startTs int64, endTs int64, groups []string) ([]PerfMetricSummary, error) {
	var summaries []PerfMetricSummary
	query := DB.Model(&PerfMetric{}).
		Select("model_name, SUM(request_count) as request_count, SUM(success_count) as success_count, SUM(total_latency_ms) as total_latency_ms, SUM(output_tokens) as output_tokens, SUM(generation_ms) as generation_ms, SUM(cache_tokens) as cache_tokens, SUM(prompt_tokens) as prompt_tokens").
		Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs)
	if groups != nil {
		if len(groups) == 0 {
			return summaries, nil
		}
		query = query.Where(commonGroupCol+" IN ?", groups)
	}
	err := query.
		Group("model_name").
		Having("SUM(request_count) > 0").
		Find(&summaries).Error
	return summaries, err
}

func DeletePerfMetricsBefore(cutoffTs int64) error {
	if cutoffTs <= 0 {
		return nil
	}
	return DB.Where("bucket_ts < ?", cutoffTs).Delete(&PerfMetric{}).Error
}

func PerfMetricStartTime(hours int) int64 {
	if hours <= 0 {
		hours = 24
	}
	return time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
}
