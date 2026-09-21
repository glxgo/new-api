package model

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UsageMetricGranularity identifies the wall-clock size of a usage metric
// bucket. The caller supplies the location used to calculate BucketStart;
// this keeps day/hour boundaries stable when the writer and reader run in
// different server time zones.
type UsageMetricGranularity string

const (
	UsageMetricGranularityMinute UsageMetricGranularity = "minute"
	UsageMetricGranularityHour   UsageMetricGranularity = "hour"
	UsageMetricGranularityDay    UsageMetricGranularity = "day"

	// Short aliases make call sites read naturally while retaining the more
	// explicit names above for public API documentation.
	UsageMetricHour   = UsageMetricGranularityHour
	UsageMetricDay    = UsageMetricGranularityDay
	UsageMetricMinute = UsageMetricGranularityMinute
)

const (
	// UsageMetricSourceVersionV1 is the initial projection algorithm version.
	// A worker should use a stable version string for a whole projection pass;
	// changing it is an explicit re-computation/migration decision.
	UsageMetricSourceVersionV1 = "v1"
	usageMetricDimensionHashV1 = "usage-metric-dimension-v1"
)

// UsageMetricBucket is a durable, query-oriented projection of usage logs.
//
// It intentionally contains no request content, IP, request IDs, or free-form
// metadata. Settled is part of the dimension so a later reconciliation pass
// can distinguish pending rows from settled billing rows without silently
// changing the meaning of an existing bucket. Consumers that report billed
// usage should explicitly filter settled=true (or define their own pending
// policy) rather than assuming the projection is a financial ledger.
//
// The projection is introduced behind an explicit migration and a disabled
// by-default worker. It never replaces the raw log/audit source of truth.
type UsageMetricBucket struct {
	Id             int                    `json:"id" gorm:"primaryKey"`
	Granularity    UsageMetricGranularity `json:"granularity" gorm:"type:varchar(8);not null;uniqueIndex:uk_usage_metric_bucket,priority:1;index:idx_usage_metric_range,priority:1;index:idx_usage_metric_type,priority:1"`
	BucketStart    int64                  `json:"bucket_start" gorm:"type:bigint;not null;uniqueIndex:uk_usage_metric_bucket,priority:2;index:idx_usage_metric_range,priority:2;index:idx_usage_metric_type,priority:2"`
	UserId         int                    `json:"user_id" gorm:"not null;default:0;index:idx_usage_metric_range,priority:3"`
	Type           int                    `json:"type" gorm:"not null;default:0;index:idx_usage_metric_type,priority:3"`
	Settled        bool                   `json:"settled" gorm:"not null;default:false;index:idx_usage_metric_settled"`
	ModelName      string                 `json:"model_name" gorm:"type:varchar(191);not null;default:'';index:idx_usage_metric_model"`
	ChannelId      int                    `json:"channel_id" gorm:"not null;default:0"`
	TokenId        int                    `json:"token_id" gorm:"not null;default:0"`
	GroupName      string                 `json:"group" gorm:"column:group_name;type:varchar(64);not null;default:''"`
	BillingSource  string                 `json:"billing_source" gorm:"type:varchar(32);not null;default:''"`
	SubscriptionId int                    `json:"subscription_id" gorm:"not null;default:0"`

	// DimensionHash is a fixed-width, binary-safe unique-key component. A
	// composite unique index over all the human-readable dimensions would be
	// too wide on some MySQL 5.7 installations with utf8mb4.
	DimensionHash string `json:"dimension_hash" gorm:"type:char(64);not null;uniqueIndex:uk_usage_metric_bucket,priority:3"`

	RequestCount          int64 `json:"request_count" gorm:"type:bigint;not null;default:0"`
	SuccessCount          int64 `json:"success_count" gorm:"type:bigint;not null;default:0"`
	ErrorCount            int64 `json:"error_count" gorm:"type:bigint;not null;default:0"`
	StreamCount           int64 `json:"stream_count" gorm:"type:bigint;not null;default:0"`
	Quota                 int64 `json:"quota" gorm:"type:bigint;not null;default:0"`
	PreDiscountQuota      int64 `json:"pre_discount_quota" gorm:"type:bigint;not null;default:0"`
	PromptTokens          int64 `json:"prompt_tokens" gorm:"type:bigint;not null;default:0"`
	CacheTokens           int64 `json:"cache_tokens" gorm:"type:bigint;not null;default:0"`
	EffectivePromptTokens int64 `json:"effective_prompt_tokens" gorm:"type:bigint;not null;default:0"`
	CompletionTokens      int64 `json:"completion_tokens" gorm:"type:bigint;not null;default:0"`
	UseTime               int64 `json:"use_time" gorm:"type:bigint;not null;default:0"`
	Cost                  int64 `json:"cost" gorm:"type:bigint;not null;default:0"`
	PaidQuota             int64 `json:"paid_quota" gorm:"type:bigint;not null;default:0"`
	PaidGiftQuota         int64 `json:"paid_gift_quota" gorm:"type:bigint;not null;default:0"`

	FirstLogAt int64 `json:"first_log_at" gorm:"type:bigint;not null;default:0"`
	LastLogAt  int64 `json:"last_log_at" gorm:"type:bigint;not null;default:0"`

	// Audit/projection metadata. Watermark is the maximum source Log.Id known
	// to be included in this projection pass, not a timestamp.
	ComputedAt    int64  `json:"computed_at" gorm:"type:bigint;not null;default:0"`
	Watermark     int64  `json:"watermark" gorm:"type:bigint;not null;default:0"`
	SourceVersion string `json:"source_version" gorm:"type:varchar(128);not null;default:''"`
	Timezone      string `json:"timezone" gorm:"type:varchar(64);not null;default:'UTC'"`
	CoverageStart int64  `json:"coverage_start" gorm:"type:bigint;not null;default:0"`
	CoverageEnd   int64  `json:"coverage_end" gorm:"type:bigint;not null;default:0"`
	IsComplete    bool   `json:"is_complete" gorm:"not null;default:false;index:idx_usage_metric_complete"`
}

func (UsageMetricBucket) TableName() string {
	return "usage_metric_buckets"
}

type usageMetricDimensions struct {
	UserId         int
	Type           int
	Settled        bool
	ModelName      string
	ChannelId      int
	TokenId        int
	GroupName      string
	BillingSource  string
	SubscriptionId int
}

type usageMetricGroupKey struct {
	Granularity UsageMetricGranularity
	BucketStart int64
	usageMetricDimensions
}

func (bucket UsageMetricBucket) dimensions() usageMetricDimensions {
	return usageMetricDimensions{
		UserId:         bucket.UserId,
		Type:           bucket.Type,
		Settled:        bucket.Settled,
		ModelName:      bucket.ModelName,
		ChannelId:      bucket.ChannelId,
		TokenId:        bucket.TokenId,
		GroupName:      bucket.GroupName,
		BillingSource:  bucket.BillingSource,
		SubscriptionId: bucket.SubscriptionId,
	}
}

// UsageMetricDimensionHash returns the stable SHA-256 hash used by the
// bucket's unique key. Length-prefixing strings avoids ambiguous concatenated
// values (for example, ["ab", "c"] versus ["a", "bc"]).
func UsageMetricDimensionHash(bucket UsageMetricBucket) string {
	return usageMetricDimensionHash(bucket.dimensions())
}

func usageMetricDimensionHash(dimensions usageMetricDimensions) string {
	encoded := make([]byte, 0, 128)
	appendString := func(value string) {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		encoded = append(encoded, length[:]...)
		encoded = append(encoded, value...)
	}
	appendInt := func(value int) {
		var number [8]byte
		binary.BigEndian.PutUint64(number[:], uint64(int64(value)))
		encoded = append(encoded, number[:]...)
	}
	appendString(usageMetricDimensionHashV1)
	appendInt(dimensions.UserId)
	appendInt(dimensions.Type)
	if dimensions.Settled {
		encoded = append(encoded, 1)
	} else {
		encoded = append(encoded, 0)
	}
	appendString(dimensions.ModelName)
	appendInt(dimensions.ChannelId)
	appendInt(dimensions.TokenId)
	appendString(dimensions.GroupName)
	appendString(dimensions.BillingSource)
	appendInt(dimensions.SubscriptionId)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// RefreshDimensionHash recomputes DimensionHash after a caller changes one of
// the dimensions. It returns the resulting value for convenient construction.
func (bucket *UsageMetricBucket) RefreshDimensionHash() string {
	if bucket == nil {
		return ""
	}
	bucket.DimensionHash = UsageMetricDimensionHash(*bucket)
	return bucket.DimensionHash
}

func validUsageMetricGranularity(granularity UsageMetricGranularity) bool {
	return granularity == UsageMetricGranularityMinute || granularity == UsageMetricGranularityHour || granularity == UsageMetricGranularityDay
}

// UsageMetricGranularitySeconds returns the fixed wall-clock width used by a
// projection bucket. The caller is still responsible for applying the
// configured timezone when calculating the bucket start.
func UsageMetricGranularitySeconds(granularity UsageMetricGranularity) (int64, error) {
	switch granularity {
	case UsageMetricGranularityMinute:
		return 60, nil
	case UsageMetricGranularityHour:
		return 60 * 60, nil
	case UsageMetricGranularityDay:
		return 24 * 60 * 60, nil
	default:
		return 0, fmt.Errorf("unsupported usage metric granularity %q", granularity)
	}
}

// UsageMetricBucketStart converts a source timestamp to the start of its
// local hour/day bucket. A location is mandatory: relying on time.Local would
// make the same source rows land in different buckets on different servers.
func UsageMetricBucketStart(timestamp int64, granularity UsageMetricGranularity, location *time.Location) (int64, error) {
	if timestamp <= 0 {
		return 0, errors.New("usage metric timestamp must be positive")
	}
	if location == nil {
		return 0, errors.New("usage metric location is required")
	}
	if !validUsageMetricGranularity(granularity) {
		return 0, fmt.Errorf("unsupported usage metric granularity %q", granularity)
	}
	value := time.Unix(timestamp, 0).In(location)
	if granularity == UsageMetricGranularityMinute {
		return time.Date(value.Year(), value.Month(), value.Day(), value.Hour(), value.Minute(), 0, 0, location).Unix(), nil
	}
	hour := value.Hour()
	if granularity == UsageMetricGranularityDay {
		hour = 0
	}
	return time.Date(value.Year(), value.Month(), value.Day(), hour, 0, 0, 0, location).Unix(), nil
}

func usageMetricEligibleLog(log Log) bool {
	return log.Type == LogTypeConsume || log.Type == LogTypeError
}

func addUsageMetricValue(target *int64, value int64) error {
	// Avoid silently wrapping a durable counter if malformed input reaches the
	// projection. Log fields are normally small, but this check makes the pure
	// builder safe for replay/import tooling too.
	if value > 0 && *target > int64(^uint64(0)>>1)-value {
		return errors.New("usage metric counter overflow")
	}
	if value < 0 && *target < -int64(^uint64(0)>>1)-1-value {
		return errors.New("usage metric counter underflow")
	}
	*target += value
	return nil
}

func addUsageMetricLog(item *UsageMetricBucket, log Log) error {
	if err := addUsageMetricValue(&item.RequestCount, 1); err != nil {
		return err
	}
	if log.Type == LogTypeConsume {
		if err := addUsageMetricValue(&item.SuccessCount, 1); err != nil {
			return err
		}
	} else if log.Type == LogTypeError {
		if err := addUsageMetricValue(&item.ErrorCount, 1); err != nil {
			return err
		}
	}
	if log.IsStream {
		if err := addUsageMetricValue(&item.StreamCount, 1); err != nil {
			return err
		}
	}
	values := []*int64{
		&item.Quota,
		&item.PromptTokens,
		&item.CacheTokens,
		&item.CompletionTokens,
		&item.UseTime,
		&item.Cost,
		&item.PaidQuota,
		&item.PaidGiftQuota,
	}
	logValues := []int64{
		int64(log.Quota),
		int64(log.PromptTokens),
		int64(log.CacheTokens),
		int64(log.CompletionTokens),
		int64(log.UseTime),
		int64(log.Cost),
		int64(log.PaidQuota),
		int64(log.PaidGiftQuota),
	}
	for index := range values {
		if err := addUsageMetricValue(values[index], logValues[index]); err != nil {
			return err
		}
	}
	preDiscount := int64(log.PreDiscountQuota)
	if log.PreDiscountQuota <= 0 {
		preDiscount = int64(log.Quota)
	}
	if err := addUsageMetricValue(&item.PreDiscountQuota, preDiscount); err != nil {
		return err
	}
	if log.PromptTokens > 0 {
		effectivePrompt := int64(log.PromptTokens)
		if log.CacheTokens > log.PromptTokens {
			if err := addUsageMetricValue(&effectivePrompt, int64(log.CacheTokens)); err != nil {
				return err
			}
		}
		if err := addUsageMetricValue(&item.EffectivePromptTokens, effectivePrompt); err != nil {
			return err
		}
	}
	if item.FirstLogAt == 0 || log.CreatedAt < item.FirstLogAt {
		item.FirstLogAt = log.CreatedAt
	}
	if log.CreatedAt > item.LastLogAt {
		item.LastLogAt = log.CreatedAt
	}
	return nil
}

// BuildUsageMetricBuckets creates deterministic full-bucket snapshots from a
// batch of source logs. Only consume and error logs are included; top-up,
// audit, login and other administrative rows are deliberately excluded.
//
// watermark is the source-log high-water mark for the pass. Passing zero for
// a non-empty batch derives it from the largest Log.Id. A non-zero watermark
// smaller than an input Log.Id is rejected because it would make the audit
// metadata claim less progress than the data actually included.
func BuildUsageMetricBuckets(
	logs []Log,
	granularity UsageMetricGranularity,
	location *time.Location,
	sourceVersion string,
	computedAt int64,
	watermark int64,
) ([]UsageMetricBucket, error) {
	if !validUsageMetricGranularity(granularity) {
		return nil, fmt.Errorf("unsupported usage metric granularity %q", granularity)
	}
	if location == nil {
		return nil, errors.New("usage metric location is required")
	}
	sourceVersion = strings.TrimSpace(sourceVersion)
	if sourceVersion == "" {
		return nil, errors.New("usage metric source version is required")
	}
	if computedAt <= 0 {
		return nil, errors.New("usage metric computed_at must be positive")
	}
	if watermark < 0 {
		return nil, errors.New("usage metric watermark cannot be negative")
	}

	maxLogID := int64(0)
	grouped := make(map[usageMetricGroupKey]*UsageMetricBucket)
	for index := range logs {
		log := logs[index]
		if !usageMetricEligibleLog(log) {
			continue
		}
		if int64(log.Id) > maxLogID {
			maxLogID = int64(log.Id)
		}
		bucketStart, err := UsageMetricBucketStart(log.CreatedAt, granularity, location)
		if err != nil {
			return nil, fmt.Errorf("log %d: %w", log.Id, err)
		}
		key := usageMetricGroupKey{
			Granularity: granularity,
			BucketStart: bucketStart,
			usageMetricDimensions: usageMetricDimensions{
				UserId:         log.UserId,
				Type:           log.Type,
				Settled:        log.Settled,
				ModelName:      log.ModelName,
				ChannelId:      log.ChannelId,
				TokenId:        log.TokenId,
				GroupName:      log.Group,
				BillingSource:  log.BillingSource,
				SubscriptionId: log.SubscriptionId,
			},
		}
		item := grouped[key]
		if item == nil {
			item = &UsageMetricBucket{
				Granularity:    granularity,
				BucketStart:    bucketStart,
				UserId:         log.UserId,
				Type:           log.Type,
				Settled:        log.Settled,
				ModelName:      log.ModelName,
				ChannelId:      log.ChannelId,
				TokenId:        log.TokenId,
				GroupName:      log.Group,
				BillingSource:  log.BillingSource,
				SubscriptionId: log.SubscriptionId,
				ComputedAt:     computedAt,
				SourceVersion:  sourceVersion,
			}
			grouped[key] = item
		}
		if err := addUsageMetricLog(item, log); err != nil {
			return nil, fmt.Errorf("aggregate log %d: %w", log.Id, err)
		}
	}
	if watermark == 0 {
		watermark = maxLogID
	} else if maxLogID > watermark {
		return nil, fmt.Errorf("usage metric watermark %d is below input log id %d", watermark, maxLogID)
	}

	result := make([]UsageMetricBucket, 0, len(grouped))
	for _, item := range grouped {
		item.Watermark = watermark
		item.CoverageStart = item.BucketStart
		coverageEnd, endErr := usageMetricReadBucketEnd(item.BucketStart, granularity, location)
		if endErr != nil {
			return nil, endErr
		}
		item.CoverageEnd = coverageEnd
		item.RefreshDimensionHash()
		result = append(result, *item)
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Granularity != result[right].Granularity {
			return result[left].Granularity < result[right].Granularity
		}
		if result[left].BucketStart != result[right].BucketStart {
			return result[left].BucketStart < result[right].BucketStart
		}
		if result[left].DimensionHash != result[right].DimensionHash {
			return result[left].DimensionHash < result[right].DimensionHash
		}
		return result[left].UserId < result[right].UserId
	})
	return result, nil
}

func validateUsageMetricBucket(bucket UsageMetricBucket) error {
	if !validUsageMetricGranularity(bucket.Granularity) {
		return fmt.Errorf("unsupported usage metric granularity %q", bucket.Granularity)
	}
	if strings.TrimSpace(bucket.DimensionHash) == "" {
		return errors.New("usage metric dimension hash is required")
	}
	if bucket.DimensionHash != UsageMetricDimensionHash(bucket) {
		return errors.New("usage metric dimension hash does not match dimensions")
	}
	if strings.TrimSpace(bucket.SourceVersion) == "" {
		return errors.New("usage metric source version is required")
	}
	if bucket.ComputedAt <= 0 {
		return errors.New("usage metric computed_at must be positive")
	}
	if bucket.Watermark < 0 {
		return errors.New("usage metric watermark cannot be negative")
	}
	if bucket.CoverageStart < 0 || bucket.CoverageEnd < 0 || (bucket.CoverageEnd > 0 && bucket.CoverageStart >= bucket.CoverageEnd) {
		return errors.New("usage metric coverage range is invalid")
	}
	if bucket.FirstLogAt < 0 || bucket.LastLogAt < 0 {
		return errors.New("usage metric log timestamps cannot be negative")
	}
	if bucket.FirstLogAt > 0 && bucket.LastLogAt > 0 && bucket.FirstLogAt > bucket.LastLogAt {
		return errors.New("usage metric first_log_at is after last_log_at")
	}
	return nil
}

// ValidateUsageMetricBuckets checks persistence invariants and duplicate
// unique keys before a write begins.
func ValidateUsageMetricBuckets(buckets []UsageMetricBucket) error {
	seen := make(map[string]struct{}, len(buckets))
	for index := range buckets {
		bucket := buckets[index]
		if err := validateUsageMetricBucket(bucket); err != nil {
			return fmt.Errorf("bucket %d: %w", index, err)
		}
		key := fmt.Sprintf("%s\x00%d\x00%s", bucket.Granularity, bucket.BucketStart, bucket.DimensionHash)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate usage metric bucket key %q", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

var usageMetricBucketUpdateColumns = []string{
	"user_id", "type", "settled", "model_name", "channel_id", "token_id", "group_name",
	"billing_source", "subscription_id", "dimension_hash", "request_count", "success_count",
	"error_count", "stream_count", "quota", "pre_discount_quota", "prompt_tokens", "cache_tokens",
	"effective_prompt_tokens", "completion_tokens", "use_time", "cost", "paid_quota", "paid_gift_quota",
	"first_log_at", "last_log_at", "computed_at", "watermark", "source_version",
	"timezone", "coverage_start", "coverage_end", "is_complete",
}

// UpsertUsageMetricBuckets writes complete snapshots atomically. On a unique
// key conflict the incoming snapshot replaces the previous counters; it does
// not add them. This is intentional: replaying the same source batch is
// idempotent and cannot double-count. A future incremental worker should first
// read/validate its source watermark and then build a complete bucket snapshot
// (or introduce a separately specified delta table).
func UpsertUsageMetricBuckets(db *gorm.DB, buckets []UsageMetricBucket) error {
	return UpsertUsageMetricBucketsWithContext(context.Background(), db, buckets)
}

func UpsertUsageMetricBucketsWithContext(ctx context.Context, db *gorm.DB, buckets []UsageMetricBucket) error {
	if db == nil {
		return errors.New("usage metric database is unavailable")
	}
	if len(buckets) == 0 {
		return nil
	}
	if err := ValidateUsageMetricBuckets(buckets); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// Clear caller-owned primary keys. The projection identity is the explicit
	// three-column unique key, not a copied row id from another database.
	normalized := make([]UsageMetricBucket, len(buckets))
	copy(normalized, buckets)
	for index := range normalized {
		normalized[index].Id = 0
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return upsertUsageMetricBucketsTx(tx, normalized)
	})
}

func upsertUsageMetricBucketsTx(tx *gorm.DB, buckets []UsageMetricBucket) error {
	if tx == nil || len(buckets) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "granularity"},
			{Name: "bucket_start"},
			{Name: "dimension_hash"},
		},
		DoUpdates: clause.AssignmentColumns(usageMetricBucketUpdateColumns),
	}).CreateInBatches(&buckets, 20).Error
}

// ReplaceUsageMetricBucketWindow atomically replaces all dimensions for the
// supplied bucket starts. Replacing (rather than only upserting) removes stale
// dimensions when a source log changes settled state or is deleted during a
// reconciliation pass.
func ReplaceUsageMetricBucketWindow(ctx context.Context, db *gorm.DB, granularity UsageMetricGranularity, bucketStarts []int64, buckets []UsageMetricBucket) error {
	if db == nil {
		return errors.New("usage metric database is unavailable")
	}
	if !validUsageMetricGranularity(granularity) {
		return fmt.Errorf("unsupported usage metric granularity %q", granularity)
	}
	if len(bucketStarts) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ValidateUsageMetricBuckets(buckets); err != nil {
		return err
	}
	normalized := append([]UsageMetricBucket(nil), buckets...)
	for index := range normalized {
		normalized[index].Id = 0
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("granularity = ? AND bucket_start IN ?", granularity, bucketStarts).Delete(&UsageMetricBucket{}).Error; err != nil {
			return err
		}
		return upsertUsageMetricBucketsTx(tx, normalized)
	})
}

// UsageMetricBucketQuery describes a bounded read of the projection. The
// time range is half-open [StartTime, EndTime). UserId=0 means all users,
// while Type/Settled nil means no filter for that dimension.
type UsageMetricBucketQuery struct {
	Granularity    UsageMetricGranularity
	StartTime      int64
	EndTime        int64
	UserId         int
	Type           *int
	Settled        *bool
	ModelName      string
	ChannelId      *int
	TokenId        *int
	BillingSource  string
	SubscriptionId *int
}

func ListUsageMetricBuckets(ctx context.Context, db *gorm.DB, query UsageMetricBucketQuery) ([]UsageMetricBucket, error) {
	if db == nil {
		return nil, errors.New("usage metric database is unavailable")
	}
	if !validUsageMetricGranularity(query.Granularity) {
		return nil, fmt.Errorf("unsupported usage metric granularity %q", query.Granularity)
	}
	if query.StartTime < 0 || query.EndTime <= query.StartTime {
		return nil, errors.New("invalid usage metric time range")
	}
	if query.UserId < 0 {
		return nil, errors.New("usage metric user id cannot be negative")
	}
	if query.Type != nil && *query.Type < 0 {
		return nil, errors.New("usage metric type cannot be negative")
	}
	if query.ChannelId != nil && *query.ChannelId < 0 {
		return nil, errors.New("usage metric channel id cannot be negative")
	}
	if query.TokenId != nil && *query.TokenId < 0 {
		return nil, errors.New("usage metric token id cannot be negative")
	}
	if query.SubscriptionId != nil && *query.SubscriptionId < 0 {
		return nil, errors.New("usage metric subscription id cannot be negative")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	tx := db.WithContext(ctx).Model(&UsageMetricBucket{}).
		Where("granularity = ? AND bucket_start >= ? AND bucket_start < ?", query.Granularity, query.StartTime, query.EndTime)
	if query.UserId > 0 {
		tx = tx.Where("user_id = ?", query.UserId)
	}
	if query.Type != nil {
		tx = tx.Where("type = ?", *query.Type)
	}
	if query.Settled != nil {
		tx = tx.Where("settled = ?", *query.Settled)
	}
	if query.ModelName != "" {
		tx = tx.Where("model_name = ?", query.ModelName)
	}
	if query.ChannelId != nil {
		tx = tx.Where("channel_id = ?", *query.ChannelId)
	}
	if query.TokenId != nil {
		tx = tx.Where("token_id = ?", *query.TokenId)
	}
	if query.BillingSource != "" {
		tx = tx.Where("billing_source = ?", query.BillingSource)
	}
	if query.SubscriptionId != nil {
		tx = tx.Where("subscription_id = ?", *query.SubscriptionId)
	}
	var result []UsageMetricBucket
	err := tx.Order("bucket_start ASC, user_id ASC, type ASC, settled ASC, dimension_hash ASC").Find(&result).Error
	return result, err
}

// ReadUsageMetricBuckets is the stable read-side name for callers that do not
// need to know whether the backing implementation is a table, view, or a
// future remote read replica. It currently delegates to the bounded local
// query helper and keeps the context/database explicit.
func ReadUsageMetricBuckets(ctx context.Context, db *gorm.DB, query UsageMetricBucketQuery) ([]UsageMetricBucket, error) {
	return ListUsageMetricBuckets(ctx, db, query)
}

// UsageMetricBucketsAvailable reports whether at least one projection row is
// readable. It is deliberately a cheap existence probe and does not imply
// freshness, completeness, or financial consistency; callers must inspect
// ComputedAt/Watermark and apply their own freshness policy.
func UsageMetricBucketsAvailable(ctx context.Context, db *gorm.DB) (bool, error) {
	if db == nil {
		return false, errors.New("usage metric database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var marker UsageMetricBucket
	err := db.WithContext(ctx).Model(&UsageMetricBucket{}).Select("id").Limit(1).Take(&marker).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
