/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// LogQueryIndexMigrationEnv gates creation of the optional composite indexes
// used by administrator log searches, token usage lookups, and retention.
// These indexes can require a table scan/metadata lock on a large existing
// logs table, so they are deliberately excluded from ordinary AutoMigrate.
const LogQueryIndexMigrationEnv = "LOG_QUERY_INDEX_MIGRATION"

// logQueryIndexSchema describes only the columns and indexes that are added
// for the high-volume log query paths.  It intentionally has no relationship
// to Log's AutoMigrate tags: keeping these definitions separate prevents a
// normal application boot from implicitly issuing CREATE INDEX on logs.
//
// The table name is supplied explicitly because this is a schema-only helper;
// no rows are ever read or written through this type.
type logQueryIndexSchema struct {
	Quota     int    `gorm:"index:idx_logs_token_usage_quota,priority:5"`
	Username  string `gorm:"index:idx_username_created_at,priority:1;index:idx_username_type_created_at,priority:1"`
	Type      int    `gorm:"index:idx_username_type_created_at,priority:2;index:idx_log_retention,priority:1;index:idx_logs_token_usage,priority:2;index:idx_logs_token_usage_quota,priority:2"`
	CreatedAt int64  `gorm:"type:bigint;index:idx_username_created_at,priority:2;index:idx_username_type_created_at,priority:3;index:idx_log_retention,priority:3;index:idx_logs_token_usage,priority:4;index:idx_logs_token_usage_quota,priority:4"`
	Id        int    `gorm:"index:idx_username_created_at,priority:3;index:idx_log_retention,priority:4"`
	TokenId   int    `gorm:"column:token_id;index:idx_logs_token_usage,priority:1;index:idx_logs_token_usage_quota,priority:1"`
	Settled   bool   `gorm:"column:settled;index:idx_logs_token_usage,priority:3;index:idx_log_retention,priority:2;index:idx_logs_token_usage_quota,priority:3"`
}

func (logQueryIndexSchema) TableName() string { return "logs" }

type logQueryIndexDefinition struct {
	name string
}

var logQueryIndexDefinitions = [...]logQueryIndexDefinition{
	{name: "idx_username_created_at"},
	{name: "idx_username_type_created_at"},
	{name: "idx_logs_token_usage"},
	{name: "idx_logs_token_usage_quota"},
	{name: "idx_log_retention"},
}

// MigrateLogQueryIndexes creates the optional composite log indexes in an
// idempotent, operator-controlled step.  It uses GORM's dialect-specific
// migrator, so SQLite, MySQL and PostgreSQL receive their native quoting and
// index DDL without database-specific SQL in application code.
func MigrateLogQueryIndexes(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("log database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	db = db.WithContext(ctx)
	migrator := db.Migrator()
	if !migrator.HasTable(&Log{}) {
		return errors.New("logs table does not exist")
	}
	for _, definition := range logQueryIndexDefinitions {
		if migrator.HasIndex(&logQueryIndexSchema{}, definition.name) {
			continue
		}
		if err := migrator.CreateIndex(&logQueryIndexSchema{}, definition.name); err != nil {
			return fmt.Errorf("create log query index %s: %w", definition.name, err)
		}
	}
	return nil
}

// MaybeMigrateLogQueryIndexes keeps ordinary startup free of potentially
// expensive index DDL.  Set LOG_QUERY_INDEX_MIGRATION=true only during a
// reviewed maintenance window, or call MigrateLogQueryIndexes directly from
// an operator-controlled migration command.
func MaybeMigrateLogQueryIndexes(ctx context.Context, db *gorm.DB) error {
	if !common.GetEnvOrDefaultBool(LogQueryIndexMigrationEnv, false) {
		return nil
	}
	return MigrateLogQueryIndexes(ctx, db)
}
