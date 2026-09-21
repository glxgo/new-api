package model

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openLogQueryIndexMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:log-query-index-migration-%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	return db
}

func TestLogQueryIndexesAreOptIn(t *testing.T) {
	db := openLogQueryIndexMigrationTestDB(t)
	for _, definition := range logQueryIndexDefinitions {
		require.False(t, db.Migrator().HasIndex(&logQueryIndexSchema{}, definition.name), definition.name)
	}

	t.Setenv(LogQueryIndexMigrationEnv, "false")
	require.NoError(t, MaybeMigrateLogQueryIndexes(context.Background(), db))
	for _, definition := range logQueryIndexDefinitions {
		require.False(t, db.Migrator().HasIndex(&logQueryIndexSchema{}, definition.name), definition.name)
	}

	t.Setenv(LogQueryIndexMigrationEnv, "true")
	require.NoError(t, MaybeMigrateLogQueryIndexes(context.Background(), db))
	for _, definition := range logQueryIndexDefinitions {
		require.True(t, db.Migrator().HasIndex(&logQueryIndexSchema{}, definition.name), definition.name)
	}
	// A reviewed migration may be safely retried after an interrupted deploy.
	require.NoError(t, MigrateLogQueryIndexes(nil, db))
}

func TestMaybeMigrateLogQueryIndexesDoesNotRequireTableWhenDisabled(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:log-query-index-empty-%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	t.Setenv(LogQueryIndexMigrationEnv, "false")
	require.NoError(t, MaybeMigrateLogQueryIndexes(context.Background(), db))

	t.Setenv(LogQueryIndexMigrationEnv, "true")
	require.ErrorContains(t, MaybeMigrateLogQueryIndexes(context.Background(), db), "logs table does not exist")
}
