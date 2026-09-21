package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const (
	DetailedUsageLogRetentionDays = 30
	usageLogAggregateBucket       = int64(24 * 60 * 60)
)

// UsageLogDailyAggregate deliberately excludes request content, IPs, request
// IDs, upstream IDs, token names and free-form Other metadata. It keeps only
// the dimensions, counters and source boundary metadata required for
// historical usage/billing reports.
// Multiple rows for the same dimensions are valid: each cleanup batch writes
// and deletes atomically, so report queries simply SUM all segments.
type UsageLogDailyAggregate struct {
	Id                    int    `json:"id"`
	BucketStart           int64  `json:"bucket_start" gorm:"type:bigint;not null;index:idx_usage_aggregate_bucket_user,priority:1"`
	UserId                int    `json:"user_id" gorm:"not null;index:idx_usage_aggregate_bucket_user,priority:2;index"`
	Type                  int    `json:"type" gorm:"not null;index"`
	ModelName             string `json:"model_name" gorm:"type:varchar(191);not null;default:'';index"`
	ChannelId             int    `json:"channel_id" gorm:"not null;default:0;index"`
	TokenId               int    `json:"token_id" gorm:"not null;default:0;index"`
	GroupName             string `json:"group" gorm:"column:group_name;type:varchar(64);not null;default:'';index"`
	BillingSource         string `json:"billing_source" gorm:"type:varchar(32);not null;default:'';index"`
	SubscriptionId        int    `json:"subscription_id" gorm:"not null;default:0;index"`
	RequestCount          int64  `json:"request_count" gorm:"type:bigint;not null;default:0"`
	StreamCount           int64  `json:"stream_count" gorm:"type:bigint;not null;default:0"`
	Quota                 int64  `json:"quota" gorm:"type:bigint;not null;default:0"`
	PreDiscountQuota      int64  `json:"pre_discount_quota" gorm:"type:bigint;not null;default:0"`
	PromptTokens          int64  `json:"prompt_tokens" gorm:"type:bigint;not null;default:0"`
	CacheTokens           int64  `json:"cache_tokens" gorm:"type:bigint;not null;default:0"`
	EffectivePromptTokens int64  `json:"effective_prompt_tokens" gorm:"type:bigint;not null;default:0"`
	CompletionTokens      int64  `json:"completion_tokens" gorm:"type:bigint;not null;default:0"`
	UseTime               int64  `json:"use_time" gorm:"type:bigint;not null;default:0"`
	Cost                  int64  `json:"cost" gorm:"type:bigint;not null;default:0"`
	PaidQuota             int64  `json:"paid_quota" gorm:"type:bigint;not null;default:0"`
	PaidGiftQuota         int64  `json:"paid_gift_quota" gorm:"type:bigint;not null;default:0"`
	BalanceAfter          *int64 `json:"balance_after" gorm:"default:null"`
	FirstLogAt            int64  `json:"first_log_at" gorm:"type:bigint;not null"`
	LastLogAt             int64  `json:"last_log_at" gorm:"type:bigint;not null"`
	// Preserve the source-log tie breaker after detail rows are compacted.
	LastLogId int64 `json:"last_log_id" gorm:"type:bigint;not null;default:0"`
	CreatedAt int64 `json:"created_at" gorm:"type:bigint;not null"`
}

type usageLogAggregateKey struct {
	BucketStart, UserId, Type, ChannelId, TokenId, SubscriptionId int64
	ModelName, GroupName, BillingSource                           string
}

func usageLogAggregateBucketStart(timestamp int64) int64 {
	value := time.Unix(timestamp, 0).In(time.Local)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.Local).Unix()
}

func aggregateDetailedUsageLogs(logs []Log, archivedAt int64) []UsageLogDailyAggregate {
	grouped := make(map[usageLogAggregateKey]*UsageLogDailyAggregate)
	for i := range logs {
		log := logs[i]
		key := usageLogAggregateKey{
			BucketStart: usageLogAggregateBucketStart(log.CreatedAt),
			UserId:      int64(log.UserId), Type: int64(log.Type), ChannelId: int64(log.ChannelId),
			TokenId: int64(log.TokenId), SubscriptionId: int64(log.SubscriptionId),
			ModelName: log.ModelName, GroupName: log.Group, BillingSource: log.BillingSource,
		}
		item := grouped[key]
		if item == nil {
			item = &UsageLogDailyAggregate{
				BucketStart: key.BucketStart, UserId: log.UserId, Type: log.Type,
				ModelName: log.ModelName, ChannelId: log.ChannelId, TokenId: log.TokenId,
				GroupName: log.Group, BillingSource: log.BillingSource, SubscriptionId: log.SubscriptionId,
				FirstLogAt: log.CreatedAt, LastLogAt: log.CreatedAt, LastLogId: int64(log.Id), BalanceAfter: log.BalanceAfter, CreatedAt: archivedAt,
			}
			grouped[key] = item
		}
		item.RequestCount++
		if log.IsStream {
			item.StreamCount++
		}
		item.Quota += int64(log.Quota)
		if log.PreDiscountQuota > 0 {
			item.PreDiscountQuota += int64(log.PreDiscountQuota)
		} else {
			item.PreDiscountQuota += int64(log.Quota)
		}
		item.PromptTokens += int64(log.PromptTokens)
		item.CacheTokens += int64(log.CacheTokens)
		if log.PromptTokens > 0 {
			effectivePrompt := log.PromptTokens
			if log.CacheTokens > log.PromptTokens {
				effectivePrompt += log.CacheTokens
			}
			item.EffectivePromptTokens += int64(effectivePrompt)
		}
		item.CompletionTokens += int64(log.CompletionTokens)
		item.UseTime += int64(log.UseTime)
		item.Cost += int64(log.Cost)
		item.PaidQuota += int64(log.PaidQuota)
		item.PaidGiftQuota += int64(log.PaidGiftQuota)
		if log.CreatedAt < item.FirstLogAt {
			item.FirstLogAt = log.CreatedAt
		}
		if log.CreatedAt > item.LastLogAt || (log.CreatedAt == item.LastLogAt && int64(log.Id) > item.LastLogId) {
			item.LastLogAt = log.CreatedAt
			item.LastLogId = int64(log.Id)
			item.BalanceAfter = log.BalanceAfter
		}
	}
	result := make([]UsageLogDailyAggregate, 0, len(grouped))
	for _, item := range grouped {
		result = append(result, *item)
	}
	return result
}

// ArchiveDetailedUsageLogs atomically compacts one bounded batch. Only relay
// errors and already-settled consume rows are eligible; financial/audit/login
// rows and unresolved legacy settlement rows are retained verbatim.
func ArchiveDetailedUsageLogs(ctx context.Context, cutoff int64, limit int) (archived, deleted int64, err error) {
	if LOG_DB == nil {
		return 0, 0, errors.New("log database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if cutoff <= 0 {
		return 0, 0, errors.New("invalid detailed log retention cutoff")
	}
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	archiveHasLastLogId := walletConsumeArchiveHasLastLogId(LOG_DB)
	err = LOG_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		logs, queryErr := findOldDetailedUsageLogs(tx, cutoff, limit)
		if queryErr != nil {
			return queryErr
		}
		if len(logs) == 0 {
			return nil
		}
		aggregates := aggregateDetailedUsageLogs(logs, cutoff)
		if len(aggregates) > 0 {
			archiveWrite := tx
			if !archiveHasLastLogId {
				// A pre-v2 archive table may still be present while the explicit
				// metadata migration is pending. Keep retention operational and
				// let the source-id field take its default once it is added.
				archiveWrite = tx.Omit("last_log_id")
			}
			if createErr := archiveWrite.CreateInBatches(&aggregates, 200).Error; createErr != nil {
				return fmt.Errorf("archive usage log aggregates: %w", createErr)
			}
		}
		ids := make([]int, 0, len(logs))
		for i := range logs {
			ids = append(ids, logs[i].Id)
		}
		result := tx.Where("id IN ?", ids).Delete(&Log{})
		if result.Error != nil {
			return fmt.Errorf("delete archived usage log details: %w", result.Error)
		}
		if result.RowsAffected != int64(len(ids)) {
			return fmt.Errorf("delete archived usage log details: expected %d rows, deleted %d", len(ids), result.RowsAffected)
		}
		archived = int64(len(logs))
		deleted = result.RowsAffected
		return nil
	})
	return archived, deleted, err
}

// findOldDetailedUsageLogs deliberately avoids the OR predicate used by the
// old retention query. The two eligible log classes have different indexed
// predicates; querying them separately lets MySQL use
// idx_log_retention(type, settled, created_at, id), then we merge only the
// oldest rows in memory. This keeps an empty retention pass cheap and bounds
// the amount of data loaded into a transaction.
func findOldDetailedUsageLogs(tx *gorm.DB, cutoff int64, limit int) ([]Log, error) {
	if limit <= 0 {
		return nil, nil
	}
	var errors, consumes []Log
	if err := tx.Where("type = ? AND created_at < ?", LogTypeError, cutoff).
		Order("id asc").Limit(limit).Find(&errors).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("type = ? AND settled = ? AND created_at < ?", LogTypeConsume, true, cutoff).
		Order("id asc").Limit(limit).Find(&consumes).Error; err != nil {
		return nil, err
	}
	merged := make([]Log, 0, minInt(limit, len(errors)+len(consumes)))
	errorIndex, consumeIndex := 0, 0
	for len(merged) < limit && (errorIndex < len(errors) || consumeIndex < len(consumes)) {
		if consumeIndex >= len(consumes) ||
			(errorIndex < len(errors) && errors[errorIndex].Id <= consumes[consumeIndex].Id) {
			merged = append(merged, errors[errorIndex])
			errorIndex++
		} else {
			merged = append(merged, consumes[consumeIndex])
			consumeIndex++
		}
	}
	return merged, nil
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
