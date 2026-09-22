package model

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	gomysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Opt-in DSNs are restricted to loopback and a clearly named disposable DB.
// Never reuse SQL_DSN, production databases, or connection strings from memory.
func TestPelicanDatabaseMatrix(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			var dialect gorm.Dialector
			switch engine {
			case "sqlite":
				dialect = sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))
			case "mysql":
				dsn := os.Getenv("PELICAN_TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("local MySQL DSN not provided")
				}
				cfg, err := gomysql.ParseDSN(dsn)
				require.NoError(t, err)
				host, _, err := net.SplitHostPort(cfg.Addr)
				require.NoError(t, err)
				require.Equal(t, "tcp", cfg.Net)
				require.True(t, net.ParseIP(host).IsLoopback(), "test engine must be loopback")
				require.True(t, strings.HasPrefix(cfg.DBName, "pelican_test_"), "use a fresh disposable database")
				dialect = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("PELICAN_TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("local PostgreSQL DSN not provided")
				}
				u, err := url.Parse(dsn)
				require.NoError(t, err)
				require.Equal(t, "postgres", u.Scheme)
				require.True(t, net.ParseIP(u.Hostname()).IsLoopback(), "test engine must be loopback")
				require.Empty(t, u.Query().Get("host"), "DSN cannot override host")
				require.True(t, strings.HasPrefix(strings.TrimPrefix(u.Path, "/"), "pelican_test_"))
				dialect = postgres.Open(dsn)
			}
			db, err := gorm.Open(dialect, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			pool, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = pool.Close() })
			pool.SetMaxOpenConns(8)
			tables, err := db.Migrator().GetTables()
			require.NoError(t, err)
			require.Empty(t, tables, "refuse to run on nonempty databases")
			// A sentinel legacy table proves the new migration leaves old columns
			// and historical values intact, including the retained profit fields.
			type legacy struct {
				ID                int
				FixedProfitAmount float64
				ProfitQuota       int64
			}
			require.NoError(t, db.Table("pelican_legacy_sentinel").AutoMigrate(&legacy{}))
			require.NoError(t, db.Table("pelican_legacy_sentinel").Create(&legacy{ID: 1, FixedProfitAmount: 1.25, ProfitQuota: 987}).Error)
			require.NoError(t, MigratePelicanSchema(db))
			before, err := db.Migrator().GetTables()
			require.NoError(t, err)
			require.Len(t, before, 6, "only five archive/display tables plus sentinel")
			require.NoError(t, MigratePelicanSchema(db))
			after, err := db.Migrator().GetTables()
			require.NoError(t, err)
			require.ElementsMatch(t, before, after)
			verifyPelicanImportAtomicDedupRevisionsAndPause(t, db)
			// Re-running migration after data exists must preserve evidence.
			require.NoError(t, MigratePelicanSchema(db))
			var rows []PelicanRecord
			require.NoError(t, db.Find(&rows).Error)
			require.Len(t, rows, 2)
			large := strings.Repeat("真实证据🙂", 20000)
			require.NoError(t, db.Model(&rows[0]).Update("prompt", large).Error)
			var reread PelicanRecord
			require.NoError(t, db.First(&reread, "id = ?", rows[0].ID).Error)
			require.Equal(t, large, reread.Prompt, "Unicode evidence exceeding 64KiB must round trip")
			current, err := GetPelicanControl(db)
			require.NoError(t, err)
			var wins atomic.Int32
			var wg sync.WaitGroup
			start := make(chan struct{})
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					_, err := UpdatePelicanControl(db, current.Revision, 1, "concurrent-preview", func(_ *gorm.DB, c *PelicanControl) error { c.Visible = true; return nil })
					if err == nil {
						wins.Add(1)
					}
				}()
			}
			close(start)
			wg.Wait()
			require.EqualValues(t, 1, wins.Load(), "only one concurrent configuration wins")
			final, err := GetPelicanControl(db)
			require.NoError(t, err)
			require.Equal(t, current.Revision+1, final.Revision)
			var preserved legacy
			require.NoError(t, db.Table("pelican_legacy_sentinel").First(&preserved, 1).Error)
			require.Equal(t, legacy{ID: 1, FixedProfitAmount: 1.25, ProfitQuota: 987}, preserved)
			var version string
			query := "SELECT version()"
			if engine == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(query).Scan(&version).Error)
			b, _ := common.Marshal(map[string]any{"engine": engine, "version": version, "checks": "schema-repeat/import/dedup/revisions/rollback/pause/mapping/unicode/concurrency/legacy-preserved"})
			t.Log(string(b))
		})
	}
}
