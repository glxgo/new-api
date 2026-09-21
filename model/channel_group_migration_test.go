package model

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestChannelGroupDialectTypes(t *testing.T) {
	s, err := schema.Parse(&Channel{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)
	f := s.LookUpField("Group")
	require.False(t, f.HasDefaultValue, "MySQL 5.7 must not receive a TEXT default")
	require.Equal(t, "longtext", mysql.New(mysql.Config{}).DataTypeOf(f))
	require.Equal(t, "text", postgres.New(postgres.Config{}).DataTypeOf(f))
}

func TestChannelGroupLongList(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE channels (id INTEGER PRIMARY KEY, `group` VARCHAR(64) DEFAULT 'default')").Error)
	checkChannelGroupLongList(t, db)
}

// Opt-in integration test against a disposable local schema, never production.
func TestChannelGroupMySQLMigration(t *testing.T) {
	dsn := os.Getenv("CHANNEL_GROUP_TEST_DSN")
	if dsn == "" {
		t.Skip("local disposable MySQL schema not configured")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE channels (id BIGINT AUTO_INCREMENT PRIMARY KEY, `group` VARCHAR(64) DEFAULT 'default')").Error)
	require.NoError(t, db.Exec("INSERT INTO channels (`group`) VALUES ('original,vip')").Error)
	require.NoError(t, migrateChannelGroupsToLongText(db))
	var kept string
	require.NoError(t, db.Table("channels").Select("`group`").Where("id = 1").Scan(&kept).Error)
	require.Equal(t, "original,vip", kept)
	checkChannelGroupLongList(t, db)
	var typ string
	require.NoError(t, db.Raw("SELECT DATA_TYPE FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name='channels' AND column_name='group'").Scan(&typ).Error)
	require.Equal(t, "longtext", typ)
	// Exercise the metrics migration and aggregate SQL with the same dialect.
	require.NoError(t, db.AutoMigrate(&ChannelPerfMetric{}))
	require.NoError(t, db.Migrator().DropColumn(&ChannelPerfMetric{}, "BucketSeconds"))
	require.NoError(t, db.AutoMigrate(&ChannelPerfMetric{}))
	require.NoError(t, db.Create(&ChannelPerfMetric{ModelName: "a", ChannelId: 77, BucketTs: 120, BucketSeconds: 60, RequestCount: 9, SuccessCount: 8, TtftSumMs: 500, TtftCount: 2}).Error)
	rows, err := GetChannelMetricTotals(t.Context(), 120, 180, []int{77})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.EqualValues(t, 9, rows[0].RequestCount)
	require.EqualValues(t, 0, rows[0].LegacyBuckets)
	require.True(t, db.Migrator().HasIndex(&ChannelPerfMetric{}, "idx_channel_perf_channel_bucket"))
}

func checkChannelGroupLongList(t *testing.T, db *gorm.DB) {
	t.Helper()
	oldDB, oldGroup := DB, commonGroupCol
	DB, commonGroupCol = db, "`group`"
	t.Cleanup(func() { DB, commonGroupCol = oldDB, oldGroup; sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	for range 2 {
		require.NoError(t, migrateChannelGroupsToLongText(db))
		require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))
	}
	groups := make([]string, 5000)
	for i := range groups {
		groups[i] = fmt.Sprintf("测试分组-%04d", i)
	}
	want := strings.Join(groups, ",")
	require.Greater(t, len(want), 65535)
	channel := Channel{Name: "long", Key: "test", Models: "test-model", Group: want, Status: 1}
	require.NoError(t, channel.Insert())
	var got Channel
	require.NoError(t, db.First(&got, channel.Id).Error)
	require.Equal(t, want, got.Group)
	var count int64
	require.NoError(t, db.Model(&Ability{}).Where("channel_id = ?", channel.Id).Count(&count).Error)
	require.EqualValues(t, len(groups), count)
	channel.Group += ",新增分组"
	require.NoError(t, channel.Update())
	require.NoError(t, db.First(&got, channel.Id).Error)
	require.Equal(t, channel.Group, got.Group)
	require.NoError(t, BatchInsertChannels([]Channel{{Name: "batch-default", Key: "test", Models: "test-model"}, {Name: "batch-long", Key: "test", Models: "test-model", Group: want}}))
	var defaults Channel
	require.NoError(t, db.Where("name = ?", "batch-default").First(&defaults).Error)
	require.Equal(t, "default", defaults.Group)
	require.NoError(t, db.AutoMigrate(&Channel{}))
	require.NoError(t, db.First(&got, channel.Id).Error)
	require.Equal(t, channel.Group, got.Group)
}
