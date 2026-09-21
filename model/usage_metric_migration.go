package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// UsageMetricSchemaMigrationEnv is intentionally opt-in. Creating the
	// projection and its indexes can touch a large log database, so ordinary
	// application startup must never run this DDL implicitly.
	UsageMetricSchemaMigrationEnv = "USAGE_METRIC_SCHEMA_MIGRATION"
	UsageMetricWorkerEnableEnv    = "USAGE_METRIC_WORKER_ENABLE"
	UsageMetricReadEnableEnv      = "USAGE_METRIC_READ_ENABLE"
)

// UsageMetricCheckpoint stores durable progress for one projection worker.
// The name is scoped by schema/version so an algorithm change can replay from
// a fresh checkpoint without mutating the old projection's audit trail.
type UsageMetricCheckpoint struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	Name             string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex"`
	Watermark        int64  `json:"watermark" gorm:"type:bigint;not null;default:0"`
	LastReconciledAt int64  `json:"last_reconciled_at" gorm:"type:bigint;not null;default:0"`
	UpdatedAt        int64  `json:"updated_at" gorm:"type:bigint;not null;default:0"`
}

func (UsageMetricCheckpoint) TableName() string { return "usage_metric_checkpoints" }

// UsageMetricBatch is a compact idempotency ledger. Replaying a batch is
// still safe because bucket replacement is snapshot-based; the unique key
// makes the successful application observable after a process restart.
type UsageMetricBatch struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	BatchKey      string `json:"batch_key" gorm:"type:varchar(191);not null;uniqueIndex"`
	FromWatermark int64  `json:"from_watermark" gorm:"type:bigint;not null;default:0"`
	ToWatermark   int64  `json:"to_watermark" gorm:"type:bigint;not null;default:0"`
	AppliedAt     int64  `json:"applied_at" gorm:"type:bigint;not null;default:0"`
}

func (UsageMetricBatch) TableName() string { return "usage_metric_batches" }

// UsageMetricCoverage records an explicit empty/non-empty bucket marker.
// Without this table a missing row is ambiguous: it can mean either “no
// traffic” or “the worker has not reached this bucket yet”.
type UsageMetricCoverage struct {
	Id            int                    `json:"id" gorm:"primaryKey"`
	Granularity   UsageMetricGranularity `json:"granularity" gorm:"type:varchar(8);not null;uniqueIndex:uk_usage_metric_coverage,priority:1"`
	BucketStart   int64                  `json:"bucket_start" gorm:"type:bigint;not null;uniqueIndex:uk_usage_metric_coverage,priority:2"`
	BucketEnd     int64                  `json:"bucket_end" gorm:"type:bigint;not null;default:0"`
	Timezone      string                 `json:"timezone" gorm:"type:varchar(64);not null;default:'UTC'"`
	ComputedAt    int64                  `json:"computed_at" gorm:"type:bigint;not null;default:0"`
	Watermark     int64                  `json:"watermark" gorm:"type:bigint;not null;default:0"`
	SourceVersion string                 `json:"source_version" gorm:"type:varchar(128);not null;default:''"`
	IsComplete    bool                   `json:"is_complete" gorm:"not null;default:false;index:idx_usage_metric_coverage_complete"`
}

func (UsageMetricCoverage) TableName() string { return "usage_metric_coverage" }

// MigrateUsageMetricSchema explicitly creates the projection tables. Callers
// should run this from a reviewed maintenance step (or with the opt-in
// startup flag), never as an unconditional part of the normal boot path.
// Passing DB or LOG_DB is supported, which covers both the shared-database
// topology and an independent LOG_SQL_DSN topology.
func MigrateUsageMetricSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("usage metric database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := db.WithContext(ctx).AutoMigrate(
		// Wallet projections use the archive table's source-log tie breaker
		// when rebuilding historical days. Include the archive model in the
		// explicit migration so an operator-controlled migration also upgrades
		// pre-v2 installations before the retention worker writes new rows.
		&UsageLogDailyAggregate{},
		&UsageMetricBucket{},
		&UsageMetricCheckpoint{},
		&UsageMetricBatch{},
		&UsageMetricCoverage{},
		&WalletConsumeDailyAggregate{},
		&WalletConsumeDailyAggregateCoverage{},
		&WalletConsumeAggregateCheckpoint{},
	); err != nil {
		return err
	}
	invalidateWalletConsumeArchiveSchemaCache(db)
	return nil
}

// MaybeMigrateUsageMetricSchema is used only after LOG_DB has been opened.
// It is false by default and therefore cannot add a startup DDL surprise to
// existing installations.
func MaybeMigrateUsageMetricSchema(ctx context.Context) error {
	if !common.GetEnvOrDefaultBool(UsageMetricSchemaMigrationEnv, false) {
		return nil
	}
	return MigrateUsageMetricSchema(ctx, LOG_DB)
}

func UsageMetricWorkerEnabled() bool {
	return common.GetEnvOrDefaultBool(UsageMetricWorkerEnableEnv, false)
}

func UsageMetricReadEnabled() bool {
	return common.GetEnvOrDefaultBool(UsageMetricReadEnableEnv, false)
}

func usageMetricTimezoneName() string {
	value := strings.TrimSpace(common.GetEnvOrDefaultString("USAGE_METRIC_TIMEZONE", "UTC"))
	if value == "" {
		return "UTC"
	}
	return value
}

func UsageMetricLocation() (*time.Location, string, error) {
	name := usageMetricTimezoneName()
	location, err := time.LoadLocation(name)
	if err != nil {
		return nil, name, err
	}
	return location, name, nil
}

func LoadUsageMetricCheckpoint(ctx context.Context, db *gorm.DB, name string) (UsageMetricCheckpoint, error) {
	var checkpoint UsageMetricCheckpoint
	if db == nil {
		return checkpoint, errors.New("usage metric database is unavailable")
	}
	if strings.TrimSpace(name) == "" {
		return checkpoint, errors.New("usage metric checkpoint name is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	err := db.WithContext(ctx).Where("name = ?", name).First(&checkpoint).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return UsageMetricCheckpoint{Name: name}, nil
	}
	return checkpoint, err
}

func SaveUsageMetricCheckpoint(ctx context.Context, db *gorm.DB, checkpoint UsageMetricCheckpoint) error {
	if db == nil {
		return errors.New("usage metric database is unavailable")
	}
	if strings.TrimSpace(checkpoint.Name) == "" || checkpoint.Watermark < 0 || checkpoint.LastReconciledAt < 0 || checkpoint.UpdatedAt <= 0 {
		return errors.New("invalid usage metric checkpoint")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	checkpoint.Id = 0
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"watermark", "last_reconciled_at", "updated_at",
		}),
	}).Create(&checkpoint).Error
}

func RecordUsageMetricBatch(ctx context.Context, db *gorm.DB, batch UsageMetricBatch) error {
	if db == nil {
		return errors.New("usage metric database is unavailable")
	}
	if strings.TrimSpace(batch.BatchKey) == "" || batch.FromWatermark < 0 || batch.ToWatermark < batch.FromWatermark || batch.AppliedAt <= 0 {
		return errors.New("invalid usage metric batch")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	batch.Id = 0
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "batch_key"}},
		DoNothing: true,
	}).Create(&batch).Error
}

func UpsertUsageMetricCoverage(ctx context.Context, db *gorm.DB, coverage []UsageMetricCoverage) error {
	if db == nil {
		return errors.New("usage metric database is unavailable")
	}
	if len(coverage) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for index := range coverage {
		item := &coverage[index]
		if !validUsageMetricGranularity(item.Granularity) || item.BucketStart < 0 || item.BucketEnd <= item.BucketStart || item.ComputedAt <= 0 || item.Watermark < 0 || strings.TrimSpace(item.SourceVersion) == "" {
			return errors.New("invalid usage metric coverage")
		}
		item.Id = 0
	}
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "granularity"}, {Name: "bucket_start"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"bucket_end", "timezone", "computed_at", "watermark", "source_version", "is_complete",
		}),
	}).CreateInBatches(&coverage, 50).Error
}
