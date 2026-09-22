package service

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/pelicanarchive"
	"github.com/glebarez/sqlite"
	gomysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func bootstrapDB(t *testing.T, dialect gorm.Dialector) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(dialect, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	tables, err := db.Migrator().GetTables()
	require.NoError(t, err)
	require.Empty(t, tables, "refuse a nonempty database")
	require.NoError(t, model.MigratePelicanSchema(db))
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Option{}, &model.User{}))
	return db
}

func bootstrapFixture(t *testing.T, db *gorm.DB) (pelicanarchive.Snapshot, PelicanBindings) {
	t.Helper()
	require.NoError(t, db.Create(&model.User{Id: 71, Username: "bootstrap-fixture", Role: common.RoleRootUser, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Option{Key: "GroupRatio", Value: `{"A":1.5,"B":2}`}).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 83, Key: "untouched-secret", Models: "m", Group: "A,B", Status: common.ChannelStatusManuallyDisabled}).Error)
	require.NoError(t, db.Create(&[]model.Ability{{ChannelId: 83, Group: "A", Model: "m", Enabled: false}, {ChannelId: 83, Group: "B", Model: "m", Enabled: false}}).Error)
	key := "A"
	require.NoError(t, db.Create(&model.CapabilityGroupPresentation{GroupUID: "existing-group-uid", RoutingKey: &key}).Error)
	now := time.Now().UTC()
	s := pelicanarchive.Snapshot{Schema: 1, SourceID: "fixture", CapturedAt: now.Format(time.RFC3339Nano), Config: pelicanarchive.Config{Prompt: "real question", PromptHash: pelicanarchive.PromptHash("real question"), ExpectedAnswer: 21, IntervalMinutes: 60, AutoRun: true}, Targets: []pelicanarchive.Target{{ProviderID: "p", Model: "m", Enabled: 1, IntervalMinutes: 0}}, Runs: []pelicanarchive.Run{{ID: 1, ProviderID: "p", Model: "m", Grade: "wrong", Expected: 21, Attempts: 1, CreatedAt: now.Format(time.RFC3339Nano), SVG: `<svg xmlns="http://www.w3.org/2000/svg"><text>29</text></svg>`, PromptHash: pelicanarchive.PromptHash("real question")}}}
	return s, PelicanBindings{SourceID: "fixture", Bindings: []PelicanBinding{{ProviderID: "p", Model: "m", ChannelID: 83}}}
}

func assertBootstrapEmpty(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []any{&model.PelicanControl{}, &model.PelicanTarget{}, &model.PelicanRecord{}, &model.PelicanEvent{}} {
		var count int64
		require.NoError(t, db.Model(table).Count(&count).Error)
		require.Zero(t, count)
	}
	var ids []model.CapabilityGroupPresentation
	require.NoError(t, db.Find(&ids).Error)
	require.Len(t, ids, 1)
	require.Equal(t, "existing-group-uid", ids[0].GroupUID)
}

func verifyBootstrap(t *testing.T, db *gorm.DB) {
	t.Helper()
	s, b := bootstrapFixture(t, db)
	plan, err := BootstrapPelicanArchive(db, s, b, 71, false, "")
	require.NoError(t, err)
	require.False(t, plan.Applied)
	require.Equal(t, 2, plan.Memberships)
	assertBootstrapEmpty(t, db)
	_, err = BootstrapPelicanArchive(db, s, b, 71, true, "different")
	require.ErrorContains(t, err, "plan_changed")
	assertBootstrapEmpty(t, db)
	// A changed business grouping invalidates the reviewed plan, even when
	// another eligible group remains. Neither the source nor the business row
	// is silently changed by initialization.
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 83).Update("group", "B").Error)
	_, err = BootstrapPelicanArchive(db, s, b, 71, true, plan.PlanHash)
	require.ErrorContains(t, err, "plan_changed")
	assertBootstrapEmpty(t, db)
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 83).Update("group", "A,B").Error)
	applied, err := BootstrapPelicanArchive(db, s, b, 71, true, plan.PlanHash)
	require.NoError(t, err)
	require.True(t, applied.Applied)
	control, err := model.GetPelicanControl(db)
	require.NoError(t, err)
	require.False(t, control.Visible)
	require.False(t, control.SyncEnabled)
	require.EqualValues(t, 1, control.Revision)
	var ids []model.CapabilityGroupPresentation
	require.NoError(t, db.Find(&ids).Error)
	require.Len(t, ids, 2)
	var old model.CapabilityGroupPresentation
	require.NoError(t, db.First(&old, "group_uid = ?", "existing-group-uid").Error)
	require.Equal(t, "A", *old.RoutingKey)
	a, err := PelicanResults(db, CapabilityGroupView{RoutingKey: "A"}, "m", true, 3)
	require.NoError(t, err)
	bb, err := PelicanResults(db, CapabilityGroupView{RoutingKey: "B"}, "m", true, 3)
	require.NoError(t, err)
	require.Len(t, a.Gallery, 1)
	require.Equal(t, a.Gallery, bb.Gallery)
	var record model.PelicanRecord
	require.NoError(t, db.First(&record, "id = ?", a.Gallery[0].ID).Error)
	run, err := record.Run()
	require.NoError(t, err)
	require.Equal(t, s.Runs[0], run)
	_, err = BootstrapPelicanArchive(db, s, b, 71, true, plan.PlanHash)
	require.ErrorContains(t, err, "empty_archive_tables")
	// Enabling the independent importer updates the external schedule without
	// changing our own interval, mapping, previous record or source answers.
	control, err = model.UpdatePelicanControl(db, control.Revision, 71, "resume", func(_ *gorm.DB, c *model.PelicanControl) error { c.SyncEnabled = true; return nil })
	require.NoError(t, err)
	s.CapturedAt = time.Now().Add(time.Second).UTC().Format(time.RFC3339Nano)
	s.Config.IntervalMinutes, s.Targets[0].IntervalMinutes = 30, 120
	n, err := model.ImportPelicanSnapshot(db, s, control.Revision)
	require.NoError(t, err)
	require.Zero(t, n)
	control, err = model.GetPelicanControl(db)
	require.NoError(t, err)
	require.Equal(t, 5, control.IntervalMinutes)
	var cfg pelicanarchive.Config
	require.NoError(t, common.UnmarshalJsonStr(control.SourceConfig, &cfg))
	require.Equal(t, 30, cfg.IntervalMinutes)
	var target model.PelicanTarget
	require.NoError(t, db.First(&target).Error)
	require.Equal(t, 120, target.IntervalMinutes)
	require.Equal(t, 83, target.ChannelID)
	// Source finished a new result: both groups reference that same new ID;
	// original result remains in history.
	s.CapturedAt = time.Now().Add(2 * time.Second).UTC().Format(time.RFC3339Nano)
	s.Runs = append(s.Runs, s.Runs[0])
	s.Runs[1].ID = 2
	s.Runs[1].CreatedAt = s.CapturedAt
	n, err = model.ImportPelicanSnapshot(db, s, control.Revision)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	for _, group := range []string{"A", "B"} {
		result, err := PelicanResults(db, CapabilityGroupView{RoutingKey: group}, "m", true, 3)
		require.NoError(t, err)
		require.NotEqual(t, a.Gallery[0].ID, result.Gallery[0].ID)
		require.Len(t, result.History, 2)
	}
	var channel model.Channel
	require.NoError(t, db.First(&channel, 83).Error)
	require.Equal(t, "untouched-secret", channel.Key)
	require.Equal(t, common.ChannelStatusManuallyDisabled, channel.Status)
}

func TestPelicanBootstrap(t *testing.T) {
	verifyBootstrap(t, bootstrapDB(t, sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))))
}

func TestPelicanBootstrapRejectsAndRollsBack(t *testing.T) {
	for _, kind := range []string{"wrong-source", "missing-target", "duplicate-binding", "missing-model", "no-membership", "actor-disabled", "stale", "write-failure"} {
		t.Run(kind, func(t *testing.T) {
			db := bootstrapDB(t, sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())))
			s, b := bootstrapFixture(t, db)
			plan, err := BootstrapPelicanArchive(db, s, b, 71, false, "")
			require.NoError(t, err)
			switch kind {
			case "wrong-source":
				b.SourceID = "other"
			case "missing-target":
				b.Bindings[0].ProviderID = "missing"
			case "duplicate-binding":
				b.Bindings = append(b.Bindings, b.Bindings[0])
			case "missing-model":
				require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 83).Update("models", "other").Error)
			case "no-membership":
				require.NoError(t, db.Where("channel_id = ?", 83).Delete(&model.Ability{}).Error)
			case "actor-disabled":
				require.NoError(t, db.Model(&model.User{}).Where("id = ?", 71).Update("status", common.UserStatusDisabled).Error)
			case "stale":
				s.CapturedAt = time.Now().Add(-25 * time.Hour).Format(time.RFC3339Nano)
			case "write-failure":
				require.NoError(t, db.Exec("CREATE TRIGGER fail_bootstrap BEFORE UPDATE OF channel_id ON pelican_targets BEGIN SELECT RAISE(ABORT, 'simulated mapping failure'); END").Error)
			}
			_, err = BootstrapPelicanArchive(db, s, b, 71, true, plan.PlanHash)
			require.Error(t, err)
			assertBootstrapEmpty(t, db)
		})
	}
}

func TestPelicanBootstrapDatabaseMatrix(t *testing.T) {
	for _, engine := range []string{"mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			dsn := os.Getenv("PELICAN_BOOTSTRAP_" + strings.ToUpper(engine) + "_DSN")
			if dsn == "" {
				t.Skip("local isolated database not configured")
			}
			var dialect gorm.Dialector
			if engine == "mysql" {
				cfg, err := gomysql.ParseDSN(dsn)
				require.NoError(t, err)
				host, _, err := net.SplitHostPort(cfg.Addr)
				require.NoError(t, err)
				require.Equal(t, "tcp", cfg.Net)
				require.True(t, net.ParseIP(host).IsLoopback())
				require.True(t, strings.HasPrefix(cfg.DBName, "pelican_test_"))
				dialect = mysql.Open(dsn)
			} else {
				u, err := url.Parse(dsn)
				require.NoError(t, err)
				require.Equal(t, "postgres", u.Scheme)
				require.True(t, net.ParseIP(u.Hostname()).IsLoopback())
				require.Empty(t, u.Query().Get("host"))
				require.True(t, strings.HasPrefix(strings.TrimPrefix(u.Path, "/"), "pelican_test_"))
				dialect = postgres.Open(dsn)
			}
			verifyBootstrap(t, bootstrapDB(t, dialect))
		})
	}
}
