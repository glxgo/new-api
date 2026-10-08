package main

import (
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAdditiveMigrationKeepsLegacyData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "migration.db")), &gorm.Config{})
	require.NoError(t, err)
	for _, sql := range []string{
		"CREATE TABLE users (id INTEGER PRIMARY KEY, quota INTEGER, gift_quota INTEGER, password TEXT, legacy_profit INTEGER)",
		"CREATE TABLE two_fas (id INTEGER PRIMARY KEY, user_id INTEGER, secret TEXT)",
		"CREATE TABLE passkey_credentials (id INTEGER PRIMARY KEY, user_id INTEGER, credential_id TEXT)",
		"INSERT INTO users VALUES (1, 700, 120, 'legacy-hash', 42)",
		"INSERT INTO two_fas VALUES (1, 1, 'legacy-secret')",
		"INSERT INTO passkey_credentials VALUES (1, 1, 'legacy-credential')",
	} {
		require.NoError(t, db.Exec(sql).Error)
	}
	for i := 0; i < 2; i++ {
		require.NoError(t, migrate(db))
	}
	var row struct {
		Quota, GiftQuota, LegacyProfit, AuthVersion int
		Password                                    string
	}
	require.NoError(t, db.Table("users").Where("id = 1").Scan(&row).Error)
	require.Equal(t, 700, row.Quota)
	require.Equal(t, 120, row.GiftQuota)
	require.Equal(t, 42, row.LegacyProfit)
	require.Equal(t, "legacy-hash", row.Password)
	require.Zero(t, row.AuthVersion)
	for _, table := range []any{&model.SecurityProof{}, &model.SecurityCeremony{}, &model.DashboardSession{}, &model.KiroCreditSettlement{}, &model.KiroCreditAttempt{}} {
		require.True(t, db.Migrator().HasTable(table))
	}
	require.True(t, db.Migrator().HasIndex(&model.PasskeyCredential{}, "RPID"))
	var factor model.TwoFA
	require.NoError(t, db.Table("two_fas").Select("id,user_id,secret,last_verified_step").Scan(&factor).Error)
	require.Equal(t, "legacy-secret", factor.Secret)
	require.Zero(t, factor.LastVerifiedStep)
}
