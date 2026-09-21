package model

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetAllLogsWithContextCursorStableOrder(t *testing.T) {
	oldDB, oldLogDB := DB, LOG_DB
	oldSQLite, oldMySQL, oldPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	DB, LOG_DB = db, db
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPostgres
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	rows := []Log{
		{UserId: 1, CreatedAt: 200, Type: LogTypeConsume, ModelName: "a"},
		{UserId: 1, CreatedAt: 200, Type: LogTypeConsume, ModelName: "b"},
		{UserId: 1, CreatedAt: 100, Type: LogTypeConsume, ModelName: "c"},
	}
	require.NoError(t, db.Create(&rows).Error)
	first, total, err := GetAllLogsWithContextCursor(context.Background(), LogTypeUnknown, 0, 300, "", "", "", 0, 2, 0, "", "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, first, 2)
	require.Equal(t, "b", first[0].ModelName)
	require.Equal(t, "a", first[1].ModelName)
	cursor := fmt.Sprintf("%d:%d", first[1].CreatedAt, first[1].Id)
	second, secondTotal, err := GetAllLogsWithContextCursor(context.Background(), LogTypeUnknown, 0, 300, "", "", "", 0, 2, 0, "", "", "", cursor)
	require.NoError(t, err)
	require.EqualValues(t, 0, secondTotal)
	require.Len(t, second, 1)
	require.Equal(t, "c", second[0].ModelName)
}

func TestLegacyLogPageTotalReflectsNewRows(t *testing.T) {
	oldDB, oldLogDB := DB, LOG_DB
	oldSQLite, oldMySQL, oldPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	DB, LOG_DB = db, db
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPostgres
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, db.Create(&Log{UserId: 1, CreatedAt: 100, Type: LogTypeConsume}).Error)
	_, total, err := GetUserLogsWithContext(context.Background(), 1, LogTypeUnknown, 0, 200, "", "", 0, 20, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)

	require.NoError(t, db.Create(&Log{UserId: 1, CreatedAt: 101, Type: LogTypeConsume}).Error)
	_, total, err = GetUserLogsWithContext(context.Background(), 1, LogTypeUnknown, 0, 200, "", "", 0, 20, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
}
